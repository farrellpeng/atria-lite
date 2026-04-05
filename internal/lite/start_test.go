package lite

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

func TestStartRejectsMissingWezTermPaneEnv(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "")

	err := Start(StartOptions{})
	if err == nil {
		t.Fatal("Start() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "WEZTERM_PANE is not set") {
		t.Fatalf("Start() error = %q, want WEZTERM_PANE message", err)
	}
}

func TestStartSplitsTopMonitorWithTopLevelPercent(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "101")

	runtime := &mockStartRuntime{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
		},
		windowPanes: [][]wezterm.PaneInfo{
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "notes"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "notes"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
			},
		},
		splitPaneID: 999,
	}
	installMockStartRuntime(t, runtime)

	if err := Start(StartOptions{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	wantMoves := []movePaneCall{
		{PaneID: 202, WindowID: 700},
	}
	if !reflect.DeepEqual(runtime.moveCalls, wantMoves) {
		t.Fatalf("MovePaneToNewTab() calls = %#v, want %#v", runtime.moveCalls, wantMoves)
	}
	if len(runtime.splitCalls) != 2 {
		t.Fatalf("SplitPane() call count = %d, want 2", len(runtime.splitCalls))
	}

	workspaceSplit := runtime.splitCalls[0]
	if workspaceSplit.PaneID != 201 {
		t.Fatalf("workspace SplitPane() PaneID = %d, want 201", workspaceSplit.PaneID)
	}
	if workspaceSplit.Direction != "right" {
		t.Fatalf("workspace SplitPane() Direction = %q, want %q", workspaceSplit.Direction, "right")
	}
	if workspaceSplit.Percent != 50 {
		t.Fatalf("workspace SplitPane() Percent = %d, want 50", workspaceSplit.Percent)
	}
	if workspaceSplit.MovePaneID != 101 {
		t.Fatalf("workspace SplitPane() MovePaneID = %d, want 101", workspaceSplit.MovePaneID)
	}

	got := runtime.splitCalls[1]
	if got.PaneID != 201 {
		t.Fatalf("SplitPane() PaneID = %d, want 201", got.PaneID)
	}
	if got.Direction != "top" {
		t.Fatalf("SplitPane() Direction = %q, want %q", got.Direction, "top")
	}
	if !got.TopLevel {
		t.Fatal("SplitPane() TopLevel = false, want true")
	}
	if got.Percent != 35 {
		t.Fatalf("SplitPane() Percent = %d, want 35", got.Percent)
	}
	if len(runtime.activateCalls) != 1 || runtime.activateCalls[0] != "999" {
		t.Fatalf("ActivatePane() calls = %v, want [999]", runtime.activateCalls)
	}

	if len(got.Command) != 8 {
		t.Fatalf("SplitPane() Command = %v, want 8 args", got.Command)
	}
	wantCommandPrefix := []string{"env", "-u", "NO_COLOR", "CLICOLOR_FORCE=1", "atria-lite", "monitor", "--context-base64"}
	if !reflect.DeepEqual(got.Command[:7], wantCommandPrefix) {
		t.Fatalf("SplitPane() Command prefix = %v, want %v", got.Command[:7], wantCommandPrefix)
	}

	ctx, err := decodeMonitorCommandContext(got.Command)
	if err != nil {
		t.Fatalf("DecodeMonitorContext() error = %v", err)
	}

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 201, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 101, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(ctx.SlotBindings, wantBindings) {
		t.Fatalf("SlotBindings = %#v, want %#v", ctx.SlotBindings, wantBindings)
	}
	if !reflect.DeepEqual(ctx.WorkspacePaneIDs, []int{201, 101}) {
		t.Fatalf("WorkspacePaneIDs = %v, want [201 101]", ctx.WorkspacePaneIDs)
	}
	if ctx.StarterPaneID != 101 {
		t.Fatalf("StarterPaneID = %d, want 101", ctx.StarterPaneID)
	}
	if ctx.WindowID != 700 {
		t.Fatalf("WindowID = %d, want 700", ctx.WindowID)
	}
	if ctx.TabID != 701 {
		t.Fatalf("TabID = %d, want 701", ctx.TabID)
	}
	if ctx.SelfPaneID != 0 {
		t.Fatalf("SelfPaneID = %d, want 0 before monitor fills from env", ctx.SelfPaneID)
	}
	if !reflect.DeepEqual(runtime.sendTextCalls, []sendTextCall{{SessionID: "101", Text: "\f"}}) {
		t.Fatalf("SendText() calls = %#v, want starter pane cleared once", runtime.sendTextCalls)
	}
}

func TestStartResizesMonitorToFixedSevenRows(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "101")

	runtime := &mockStartRuntime{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
		},
		windowPanes: [][]wezterm.PaneInfo{
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 999, WindowID: 700, TabID: 701, Title: "atria-lite", Rows: 12},
			},
		},
		splitPaneIDs: []int{202, 999},
	}
	installMockStartRuntime(t, runtime)

	if err := Start(StartOptions{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	wantAdjust := []adjustPaneCall{
		{PaneID: 999, Direction: "Up", Amount: 5},
	}
	if !reflect.DeepEqual(runtime.adjustCalls, wantAdjust) {
		t.Fatalf("AdjustPaneSize() calls = %#v, want %#v", runtime.adjustCalls, wantAdjust)
	}
}

func TestStartSkipsMonitorResizeWhenAlreadySevenRows(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "101")

	runtime := &mockStartRuntime{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
		},
		windowPanes: [][]wezterm.PaneInfo{
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 999, WindowID: 700, TabID: 701, Title: "atria-lite", Rows: 7},
			},
		},
		splitPaneIDs: []int{202, 999},
	}
	installMockStartRuntime(t, runtime)

	if err := Start(StartOptions{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if len(runtime.adjustCalls) != 0 {
		t.Fatalf("AdjustPaneSize() calls = %#v, want none", runtime.adjustCalls)
	}
}

func TestBuildMonitorCommandForcesColorInMonitorPane(t *testing.T) {
	got := buildMonitorCommand([]string{"/tmp/atria-lite", "monitor"}, "encoded")
	want := []string{
		"env",
		"-u",
		"NO_COLOR",
		"CLICOLOR_FORCE=1",
		"/tmp/atria-lite",
		"monitor",
		"--context-base64",
		"encoded",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildMonitorCommand() = %v, want %v", got, want)
	}
}

func TestStartCreatesSecondNormalSlotWhenOnlyStarterPaneExists(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "101")

	runtime := &mockStartRuntime{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
		},
		windowPanes: [][]wezterm.PaneInfo{
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
			},
		},
		splitPaneIDs: []int{202, 999},
	}
	installMockStartRuntime(t, runtime)

	if err := Start(StartOptions{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if len(runtime.moveCalls) != 0 {
		t.Fatalf("MovePaneToNewTab() calls = %#v, want none", runtime.moveCalls)
	}
	if len(runtime.splitCalls) != 2 {
		t.Fatalf("SplitPane() call count = %d, want 2", len(runtime.splitCalls))
	}
	wantWorkspaceSplit := wezterm.SplitPaneOptions{
		PaneID:    101,
		Direction: "right",
		Percent:   50,
	}
	if !reflect.DeepEqual(runtime.splitCalls[0], wantWorkspaceSplit) {
		t.Fatalf("workspace SplitPane() call = %#v, want %#v", runtime.splitCalls[0], wantWorkspaceSplit)
	}
	monitorSplit := runtime.splitCalls[1]
	if monitorSplit.PaneID != 101 || monitorSplit.Direction != "top" || !monitorSplit.TopLevel {
		t.Fatalf("monitor SplitPane() call = %#v, want top-level split from pane 101", monitorSplit)
	}
	ctx, err := decodeMonitorCommandContext(monitorSplit.Command)
	if err != nil {
		t.Fatalf("DecodeMonitorContext() error = %v", err)
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 101, Kind: OccupantNormal},
		{Slot: Slot2, PaneID: 202, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(ctx.SlotBindings, wantBindings) {
		t.Fatalf("SlotBindings = %#v, want %#v", ctx.SlotBindings, wantBindings)
	}
	if !reflect.DeepEqual(ctx.WorkspacePaneIDs, []int{101, 202}) {
		t.Fatalf("WorkspacePaneIDs = %v, want [101 202]", ctx.WorkspacePaneIDs)
	}
	if !reflect.DeepEqual(runtime.sendTextCalls, []sendTextCall{{SessionID: "101", Text: "\f"}}) {
		t.Fatalf("SendText() calls = %#v, want starter pane cleared once", runtime.sendTextCalls)
	}
}

func TestStartMovesOverflowPanesToNewTabInSameWindow(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "101")

	runtime := &mockStartRuntime{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
		},
		windowPanes: [][]wezterm.PaneInfo{
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "codex"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "opencode"},
				{PaneID: 204, WindowID: 700, TabID: 702, Title: "notes"},
				{PaneID: 205, WindowID: 700, TabID: 703, Title: "shell"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "codex"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "opencode"},
				{PaneID: 204, WindowID: 700, TabID: 702, Title: "notes"},
				{PaneID: 205, WindowID: 700, TabID: 703, Title: "shell"},
			},
			{
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "codex"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "opencode"},
				{PaneID: 204, WindowID: 700, TabID: 702, Title: "notes"},
				{PaneID: 205, WindowID: 700, TabID: 703, Title: "shell"},
			},
			{
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "codex"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "opencode"},
				{PaneID: 205, WindowID: 700, TabID: 703, Title: "shell"},
			},
			{
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "codex"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "opencode"},
			},
			{
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "codex"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "opencode"},
			},
		},
		splitPaneID: 999,
	}
	installMockStartRuntime(t, runtime)

	if err := Start(StartOptions{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	wantMoves := []movePaneCall{
		{PaneID: 101, WindowID: 700},
		{PaneID: 204, WindowID: 700},
		{PaneID: 205, WindowID: 700},
	}
	if !reflect.DeepEqual(runtime.moveCalls, wantMoves) {
		t.Fatalf("MovePaneToNewTab() calls = %#v, want %#v", runtime.moveCalls, wantMoves)
	}
	if len(runtime.splitCalls) != 3 {
		t.Fatalf("SplitPane() call count = %d, want 3", len(runtime.splitCalls))
	}
	wantWorkspaceSplits := []wezterm.SplitPaneOptions{
		{PaneID: 201, Direction: "right", Percent: 67, MovePaneID: 202},
		{PaneID: 202, Direction: "right", Percent: 50, MovePaneID: 203},
	}
	if !reflect.DeepEqual(runtime.splitCalls[:2], wantWorkspaceSplits) {
		t.Fatalf("workspace SplitPane() calls = %#v, want %#v", runtime.splitCalls[:2], wantWorkspaceSplits)
	}
	if runtime.splitCalls[2].PaneID != 201 {
		t.Fatalf("monitor SplitPane() PaneID = %d, want 201", runtime.splitCalls[2].PaneID)
	}
	if len(runtime.activateCalls) != 1 || runtime.activateCalls[0] != "999" {
		t.Fatalf("ActivatePane() calls = %v, want [999]", runtime.activateCalls)
	}
}

func TestStartKeepsStarterPaneWhenItIsPartOfWorkspace(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "101")

	runtime := &mockStartRuntime{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
		},
		windowPanes: [][]wezterm.PaneInfo{
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 702, Title: "claude code"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 702, Title: "claude code"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 702, Title: "claude code"},
			},
		},
		splitPaneID: 999,
	}
	installMockStartRuntime(t, runtime)

	if err := Start(StartOptions{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if len(runtime.moveCalls) != 0 {
		t.Fatalf("MovePaneToNewTab() calls = %#v, want none when starter stays in workspace", runtime.moveCalls)
	}
	if len(runtime.splitCalls) != 2 {
		t.Fatalf("SplitPane() call count = %d, want 2", len(runtime.splitCalls))
	}
	wantWorkspaceSplit := wezterm.SplitPaneOptions{
		PaneID:     201,
		Direction:  "right",
		Percent:    50,
		MovePaneID: 101,
	}
	if !reflect.DeepEqual(runtime.splitCalls[0], wantWorkspaceSplit) {
		t.Fatalf("workspace SplitPane() call = %#v, want %#v", runtime.splitCalls[0], wantWorkspaceSplit)
	}
	if runtime.splitCalls[1].PaneID != 201 || runtime.splitCalls[1].Direction != "top" || !runtime.splitCalls[1].TopLevel {
		t.Fatalf("monitor SplitPane() call = %#v, want top-level split from pane 201", runtime.splitCalls[1])
	}
	ctx, err := decodeMonitorCommandContext(runtime.splitCalls[1].Command)
	if err != nil {
		t.Fatalf("DecodeMonitorContext() error = %v", err)
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 201, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 101, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(ctx.SlotBindings, wantBindings) {
		t.Fatalf("SlotBindings = %#v, want %#v", ctx.SlotBindings, wantBindings)
	}
}

func TestStartRechecksPaneExistenceBeforeMutating(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "101")

	runtime := &mockStartRuntime{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
		},
		windowPanes: [][]wezterm.PaneInfo{
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "notes"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "shell"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "notes"},
			},
		},
	}
	installMockStartRuntime(t, runtime)

	err := Start(StartOptions{})
	if err == nil {
		t.Fatal("Start() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "pane 202") {
		t.Fatalf("Start() error = %q, want missing pane 202 message", err)
	}
	if len(runtime.moveCalls) != 0 {
		t.Fatalf("MovePaneToNewTab() calls = %v, want none", runtime.moveCalls)
	}
	if len(runtime.splitCalls) != 0 {
		t.Fatalf("SplitPane() calls = %v, want none", runtime.splitCalls)
	}
	if len(runtime.activateCalls) != 0 {
		t.Fatalf("ActivatePane() calls = %v, want none", runtime.activateCalls)
	}
}

func TestStartUsesScreenFallbackToDetectAgents(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "101")

	runtime := &mockStartRuntime{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
		},
		windowPanes: [][]wezterm.PaneInfo{
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "notes"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "notes"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "notes"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "shell"},
			},
		},
		readScreens: map[int]string{
			201: "Claude Code\n",
			202: "OpenAI Codex\n› ",
		},
		splitPaneID: 999,
	}
	installMockStartRuntime(t, runtime)

	if err := Start(StartOptions{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if len(runtime.splitCalls) != 3 {
		t.Fatalf("SplitPane() call count = %d, want 3", len(runtime.splitCalls))
	}
	wantWorkspaceSplits := []wezterm.SplitPaneOptions{
		{PaneID: 201, Direction: "right", Percent: 67, MovePaneID: 202},
		{PaneID: 202, Direction: "right", Percent: 50, MovePaneID: 101},
	}
	if !reflect.DeepEqual(runtime.splitCalls[:2], wantWorkspaceSplits) {
		t.Fatalf("workspace SplitPane() calls = %#v, want %#v", runtime.splitCalls[:2], wantWorkspaceSplits)
	}
	if runtime.splitCalls[2].PaneID != 201 {
		t.Fatalf("monitor SplitPane() PaneID = %d, want 201", runtime.splitCalls[2].PaneID)
	}
	ctx, err := decodeMonitorCommandContext(runtime.splitCalls[2].Command)
	if err != nil {
		t.Fatalf("DecodeMonitorContext() error = %v", err)
	}

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 201, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 202, Kind: OccupantAgent},
		{Slot: Slot3, PaneID: 101, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(ctx.SlotBindings, wantBindings) {
		t.Fatalf("SlotBindings = %#v, want %#v", ctx.SlotBindings, wantBindings)
	}
	if !reflect.DeepEqual(runtime.moveCalls, []movePaneCall{{PaneID: 203, WindowID: 700}}) {
		t.Fatalf("MovePaneToNewTab() calls = %#v, want notes moved only", runtime.moveCalls)
	}
}

func TestStartRetriesMonitorActivation(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "101")

	runtime := &mockStartRuntime{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
		},
		windowPanes: [][]wezterm.PaneInfo{
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
			},
			{
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
			},
		},
		splitPaneID: 999,
		activateErrs: []error{
			strconv.ErrSyntax,
			strconv.ErrRange,
		},
	}
	installMockStartRuntime(t, runtime)

	oldSleep := sleepForActivationRetry
	sleepForActivationRetry = func(time.Duration) {}
	t.Cleanup(func() {
		sleepForActivationRetry = oldSleep
	})

	if err := Start(StartOptions{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if len(runtime.activateCalls) != 3 {
		t.Fatalf("ActivatePane() calls = %v, want 3 retries", runtime.activateCalls)
	}
	for _, call := range runtime.activateCalls {
		if call != "999" {
			t.Fatalf("ActivatePane() call = %q, want 999", call)
		}
	}
}

func decodeMonitorCommandContext(command []string) (MonitorContext, error) {
	if len(command) == 0 {
		return MonitorContext{}, fmt.Errorf("decode monitor context: empty command")
	}
	return DecodeMonitorContext(command[len(command)-1])
}

type mockStartRuntime struct {
	panes           []wezterm.PaneInfo
	listPanesErr    error
	windowPanes     [][]wezterm.PaneInfo
	listWindowErr   error
	listWindowCalls int
	readScreens     map[int]string
	getVars         map[int]string
	moveCalls       []movePaneCall
	moveErr         error
	splitCalls      []wezterm.SplitPaneOptions
	splitPaneID     int
	splitPaneIDs    []int
	splitErr        error
	adjustCalls     []adjustPaneCall
	adjustErr       error
	sendTextCalls   []sendTextCall
	sendTextErr     error
	activateCalls   []string
	activateErrs    []error
	activateErr     error
}

type movePaneCall struct {
	PaneID   int
	WindowID int
}

type adjustPaneCall struct {
	PaneID    int
	Direction string
	Amount    int
}

type sendTextCall struct {
	SessionID string
	Text      string
}

func (m *mockStartRuntime) ListPanes() ([]wezterm.PaneInfo, error) {
	if m.listPanesErr != nil {
		return nil, m.listPanesErr
	}
	return append([]wezterm.PaneInfo(nil), m.panes...), nil
}

func (m *mockStartRuntime) ListWindowPanes(windowID int) ([]wezterm.PaneInfo, error) {
	if m.listWindowErr != nil {
		return nil, m.listWindowErr
	}
	if len(m.windowPanes) == 0 {
		return nil, nil
	}

	idx := m.listWindowCalls
	if idx >= len(m.windowPanes) {
		idx = len(m.windowPanes) - 1
	}
	m.listWindowCalls++

	snapshot := m.windowPanes[idx]
	return append([]wezterm.PaneInfo(nil), snapshot...), nil
}

func (m *mockStartRuntime) MovePaneToNewTab(paneID, windowID int) error {
	m.moveCalls = append(m.moveCalls, movePaneCall{PaneID: paneID, WindowID: windowID})
	return m.moveErr
}

func (m *mockStartRuntime) ReadScreen(sessionID string, lines int) (string, error) {
	for paneID, text := range m.readScreens {
		if sessionID == strconv.Itoa(paneID) {
			return text, nil
		}
	}
	return "", nil
}

func (m *mockStartRuntime) GetVar(sessionID, varName string) (string, error) {
	if varName != "path" {
		return "", nil
	}
	for paneID, path := range m.getVars {
		if sessionID == strconv.Itoa(paneID) {
			return path, nil
		}
	}
	return "", nil
}

func (m *mockStartRuntime) SplitPane(opts wezterm.SplitPaneOptions) (int, error) {
	m.splitCalls = append(m.splitCalls, opts)
	if m.splitErr != nil {
		return 0, m.splitErr
	}
	if len(m.splitPaneIDs) > 0 {
		paneID := m.splitPaneIDs[0]
		m.splitPaneIDs = m.splitPaneIDs[1:]
		return paneID, nil
	}
	if m.splitPaneID == 0 {
		m.splitPaneID = 999
	}
	return m.splitPaneID, nil
}

func (m *mockStartRuntime) AdjustPaneSize(paneID int, direction string, amount int) error {
	m.adjustCalls = append(m.adjustCalls, adjustPaneCall{
		PaneID:    paneID,
		Direction: direction,
		Amount:    amount,
	})
	return m.adjustErr
}

func (m *mockStartRuntime) SendText(sessionID, text string) error {
	m.sendTextCalls = append(m.sendTextCalls, sendTextCall{SessionID: sessionID, Text: text})
	return m.sendTextErr
}

func (m *mockStartRuntime) ActivatePane(sessionID string) error {
	m.activateCalls = append(m.activateCalls, sessionID)
	if len(m.activateErrs) > 0 {
		err := m.activateErrs[0]
		m.activateErrs = m.activateErrs[1:]
		return err
	}
	return m.activateErr
}

func (m *mockStartRuntime) ActivateTab(tabID int) error {
	return nil
}

func installMockStartRuntime(t *testing.T, runtime *mockStartRuntime) {
	t.Helper()

	oldFactory := newWezTermRuntime
	newWezTermRuntime = func(string) wezTermRuntime {
		return runtime
	}
	t.Cleanup(func() {
		newWezTermRuntime = oldFactory
	})
}

var _ wezTermRuntime = (*mockStartRuntime)(nil)
