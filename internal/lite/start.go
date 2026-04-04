package lite

import (
	"fmt"

	"github.com/sethdeckard/atria/internal/model"
	"github.com/sethdeckard/atria/internal/terminal"
	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

const defaultMonitorPercent = 35

type StartOptions struct {
	WezTermPath    string
	MonitorPercent int
	MonitorCommand []string
}

type wezTermRuntime interface {
	ListPanes() ([]wezterm.PaneInfo, error)
	ListWindowPanes(windowID int) ([]wezterm.PaneInfo, error)
	SplitPane(opts wezterm.SplitPaneOptions) (int, error)
	MovePaneToNewTab(paneID, windowID int) error
}

var (
	currentPaneIDFromEnv = wezterm.CurrentPaneIDFromEnv
	newWezTermRuntime    = func(path string) wezTermRuntime { return wezterm.NewClient(path) }
)

func Start(opts StartOptions) error {
	starterPaneID, err := currentPaneIDFromEnv()
	if err != nil {
		return fmt.Errorf("resolve current wezterm pane: %w", err)
	}
	return startWithRuntime(newWezTermRuntime(opts.WezTermPath), starterPaneID, opts)
}

func startWithRuntime(runtime wezTermRuntime, starterPaneID int, opts StartOptions) error {
	starterPane, err := findStarterPane(runtime, starterPaneID)
	if err != nil {
		return err
	}

	windowPanes, err := runtime.ListWindowPanes(starterPane.WindowID)
	if err != nil {
		return fmt.Errorf("list window panes for window %d: %w", starterPane.WindowID, err)
	}
	if err := requireWindowPanes(windowPanes, starterPane.WindowID, starterPaneID); err != nil {
		return err
	}

	bindings, overflow := PlanInitialLayout(classifyCandidates(windowPanes, starterPaneID))

	for _, paneID := range overflow {
		windowPanes, err := runtime.ListWindowPanes(starterPane.WindowID)
		if err != nil {
			return fmt.Errorf("recheck window panes for window %d: %w", starterPane.WindowID, err)
		}
		if err := requireWindowPanes(windowPanes, starterPane.WindowID, starterPaneID, paneID); err != nil {
			return err
		}
		if err := runtime.MovePaneToNewTab(paneID, starterPane.WindowID); err != nil {
			return fmt.Errorf("move pane %d to new tab in window %d: %w", paneID, starterPane.WindowID, err)
		}
	}

	windowPanes, err = runtime.ListWindowPanes(starterPane.WindowID)
	if err != nil {
		return fmt.Errorf("recheck window panes before monitor split for window %d: %w", starterPane.WindowID, err)
	}
	if err := requireWindowPanes(windowPanes, starterPane.WindowID, starterPaneID); err != nil {
		return err
	}

	bindings = ShrinkBindings(bindings, paneIDSet(windowPanes))
	ctx := MonitorContext{
		StarterPaneID:    starterPaneID,
		WindowID:         starterPane.WindowID,
		TabID:            starterPane.TabID,
		SlotBindings:     bindings,
		WorkspacePaneIDs: bindingPaneIDs(bindings),
	}
	encodedContext, err := EncodeMonitorContext(ctx)
	if err != nil {
		return fmt.Errorf("encode monitor context: %w", err)
	}

	_, err = runtime.SplitPane(wezterm.SplitPaneOptions{
		PaneID:    starterPaneID,
		Direction: "top",
		TopLevel:  true,
		Percent:   monitorPercent(opts.MonitorPercent),
		Command:   buildMonitorCommand(opts.MonitorCommand, encodedContext),
	})
	if err != nil {
		return fmt.Errorf("start monitor pane: %w", err)
	}

	return nil
}

func findStarterPane(runtime wezTermRuntime, starterPaneID int) (wezterm.PaneInfo, error) {
	panes, err := runtime.ListPanes()
	if err != nil {
		return wezterm.PaneInfo{}, fmt.Errorf("list wezterm panes: %w", err)
	}
	for _, pane := range panes {
		if pane.PaneID == starterPaneID {
			return pane, nil
		}
	}
	return wezterm.PaneInfo{}, fmt.Errorf("starter pane %d was not found", starterPaneID)
}

func classifyCandidates(panes []wezterm.PaneInfo, starterPaneID int) []CandidatePane {
	candidates := make([]CandidatePane, 0, len(panes))
	for _, pane := range panes {
		if pane.PaneID == starterPaneID {
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

func requireWindowPanes(panes []wezterm.PaneInfo, windowID int, requiredPaneIDs ...int) error {
	live := paneIDSet(panes)
	for _, paneID := range requiredPaneIDs {
		if !live[paneID] {
			return fmt.Errorf("pane %d is no longer present in window %d", paneID, windowID)
		}
	}
	return nil
}

func paneIDSet(panes []wezterm.PaneInfo) map[int]bool {
	live := make(map[int]bool, len(panes))
	for _, pane := range panes {
		live[pane.PaneID] = true
	}
	return live
}

func bindingPaneIDs(bindings []SlotBinding) []int {
	ids := make([]int, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.PaneID)
	}
	return ids
}

func monitorPercent(percent int) int {
	if percent == 0 {
		return defaultMonitorPercent
	}
	return percent
}

func buildMonitorCommand(prefix []string, encodedContext string) []string {
	if len(prefix) == 0 {
		prefix = []string{"atria-lite", "monitor"}
	}

	command := make([]string, 0, len(prefix)+2)
	command = append(command, prefix...)
	command = append(command, "--context-base64", encodedContext)
	return command
}
