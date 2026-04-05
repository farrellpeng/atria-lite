package lite

import (
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sethdeckard/atria/internal/model"
	"github.com/sethdeckard/atria/internal/terminal"
	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

const liteScreenReadLines = 40

const (
	liteDiscoveryInterval = 3 * time.Second
	liteStatusInterval    = 1 * time.Second
	liteSpinnerInterval   = 100 * time.Millisecond
)

const (
	workspaceSettleAttempts   = 5
	workspaceSettleRetryDelay = 50 * time.Millisecond
)

var sleepForWorkspaceSettle = time.Sleep

func refreshTickCmd() tea.Cmd {
	return tea.Tick(liteDiscoveryInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func statusTickCmd() tea.Cmd {
	return tea.Tick(liteStatusInterval, func(time.Time) tea.Msg {
		return statusTickMsg{}
	})
}

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(liteSpinnerInterval, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func refreshWindowPanes(client windowPaneClient, ctx MonitorContext) tea.Cmd {
	if client == nil || ctx.WindowID == 0 {
		return nil
	}

	return func() tea.Msg {
		panes, err := client.ListWindowPanes(ctx.WindowID)
		if err != nil {
			return windowPanesLoadFailedMsg{err: err}
		}

		candidates := make([]CandidatePane, 0, len(panes))
		for _, pane := range panes {
			if pane.WindowID != ctx.WindowID || pane.PaneID == ctx.SelfPaneID {
				continue
			}
			candidates = append(candidates, classifyCandidatePane(client, pane))
		}
		liteDebugf("refresh window=%d tab=%d self=%d panes=%v candidates=%v", ctx.WindowID, ctx.TabID, ctx.SelfPaneID, summarizePaneInfos(panes), summarizeCandidates(candidates))
		return candidatePanesLoadedMsg{panes: candidates}
	}
}

func refreshPaneStatuses(client windowPaneClient, panes []CandidatePane) tea.Cmd {
	if client == nil {
		return nil
	}

	agentPanes := make([]CandidatePane, len(panes))
	copy(agentPanes, panes)
	return func() tea.Msg {
		for i, pane := range agentPanes {
			agentPanes[i] = refreshCandidatePane(client, pane)
		}
		return paneStatusesLoadedMsg{panes: agentPanes}
	}
}

func syncWorkspaceBindings(client windowPaneClient, ctx MonitorContext, workspacePaneIDs []int, bindings []SlotBinding, statusText string) tea.Cmd {
	if client == nil {
		return nil
	}
	if !needsWorkspaceMaterialization(workspacePaneIDs, bindings) {
		return nil
	}

	return func() tea.Msg {
		desired := bindingPaneIDs(bindings)
		liteDebugf("sync workspace current=%v desired=%v bindings=%v", workspacePaneIDs, desired, bindings)
		restoreMonitorFocus(client, ctx)
		if shouldRecreateWorkspaceFromMonitor(workspacePaneIDs, desired) {
			liteDebugf("sync action=recreate-from-monitor desired=%v", desired)
			if err := recreateWorkspaceFromMonitor(client, ctx, desired); err != nil {
				return slotActionFailedMsg{err: fmt.Errorf("recreate workspace: %w", err)}
			}
		} else if err := rebalanceWorkspaceWithCurrent(client, ctx.WindowID, workspacePaneIDs, desired); err != nil {
			return slotActionFailedMsg{err: fmt.Errorf("rebalance workspace: %w", err)}
		}
		restoreMonitorFocus(client, ctx)

		return slotActionCompletedMsg{
			bindings:   bindings,
			statusText: statusText,
		}
	}
}

func shouldRecreateWorkspaceFromMonitor(currentPaneIDs, desiredPaneIDs []int) bool {
	if len(desiredPaneIDs) == 0 {
		return false
	}
	if len(currentPaneIDs) == 0 {
		return true
	}
	if len(currentPaneIDs) == 1 && len(desiredPaneIDs) > 1 {
		return true
	}
	return false
}

func recreateWorkspaceFromMonitor(client windowPaneClient, ctx MonitorContext, desiredPaneIDs []int) error {
	if len(desiredPaneIDs) == 0 {
		return nil
	}
	if ctx.SelfPaneID == 0 {
		return fmt.Errorf("monitor pane id is required to recreate workspace")
	}

	if _, err := client.SplitPane(wezterm.SplitPaneOptions{
		PaneID:     ctx.SelfPaneID,
		Direction:  "bottom",
		TopLevel:   true,
		Percent:    100 - defaultMonitorPercent,
		MovePaneID: desiredPaneIDs[0],
	}); err != nil {
		return fmt.Errorf("split monitor pane for workspace: %w", err)
	}
	liteDebugf("recreate monitor=%d moved=%d desired=%v", ctx.SelfPaneID, desiredPaneIDs[0], desiredPaneIDs)
	waitForWorkspaceAnchor(client, ctx, desiredPaneIDs[0])
	if len(desiredPaneIDs) == 1 {
		return nil
	}
	return rebalanceWorkspaceWithCurrent(client, ctx.WindowID, []int{desiredPaneIDs[0]}, desiredPaneIDs)
}

func waitForWorkspaceAnchor(client windowPaneClient, ctx MonitorContext, anchorPaneID int) {
	if client == nil || ctx.WindowID == 0 || ctx.SelfPaneID == 0 || anchorPaneID == 0 {
		return
	}

	for attempt := 0; attempt < workspaceSettleAttempts; attempt++ {
		panes, err := client.ListWindowPanes(ctx.WindowID)
		if err == nil {
			var monitorPane *wezterm.PaneInfo
			var anchorPane *wezterm.PaneInfo
			for _, pane := range panes {
				pane := pane
				switch pane.PaneID {
				case ctx.SelfPaneID:
					monitorPane = &pane
				case anchorPaneID:
					anchorPane = &pane
				}
			}
			if monitorPane != nil && anchorPane != nil &&
				anchorPane.TabID == monitorPane.TabID &&
				anchorPane.TopRow > monitorPane.TopRow {
				liteDebugf("workspace anchor settled monitor=%d anchor=%d monitorTop=%d anchorTop=%d", ctx.SelfPaneID, anchorPaneID, monitorPane.TopRow, anchorPane.TopRow)
				return
			}
		}

		if attempt < workspaceSettleAttempts-1 {
			sleepForWorkspaceSettle(workspaceSettleRetryDelay)
		}
	}
	liteDebugf("workspace anchor not settled monitor=%d anchor=%d", ctx.SelfPaneID, anchorPaneID)
}

func replaceSlot(client windowPaneClient, ctx MonitorContext, bindings []SlotBinding, pane CandidatePane, slot SlotID) tea.Cmd {
	if client == nil {
		return nil
	}

	return func() tea.Msg {
		nextBindings, replacedPaneID, err := replaceBinding(bindings, slot, pane)
		if err != nil {
			return slotActionFailedMsg{err: err}
		}
		if replacedPaneID != 0 && replacedPaneID != pane.PaneID {
			if err := client.MovePaneToNewTab(replacedPaneID, ctx.WindowID); err != nil {
				return slotActionFailedMsg{err: fmt.Errorf("move replaced pane %d to new tab: %w", replacedPaneID, err)}
			}
		}
		workspace := append([]int(nil), ctx.WorkspacePaneIDs...)
		for i, id := range workspace {
			if id == replacedPaneID {
				workspace = append(workspace[:i], workspace[i+1:]...)
				break
			}
		}
		restoreMonitorFocus(client, ctx)
		if err := rebalanceWorkspaceWithCurrent(client, ctx.WindowID, workspace, bindingPaneIDs(nextBindings)); err != nil {
			return slotActionFailedMsg{err: fmt.Errorf("rebalance workspace: %w", err)}
		}
		restoreMonitorFocus(client, ctx)
		return slotActionCompletedMsg{
			bindings:   nextBindings,
			statusText: fmt.Sprintf("Loaded %s into %s", paneLabel(pane), slot),
		}
	}
}

func restoreMonitorFocus(client windowPaneClient, ctx MonitorContext) {
	if client == nil {
		return
	}
	if ctx.TabID != 0 {
		liteDebugf("focus activate-tab %d", ctx.TabID)
		_ = client.ActivateTab(ctx.TabID)
	}
	if ctx.SelfPaneID != 0 {
		liteDebugf("focus activate-pane %d", ctx.SelfPaneID)
		_ = client.ActivatePane(strconv.Itoa(ctx.SelfPaneID))
	}
}

func classifyCandidatePane(client windowPaneClient, pane wezterm.PaneInfo) CandidatePane {
	candidate := CandidatePane{
		PaneID:   pane.PaneID,
		WindowID: pane.WindowID,
		TabID:    pane.TabID,
		Title:    pane.Title,
		CWD:      pane.CWD,
		Kind:     OccupantNormal,
	}

	var content string
	if client != nil {
		screen, err := client.ReadScreen(strconv.Itoa(pane.PaneID), liteScreenReadLines)
		if err == nil {
			content = screen
		}
	}

	agentType := terminal.DetectAgent(pane.Title)
	if agentType == "" && content != "" {
		agentType = terminal.InferAgentFromScreen(content)
	}
	if agentType != "" {
		candidate.Kind = OccupantAgent
		candidate.AgentType = agentType
		candidate.CWD = discoverPaneCWD(client, pane)
		candidate.Activity = terminal.ExtractActivity(pane.Title)
		if isGenericPaneTitle(candidate.Activity, candidate.CWD) {
			candidate.Activity = ""
		}
		if content != "" {
			status, matchLine := terminal.ClassifyScreen(content, agentType)
			candidate.Status = status
			if status == model.StatusNeedsInput || status == model.StatusError {
				candidate.Attention = matchLine
			}
		}
	}

	return candidate
}

func refreshCandidatePane(client windowPaneClient, pane CandidatePane) CandidatePane {
	if client == nil || pane.Kind != OccupantAgent || pane.AgentType == "" {
		return pane
	}

	content, err := client.ReadScreen(strconv.Itoa(pane.PaneID), liteScreenReadLines)
	if err != nil || content == "" {
		return pane
	}

	status, matchLine := terminal.ClassifyScreen(content, pane.AgentType)
	if status == "" {
		return pane
	}
	pane.Status = status
	pane.Attention = ""
	if status == model.StatusNeedsInput || status == model.StatusError {
		pane.Attention = matchLine
	}
	return pane
}

func discoverPaneCWD(client windowPaneClient, pane wezterm.PaneInfo) string {
	if client == nil {
		return pane.CWD
	}

	sessionID := strconv.Itoa(pane.PaneID)
	watchDirs := make([]string, 0, 1)
	if pane.CWD != "" {
		watchDirs = append(watchDirs, pane.CWD)
	} else if cwd, err := client.GetVar(sessionID, "path"); err == nil && cwd != "" {
		watchDirs = append(watchDirs, cwd)
	}
	if len(watchDirs) == 0 && pane.TTYName != "" {
		watchDirs = append(watchDirs, "/")
	}

	session := terminal.Session{
		ID:   sessionID,
		Name: pane.Title,
		TTY:  pane.TTYName,
	}
	if cwd := terminal.DiscoverCWD(liteBackend{client: client}, session, watchDirs, watchDirs); cwd != "" {
		return cwd
	}
	if pane.CWD != "" {
		return pane.CWD
	}
	cwd, _ := client.GetVar(sessionID, "path")
	return cwd
}

type liteBackend struct {
	client windowPaneClient
}

func (b liteBackend) Available() error { return nil }

func (b liteBackend) ListSessions() ([]terminal.Session, error) { return nil, fmt.Errorf("unsupported") }

func (b liteBackend) NewSession() (string, error) { return "", fmt.Errorf("unsupported") }

func (b liteBackend) SendText(sessionID, text string) error { return fmt.Errorf("unsupported") }

func (b liteBackend) RunCommand(sessionID, cmd string) error { return fmt.Errorf("unsupported") }

func (b liteBackend) FocusSession(sessionID string) error { return fmt.Errorf("unsupported") }

func (b liteBackend) ReadScreen(sessionID string, lines int) (string, error) {
	return b.client.ReadScreen(sessionID, lines)
}

func (b liteBackend) GetVar(sessionID, varName string) (string, error) {
	return b.client.GetVar(sessionID, varName)
}

func (b liteBackend) MonitorOutput(sessionID, logPath, patterns string) (int, error) {
	return 0, fmt.Errorf("unsupported")
}

func replaceBinding(bindings []SlotBinding, target SlotID, pane CandidatePane) ([]SlotBinding, int, error) {
	current := normalizeBindings(bindings)
	targetIndex := -1
	for i, binding := range current {
		if binding.Slot == target {
			targetIndex = i
			break
		}
	}
	if targetIndex == -1 {
		return nil, 0, fmt.Errorf("slot %s is not active", target)
	}

	if existingIndex, found := findPaneIndex(current, pane.PaneID); found {
		current = append(append([]SlotBinding(nil), current[:existingIndex]...), current[existingIndex+1:]...)
		if existingIndex < targetIndex {
			targetIndex--
		}
	}

	replacedPaneID := current[targetIndex].PaneID
	current[targetIndex] = SlotBinding{
		Slot:   target,
		PaneID: pane.PaneID,
		Kind:   pane.Kind,
	}
	return reindex(current), replacedPaneID, nil
}

func allowedReplaceSlots(bindings []SlotBinding, pane CandidatePane) []SlotID {
	current := normalizeBindings(bindings)
	if len(current) == 0 {
		return nil
	}
	if pane.Kind == OccupantNormal {
		return []SlotID{current[len(current)-1].Slot}
	}

	slots := make([]SlotID, 0, len(current))
	for _, binding := range current {
		slots = append(slots, binding.Slot)
	}
	return slots
}

func needsWorkspaceMaterialization(workspacePaneIDs []int, bindings []SlotBinding) bool {
	for _, paneID := range bindingPaneIDs(bindings) {
		if !workspaceContains(workspacePaneIDs, paneID) {
			return true
		}
	}
	return false
}

func workspaceContains(workspacePaneIDs []int, paneID int) bool {
	for _, id := range workspacePaneIDs {
		if id == paneID {
			return true
		}
	}
	return false
}

func insertPaneIDAt(workspacePaneIDs []int, index, paneID int) []int {
	if index >= len(workspacePaneIDs) {
		return append(workspacePaneIDs, paneID)
	}

	workspacePaneIDs = append(workspacePaneIDs, 0)
	copy(workspacePaneIDs[index+1:], workspacePaneIDs[index:])
	workspacePaneIDs[index] = paneID
	return workspacePaneIDs
}
