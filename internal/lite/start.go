package lite

import (
	"fmt"
	"strconv"
	"time"

	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

const defaultMonitorPercent = 35
const minimumStartupSlots = 2
const fixedMonitorRows = 7

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
	SendText(sessionID, text string) error
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
	windowPanes = panesInTab(windowPanes, starterPane.TabID)
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
		windowPanes = panesInTab(windowPanes, starterPane.TabID)
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
	windowPanes = panesInTab(windowPanes, starterPane.TabID)
	requiredPaneIDs := bindingPaneIDs(bindings)
	if len(requiredPaneIDs) == 0 {
		requiredPaneIDs = []int{starterPaneID}
	}
	if err := requireWindowPanes(windowPanes, starterPane.WindowID, requiredPaneIDs...); err != nil {
		return err
	}

	bindings = ShrinkBindings(bindings, paneIDSet(windowPanes))
	bindings, createdStartupNormals, err := ensureMinimumStartupBindings(runtime, starterPaneID, bindings)
	if err != nil {
		return fmt.Errorf("ensure minimum startup slots: %w", err)
	}
	if !createdStartupNormals {
		if err := rebalanceWorkspace(runtime, starterPane.WindowID, bindingPaneIDs(bindings)); err != nil {
			return fmt.Errorf("rebalance initial workspace: %w", err)
		}
	}
	if err := clearStarterPaneIfReused(runtime, starterPaneID, bindings); err != nil {
		return fmt.Errorf("clear starter pane %d: %w", starterPaneID, err)
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
				windowPanes = panesInTab(windowPanes, starterPane.TabID)
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
	if err := resizeMonitorToFixedRows(runtime, starterPane.WindowID, monitorPaneID, fixedMonitorRows); err != nil {
		return fmt.Errorf("resize monitor pane %d: %w", monitorPaneID, err)
	}
	if err := activateMonitorPane(runtime, strconv.Itoa(monitorPaneID)); err != nil {
		return fmt.Errorf("activate monitor pane %d: %w", monitorPaneID, err)
	}

	return nil
}

func panesInTab(panes []wezterm.PaneInfo, tabID int) []wezterm.PaneInfo {
	if tabID == 0 {
		return append([]wezterm.PaneInfo(nil), panes...)
	}

	filtered := make([]wezterm.PaneInfo, 0, len(panes))
	for _, pane := range panes {
		if pane.TabID == tabID {
			filtered = append(filtered, pane)
		}
	}
	return filtered
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

func clearStarterPaneIfReused(runtime wezTermRuntime, starterPaneID int, bindings []SlotBinding) error {
	if runtime == nil || starterPaneID == 0 {
		return nil
	}
	for _, binding := range bindings {
		if binding.PaneID == starterPaneID && binding.Kind == OccupantNormal {
			if err := runtime.SendText(strconv.Itoa(starterPaneID), "\f"); err != nil {
				return fmt.Errorf("send clear-screen to starter pane: %w", err)
			}
			return nil
		}
	}
	return nil
}

func resizeMonitorToFixedRows(runtime wezTermRuntime, windowID, monitorPaneID, targetRows int) error {
	if runtime == nil || windowID == 0 || monitorPaneID == 0 || targetRows <= 0 {
		return nil
	}

	panes, err := runtime.ListWindowPanes(windowID)
	if err != nil {
		return fmt.Errorf("list window panes after monitor split: %w", err)
	}

	var monitorPane wezterm.PaneInfo
	found := false
	for _, pane := range panes {
		if pane.PaneID == monitorPaneID {
			monitorPane = pane
			found = true
			break
		}
	}
	if !found || monitorPane.Rows <= 0 || monitorPane.Rows == targetRows {
		return nil
	}

	direction := "Down"
	amount := targetRows - monitorPane.Rows
	if amount < 0 {
		direction = "Up"
		amount = -amount
	}
	if amount == 0 {
		return nil
	}
	if err := runtime.AdjustPaneSize(monitorPaneID, direction, amount); err != nil {
		return fmt.Errorf("adjust monitor pane to %d rows: %w", targetRows, err)
	}
	return nil
}

func ensureMinimumStartupBindings(runtime wezTermRuntime, starterPaneID int, bindings []SlotBinding) ([]SlotBinding, bool, error) {
	current := normalizeBindings(bindings)
	created := false
	for len(current) < minimumStartupSlots {
		anchorPaneID := starterPaneID
		if len(current) > 0 {
			anchorPaneID = current[len(current)-1].PaneID
		}
		if anchorPaneID == 0 {
			return nil, created, fmt.Errorf("workspace has no pane to split for startup slot creation")
		}
		paneID, err := runtime.SplitPane(wezterm.SplitPaneOptions{
			PaneID:    anchorPaneID,
			Direction: "right",
			Percent:   50,
		})
		if err != nil {
			return nil, created, fmt.Errorf("create startup normal pane from %d: %w", anchorPaneID, err)
		}
		current = append(current, SlotBinding{
			Slot:   slotOrder[len(current)],
			PaneID: paneID,
			Kind:   OccupantNormal,
		})
		current = normalizeBindings(current)
		created = true
	}
	return current, created, nil
}
