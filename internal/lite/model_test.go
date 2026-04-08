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
	"github.com/sethdeckard/atria/internal/tui"
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

	if !reflect.DeepEqual(client.readScreenCalls, []string{"11", "12"}) {
		t.Fatalf("ReadScreen() calls = %#v, want reads for agent status and screen fallback panes", client.readScreenCalls)
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

func TestRefreshIgnoresStaleAgentBrandingInShellScrollback(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "shell"},
		},
		readScreens: map[int]string{
			12: "OpenAI Codex\nmodel: gpt-5.4\nold output\n\nuser@host $ ",
		},
		getVars: map[int]string{
			12: "/projects/bravo",
		},
	}
	m := NewModel(client, ctx)

	msg := runCmd(t, m.Init())
	updated, _ := m.Update(msg)
	got := updated.(Model)

	if len(got.panes) != 1 {
		t.Fatalf("panes len = %d, want 1", len(got.panes))
	}
	if got.panes[0].Kind != OccupantNormal {
		t.Fatalf("pane kind = %q, want normal when only stale branding remains in scrollback", got.panes[0].Kind)
	}
	if got.panes[0].AgentType != "" {
		t.Fatalf("pane agent = %q, want empty for shell pane", got.panes[0].AgentType)
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

func TestRefreshBuildsDynamicStatusRowsFromScreen(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: ": atria", CWD: "/home/farrell/project/atria/"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "✳ Read project README file", CWD: "/home/farrell/project/atria/"},
		},
		readScreens: map[int]string{
			11: "OpenAI Codex\n• Working (3m 09s • esc to interrupt)\n› Improve documentation in @filename",
			12: "Claude Code v2.1.92\n? for shortcuts\n❯ /review",
		},
		getVars: map[int]string{
			11: "/home/farrell/project/atria/",
			12: "/home/farrell/project/atria/",
		},
	}
	m := NewModel(client, ctx)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	msg := runCmd(t, m.Init())
	updated, _ = m.Update(msg)
	got := updated.(Model)

	if got.panes[0].Status != model.StatusWorking {
		t.Fatalf("first pane status = %q, want working", got.panes[0].Status)
	}
	if got.panes[1].Status != model.StatusIdle {
		t.Fatalf("second pane status = %q, want idle", got.panes[1].Status)
	}
	if got.panes[1].Activity != "Read project README file" {
		t.Fatalf("second pane activity = %q, want extracted activity", got.panes[1].Activity)
	}

	view := got.View()
	for _, want := range []string{"status", "Codex", "working...", "Read project README file"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want to contain %q", view, want)
		}
	}
	if strings.Contains(view, ": atria") {
		t.Fatalf("View() = %q, should not keep decorated generic project title", view)
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
			{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
			{Slot: Slot3, PaneID: 13, Kind: OccupantNormal},
		},
	}
	m := NewModel(nil, ctx)
	m.width = 140

	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "Claude Code"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "shell"},
		},
	})
	got := updated.(Model)

	view := got.View()
	for _, want := range []string{"agents", "atria", "slots", "slot1", "slot2", "enter:load", "n:normal panes", "r:refresh"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want to contain %q", view, want)
		}
	}
	if strings.Contains(view, "slot3 pane 13") {
		t.Fatalf("View() = %q, should not show fixed slot3 in the slot summary", view)
	}
	if strings.Contains(view, "agent pane(s) visible") {
		t.Fatalf("View() = %q, should not show the visible-pane status summary in list mode", view)
	}
	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("View() = %q, want title bar plus content lines", view)
	}
	if strings.TrimSpace(lines[2]) == "" {
		t.Fatalf("View() = %q, should not leave a blank line between the title bar and monitor content", view)
	}
	helpIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "enter:load") {
			helpIndex = i
			break
		}
	}
	if helpIndex < 1 {
		t.Fatalf("View() = %q, want footer help to be present with a preceding divider", view)
	}
	if strings.Trim(strings.ReplaceAll(lines[helpIndex-1], "─", ""), " ") != "" {
		t.Fatalf("View() = %q, want a divider line immediately above the footer help", view)
	}
	if !strings.HasPrefix(lines[helpIndex], "  enter:load") {
		t.Fatalf("View() = %q, want footer help to be indented by two spaces", view)
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
	msgs := runCmds(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("msgs len = %d, want 1 discovery tick", len(msgs))
	}
	if _, ok := msgs[0].(refreshTickMsg); !ok {
		t.Fatalf("tick msg = %#v, want refreshTickMsg", msgs[0])
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
	msgs := runCmds(t, next)
	if len(msgs) != 1 {
		t.Fatalf("msgs len = %d, want 1 discovery tick", len(msgs))
	}
	if _, ok := msgs[0].(refreshTickMsg); !ok {
		t.Fatalf("next tick msg = %#v, want refreshTickMsg", msgs[0])
	}
}

func TestInitSchedulesStatusAndSpinnerTicksForWorkingAgent(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
		},
		readScreens: map[int]string{
			11: "OpenAI Codex\n• Working (30s • esc to interrupt)\n› Review changes",
		},
	}
	m := NewModel(client, ctx)

	msg := runCmd(t, m.Init())
	updated, cmd := m.Update(msg)
	m = updated.(Model)

	msgs := runCmds(t, cmd)
	assertMsgTypes(t, msgs, refreshTickMsg{}, statusTickMsg{}, spinnerTickMsg{}, codexQuotaMsg{})
	if !m.statusTickActive {
		t.Fatal("statusTickActive = false, want true after scheduling status polling")
	}
	if !m.spinnerTickActive {
		t.Fatal("spinnerTickActive = false, want true after scheduling spinner")
	}
}

func TestStatusTickRefreshesStatusesAndReschedulesStatusTick(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
	}
	client := &stubWindowPaneClient{
		readScreens: map[int]string{
			11: "OpenAI Codex\n• Working (30s • esc to interrupt)\n› Review changes",
		},
	}
	m := NewModel(client, ctx)
	m.panes = []CandidatePane{{
		PaneID:    11,
		WindowID:  7,
		TabID:     70,
		Title:     "codex",
		CWD:       "/home/farrell/project/atria/",
		Kind:      OccupantAgent,
		AgentType: model.AgentCodex,
		Status:    model.StatusIdle,
	}}

	updated, cmd := m.Update(statusTickMsg{})
	m = updated.(Model)
	if m.statusTickActive {
		t.Fatal("statusTickActive = true, want false while screen read command is in flight")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want status refresh command")
	}

	msg := runCmd(t, cmd)
	statusMsg, ok := msg.(paneStatusesLoadedMsg)
	if !ok {
		t.Fatalf("msg = %#v, want paneStatusesLoadedMsg", msg)
	}

	updated, next := m.Update(statusMsg)
	got := updated.(Model)
	if got.panes[0].Status != model.StatusWorking {
		t.Fatalf("pane status = %q, want working", got.panes[0].Status)
	}
	msgs := runCmds(t, next)
	assertMsgTypes(t, msgs, statusTickMsg{}, spinnerTickMsg{}, codexQuotaMsg{})
}

func TestStatusTickDemotesExitedAgentPaneToNormalAfterStableShellReads(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
	}
	client := &stubWindowPaneClient{
		readScreens: map[int]string{
			11: "user@host $ ",
		},
	}
	m := NewModel(client, ctx)
	m.panes = []CandidatePane{{
		PaneID:    11,
		WindowID:  7,
		TabID:     70,
		Title:     "codex",
		CWD:       "/home/farrell/project/atria/",
		Kind:      OccupantAgent,
		AgentType: model.AgentCodex,
		Status:    model.StatusWorking,
	}}

	for i := 0; i < 5; i++ {
		updated, cmd := m.Update(statusTickMsg{})
		m = updated.(Model)
		msg := runCmd(t, cmd)
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}

	if len(m.panes) != 1 {
		t.Fatalf("panes len = %d, want 1", len(m.panes))
	}
	if m.panes[0].Kind != OccupantNormal {
		t.Fatalf("pane kind = %q, want normal after repeated shell reads", m.panes[0].Kind)
	}
	if m.panes[0].AgentType != "" {
		t.Fatalf("pane agent = %q, want empty after agent exit", m.panes[0].AgentType)
	}
}

func TestSpinnerTickAdvancesWhileWorking(t *testing.T) {
	m := NewModel(nil, MonitorContext{SelfPaneID: 200, WindowID: 7, TabID: 70})
	m.panes = []CandidatePane{{
		PaneID:    11,
		Kind:      OccupantAgent,
		AgentType: model.AgentCodex,
		Status:    model.StatusWorking,
	}}
	m.spinnerTickActive = true

	updated, cmd := m.Update(spinnerTickMsg{})
	got := updated.(Model)

	if got.spinnerFrame != 1 {
		t.Fatalf("spinnerFrame = %d, want 1", got.spinnerFrame)
	}
	if !got.spinnerTickActive {
		t.Fatal("spinnerTickActive = false, want spinner to keep running")
	}
	msgs := runCmds(t, cmd)
	assertMsgTypes(t, msgs, spinnerTickMsg{})
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
	m.width = 140

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
	m.width = 140

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

func TestMonitorDoesNotAutoLoadNewTabPaneIntoWorkspaceSlot(t *testing.T) {
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
	got := updated.(Model)
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil when only a different-tab pane is newly discovered", cmd)
	}

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if !reflect.DeepEqual(got.ctx.WorkspacePaneIDs, []int{11}) {
		t.Fatalf("workspace pane ids = %v, want [11]", got.ctx.WorkspacePaneIDs)
	}
	if len(client.splitPaneCalls) != 0 {
		t.Fatalf("SplitPane() calls = %#v, want none", client.splitPaneCalls)
	}
}

func TestMonitorIgnoresNewTabPaneWhenReanchoringWorkspace(t *testing.T) {
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
	got := updated.(Model)
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil when only the new pane is on another tab", cmd)
	}

	if len(client.splitPaneCalls) != 0 {
		t.Fatalf("SplitPane() calls = %#v, want none", client.splitPaneCalls)
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
}

func TestMonitorBootstrapsStarterPaneWithoutAutoLoadingDifferentTabAgent(t *testing.T) {
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
	got := updated.(Model)
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil when discovered agent only exists on another tab", cmd)
	}

	if len(client.splitPaneCalls) != 0 {
		t.Fatalf("SplitPane() calls = %#v, want none", client.splitPaneCalls)
	}

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 100, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if !reflect.DeepEqual(got.ctx.WorkspacePaneIDs, []int{100}) {
		t.Fatalf("workspace pane ids = %v, want [100]", got.ctx.WorkspacePaneIDs)
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

func TestMonitorDoesNotRecreateWorkspaceFromDifferentTabAgentWhenAllSlotsClosed(t *testing.T) {
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
	got := updated.(Model)
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil when the only agent is on a different tab", cmd)
	}
	if len(client.splitPaneCalls) != 0 {
		t.Fatalf("SplitPane() calls = %#v, want none", client.splitPaneCalls)
	}
	if len(got.bindings) != 0 {
		t.Fatalf("bindings = %#v, want empty when no current-tab pane can seed the workspace", got.bindings)
	}
	if len(got.ctx.WorkspacePaneIDs) != 0 {
		t.Fatalf("workspace pane ids = %v, want empty", got.ctx.WorkspacePaneIDs)
	}
}

func TestMonitorDoesNotAutoLoadDifferentTabAgentAfterBoundAgentExits(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:    200,
		StarterPaneID: 100,
		WindowID:      7,
		TabID:         70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
		},
		WorkspacePaneIDs: []int{11, 12},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "claude"},
			{PaneID: 99, WindowID: 7, TabID: 71, Title: "codex"},
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(windowPanesLoadedMsg{panes: client.panes})
	got := updated.(Model)

	if cmd != nil {
		t.Fatalf("cmd = %v, want nil when replacement candidate only exists on another tab", cmd)
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if !reflect.DeepEqual(got.ctx.WorkspacePaneIDs, []int{11}) {
		t.Fatalf("workspace pane ids = %v, want [11]", got.ctx.WorkspacePaneIDs)
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

func TestSelectingNewNormalPaneAddsNextEmptySlot(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 12, Kind: OccupantNormal},
		},
	}
	m := NewModel(nil, ctx)
	updated, _ := m.Update(windowPanesLoadedMsg{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "Claude Code"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "shell"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "shell"},
		},
	})
	m = updated.(Model)

	updated, _ = m.Update(keyMsg("n"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("enter"))
	got := updated.(Model)

	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 12, Kind: OccupantNormal},
		{Slot: Slot3, PaneID: 13, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if got.mode != ModeList {
		t.Fatalf("mode = %v, want %v after loading the new normal pane", got.mode, ModeList)
	}
}

func TestSelectingNewNormalPaneReflowsWorkspaceToRightmostSlot(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID:       200,
		WindowID:         7,
		TabID:            70,
		SlotBindings:     []SlotBinding{{Slot: Slot1, PaneID: 11, Kind: OccupantAgent}, {Slot: Slot2, PaneID: 12, Kind: OccupantNormal}},
		WorkspacePaneIDs: []int{11, 12},
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "Claude Code"},
			{PaneID: 12, WindowID: 7, TabID: 70, Title: "shell"},
			{PaneID: 13, WindowID: 7, TabID: 70, Title: "shell"},
		},
	}
	m := NewModel(client, ctx)
	updated, _ := m.Update(windowPanesLoadedMsg{panes: client.panes})
	m = updated.(Model)

	updated, _ = m.Update(keyMsg("n"))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(Model)
	updated, cmd := m.Update(keyMsg("enter"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want workspace sync command")
	}
	msg := runCmd(t, cmd)
	updated, redraw := m.Update(msg)
	got := updated.(Model)
	if redraw == nil {
		t.Fatal("redraw cmd = nil, want forced redraw after loading new normal pane")
	}

	wantSplits := []wezterm.SplitPaneOptions{
		{PaneID: 11, Direction: "right", Percent: 67, MovePaneID: 12},
		{PaneID: 12, Direction: "right", Percent: 50, MovePaneID: 13},
	}
	if !reflect.DeepEqual(client.splitPaneCalls, wantSplits) {
		t.Fatalf("SplitPane() calls = %#v, want %#v", client.splitPaneCalls, wantSplits)
	}
	wantBindings := []SlotBinding{
		{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 12, Kind: OccupantNormal},
		{Slot: Slot3, PaneID: 13, Kind: OccupantNormal},
	}
	if !reflect.DeepEqual(got.bindings, wantBindings) {
		t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", wantBindings, got.bindings)
	}
	if !reflect.DeepEqual(got.ctx.WorkspacePaneIDs, []int{11, 12, 13}) {
		t.Fatalf("workspace pane ids = %v, want [11 12 13]", got.ctx.WorkspacePaneIDs)
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
	m.width = 140

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

func TestRefreshKeyAlsoFetchesCodexQuota(t *testing.T) {
	ctx := MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
	}
	client := &stubWindowPaneClient{
		panes: []wezterm.PaneInfo{
			{PaneID: 11, WindowID: 7, TabID: 70, Title: "codex"},
		},
		readScreens: map[int]string{
			11: "OpenAI Codex\n›",
		},
	}
	m := NewModel(client, ctx)

	updated, cmd := m.Update(keyMsg("r"))
	m = updated.(Model)

	msgs := runCmds(t, cmd)
	assertMsgTypes(t, msgs, candidatePanesLoadedMsg{}, codexQuotaMsg{})
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
	m.width = 140

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

func TestPaneLabelUsesAgentNameForDecoratedGenericProjectTitle(t *testing.T) {
	pane := CandidatePane{
		PaneID:    12,
		Title:     ": atria",
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

func TestSelectedAgentRowKeepsStatusColor(t *testing.T) {
	m := NewModel(nil, MonitorContext{SelfPaneID: 200, WindowID: 7, TabID: 70})
	m.width = 120
	pane := CandidatePane{
		PaneID:    11,
		CWD:       "/home/farrell/project/atria/",
		Kind:      OccupantAgent,
		AgentType: model.AgentCodex,
		Status:    model.StatusWorking,
	}

	row := m.renderPaneRow(pane, "slot1", true)
	_, _, _, statusWidth, _, _ := m.columnWidths()
	statusText, statusStyle := tui.FormatAgentStatus(pane.Status, pane.Activity, pane.Attention, m.spinnerFrame)
	want := renderStatusCell(statusText, statusStyle, true, statusWidth)

	if !strings.Contains(row, want) {
		t.Fatalf("row = %q, want selected status cell %q", row, want)
	}
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("cmd = nil, want refresh command")
	}
	return cmd()
}

func runCmds(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	msg := runCmd(t, cmd)
	if batch, ok := msg.(tea.BatchMsg); ok {
		out := make([]tea.Msg, 0, len(batch))
		for _, subcmd := range batch {
			if subcmd == nil {
				continue
			}
			out = append(out, subcmd())
		}
		return out
	}
	return []tea.Msg{msg}
}

func assertMsgTypes(t *testing.T, msgs []tea.Msg, wants ...tea.Msg) {
	t.Helper()
	if len(msgs) != len(wants) {
		t.Fatalf("msgs len = %d, want %d (%#v)", len(msgs), len(wants), msgs)
	}
	for _, want := range wants {
		wantType := reflect.TypeOf(want)
		found := false
		for _, msg := range msgs {
			if reflect.TypeOf(msg) == wantType {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("msgs = %#v, want type %v", msgs, wantType)
		}
	}
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
