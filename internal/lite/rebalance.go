package lite

import (
	"fmt"

	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

type workspaceLayoutClient interface {
	ListWindowPanes(windowID int) ([]wezterm.PaneInfo, error)
	SplitPane(opts wezterm.SplitPaneOptions) (int, error)
	AdjustPaneSize(paneID int, direction string, amount int) error
}

func rebalanceWorkspace(client workspaceLayoutClient, windowID int, paneIDs []int) error {
	if client == nil || windowID == 0 || len(paneIDs) < 2 {
		return nil
	}
	return rebalanceWorkspaceWithCurrent(client, windowID, paneIDs, paneIDs)
}

func rebalanceWorkspaceWithCurrent(client workspaceLayoutClient, windowID int, currentPaneIDs, desiredPaneIDs []int) error {
	if client == nil || windowID == 0 || len(desiredPaneIDs) < 2 {
		return nil
	}
	if reflowed, err := reflowWorkspace(client, currentPaneIDs, desiredPaneIDs); err != nil {
		return err
	} else if reflowed {
		nudgeWorkspaceLayout(client, desiredPaneIDs)
		return nil
	}

	workspace, err := materializeWorkspace(client, currentPaneIDs, desiredPaneIDs)
	if err != nil {
		return err
	}
	if reflowed, err := reflowWorkspace(client, workspace, desiredPaneIDs); err != nil {
		return err
	} else if reflowed {
		nudgeWorkspaceLayout(client, desiredPaneIDs)
		return nil
	}

	return adjustWorkspaceWidths(client, windowID, desiredPaneIDs)
}

func materializeWorkspace(client workspaceLayoutClient, currentPaneIDs, desiredPaneIDs []int) ([]int, error) {
	workspace := append([]int(nil), currentPaneIDs...)
	for i, paneID := range desiredPaneIDs {
		if workspaceContains(workspace, paneID) {
			continue
		}
		if len(workspace) == 0 {
			return nil, fmt.Errorf("workspace has no anchor pane for %d", paneID)
		}

		anchorPaneID := workspace[0]
		direction := "left"
		if i > 0 {
			anchorPaneID = workspace[i-1]
			direction = "right"
		}

		if _, err := client.SplitPane(wezterm.SplitPaneOptions{
			PaneID:     anchorPaneID,
			Direction:  direction,
			MovePaneID: paneID,
		}); err != nil {
			return nil, fmt.Errorf("move pane %d into workspace: %w", paneID, err)
		}
		workspace = insertPaneIDAt(workspace, i, paneID)
	}
	return workspace, nil
}

func adjustWorkspaceWidths(client workspaceLayoutClient, windowID int, desiredPaneIDs []int) error {
	panes, err := client.ListWindowPanes(windowID)
	if err != nil {
		return fmt.Errorf("list window panes for rebalance: %w", err)
	}

	panesByID := make(map[int]wezterm.PaneInfo, len(panes))
	totalCols := 0
	for _, pane := range panes {
		panesByID[pane.PaneID] = pane
	}
	widths := make([]int, 0, len(desiredPaneIDs))
	for _, paneID := range desiredPaneIDs {
		pane, ok := panesByID[paneID]
		if !ok {
			return fmt.Errorf("pane %d not found for rebalance", paneID)
		}
		if pane.Cols <= 0 {
			return nil
		}
		widths = append(widths, pane.Cols)
		totalCols += pane.Cols
	}

	prefixCols := 0
	n := len(widths)
	for i, width := range widths[:n-1] {
		prefixCols += width
		targetPrefix := roundDiv(totalCols*(i+1), n)
		delta := targetPrefix - prefixCols
		if delta == 0 {
			continue
		}

		direction := "Right"
		amount := delta
		if delta < 0 {
			direction = "Left"
			amount = -delta
		}
		if err := client.AdjustPaneSize(desiredPaneIDs[i], direction, amount); err != nil {
			return fmt.Errorf("adjust pane %d %s by %d: %w", desiredPaneIDs[i], direction, amount, err)
		}
		prefixCols = targetPrefix
	}

	return nil
}

func reflowWorkspace(client workspaceLayoutClient, currentPaneIDs, desiredPaneIDs []int) (bool, error) {
	switch len(desiredPaneIDs) {
	case 2:
		if workspaceContains(currentPaneIDs, desiredPaneIDs[0]) {
			if _, err := client.SplitPane(wezterm.SplitPaneOptions{
				PaneID:     desiredPaneIDs[0],
				Direction:  "right",
				Percent:    50,
				MovePaneID: desiredPaneIDs[1],
			}); err != nil {
				return false, fmt.Errorf("reflow 2-pane workspace from slot1: %w", err)
			}
			return true, nil
		}
		if workspaceContains(currentPaneIDs, desiredPaneIDs[1]) {
			if _, err := client.SplitPane(wezterm.SplitPaneOptions{
				PaneID:     desiredPaneIDs[1],
				Direction:  "left",
				Percent:    50,
				MovePaneID: desiredPaneIDs[0],
			}); err != nil {
				return false, fmt.Errorf("reflow 2-pane workspace from slot2: %w", err)
			}
			return true, nil
		}
	case 3:
		if workspaceContains(currentPaneIDs, desiredPaneIDs[0]) {
			if _, err := client.SplitPane(wezterm.SplitPaneOptions{
				PaneID:     desiredPaneIDs[0],
				Direction:  "right",
				Percent:    67,
				MovePaneID: desiredPaneIDs[1],
			}); err != nil {
				return false, fmt.Errorf("reflow 3-pane workspace first split: %w", err)
			}
			if _, err := client.SplitPane(wezterm.SplitPaneOptions{
				PaneID:     desiredPaneIDs[1],
				Direction:  "right",
				Percent:    50,
				MovePaneID: desiredPaneIDs[2],
			}); err != nil {
				return false, fmt.Errorf("reflow 3-pane workspace second split: %w", err)
			}
			return true, nil
		}
	}
	return false, nil
}

func nudgeWorkspaceLayout(client workspaceLayoutClient, paneIDs []int) {
	if client == nil || len(paneIDs) < 2 {
		return
	}
	for i := 0; i < len(paneIDs)-1; i++ {
		_ = client.AdjustPaneSize(paneIDs[i], "Right", 1)
		_ = client.AdjustPaneSize(paneIDs[i+1], "Left", 1)
	}
}

func roundDiv(numerator, denominator int) int {
	if denominator == 0 {
		return 0
	}
	return (numerator + denominator/2) / denominator
}
