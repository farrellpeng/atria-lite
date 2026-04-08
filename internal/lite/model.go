package lite

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sethdeckard/atria/internal/codex"
	"github.com/sethdeckard/atria/internal/model"
	"github.com/sethdeckard/atria/internal/terminal"
	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

type Mode int

const (
	ModeList Mode = iota
	ModeReplacePrompt
	ModeNormalPanePicker
)

const loadedPaneMissingGraceCycles = 2

type windowPaneClient interface {
	ListWindowPanes(windowID int) ([]wezterm.PaneInfo, error)
	ReadScreen(sessionID string, lines int) (string, error)
	GetVar(sessionID, varName string) (string, error)
	SplitPane(opts wezterm.SplitPaneOptions) (int, error)
	AdjustPaneSize(paneID int, direction string, amount int) error
	MovePaneToNewTab(paneID, windowID int) error
	ActivateTab(tabID int) error
	ActivatePane(sessionID string) error
}

type Model struct {
	client windowPaneClient
	ctx    MonitorContext
	mode   Mode

	panes       []CandidatePane
	bindings    []SlotBinding
	cursor      int
	replacePane CandidatePane

	statusText   string
	width        int
	height       int
	spinnerFrame int

	statusTickActive  bool
	spinnerTickActive bool

	missingPaneGrace map[int]int

	codexClient *codex.Client    // nil if codex binary not found
	codexQuota  *codex.QuotaInfo // global account-level quota cache
}

func NewModel(client windowPaneClient, ctx MonitorContext) Model {
	bindings := normalizeBindings(ctx.SlotBindings)
	ctx.SlotBindings = bindings
	ctx.WorkspacePaneIDs = bindingPaneIDs(bindings)

	return Model{
		client:           client,
		ctx:              ctx,
		mode:             ModeList,
		bindings:         bindings,
		missingPaneGrace: make(map[int]int),
		codexClient:      codex.NewClient(),
	}
}

func (m Model) Init() tea.Cmd {
	return refreshWindowPanes(m.client, m.ctx)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case windowPanesLoadedMsg:
		m.panes = classifyWindowPanes(msg.panes, m.ctx)
		nextBindings, autoloadedPane, recoverWorkspace := m.reconcileBindings()
		m.syncReplacePrompt()
		m.clampCursor()
		m.statusText = m.modeStatusText()
		if autoloadedPane != nil {
			if cmd := syncWorkspaceBindings(m.client, m.ctx, m.ctx.WorkspacePaneIDs, nextBindings, fmt.Sprintf("Loaded %s", paneLabel(*autoloadedPane))); cmd != nil {
				return m, cmd
			}
			m.applyBindings(nextBindings)
			m.statusText = fmt.Sprintf("Loaded %s", paneLabel(*autoloadedPane))
		} else if recoverWorkspace != nil {
			if cmd := syncWorkspaceBindings(m.client, m.ctx, recoverWorkspace, nextBindings, "Restoring workspace"); cmd != nil {
				return m, cmd
			}
		}
	case candidatePanesLoadedMsg:
		m.panes = m.mergePaneState(msg.panes)
		nextBindings, autoloadedPane, recoverWorkspace := m.reconcileBindings()
		m.syncReplacePrompt()
		m.clampCursor()
		m.statusText = m.modeStatusText()
		if autoloadedPane != nil {
			if cmd := syncWorkspaceBindings(m.client, m.ctx, m.ctx.WorkspacePaneIDs, nextBindings, fmt.Sprintf("Loaded %s", paneLabel(*autoloadedPane))); cmd != nil {
				return m, tea.Batch(cmd, refreshTickCmd(), m.ensureStatusTick(), m.ensureSpinnerTick(), m.ensureQuotaTick())
			}
			m.applyBindings(nextBindings)
			m.statusText = fmt.Sprintf("Loaded %s", paneLabel(*autoloadedPane))
		} else if recoverWorkspace != nil {
			if cmd := syncWorkspaceBindings(m.client, m.ctx, recoverWorkspace, nextBindings, "Restoring workspace"); cmd != nil {
				return m, tea.Batch(cmd, refreshTickCmd(), m.ensureStatusTick(), m.ensureSpinnerTick(), m.ensureQuotaTick())
			}
		}
		return m, tea.Batch(refreshTickCmd(), m.ensureStatusTick(), m.ensureSpinnerTick(), m.ensureQuotaTick())
	case windowPanesLoadFailedMsg:
		m.statusText = fmt.Sprintf("Refresh failed: %v", msg.err)
		return m, refreshTickCmd()
	case refreshTickMsg:
		return m, refreshWindowPanes(m.client, m.ctx)
	case statusTickMsg:
		m.statusTickActive = false
		if !m.hasAgentPanes() {
			return m, nil
		}
		return m, refreshPaneStatuses(m.client, m.panes)
	case paneStatusesLoadedMsg:
		m.panes = msg.panes
		m.syncBindingKindsFromPanes()
		m.syncReplacePrompt()
		m.clampCursor()
		return m, tea.Batch(m.ensureStatusTick(), m.ensureSpinnerTick(), m.ensureQuotaTick())
	case paneStatusesLoadFailedMsg:
		return m, m.ensureStatusTick()
	case spinnerTickMsg:
		m.spinnerTickActive = false
		if !m.hasWorkingPanes() {
			return m, nil
		}
		m.spinnerFrame++
		return m, m.ensureSpinnerTick()
	case slotActionCompletedMsg:
		m.mode = ModeList
		m.replacePane = CandidatePane{}
		m.applyBindings(msg.bindings)
		m.clampCursor()
		m.statusText = msg.statusText
		return m, tea.Batch(tea.ClearScreen, tea.WindowSize(), m.ensureStatusTick(), m.ensureSpinnerTick())
	case slotActionFailedMsg:
		m.statusText = fmt.Sprintf("Action failed: %v", msg.err)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case codexQuotaTickMsg:
		if m.codexClient != nil && m.codexClient.Available() && m.hasCodexPanes() {
			return m, fetchCodexQuota(m.codexClient)
		}
		return m, nil
	case codexQuotaMsg:
		if msg.quota != nil {
			m.codexQuota = msg.quota
		}
		if m.hasCodexPanes() {
			return m, quotaTickCmd()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case ModeList:
		return m.handleListKey(msg)
	case ModeReplacePrompt, ModeNormalPanePicker:
		if m.mode == ModeReplacePrompt {
			return m.handleReplacePromptKey(msg)
		}
		return m.handleNormalPanePickerKey(msg)
	}

	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if last := len(m.agentPanes()) - 1; m.cursor < last {
			m.cursor++
		}
	case "enter":
		agents := m.agentPanes()
		if len(agents) == 0 {
			return m, nil
		}
		selected := agents[m.cursor]
		nextBindings, prompt := PlanAgentLoad(m.bindings, selected)
		if prompt {
			m.mode = ModeReplacePrompt
			m.replacePane = selected
			m.statusText = m.modeStatusText()
			return m, nil
		}
		if cmd := syncWorkspaceBindings(m.client, m.ctx, m.ctx.WorkspacePaneIDs, nextBindings, fmt.Sprintf("Loaded %s", paneLabel(selected))); cmd != nil {
			return m, cmd
		}
		m.applyBindings(nextBindings)
		m.statusText = fmt.Sprintf("Loaded %s", paneLabel(selected))
	case "n":
		m.mode = ModeNormalPanePicker
		m.cursor = 0
		m.statusText = m.modeStatusText()
	case "r":
		return m, m.refreshAllCmd()
	}

	return m, nil
}

func (m Model) handleNormalPanePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if last := len(m.normalPanes()) - 1; m.cursor < last {
			m.cursor++
		}
	case "enter":
		normals := m.normalPanes()
		if len(normals) == 0 {
			return m, nil
		}
		selected := normals[m.cursor]
		nextBindings, prompt := PlanNormalLoad(m.bindings, selected)
		if prompt {
			m.mode = ModeReplacePrompt
			m.replacePane = selected
			m.statusText = m.modeStatusText()
			return m, nil
		}
		m.mode = ModeList
		if cmd := syncWorkspaceBindings(m.client, m.ctx, m.ctx.WorkspacePaneIDs, nextBindings, fmt.Sprintf("Loaded %s", paneLabel(selected))); cmd != nil {
			return m, cmd
		}
		m.applyBindings(nextBindings)
		m.clampCursor()
		m.statusText = fmt.Sprintf("Loaded %s", paneLabel(selected))
	case "esc", "q":
		m.mode = ModeList
		m.replacePane = CandidatePane{}
		m.clampCursor()
		m.statusText = m.modeStatusText()
	case "r":
		return m, m.refreshAllCmd()
	}
	return m, nil
}

func (m Model) handleReplacePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = ModeList
		m.replacePane = CandidatePane{}
		m.statusText = m.modeStatusText()
		return m, nil
	case "r":
		return m, m.refreshAllCmd()
	case "1", "2", "3":
		target := SlotID("slot" + msg.String())
		if !isAllowedReplaceSlot(allowedReplaceSlots(m.bindings, m.replacePane), target) {
			m.statusText = fmt.Sprintf("%s cannot replace %s", paneLabel(m.replacePane), target)
			return m, nil
		}
		return m, replaceSlot(m.client, m.ctx, m.bindings, m.replacePane, target)
	}
	return m, nil
}

func (m Model) refreshAllCmd() tea.Cmd {
	return tea.Batch(
		refreshWindowPanes(m.client, m.ctx),
		fetchCodexQuota(m.codexClient),
	)
}

func classifyWindowPanes(panes []wezterm.PaneInfo, ctx MonitorContext) []CandidatePane {
	candidates := make([]CandidatePane, 0, len(panes))
	for _, pane := range panes {
		if pane.WindowID != ctx.WindowID || pane.PaneID == ctx.SelfPaneID {
			continue
		}

		agentType := terminal.DetectAgent(pane.Title)
		kind := OccupantNormal
		if agentType != model.AgentType("") {
			kind = OccupantAgent
		}

		candidates = append(candidates, CandidatePane{
			PaneID:    pane.PaneID,
			WindowID:  pane.WindowID,
			TabID:     pane.TabID,
			Title:     pane.Title,
			Kind:      kind,
			AgentType: agentType,
		})
	}
	return candidates
}

func (m *Model) reconcileBindings() ([]SlotBinding, *CandidatePane, []int) {
	liveKinds := make(map[int]OccupantKind, len(m.panes))
	livePaneByID := make(map[int]CandidatePane, len(m.panes))
	livePaneIDs := make(map[int]bool, len(m.panes))
	for _, pane := range m.panes {
		liveKinds[pane.PaneID] = pane.Kind
		livePaneByID[pane.PaneID] = pane
		livePaneIDs[pane.PaneID] = true
	}

	current := normalizeBindings(m.bindings)
	if len(current) == 0 && len(m.ctx.WorkspacePaneIDs) == 0 {
		current = m.bootstrapBindingsFromStarter(livePaneByID)
	}
	for i := range current {
		if kind, ok := liveKinds[current[i].PaneID]; ok {
			current[i].Kind = kind
		}
	}

	current = m.retainTransientMissingBindings(current, livePaneIDs)
	m.applyBindings(current)
	m.ctx.WorkspacePaneIDs = m.visibleWorkspacePaneIDs(current, livePaneByID)
	liteDebugf("reconcile current=%v visible=%v panes=%v", current, m.ctx.WorkspacePaneIDs, summarizeCandidates(m.panes))

	if candidate, ok := livePaneByID[m.replacePane.PaneID]; ok {
		if candidate.Kind == m.replacePane.Kind {
			m.replacePane = candidate
		}
	}
	nextBindings, autoloadedPane := m.autoLoadDiscoveredAgents(current)
	recoverWorkspace := m.recoverWorkspacePaneIDs(current)
	liteDebugf("reconcile result next=%v autoload=%v recover=%v", nextBindings, summarizeCandidatePtr(autoloadedPane), recoverWorkspace)
	return nextBindings, autoloadedPane, recoverWorkspace
}

func (m Model) mergePaneState(next []CandidatePane) []CandidatePane {
	if len(m.panes) == 0 || len(next) == 0 {
		return next
	}

	prevByID := make(map[int]CandidatePane, len(m.panes))
	for _, pane := range m.panes {
		prevByID[pane.PaneID] = pane
	}

	merged := make([]CandidatePane, len(next))
	copy(merged, next)
	for i := range merged {
		prev, ok := prevByID[merged[i].PaneID]
		if !ok {
			continue
		}
		if merged[i].Kind != OccupantAgent || prev.Kind != OccupantAgent || merged[i].AgentType != prev.AgentType {
			continue
		}
		if merged[i].Status == "" {
			merged[i].Status = prev.Status
		}
		if merged[i].Activity == "" {
			merged[i].Activity = prev.Activity
		}
		if !merged[i].ScreenChecked {
			merged[i].ScreenChecked = prev.ScreenChecked
		}
		if merged[i].LastScreen == "" {
			merged[i].LastScreen = prev.LastScreen
		}
		merged[i].UnmatchedReads = prev.UnmatchedReads
		merged[i].OrphanTicks = prev.OrphanTicks
	}
	return merged
}

func (m *Model) bootstrapBindingsFromStarter(livePaneByID map[int]CandidatePane) []SlotBinding {
	starterPane, ok := livePaneByID[m.ctx.StarterPaneID]
	if !ok {
		return nil
	}
	return []SlotBinding{
		{
			Slot:   Slot1,
			PaneID: starterPane.PaneID,
			Kind:   starterPane.Kind,
		},
	}
}

func (m *Model) autoLoadDiscoveredAgents(bindings []SlotBinding) ([]SlotBinding, *CandidatePane) {
	current := normalizeBindings(bindings)
	var autoloadedPane *CandidatePane
	for _, pane := range m.panes {
		if pane.Kind != OccupantAgent {
			continue
		}
		if m.ctx.TabID != 0 && pane.TabID != 0 && pane.TabID != m.ctx.TabID {
			continue
		}
		next, prompt := PlanAgentLoad(current, pane)
		if prompt {
			break
		}
		if !sameBindings(next, current) && autoloadedPane == nil {
			paneCopy := pane
			autoloadedPane = &paneCopy
		}
		current = next
	}
	return current, autoloadedPane
}

func (m *Model) applyBindings(bindings []SlotBinding) {
	next := normalizeBindings(bindings)
	previous := make(map[int]bool, len(m.bindings))
	for _, binding := range m.bindings {
		previous[binding.PaneID] = true
	}

	active := make(map[int]bool, len(next))
	for _, binding := range next {
		active[binding.PaneID] = true
		if !previous[binding.PaneID] {
			m.missingPaneGrace[binding.PaneID] = loadedPaneMissingGraceCycles
		}
	}
	for paneID := range m.missingPaneGrace {
		if !active[paneID] {
			delete(m.missingPaneGrace, paneID)
		}
	}

	m.bindings = next
	m.ctx.SlotBindings = m.bindings
	m.ctx.WorkspacePaneIDs = bindingPaneIDs(m.bindings)
}

func (m *Model) syncBindingKindsFromPanes() {
	if len(m.bindings) == 0 || len(m.panes) == 0 {
		return
	}
	liveKinds := make(map[int]OccupantKind, len(m.panes))
	for _, pane := range m.panes {
		liveKinds[pane.PaneID] = pane.Kind
	}

	next := normalizeBindings(m.bindings)
	for i := range next {
		if kind, ok := liveKinds[next[i].PaneID]; ok {
			next[i].Kind = kind
		}
	}
	m.applyBindings(next)
}

func (m *Model) retainTransientMissingBindings(bindings []SlotBinding, livePaneIDs map[int]bool) []SlotBinding {
	current := normalizeBindings(bindings)
	kept := make([]SlotBinding, 0, len(current))

	for _, binding := range current {
		if livePaneIDs[binding.PaneID] {
			delete(m.missingPaneGrace, binding.PaneID)
			kept = append(kept, binding)
			continue
		}

		if remaining := m.missingPaneGrace[binding.PaneID]; remaining > 0 {
			m.missingPaneGrace[binding.PaneID] = remaining - 1
			kept = append(kept, binding)
		} else {
			delete(m.missingPaneGrace, binding.PaneID)
		}
	}

	return reindex(kept)
}

func (m *Model) visibleWorkspacePaneIDs(bindings []SlotBinding, livePaneByID map[int]CandidatePane) []int {
	current := normalizeBindings(bindings)
	visible := make([]int, 0, len(current))
	for _, binding := range current {
		pane, ok := livePaneByID[binding.PaneID]
		if !ok {
			continue
		}
		if m.ctx.TabID != 0 && pane.TabID != m.ctx.TabID {
			continue
		}
		visible = append(visible, binding.PaneID)
	}

	if len(visible) == 0 {
		if starter, ok := livePaneByID[m.ctx.StarterPaneID]; ok && starter.PaneID != m.ctx.SelfPaneID {
			if m.ctx.TabID == 0 || starter.TabID == m.ctx.TabID {
				visible = append(visible, starter.PaneID)
			}
		}
	}

	return visible
}

func (m *Model) recoverWorkspacePaneIDs(bindings []SlotBinding) []int {
	current := normalizeBindings(bindings)
	if len(current) == 0 {
		return nil
	}
	if len(m.ctx.WorkspacePaneIDs) >= len(current) {
		return nil
	}
	if len(m.ctx.WorkspacePaneIDs) == 0 {
		return []int{}
	}
	return append([]int(nil), m.ctx.WorkspacePaneIDs...)
}

func sameBindings(a, b []SlotBinding) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (m *Model) clampCursor() {
	var panes []CandidatePane
	if m.mode == ModeNormalPanePicker {
		panes = m.normalPanes()
	} else {
		panes = m.agentPanes()
	}
	if len(panes) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(panes) {
		m.cursor = len(panes) - 1
	}
}

func (m Model) agentPanes() []CandidatePane {
	return filterPanesByKind(m.panes, OccupantAgent)
}

func (m Model) normalPanes() []CandidatePane {
	return filterPanesByKind(m.panes, OccupantNormal)
}

func (m Model) hasAgentPanes() bool {
	return len(m.agentPanes()) > 0
}

func (m Model) hasWorkingPanes() bool {
	for _, pane := range m.agentPanes() {
		if pane.Status == model.StatusWorking {
			return true
		}
	}
	return false
}

func (m *Model) ensureStatusTick() tea.Cmd {
	if m.statusTickActive || !m.hasAgentPanes() {
		return nil
	}
	m.statusTickActive = true
	return statusTickCmd()
}

func (m *Model) ensureSpinnerTick() tea.Cmd {
	if m.spinnerTickActive || !m.hasWorkingPanes() {
		return nil
	}
	m.spinnerTickActive = true
	return spinnerTickCmd()
}

func filterPanesByKind(panes []CandidatePane, kind OccupantKind) []CandidatePane {
	out := make([]CandidatePane, 0, len(panes))
	for _, pane := range panes {
		if pane.Kind == kind {
			out = append(out, pane)
		}
	}
	return out
}

func paneLabel(pane CandidatePane) string {
	if pane.Kind == OccupantAgent && pane.AgentType != "" {
		activity := strings.TrimSpace(pane.Activity)
		if activity != "" && !isGenericPaneTitle(activity, pane.CWD) {
			return activity
		}
	}
	label := strings.TrimSpace(pane.Title)
	if pane.Kind == OccupantAgent && pane.AgentType != "" {
		preferred := preferredAgentPaneLabel(pane.AgentType)
		if label == "" || isGenericPaneTitle(label, pane.CWD) {
			return preferred
		}
	}
	if label == "" {
		if pane.Kind == OccupantAgent && pane.AgentType != "" {
			return preferredAgentPaneLabel(pane.AgentType)
		}
		label = fmt.Sprintf("pane %d", pane.PaneID)
	}
	return label
}

func preferredAgentPaneLabel(agentType model.AgentType) string {
	switch agentType {
	case model.AgentClaude:
		return "Claude Code"
	case model.AgentCodex:
		return "Codex"
	case model.AgentOpenCode:
		return "OpenCode"
	case model.AgentCopilot:
		return "Copilot"
	default:
		return string(agentType)
	}
}

func isGenericPaneTitle(title, cwd string) bool {
	trimmed := normalizePaneTitle(title)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "shell", "bash", "zsh", "fish", "sh", "cmd.exe", "powershell", "pwsh":
		return true
	}
	if cwd == "" {
		return false
	}
	base := filepath.Base(strings.TrimSuffix(strings.TrimSpace(cwd), "/"))
	return base != "." && base != "/" && strings.EqualFold(trimmed, base)
}

func normalizePaneTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	return strings.TrimSpace(strings.TrimLeftFunc(trimmed, func(r rune) bool {
		if unicode.IsSpace(r) {
			return true
		}
		switch r {
		case ':', '·', '•', '>', '_', '!', '*', '✳', '✻', '✶', '✽', '✢',
			'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏':
			return true
		default:
			return false
		}
	}))
}

func (m *Model) syncReplacePrompt() {
	if m.mode != ModeReplacePrompt {
		return
	}
	if m.replacePane.PaneID == 0 {
		m.mode = ModeList
		return
	}
	for _, pane := range m.panes {
		if pane.PaneID == m.replacePane.PaneID {
			if pane.Kind != m.replacePane.Kind {
				m.mode = ModeList
				m.replacePane = CandidatePane{}
				return
			}
			m.replacePane = pane
			return
		}
	}
	m.mode = ModeList
	m.replacePane = CandidatePane{}
}

func (m Model) modeStatusText() string {
	switch m.mode {
	case ModeReplacePrompt:
		if m.replacePane.PaneID != 0 {
			return fmt.Sprintf("Replace required for %s", paneLabel(m.replacePane))
		}
	case ModeNormalPanePicker:
		return fmt.Sprintf("%d normal pane(s) available", len(m.normalPanes()))
	}
	return fmt.Sprintf("%d agent pane(s) visible", len(m.agentPanes()))
}

func isAllowedReplaceSlot(slots []SlotID, target SlotID) bool {
	for _, slot := range slots {
		if slot == target {
			return true
		}
	}
	return false
}

func (m Model) hasCodexPanes() bool {
	for _, pane := range m.panes {
		if pane.Kind == OccupantAgent && pane.AgentType == model.AgentCodex {
			return true
		}
	}
	return false
}

func (m *Model) ensureQuotaTick() tea.Cmd {
	if m.codexClient == nil || !m.codexClient.Available() {
		return nil
	}
	if !m.hasCodexPanes() || m.codexQuota != nil {
		return nil
	}
	return fetchCodexQuota(m.codexClient)
}

func (m Model) hasCodexQuota() bool {
	return m.codexQuota != nil
}
