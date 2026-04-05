package lite

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sethdeckard/atria/internal/model"
	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

func TestRefreshUsesDetectAgentThenScreenFallback(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "Claude Code"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "shell"},
		},
		readScreens: map[int]string{
			12: "OpenAI Codex\n›",
		},
		getVars: map[int]string{
			11: "/projects/alpha",
			12: "/projects/bravo",
		},
	}
	m := NewModel(client, ctx)

	msg := runCmd(t, m.Init())
	updated, _ := m.Update(msg)
	got := updated.(Model)

	if len(client.readScreenCalls) != 1 || client.readScreenCalls[0] != "12" {
		t.Fatalf("ReadScreen() calls = %#v, want fallback for pane 12 only", client.readScreenCalls)
	}
	if len(got.panes) != 2 {
		t.Fatalf("panes len = %d, want 2", len(got.panes))
	}
	if got.panes[0].AgentType != model.AgentClaude {
		t.Fatalf("first pane agent = %q, want claude", got.panes[0].AgentType)
	}
	if got.panes[1].AgentType != model.AgentCodex {
		t.Fatalf("second pane agent = %q, want codex from screen fallback", got.panes[1].AgentType)
	}
	if got.panes[1].Kind != OccupantAgent {
		t.Fatalf("second pane kind = %q, want agent", got.panes[1].Kind)
	}
}

func TestRefreshBuildsDisplayRowsFromDiscoveredCWD(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "Claude Code"},
		},
		getVars: map[int]string{
			11: "/projects/alpha",
		},
	}
	m := NewModel(client, ctx)

	msg := runCmd(t, m.Init())
	updated, _ := m.Update(msg)
	got := updated.(Model)

	if !strings.Contains(got.View(), "/projects/alpha") {
		t.Fatalf("View() = %q, want discovered CWD to appear in display rows", got.View())
	}
}

func TestMonitorViewUsesAtriaStyleChromeAndSecondarySlots(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		},
	}
	m := NewModel(nil, ctx)

	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "shell"},
		},
	})
	got := updated.(Model)

	view := got.View()
	for _, want := range []string{"agents", "atria", "slot1", "enter:load", "n:normal panes", "r:refresh"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want to contain %q", view, want)
		}
	}
}

func TestInitSchedulesRefreshAndAutoRefreshTick(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "shell"},
		},
	}
	m := NewModel(client, ctx)

	msg := runCmd(t, m.Init())
	updated, cmd := m.Update(msg)
	m = updated.(Model)

	if len(m.panes) != 1 || m.panes[0].PaneID != 11 {
		t.Fatalf("panes = %#v, want pane 11 loaded", m.panes)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want scheduled auto-refresh tick")
	}
	tickMsg := runCmd(t, cmd)
	if _, ok := tickMsg.(refreshTickMsg); !ok {
		t.Fatalf("tick msg = %#v, want refreshTickMsg", tickMsg)
	}
}

func TestRefreshTickTriggersWindowRefreshAndReschedulesTick(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "shell"},
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(refreshTickMsg{})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want refresh command")
	}

	msg := runCmd(t, cmd)
	updated, next := m.Update(msg)
	m = updated.(Model)

	if len(m.panes) != 1 || m.panes[0].PaneID != 11 {
		t.Fatalf("panes = %#v, want pane 11 after refresh tick", m.panes)
	}
	if next == nil {
		t.Fatal("next cmd = nil, want next auto-refresh tick")
	}
	nextMsg := runCmd(t, next)
	if _, ok := nextMsg.(refreshTickMsg); !ok {
		t.Fatalf("next tick msg = %#v, want refreshTickMsg", nextMsg)
	}
}

func TestMonitorAutoLoadsDiscoveredAgentIntoNextFreeSlot(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		},
	}
	m := NewModel(nil, ctx)

	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 22, WindowID: 7, TabID: 70, Title: "claude"},
		},
	})
	got := updated.(Model)

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 22, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if !reflect.DeepEqual(got.ctx.SlotBindings, wantBindings) {
		t.Fatalf("ctx bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.ctx.SlotBindings)
	}
	if got.mode != ModeList {
		t.Fatalf("mode = %v, want %v", got.mode, ModeList)
	}
}

func TestMonitorRefreshDoesNotAutoReplaceWhenSlotsAreFull(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
			{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
		},
	}
	m := NewModel(nil, ctx)

	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "claude"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "opencode"},
			{PaneID: 99, WindowID: 7, TabID: 70, Title: "claude"},
		},
	})
	got := updated.(Model)

	if !reflect.DeepEqual(got.bindings, ctx.SlotBindings) {
		t.Fatalf("bindings changed on full auto-refresh\nwant: %#v\ngot:  %#v", ctx.SlotBindings, got.bindings)
	}
	if got.mode != ModeList {
		t.Fatalf("mode = %v, want %v", got.mode, ModeList)
	}
	if got.replacePane != (CandidatePane{}) {
		t.Fatalf("replacePane = %#v, want empty on passive refresh", got.replacePane)
	}
}

func TestMonitorAutoLoadMovesNewTabPaneIntoWorkspaceSlot(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:       200,
		StarterPaneID:    100,
		WindowID:         7,
		TabID:            70,
		SlotBindings:     []SlotBinding{{Slot: Slot1, PaneID: 11, Kind: OccupantAgent}},
		WorkspacePaneIDs: []int{11},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 22, WindowID: 7, TabID: 71, Title: "claude"},
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(windowPanesLoadedMsg{panes: client.panes})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want workspace materialization command")
	}

	msg := runCmd(t, cmd)
	updated, redraw := m.Update(msg)
	got := updated.(Model)
	if redraw == nil {
		t.Fatal("redraw cmd = nil, want forced UI redraw after workspace changes")
	}

	wantCalls := []wezterm.SplitPaneOptions{
		{PaneID: 200, Direction: "bottom", TopLevel: true, Percent: 65, MovePaneID: 11},
		{PaneID: 11, Direction: "right", Percent: 50, MovePaneID: 22},
	}
	if !reflect.DeepEqual(client.splitPaneCalls, wantCalls) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, wantCalls)
	}

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 22, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if !reflect.DeepEqual(got.ctx.WorkspacePaneIDs, []int{11, 22}) {
		t.Fatalf("workspace pane ids = %v, want [11 22]", got.ctx.WorkspacePaneIDs)
	}
	if !reflect.DeepEqual(client.activateTabCalls, []int{70, 70}) {
		t.Fatalf("ActivateTab() calls = %#v, want current tab restored before and after move", client.activateTabCalls)
	}
	if !reflect.DeepEqual(client.activatePaneCalls, []string{"200", "200"}) {
		t.Fatalf("ActivatePane() calls = %#v, want monitor pane focused before and after move", client.activatePaneCalls)
	}
}

func TestMonitorReanchorsWorkspaceBeforeAddingSecondSlot(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:       200,
		StarterPaneID:    100,
		WindowID:         7,
		TabID:            70,
		SlotBindings:     []SlotBinding{{Slot: Slot1, PaneID: 11, Kind: OccupantAgent}},
		WorkspacePaneIDs: []int{11},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 22, WindowID: 7, TabID: 71, Title: "claude"},
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(windowPanesLoadedMsg{panes: client.panes})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want workspace materialization command")
	}

	msg := runCmd(t, cmd)
	updated, _ = m.Update(msg)
	got := updated.(Model)

	wantSplits := []wezterm.SplitPaneOptions{
		{PaneID: 200, Direction: "bottom", TopLevel: true, Percent: 65, MovePaneID: 11},
		{PaneID: 11, Direction: "right", Percent: 50, MovePaneID: 22},
	}
	if !reflect.DeepEqual(client.splitPaneCalls, wantSplits) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, wantSplits)
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 22, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
}

func TestMonitorBootstrapsStarterPaneIntoWorkspaceBeforeAutoLoad(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 100, WindowID: 7, TabID: 70, Title: "shell"},
			{PaneID: 22, WindowID: 7, TabID: 71, Title: "claude"},
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(windowPanesLoadedMsg{panes: client.panes})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want workspace materialization command")
	}

	msg := runCmd(t, cmd)
	updated, _ = m.Update(msg)
	got := updated.(Model)

	wantCalls := []wezterm.SplitPaneOptions{
		{PaneID: 200, Direction: "bottom", TopLevel: true, Percent: 65, MovePaneID: 22},
		{PaneID: 22, Direction: "right", Percent: 50, MovePaneID: 100},
	}
	if !reflect.DeepEqual(client.splitPaneCalls, wantCalls) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, wantCalls)
	}

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 22, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 100, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if !reflect.DeepEqual(got.ctx.WorkspacePaneIDs, []int{22, 100}) {
		t.Fatalf("workspace pane ids = %v, want [22 100]", got.ctx.WorkspacePaneIDs)
	}
}

func TestMonitorKeepsRecentlyLoadedPaneDuringTransientMissingRefresh(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:       200,
		StarterPaneID:    100,
		WindowID:         7,
		TabID:            70,
		SlotBindings:     []SlotBinding{{Slot: Slot1, PaneID: 11, Kind: OccupantAgent}},
		WorkspacePaneIDs: []int{11},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 22, WindowID: 7, TabID: 71, Title: "claude"},
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(windowPanesLoadedMsg{panes: client.panes})
	m = updated.(Model)
	msg := runCmd(t, cmd)
	updated, _ = m.Update(msg)
	m = updated.(Model)

	client.panes = []wezterm.PaneInfo{
		{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
	}

	updated, retryCmd := m.Update(windowPanesLoadedMsg{panes: client.panes})
	got := updated.(Model)

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 22, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if retryCmd == nil {
		t.Fatal("retry cmd = nil, want workspace recovery while moved pane is transiently missing")
	}
}

func TestMonitorRestoresBoundPaneThatRemainsOnDifferentTab(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 49, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 47, Kind: OccupantAgent},
		},
		WorkspacePaneIDs: []int{49, 47},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 49, WindowID: 7, TabID: 71, Title: "claude"},
			{PaneID: 47, WindowID: 7, TabID: 70, Title: "codex"},
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(windowPanesLoadedMsg{panes: client.panes})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want workspace recovery when a bound pane remains on another tab")
	}

	msg := runCmd(t, cmd)
	updated, _ = m.Update(msg)
	got := updated.(Model)

	wantSplits := []wezterm.SplitPaneOptions{
		{PaneID: 200, Direction: "bottom", TopLevel: true, Percent: 65, MovePaneID: 49},
		{PaneID: 49, Direction: "right", Percent: 50, MovePaneID: 47},
	}
	if !reflect.DeepEqual(client.splitPaneCalls, wantSplits) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, wantSplits)
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 49, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 47, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
}

func TestMonitorAutoLoadRecreatesWorkspaceFromMonitorWhenAllSlotsClosed(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 22, WindowID: 7, TabID: 71, Title: "codex"},
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(windowPanesLoadedMsg{panes: client.panes})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want workspace recreation command when a new agent appears after all slots close")
	}

	msg := runCmd(t, cmd)
	updated, redraw := m.Update(msg)
	got := updated.(Model)
	if redraw == nil {
		t.Fatal("redraw cmd = nil, want forced redraw after recreating workspace")
	}

	wantSplit := wezterm.SplitPaneOptions{
		PaneID:     200,
		Direction:  "bottom",
		TopLevel:   true,
		Percent:    65,
		MovePaneID: 22,
	}
	if len(client.splitPaneCalls) != 1 || !reflect.DeepEqual(client.splitPaneCalls[0], wantSplit) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, []wezterm.SplitPaneOptions{wantSplit})
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 22, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if !reflect.DeepEqual(got.ctx.WorkspacePaneIDs, []int{22}) {
		t.Fatalf("workspace pane ids = %v, want [22]", got.ctx.WorkspacePaneIDs)
	}
	if !reflect.DeepEqual(client.activateTabCalls, []int{70, 70}) {
		t.Fatalf("ActivateTab() calls = %#v, want current tab restored before and after workspace recreation", client.activateTabCalls)
	}
	if !reflect.DeepEqual(client.activatePaneCalls, []string{"200", "200"}) {
		t.Fatalf("ActivatePane() calls = %#v, want monitor pane focused before and after workspace recreation", client.activatePaneCalls)
	}
}

func TestMonitorRestoresSingleBoundPaneThatRemainsOnDifferentTab(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:       200,
		StarterPaneID:    100,
		WindowID:         7,
		TabID:            70,
		SlotBindings:     []SlotBinding{{Slot: Slot1, PaneID: 22, Kind: OccupantAgent}},
		WorkspacePaneIDs: []int{22},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 22, WindowID: 7, TabID: 71, Title: "codex"},
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(windowPanesLoadedMsg{panes: client.panes})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want workspace recreation when the only bound pane remains on another tab")
	}

	msg := runCmd(t, cmd)
	updated, _ = m.Update(msg)
	got := updated.(Model)

	wantSplit := wezterm.SplitPaneOptions{
		PaneID:     200,
		Direction:  "bottom",
		TopLevel:   true,
		Percent:    65,
		MovePaneID: 22,
	}
	if len(client.splitPaneCalls) != 1 || !reflect.DeepEqual(client.splitPaneCalls[0], wantSplit) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, []wezterm.SplitPaneOptions{wantSplit})
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 22, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
}

func TestSelectingNormalPaneFromPickerLoadsIntoRightmostSlot(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		},
	}
	m := NewModel(nil, ctx)
	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "Claude Code"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "shell"},
		},
	})
	m = updated.(Model)

	updated, _ = m.Update(keyMsg("n"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("enter"))
	got := updated.(Model)

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 12, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if got.mode != ModeList {
		t.Fatalf("mode = %v, want %v after loading normal pane", got.mode, ModeList)
	}
}

func TestReplacingSlotMovesOldPaneToNewTab(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
			{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
		},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "Claude Code"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "OpenAI Codex"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "OC | old (opencode)"},
			{PaneID: 99, WindowID: 7, TabID: 70, Title: "Claude Code"},
		},
	}
	m := NewModel(client, ctx)
	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: client.panes,
	})
	m = updated.(Model)

	for i := 0; i < 3; i++ {
		updated, _ = m.Update(keyMsg("j"))
		m = updated.(Model)
	}

	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.mode != ModeReplacePrompt {
		t.Fatalf("mode = %v, want %v before choosing replacement", m.mode, ModeReplacePrompt)
	}

	updated, cmd := m.Update(keyMsg("3"))
	m = updated.(Model)
	msg := runCmd(t, cmd)
	updated, _ = m.Update(msg)
	got := updated.(Model)

	if len(client.movePaneCalls) != 1 {
		t.Fatalf("MovePaneToNewTab() calls = %#v, want 1 call", client.movePaneCalls)
	}
	if client.movePaneCalls[0].PaneID != 13 || client.movePaneCalls[0].WindowID != 7 {
		t.Fatalf("MovePaneToNewTab() call = %#v, want pane 13 in window 7", client.movePaneCalls[0])
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
		{Slot: Slot3, PaneID: 99, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if got.mode != ModeList {
		t.Fatalf("mode = %v, want %v after replacement", got.mode, ModeList)
	}
}

func TestSelectingNormalPaneWithThreeAgentsOnlyAllowsReplacingSlot3(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
			{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
		},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "Claude Code"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "OpenAI Codex"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "OC | old (opencode)"},
			{PaneID: 21, WindowID: 7, TabID: 70, Title: "shell"},
		},
	}
	m := NewModel(client, ctx)
	updated, _ := m.Update(windowPanesLoadedMsg{panes: client.panes})
	m = updated.(Model)

	updated, _ = m.Update(keyMsg("n"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.mode != ModeReplacePrompt {
		t.Fatalf("mode = %v, want %v before choosing replacement", m.mode, ModeReplacePrompt)
	}

	updated, cmd := m.Update(keyMsg("1"))
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil when normal pane tries to replace slot1", cmd)
	}
	if len(client.movePaneCalls) != 0 {
		t.Fatalf("MovePaneToNewTab() calls = %#v, want none", client.movePaneCalls)
	}

	updated, cmd = m.Update(keyMsg("3"))
	m = updated.(Model)
	msg := runCmd(t, cmd)
	updated, _ = m.Update(msg)
	got := updated.(Model)

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
		{Slot: Slot3, PaneID: 21, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if len(client.movePaneCalls) != 1 || client.movePaneCalls[0].PaneID != 13 {
		t.Fatalf("MovePaneToNewTab() calls = %#v, want replaced slot3 pane moved out", client.movePaneCalls)
	}
}

func TestMonitorFiltersToWindowAndExcludesSelf(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
	}
	m := NewModel(nil, ctx)

	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 101, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 102, WindowID: 7, TabID: 70, Title: "shell"},
			{PaneID: 200, WindowID: 7, TabID: 70, Title: "monitor"},
			{PaneID: 301, WindowID: 9, TabID: 90, Title: "claude"},
		},
	})
	got := updated.(Model)

	wantPanes := []CandidatePane{
		{PaneID: 101, WindowID: 7, TabID: 70, Title: "codex", Kind: OccupantAgent, AgentType: model.AgentCodex},
		{PaneID: 102, WindowID: 7, TabID: 70, Title: "shell", Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.panes, wantPanes) {
		t.Fatalf("panes mismatch\nwant: %#v\ngot:  %#v", wantPanes, got.panes)
	}

	view := got.View()
	if !strings.Contains(view, "codex") {
		t.Fatalf("View() = %q, want to contain agent row", view)
	}
	for _, want := range []string{"agents", "atria", "slot1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want to contain %q", view, want)
		}
	}
	if strings.Contains(view, "shell") {
		t.Fatalf("View() = %q, should not show normal panes in agent list", view)
	}
	if strings.Contains(view, "monitor") || strings.Contains(view, "claude") {
		t.Fatalf("View() = %q, should exclude self and other-window panes", view)
	}
}

func TestMonitorShowsReplacePromptWhenThreeSlotsFull(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
			{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
		},
	}
	m := NewModel(nil, ctx)
	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "claude"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "opencode"},
			{PaneID: 99, WindowID: 7, TabID: 70, Title: "claude"},
		},
	})
	m = updated.(Model)

	for i := 0; i < 3; i++ {
		updated, _ = m.Update(keyMsg("j"))
		m = updated.(Model)
	}

	updated, _ = m.Update(keyMsg("enter"))
	got := updated.(Model)

	if got.mode != ModeReplacePrompt {
		t.Fatalf("mode = %v, want %v", got.mode, ModeReplacePrompt)
	}
	if got.replacePane.PaneID != 99 {
		t.Fatalf("replacePane = %#v, want pane 99", got.replacePane)
	}
	if !reflect.DeepEqual(got.bindings, ctx.SlotBindings) {
		t.Fatalf("bindings changed during replace prompt\nwant: %#v\ngot:  %#v", ctx.SlotBindings, got.bindings)
	}
	view := got.View()
	if !strings.Contains(view, "Replace") {
		t.Fatalf("View() = %q, want replace prompt", got.View())
	}
	for _, want := range []string{"atria", "esc:back", "r:refresh"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want to contain %q", view, want)
		}
	}
	if strings.Contains(view, "enter:load") {
		t.Fatalf("View() = %q, replace prompt footer should not show list-mode actions", view)
	}
}

func TestMonitorShowsNormalPanePickerOnlyForNonAgents(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
	}
	m := NewModel(nil, ctx)
	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "shell"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "claude"},
		},
	})
	m = updated.(Model)

	updated, _ = m.Update(keyMsg("n"))
	got := updated.(Model)

	if got.mode != ModeNormalPanePicker {
		t.Fatalf("mode = %v, want %v", got.mode, ModeNormalPanePicker)
	}

	want := []CandidatePane{
		{PaneID: 12, WindowID: 7, TabID: 70, Title: "shell", Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.normalPanes(), want) {
		t.Fatalf("normal panes mismatch\nwant: %#v\ngot:  %#v", want, got.normalPanes())
	}

	view := got.View()
	if !strings.Contains(view, "shell") {
		t.Fatalf("View() = %q, want normal pane picker entry", view)
	}
	for _, want := range []string{"agents", "atria", "enter:load", "esc:back", "r:refresh"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want to contain %q", view, want)
		}
	}
	if strings.Contains(view, "codex") || strings.Contains(view, "claude") {
		t.Fatalf("View() = %q, should only list normal panes in picker", view)
	}
	if strings.Contains(view, "n:normal panes") {
		t.Fatalf("View() = %q, normal picker footer should not show list-mode actions", view)
	}
}

func TestMonitorShrinksBindingsWhenPaneDisappears(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 22, Kind: OccupantAgent},
			{Slot: Slot3, PaneID: 33, Kind: OccupantNormal},
		},
	}
	m := NewModel(nil, ctx)

	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 22, WindowID: 7, TabID: 70, Title: "claude"},
			{PaneID: 33, WindowID: 7, TabID: 70, Title: "shell"},
		},
	})
	got := updated.(Model)

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 22, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 33, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if !reflect.DeepEqual(got.ctx.SlotBindings, wantBindings) {
		t.Fatalf("ctx bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.ctx.SlotBindings)
	}
}

func TestMonitorRefreshSuccessOverridesFailureStatus(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
	}
	client := &stubWindowPaneClient{err: errors.New("boom")}
	m := NewModel(client, ctx)

	msg := runCmd(t, m.Init())
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !strings.Contains(m.statusText, "Refresh failed: boom") {
		t.Fatalf("statusText = %q, want refresh failure", m.statusText)
	}

	client.err = nil
	client.panes = []wezterm.PaneInfo{
		{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
		{PaneID: 12, WindowID: 7, TabID: 70, Title: "shell"},
	}

	updated, cmd := m.Update(keyMsg("r"))
	m = updated.(Model)
	msg = runCmd(t, cmd)
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if got, want := m.statusText, "1 agent pane(s) visible"; got != want {
		t.Fatalf("statusText = %q, want %q", got, want)
	}
	if strings.Contains(m.statusText, "Refresh failed") {
		t.Fatalf("statusText = %q, should not retain previous refresh error", m.statusText)
	}
}

func TestMonitorReclassifiesBindingsFromLivePanes(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 22, Kind: OccupantNormal},
		},
	}
	m := NewModel(nil, ctx)

	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "shell"},
			{PaneID: 22, WindowID: 7, TabID: 70, Title: "claude"},
		},
	})
	got := updated.(Model)

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 22, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 11, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}

	view := got.View()
	for _, want := range []string{"slot1", "22", "agent", "slot2", "11", "normal"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want reclassified slot summary to contain %q", view, want)
		}
	}
}

func TestMonitorReturnsToListWhenReplaceCandidateDisappears(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
			{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
		},
	}
	m := NewModel(nil, ctx)

	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "claude"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "opencode"},
			{PaneID: 99, WindowID: 7, TabID: 70, Title: "claude"},
		},
	})
	m = updated.(Model)
	for i := 0; i < 3; i++ {
		updated, _ = m.Update(keyMsg("j"))
		m = updated.(Model)
	}
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.mode != ModeReplacePrompt {
		t.Fatalf("mode = %v, want %v before refresh", m.mode, ModeReplacePrompt)
	}

	updated, _ = m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "claude"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "opencode"},
		},
	})
	got := updated.(Model)

	if got.mode != ModeList {
		t.Fatalf("mode = %v, want %v after candidate disappears", got.mode, ModeList)
	}
	if got.replacePane.PaneID != 0 {
		t.Fatalf("replacePane = %#v, want cleared replace candidate", got.replacePane)
	}
	if got.statusText != "3 agent pane(s) visible" {
		t.Fatalf("statusText = %q, want list refresh status", got.statusText)
	}
	if strings.Contains(got.View(), "Replace Prompt") {
		t.Fatalf("View() = %q, should leave replace prompt after candidate disappears", got.View())
	}
}

func TestMonitorReturnsToListWhenReplaceCandidateReclassifiesToNormal(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
			{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
		},
	}
	m := NewModel(nil, ctx)

	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "claude"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "opencode"},
			{PaneID: 99, WindowID: 7, TabID: 70, Title: "claude"},
		},
	})
	m = updated.(Model)
	for i := 0; i < 3; i++ {
		updated, _ = m.Update(keyMsg("j"))
		m = updated.(Model)
	}
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(Model)
	if m.mode != ModeReplacePrompt {
		t.Fatalf("mode = %v, want %v before refresh", m.mode, ModeReplacePrompt)
	}

	updated, _ = m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "claude"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "opencode"},
			{PaneID: 99, WindowID: 7, TabID: 70, Title: "shell"},
		},
	})
	got := updated.(Model)

	if got.mode != ModeList {
		t.Fatalf("mode = %v, want %v after candidate reclassifies to normal", got.mode, ModeList)
	}
	if got.replacePane.PaneID != 0 {
		t.Fatalf("replacePane = %#v, want cleared replace candidate", got.replacePane)
	}
	if got.statusText != "3 agent pane(s) visible" {
		t.Fatalf("statusText = %q, want list refresh status", got.statusText)
	}
	if strings.Contains(got.View(), "Replace Prompt") {
		t.Fatalf("View() = %q, should leave replace prompt after candidate reclassifies", got.View())
	}
}

func keyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func TestPaneLabelUsesAgentNameForGenericProjectTitle(t *testing.T) {
	pane := CandidatePane{
		PaneID:    12,
		Title:     "atria",
		CWD:       "/home/farrell/project/atria/",
		Kind:      OccupantAgent,
		AgentType: model.AgentCodex,
	}

	if got, want := paneLabel(pane), "Codex"; got != want {
		t.Fatalf("paneLabel() = %q, want %q", got, want)
	}
}

func TestPaneLabelKeepsNonGenericActivityTitle(t *testing.T) {
	pane := CandidatePane{
		PaneID:    14,
		Title:     "Reviewing slot sync logic",
		CWD:       "/home/farrell/project/atria/",
		Kind:      OccupantAgent,
		AgentType: model.AgentClaude,
	}

	if got, want := paneLabel(pane), "Reviewing slot sync logic"; got != want {
		t.Fatalf("paneLabel() = %q, want %q", got, want)
	}
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("cmd = nil, want refresh command")
	}
	return cmd()
}

type stubWindowPaneClient struct {
	panes             []wezterm.PaneInfo
	paneSequence      [][]wezterm.PaneInfo
	listWindowCalls   int
	err               error
	readScreens       map[int]string
	getVars           map[int]string
	readScreenCalls   []string
	movePaneCalls     []movePaneCall
	splitPaneCalls    []wezterm.SplitPaneOptions
	adjustPaneCalls   []adjustPaneCall
	activateTabCalls  []int
	activatePaneCalls []string
}

func (s *stubWindowPaneClient) ListWindowPanes(windowID int) ([]wezterm.PaneInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.paneSequence) > 0 {
		idx := s.listWindowCalls
		if idx >= len(s.paneSequence) {
			idx = len(s.paneSequence) - 1
		}
		s.listWindowCalls++
		return append([]wezterm.PaneInfo(nil), s.paneSequence[idx]...), nil
	}
	s.listWindowCalls++
	return append([]wezterm.PaneInfo(nil), s.panes...), nil
}

func (s *stubWindowPaneClient) ReadScreen(sessionID string, lines int) (string, error) {
	s.readScreenCalls = append(s.readScreenCalls, sessionID)
	if s.readScreens == nil {
		return "", nil
	}
	paneID := 0
	_, err := fmt.Sscanf(sessionID, "%d", &paneID)
	if err != nil {
		return "", err
	}
	return s.readScreens[paneID], nil
}

func (s *stubWindowPaneClient) GetVar(sessionID, varName string) (string, error) {
	if varName != "path" {
		return "", nil
	}
	if s.getVars == nil {
		return "", nil
	}
	paneID := 0
	_, err := fmt.Sscanf(sessionID, "%d", &paneID)
	if err != nil {
		return "", err
	}
	return s.getVars[paneID], nil
}

func (s *stubWindowPaneClient) MovePaneToNewTab(paneID, windowID int) error {
	s.movePaneCalls = append(s.movePaneCalls, movePaneCall{PaneID: paneID, WindowID: windowID})
	return nil
}

func (s *stubWindowPaneClient) SplitPane(opts wezterm.SplitPaneOptions) (int, error) {
	s.splitPaneCalls = append(s.splitPaneCalls, opts)
	if opts.MovePaneID != 0 {
		return opts.MovePaneID, nil
	}
	return 999, nil
}

func (s *stubWindowPaneClient) AdjustPaneSize(paneID int, direction string, amount int) error {
	s.adjustPaneCalls = append(s.adjustPaneCalls, adjustPaneCall{
		PaneID:    paneID,
		Direction: direction,
		Amount:    amount,
	})
	return nil
}

func (s *stubWindowPaneClient) ActivateTab(tabID int) error {
	s.activateTabCalls = append(s.activateTabCalls, tabID)
	return nil
}

func (s *stubWindowPaneClient) ActivatePane(sessionID string) error {
	s.activatePaneCalls = append(s.activatePaneCalls, sessionID)
	return nil
}
