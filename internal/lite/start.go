package lite

import (
	"fmt"
	"strconv"
	"time"

	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

const defaultMonitorPercent = 35

const (
	monitorActivateAttempts   = 5
	monitorActivateRetryDelay = 50 * time.Millisecond
)

type StartOptions struct {
	WezTermPath    string
	MonitorPercent int
	MonitorCommand []string
}

type wezTermRuntime interface {
	ListPanes() ([]wezterm.PaneInfo, error)
	ListWindowPanes(windowID int) ([]wezterm.PaneInfo, error)
	ReadScreen(sessionID string, lines int) (string, error)
	GetVar(sessionID, varName string) (string, error)
	SplitPane(opts wezterm.SplitPaneOptions) (int, error)
	AdjustPaneSize(paneID int, direction string, amount int) error
	MovePaneToNewTab(paneID, windowID int) error
	ActivateTab(tabID int) error
	ActivatePane(sessionID string) error
}

var (
	currentPaneIDFromEnv    = wezterm.CurrentPaneIDFromEnv
	newWezTermRuntime       = func(path string) wezTermRuntime { return wezTermClientRuntime{Client: wezterm.NewClient(path)} }
	sleepForActivationRetry = time.Sleep
)

type wezTermClientRuntime struct {
	*wezterm.Client
}

func (r wezTermClientRuntime) ActivatePane(sessionID string) error {
	return r.FocusSession(sessionID)
}

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

	bindings, overflow := PlanInitialLayout(classifyCandidates(runtime, windowPanes))

	requiredBindingPaneIDs := bindingPaneIDs(bindings)
	for i, paneID := range overflow {
		windowPanes, err := runtime.ListWindowPanes(starterPane.WindowID)
		if err != nil {
			return fmt.Errorf("recheck window panes for window %d: %w", starterPane.WindowID, err)
		}
		requiredPaneIDs := append([]int(nil), requiredBindingPaneIDs...)
		requiredPaneIDs = append(requiredPaneIDs, overflow[i:]...)
		if err := requireWindowPanes(windowPanes, starterPane.WindowID, requiredPaneIDs...); err != nil {
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
	requiredPaneIDs := bindingPaneIDs(bindings)
	if len(requiredPaneIDs) == 0 {
		requiredPaneIDs = []int{starterPaneID}
	}
	if err := requireWindowPanes(windowPanes, starterPane.WindowID, requiredPaneIDs...); err != nil {
		return err
	}

	bindings = ShrinkBindings(bindings, paneIDSet(windowPanes))
	if err := rebalanceWorkspace(runtime, starterPane.WindowID, bindingPaneIDs(bindings)); err != nil {
		return fmt.Errorf("rebalance initial workspace: %w", err)
	}
	monitorAnchorPaneID := starterPaneID
	if len(bindings) > 0 {
		boundPaneIDs := bindingPaneIDs(bindings)
		monitorAnchorPaneID = bindings[0].PaneID

		if !workspaceContains(boundPaneIDs, starterPaneID) {
			livePaneIDs := paneIDSet(windowPanes)
			if livePaneIDs[starterPaneID] {
				if err := requireWindowPanes(windowPanes, starterPane.WindowID, starterPaneID, monitorAnchorPaneID); err != nil {
					return err
				}
				if err := runtime.MovePaneToNewTab(starterPaneID, starterPane.WindowID); err != nil {
					return fmt.Errorf("move starter pane %d to new tab in window %d: %w", starterPaneID, starterPane.WindowID, err)
				}

				windowPanes, err = runtime.ListWindowPanes(starterPane.WindowID)
				if err != nil {
					return fmt.Errorf("recheck window panes before monitor split for window %d: %w", starterPane.WindowID, err)
				}
			}
			if err := requireWindowPanes(windowPanes, starterPane.WindowID, monitorAnchorPaneID); err != nil {
				return err
			}
		}
	}

	ctx := MonitorContext{
		SelfPaneID:       0,
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

	monitorPaneID, err := runtime.SplitPane(wezterm.SplitPaneOptions{
		PaneID:    monitorAnchorPaneID,
		Direction: "top",
		TopLevel:  true,
		Percent:   monitorPercent(opts.MonitorPercent),
		Command:   buildMonitorCommand(opts.MonitorCommand, encodedContext),
	})
	if err != nil {
		return fmt.Errorf("start monitor pane: %w", err)
	}
	if err := activateMonitorPane(runtime, strconv.Itoa(monitorPaneID)); err != nil {
		return fmt.Errorf("activate monitor pane %d: %w", monitorPaneID, err)
	}

	return nil
}

func activateMonitorPane(runtime wezTermRuntime, sessionID string) error {
	var lastErr error
	for attempt := 0; attempt < monitorActivateAttempts; attempt++ {
		if err := runtime.ActivatePane(sessionID); err != nil {
			lastErr = err
			if attempt < monitorActivateAttempts-1 {
				sleepForActivationRetry(monitorActivateRetryDelay)
			}
			continue
		}
		return nil
	}
	return lastErr
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

func classifyCandidates(runtime wezTermRuntime, panes []wezterm.PaneInfo) []CandidatePane {
	candidates := make([]CandidatePane, 0, len(panes))
	for _, pane := range panes {
		candidates = append(candidates, classifyCandidatePane(runtime, pane))
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

	command := make([]string, 0, len(prefix)+6)
	command = append(command, "env", "-u", "NO_COLOR", "CLICOLOR_FORCE=1")
	command = append(command, prefix...)
	command = append(command, "--context-base64", encodedContext)
	return command
}
