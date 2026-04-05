package lite

import (
	"reflect"
	"testing"

	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

func TestRebalanceWorkspaceReflowsTwoColumnSplit(t *testing.T) {
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 85, WindowID: 12, Cols: 44},
			{PaneID: 86, WindowID: 12, Cols: 35},
		},
	}

	if err := rebalanceWorkspace(client, 12, []int{85, 86}); err != nil {
		t.Fatalf("rebalanceWorkspace() error = %v", err)
	}

	want := []wezterm.SplitPaneOptions{
		{PaneID: 85, Direction: "right", Percent: 50, MovePaneID: 86},
	}
	if !reflect.DeepEqual(client.splitPaneCalls, want) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, want)
	}
	wantAdjust := []adjustPaneCall{
		{PaneID: 85, Direction: "Right", Amount: 1},
		{PaneID: 86, Direction: "Left", Amount: 1},
	}
	if !reflect.DeepEqual(client.adjustPaneCalls, wantAdjust) {
		t.Fatalf("AdjustPaneSize() calls = %#v, want %#v", client.adjustPaneCalls, wantAdjust)
	}
}

func TestRebalanceWorkspaceReflowsThreeColumnSplit(t *testing.T) {
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 77, WindowID: 11, Cols: 99},
			{PaneID: 82, WindowID: 11, Cols: 88},
			{PaneID: 84, WindowID: 11, Cols: 158},
		},
	}

	if err := rebalanceWorkspace(client, 11, []int{77, 82, 84}); err != nil {
		t.Fatalf("rebalanceWorkspace() error = %v", err)
	}

	want := []wezterm.SplitPaneOptions{
		{PaneID: 77, Direction: "right", Percent: 67, MovePaneID: 82},
		{PaneID: 82, Direction: "right", Percent: 50, MovePaneID: 84},
	}
	if !reflect.DeepEqual(client.splitPaneCalls, want) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, want)
	}
	wantAdjust := []adjustPaneCall{
		{PaneID: 77, Direction: "Right", Amount: 1},
		{PaneID: 82, Direction: "Left", Amount: 1},
		{PaneID: 82, Direction: "Right", Amount: 1},
		{PaneID: 84, Direction: "Left", Amount: 1},
	}
	if !reflect.DeepEqual(client.adjustPaneCalls, wantAdjust) {
		t.Fatalf("AdjustPaneSize() calls = %#v, want %#v", client.adjustPaneCalls, wantAdjust)
	}
}

func TestRebalanceWorkspaceMaterializesMissingLeadingPaneBeforeReflow(t *testing.T) {
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 12, WindowID: 7, Cols: 80},
			{PaneID: 13, WindowID: 7, Cols: 80},
		},
	}

	if err := rebalanceWorkspaceWithCurrent(client, 7, []int{12, 13}, []int{99, 12, 13}); err != nil {
		t.Fatalf("rebalanceWorkspaceWithCurrent() error = %v", err)
	}

	want := []wezterm.SplitPaneOptions{
		{PaneID: 12, Direction: "left", MovePaneID: 99},
		{PaneID: 99, Direction: "right", Percent: 67, MovePaneID: 12},
		{PaneID: 12, Direction: "right", Percent: 50, MovePaneID: 13},
	}
	if !reflect.DeepEqual(client.splitPaneCalls, want) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, want)
	}
	wantAdjust := []adjustPaneCall{
		{PaneID: 99, Direction: "Right", Amount: 1},
		{PaneID: 12, Direction: "Left", Amount: 1},
		{PaneID: 12, Direction: "Right", Amount: 1},
		{PaneID: 13, Direction: "Left", Amount: 1},
	}
	if !reflect.DeepEqual(client.adjustPaneCalls, wantAdjust) {
		t.Fatalf("AdjustPaneSize() calls = %#v, want %#v", client.adjustPaneCalls, wantAdjust)
	}
}
