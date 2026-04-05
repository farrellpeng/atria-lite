package lite

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/sethdeckard/atria/internal/codex"
)

func TestFormatQuotaSuffix(t *testing.T) {
	qi := &codex.QuotaInfo{
		PrimaryPct:     42.5,
		PrimaryReset:   "2h10m",
		SecondaryPct:   18.0,
		SecondaryReset: "4d12h",
	}

	text, _ := formatQuotaSuffix(qi, 50)
	if text == "" {
		t.Error("text is empty")
	}
	t.Logf("suffix: %q", text)
}

func TestFormatQuotaSuffix_TooNarrowForPrimary(t *testing.T) {
	qi := &codex.QuotaInfo{PrimaryPct: 42.5, PrimaryReset: "2h10m"}
	text, _ := formatQuotaSuffix(qi, 3)
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
}

func TestFormatQuotaSuffix_DropsSecondary(t *testing.T) {
	qi := &codex.QuotaInfo{
		PrimaryPct:   42.5,
		PrimaryReset: "2h10m",
		SecondaryPct: 18.0,
	}
	text, _ := formatQuotaSuffix(qi, 15)
	if text == "" {
		t.Error("text is empty")
	}
}

func TestRenderStatusCell(t *testing.T) {
	// Unselected, no quota
	result := renderStatusCell("● idle", lipgloss.NewStyle(), "", lipgloss.NewStyle(), false, 20)
	if result == "" {
		t.Error("result is empty")
	}

	// Selected, with quota
	result = renderStatusCell("● idle", lipgloss.NewStyle(), " · 42% (2h)", lipgloss.NewStyle(), true, 30)
	if result == "" {
		t.Error("result is empty")
	}

	// Truncation
	result = renderStatusCell("● idle", lipgloss.NewStyle(), " · 42% (2h)", lipgloss.NewStyle(), false, 10)
	if result == "" {
		t.Error("result is empty after truncation")
	}
	if lipgloss.Width(result) != 10 {
		t.Errorf("width = %d, want 10", lipgloss.Width(result))
	}
}
