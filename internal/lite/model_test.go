package lite

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sethdeckard/atria/internal/model"
	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

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
	if strings.Contains(view, "enter load agent") {
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
	if strings.Contains(view, "codex") || strings.Contains(view, "claude") {
		t.Fatalf("View() = %q, should only list normal panes in picker", view)
	}
	if strings.Contains(view, "enter load agent") {
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
	if !strings.Contains(view, "slot1: pane 22 (agent)") || !strings.Contains(view, "slot2: pane 11 (normal)") {
		t.Fatalf("View() = %q, want reclassified slot summary", view)
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

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("cmd = nil, want refresh command")
	}
	return cmd()
}

type stubWindowPaneClient struct {
	panes []wezterm.PaneInfo
	err   error
}

func (s *stubWindowPaneClient) ListWindowPanes(windowID int) ([]wezterm.PaneInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]wezterm.PaneInfo(nil), s.panes...), nil
}
