package lite

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

type windowPaneClient interface {
	ListWindowPanes(windowID int) ([]wezterm.PaneInfo, error)
}

type Model struct {
	client windowPaneClient
	ctx    MonitorContext
	mode   Mode

	panes       []CandidatePane
	bindings    []SlotBinding
	cursor      int
	replacePane CandidatePane

	replaceTarget SlotID
	statusText    string
	width         int
	height        int
}

func NewModel(client windowPaneClient, ctx MonitorContext) Model {
	bindings := normalizeBindings(ctx.SlotBindings)
	ctx.SlotBindings = bindings
	ctx.WorkspacePaneIDs = bindingPaneIDs(bindings)

	return Model{
		client:   client,
		ctx:      ctx,
		mode:     ModeList,
		bindings: bindings,
	}
}

func (m Model) Init() tea.Cmd {
	return refreshWindowPanes(m.client, m.ctx.WindowID)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case windowPanesLoadedMsg:
		m.panes = classifyWindowPanes(msg.panes, m.ctx)
		m.shrinkBindings()
		m.clampCursor()
		if m.statusText == "" {
			m.statusText = fmt.Sprintf("%d agent pane(s) visible", len(m.agentPanes()))
		}
	case windowPanesLoadFailedMsg:
		m.statusText = fmt.Sprintf("Refresh failed: %v", msg.err)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case ModeList:
		return m.handleListKey(msg)
	case ModeReplacePrompt, ModeNormalPanePicker:
		switch msg.String() {
		case "esc", "q":
			m.mode = ModeList
			m.statusText = ""
		case "r":
			return m, refreshWindowPanes(m.client, m.ctx.WindowID)
		}
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
			m.replaceTarget = firstReplaceTarget(m.bindings)
			m.statusText = fmt.Sprintf("Replace a slot to load %s", paneLabel(selected))
			return m, nil
		}
		m.bindings = nextBindings
		m.ctx.SlotBindings = nextBindings
		m.ctx.WorkspacePaneIDs = bindingPaneIDs(nextBindings)
		m.statusText = fmt.Sprintf("Loaded %s", paneLabel(selected))
	case "n":
		m.mode = ModeNormalPanePicker
		m.statusText = fmt.Sprintf("%d normal pane(s) available", len(m.normalPanes()))
	case "r":
		return m, refreshWindowPanes(m.client, m.ctx.WindowID)
	}

	return m, nil
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

func (m *Model) shrinkBindings() {
	live := make(map[int]bool, len(m.panes))
	for _, pane := range m.panes {
		live[pane.PaneID] = true
	}
	m.bindings = ShrinkBindings(m.bindings, live)
	m.ctx.SlotBindings = m.bindings
	m.ctx.WorkspacePaneIDs = bindingPaneIDs(m.bindings)
}

func (m *Model) clampCursor() {
	agents := m.agentPanes()
	if len(agents) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(agents) {
		m.cursor = len(agents) - 1
	}
}

func (m Model) agentPanes() []CandidatePane {
	return filterPanesByKind(m.panes, OccupantAgent)
}

func (m Model) normalPanes() []CandidatePane {
	return filterPanesByKind(m.panes, OccupantNormal)
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

func firstReplaceTarget(bindings []SlotBinding) SlotID {
	if len(bindings) == 0 {
		return Slot1
	}
	return normalizeBindings(bindings)[0].Slot
}

func paneLabel(pane CandidatePane) string {
	label := strings.TrimSpace(pane.Title)
	if label == "" {
		label = fmt.Sprintf("pane %d", pane.PaneID)
	}
	return label
}
