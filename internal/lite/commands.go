package lite

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sethdeckard/atria/internal/terminal"
	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

const liteScreenReadLines = 40

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
		return candidatePanesLoadedMsg{panes: candidates}
	}
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
		return slotActionCompletedMsg{
			bindings:   nextBindings,
			statusText: fmt.Sprintf("Loaded %s into %s", paneLabel(pane), slot),
		}
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

	agentType := terminal.DetectAgent(pane.Title)
	if agentType == "" && client != nil {
		content, err := client.ReadScreen(strconv.Itoa(pane.PaneID), liteScreenReadLines)
		if err == nil {
			agentType = terminal.InferAgentFromScreen(content)
		}
	}
	if agentType != "" {
		candidate.Kind = OccupantAgent
		candidate.AgentType = agentType
		candidate.CWD = discoverPaneCWD(client, pane)
	}

	return candidate
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
