package lite

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/sethdeckard/atria/internal/codex"
	"github.com/sethdeckard/atria/internal/model"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestFormatUsageText(t *testing.T) {
	qi := &codex.QuotaInfo{
		PrimaryPct:     42.5,
		PrimaryReset:   "2h10m",
		SecondaryPct:   18.0,
		SecondaryReset: "4d12h",
	}

	text, _ := formatUsageText(qi, 50)
	if text == "" {
		t.Error("text is empty")
	}
	if !strings.Contains(text, "5h 42% (2h10m)") {
		t.Fatalf("text = %q, want labeled primary usage text", text)
	}
	if !strings.Contains(text, "7d 18% (4d12h)") {
		t.Fatalf("text = %q, want labeled secondary usage text", text)
	}
}

func TestFormatUsageText_TooNarrowForPrimary(t *testing.T) {
	qi := &codex.QuotaInfo{PrimaryPct: 42.5, PrimaryReset: "2h10m"}
	text, _ := formatUsageText(qi, 2)
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
}

func TestFormatUsageText_DropsSecondary(t *testing.T) {
	qi := &codex.QuotaInfo{
		PrimaryPct:   42.5,
		PrimaryReset: "2h10m",
		SecondaryPct: 18.0,
	}
	text, _ := formatUsageText(qi, 16)
	if text == "" {
		t.Error("text is empty")
	}
	if strings.Contains(text, " / ") {
		t.Fatalf("text = %q, want primary usage only when narrow", text)
	}
	if !strings.Contains(text, "5h 42%") {
		t.Fatalf("text = %q, want labeled primary usage", text)
	}
}

func TestFormatUsageText_OmitsEmptyResetParens(t *testing.T) {
	qi := &codex.QuotaInfo{PrimaryPct: 0}
	text, _ := formatUsageText(qi, 12)
	if text != "5h 0%" {
		t.Fatalf("text = %q, want 5h 0%%", text)
	}
}

func TestFormatUsageText_ShowsSecondaryEvenWhenZero(t *testing.T) {
	qi := &codex.QuotaInfo{
		PrimaryPct:   0,
		PrimaryReset: "2h10m",
		SecondaryPct: 0,
	}
	text, _ := formatUsageText(qi, 40)
	if !strings.Contains(text, "5h 0% (2h10m) / 7d 0%") {
		t.Fatalf("text = %q, want both 5h and 7d windows", text)
	}
}

func TestRenderStatusCell(t *testing.T) {
	// Unselected, no quota
	result := renderStatusCell("● idle", lipgloss.NewStyle(), false, 20)
	if result == "" {
		t.Error("result is empty")
	}

	// Selected
	result = renderStatusCell("● idle", lipgloss.NewStyle(), true, 30)
	if result == "" {
		t.Error("result is empty")
	}

	// Truncation
	result = renderStatusCell("● idle", lipgloss.NewStyle(), false, 10)
	if result == "" {
		t.Error("result is empty after truncation")
	}
	if lipgloss.Width(result) != 10 {
		t.Errorf("width = %d, want 10", lipgloss.Width(result))
	}
}

func TestRenderUsageCell(t *testing.T) {
	result := renderUsageCell("5h 42% (2h)", lipgloss.NewStyle(), false, 14)
	if result == "" {
		t.Error("result is empty")
	}
	if lipgloss.Width(result) != 14 {
		t.Errorf("width = %d, want 14", lipgloss.Width(result))
	}
}

func TestRenderColumnHeaders_IncludesUsage(t *testing.T) {
	m := Model{
		width: 120,
		panes: []CandidatePane{
			{PaneID: 15, Kind: OccupantAgent, AgentType: model.AgentCodex},
		},
	}
	headers := stripANSI(m.renderColumnHeaders())
	if !strings.Contains(headers, "usage") {
		t.Fatalf("headers = %q, want usage column", headers)
	}
}

func TestListWidth_UsesQuarterScreenForSlotsPanel(t *testing.T) {
	m := Model{width: 140}
	if got, want := m.listWidth(), 102; got != want {
		t.Fatalf("listWidth() = %d, want %d", got, want)
	}
}

func TestViewSlotSummary_UsesAlignedColumns(t *testing.T) {
	m := Model{
		width: 140,
		bindings: []SlotBinding{
			{Slot: Slot1, PaneID: 15, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 16, Kind: OccupantNormal},
		},
	}
	lines := m.viewSlotSummary()
	if len(lines) < 3 {
		t.Fatalf("lines = %q, want title and slot rows", lines)
	}
	if got := stripANSI(lines[0]); strings.TrimSpace(got) != "slots" {
		t.Fatalf("title = %q, want slots", got)
	}
	line := stripANSI(lines[1])
	if got, want := lipgloss.Width(line), m.slotPanelWidth(); got != want {
		t.Fatalf("slot line width = %d, want %d", got, want)
	}
	if !strings.Contains(line, "slot1") || !strings.Contains(line, "15") || !strings.Contains(line, "agent") {
		t.Fatalf("line = %q, want aligned slot content", line)
	}
}

func TestRenderPaneRow_CodexQuotaUsesUsageColumn(t *testing.T) {
	m := Model{
		width: 120,
		panes: []CandidatePane{
			{PaneID: 15, Kind: OccupantAgent, AgentType: model.AgentCodex},
		},
		codexQuota: &codex.QuotaInfo{
			PrimaryPct: 0,
		},
	}
	pane := CandidatePane{
		PaneID:    15,
		Title:     "codex",
		CWD:       "/home/farrell/project/atria",
		Kind:      OccupantAgent,
		AgentType: model.AgentCodex,
		Status:    model.StatusWorking,
	}

	row := stripANSI(m.renderPaneRow(pane, "slot1", false))
	if strings.Contains(row, "()") {
		t.Fatalf("row = %q, want no empty reset parens", row)
	}
	statusIdx := strings.Index(row, "working")
	usageIdx := strings.Index(row, "5h 0%")
	cwdIdx := strings.Index(row, "/home/farr")
	if statusIdx == -1 || usageIdx == -1 || cwdIdx == -1 {
		t.Fatalf("row = %q, want status, usage, and cwd text", row)
	}
	if !(statusIdx < usageIdx && usageIdx < cwdIdx) {
		t.Fatalf("row = %q, want usage between status and cwd", row)
	}
}
