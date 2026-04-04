package lite

import (
	"reflect"
	"strings"
	"testing"

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
		},
		splitPaneID: 999,
	}
	installMockStartRuntime(t, runtime)

	if err := Start(StartOptions{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if len(runtime.moveCalls) != 0 {
		t.Fatalf("MovePaneToNewTab() calls = %v, want none", runtime.moveCalls)
	}
	if len(runtime.splitCalls) != 1 {
		t.Fatalf("SplitPane() call count = %d, want 1", len(runtime.splitCalls))
	}

	got := runtime.splitCalls[0]
	if got.PaneID != 101 {
		t.Fatalf("SplitPane() PaneID = %d, want 101", got.PaneID)
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

	if len(got.Command) != 4 {
		t.Fatalf("SplitPane() Command = %v, want 4 args", got.Command)
	}
	if !reflect.DeepEqual(got.Command[:3], []string{"atria-lite", "monitor", "--context-base64"}) {
		t.Fatalf("SplitPane() Command prefix = %v, want atria-lite monitor --context-base64", got.Command[:3])
	}

	ctx, err := DecodeMonitorContext(got.Command[3])
	if err != nil {
		t.Fatalf("DecodeMonitorContext() error = %v", err)
	}
	if err := ctx.Validate(); err != nil {
		t.Fatalf("monitor context Validate() error = %v", err)
	}

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 201, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 202, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(ctx.SlotBindings, wantBindings) {
		t.Fatalf("SlotBindings = %#v, want %#v", ctx.SlotBindings, wantBindings)
	}
	if !reflect.DeepEqual(ctx.WorkspacePaneIDs, []int{201, 202}) {
		t.Fatalf("WorkspacePaneIDs = %v, want [201 202]", ctx.WorkspacePaneIDs)
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
		t.Fatalf("SelfPaneID = %d, want 0 until monitor starts", ctx.SelfPaneID)
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
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
				{PaneID: 201, WindowID: 700, TabID: 701, Title: "claude code"},
				{PaneID: 202, WindowID: 700, TabID: 701, Title: "codex"},
				{PaneID: 203, WindowID: 700, TabID: 701, Title: "opencode"},
				{PaneID: 205, WindowID: 700, TabID: 703, Title: "shell"},
			},
			{
				{PaneID: 101, WindowID: 700, TabID: 701, Title: "shell"},
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
		{PaneID: 204, WindowID: 700},
		{PaneID: 205, WindowID: 700},
	}
	if !reflect.DeepEqual(runtime.moveCalls, wantMoves) {
		t.Fatalf("MovePaneToNewTab() calls = %#v, want %#v", runtime.moveCalls, wantMoves)
	}
	if len(runtime.splitCalls) != 1 {
		t.Fatalf("SplitPane() call count = %d, want 1", len(runtime.splitCalls))
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
}

type mockStartRuntime struct {
	panes           []wezterm.PaneInfo
	listPanesErr    error
	windowPanes     [][]wezterm.PaneInfo
	listWindowErr   error
	listWindowCalls int
	moveCalls       []movePaneCall
	moveErr         error
	splitCalls      []wezterm.SplitPaneOptions
	splitPaneID     int
	splitErr        error
}

type movePaneCall struct {
	PaneID   int
	WindowID int
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

func (m *mockStartRuntime) SplitPane(opts wezterm.SplitPaneOptions) (int, error) {
	m.splitCalls = append(m.splitCalls, opts)
	if m.splitErr != nil {
		return 0, m.splitErr
	}
	if m.splitPaneID == 0 {
		m.splitPaneID = 999
	}
	return m.splitPaneID, nil
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
