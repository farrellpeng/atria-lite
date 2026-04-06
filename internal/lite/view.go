package lite

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sethdeckard/atria/internal/codex"
	"github.com/sethdeckard/atria/internal/model"
	"github.com/sethdeckard/atria/internal/tui"
)

func (m Model) View() string {
	title := strings.TrimSuffix(tui.RenderTitleBar("agents", m.renderWidth()), "\n")
	var body string

	switch m.mode {
	case ModeReplacePrompt:
		body = strings.Join(m.viewReplacePrompt(), "\n")
	case ModeNormalPanePicker:
		body = strings.Join(m.viewNormalPanePicker(), "\n")
	default:
		body = m.viewMonitorSplit()
	}
	return title + "\n" + body + "\n" + m.viewFooter()
}

func (m Model) viewMonitorSplit() string {
	left := strings.Join(m.viewAgentList(), "\n")
	right := strings.Join(m.viewSlotSummary(), "\n")
	if right == "" || m.renderWidth() < 110 {
		return left
	}

	rightWidth := m.slotPanelWidth()
	leftWidth := m.renderWidth() - rightWidth - 3
	if leftWidth < 60 {
		return left + "\n\n" + right
	}

	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	leftHeight := len(leftLines)
	rightHeight := len(rightLines)
	if rightHeight > leftHeight {
		leftHeight = rightHeight
	}

	leftStyle := lipgloss.NewStyle().Width(leftWidth)
	rightStyle := lipgloss.NewStyle().Width(rightWidth)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftStyle.Render(left),
		"   ",
		rightStyle.Render(right),
	)
}

func (m Model) viewAgentList() []string {
	agents := m.agentPanes()
	lines := []string{m.renderColumnHeaders()}
	if len(agents) == 0 {
		return append(lines, tui.RenderDim("  No agent panes in this window."))
	}

	for i, pane := range agents {
		binding := "unbound"
		if slot, ok := m.slotForPane(pane.PaneID); ok {
			binding = string(slot)
		}
		lines = append(lines, m.renderPaneRow(pane, binding, i == m.cursor))
	}
	return lines
}

func (m Model) viewReplacePrompt() []string {
	lines := []string{
		tui.RenderDim("  replace"),
		"  " + fmt.Sprintf("All 3 slots are full. %s would require a replacement.", paneLabel(m.replacePane)),
	}
	for _, slot := range allowedReplaceSlots(m.bindings, m.replacePane) {
		lines = append(lines, "  "+fmt.Sprintf("Press %s to replace %s.", strings.TrimPrefix(string(slot), "slot"), slot))
	}
	lines = append(lines, "  Press esc to go back.")
	return lines
}

func (m Model) viewNormalPanePicker() []string {
	normals := m.normalPanes()
	lines := []string{m.renderColumnHeaders()}
	if len(normals) == 0 {
		return append(lines, tui.RenderDim("  No normal panes available."))
	}

	for i, pane := range normals {
		lines = append(lines, m.renderPaneRow(pane, "unbound", i == m.cursor))
	}
	return lines
}

func (m Model) viewSlotSummary() []string {
	width := m.slotPanelWidth()
	slotWidth, paneWidth := slotSummaryColumnWidths()
	lines := []string{tui.RenderDim("slots")}
	for _, slot := range slotOrder {
		if slot == Slot3 {
			continue
		}
		if binding, ok := m.bindingForSlot(slot); ok {
			kind := string(binding.Kind)
			if lipgloss.Width(kind) > width-2-slotWidth-paneWidth {
				kind = tui.TruncateToWidth(kind, width-2-slotWidth-paneWidth)
			}
			text := fmt.Sprintf("  %-*s%-*d%s", slotWidth, slot, paneWidth, binding.PaneID, kind)
			lines = append(lines, tui.RenderDim(padSlotSummaryLine(text, width)))
			continue
		}
		text := fmt.Sprintf("  %-*s%-*s%s", slotWidth, slot, paneWidth, "-", "empty")
		lines = append(lines, tui.RenderDim(padSlotSummaryLine(text, width)))
	}
	return lines
}

func (m Model) viewSlotSummaryLegacy() []string {
	lines := []string{tui.RenderDim("slots")}
	for _, slot := range slotOrder {
		if binding, ok := m.bindingForSlot(slot); ok {
			lines = append(lines, tui.RenderDim(fmt.Sprintf("  %-5s pane %-4d %s", slot, binding.PaneID, binding.Kind)))
			continue
		}
		lines = append(lines, tui.RenderDim(fmt.Sprintf("  %-5s empty", slot)))
	}
	return lines
}

func (m Model) viewFooter() string {
	help := "  enter:load  n:normal panes  r:refresh"
	switch m.mode {
	case ModeReplacePrompt:
		help = "  1/2/3:replace  esc:back  r:refresh"
	case ModeNormalPanePicker:
		help = "  enter:load  esc:back  r:refresh"
	}
	divider := m.renderFooterDivider()
	footer := tui.RenderDim(help)
	if m.statusText == "" || m.shouldHideStatusText() {
		return divider + "\n" + footer
	}
	return fmt.Sprintf("%s\n%s\n%s", divider, tui.RenderDim("  "+m.statusText), footer)
}

func (m Model) renderFooterDivider() string {
	sepWidth := m.renderWidth() - 2
	if sepWidth < 1 {
		sepWidth = 1
	}
	return tui.RenderDim("  " + strings.Repeat("\u2500", sepWidth))
}

func (m Model) shouldHideStatusText() bool {
	if m.mode != ModeList {
		return false
	}
	return m.statusText == fmt.Sprintf("%d agent pane(s) visible", len(m.agentPanes()))
}

func (m Model) slotForPane(paneID int) (SlotID, bool) {
	for _, binding := range m.bindings {
		if binding.PaneID == paneID {
			return binding.Slot, true
		}
	}
	return "", false
}

func (m Model) bindingForSlot(slot SlotID) (SlotBinding, bool) {
	for _, binding := range m.bindings {
		if binding.Slot == slot {
			return binding, true
		}
	}
	return SlotBinding{}, false
}

func (m Model) renderWidth() int {
	if m.width >= 40 {
		return m.width
	}
	return 80
}

func (m Model) listWidth() int {
	width := m.renderWidth()
	if width < 110 {
		return width
	}
	rightWidth := m.slotPanelWidth()
	leftWidth := width - rightWidth - 3
	if leftWidth < 60 {
		return width
	}
	return leftWidth
}

func (m Model) slotPanelWidth() int {
	width := m.renderWidth() / 8
	if width < 18 {
		return 18
	}
	return width
}

func slotSummaryColumnWidths() (slotWidth, paneWidth int) {
	return 7, 6
}

func padSlotSummaryLine(text string, width int) string {
	if lipgloss.Width(text) >= width {
		return tui.TruncateToWidth(text, width)
	}
	return text + strings.Repeat(" ", width-lipgloss.Width(text))
}

func (m Model) renderColumnHeaders() string {
	paneWidth, typeWidth, bindingWidth, statusWidth, usageWidth, cwdWidth := m.columnWidths()
	var line string
	maxWidth := 2 + paneWidth + typeWidth + bindingWidth + statusWidth + cwdWidth
	if usageWidth > 0 {
		line = fmt.Sprintf(
			"  %-*s%-*s%-*s%-*s%-*s%s",
			paneWidth, "pane",
			typeWidth, "type",
			bindingWidth, "binding",
			statusWidth, "status",
			usageWidth, "usage",
			"cwd",
		)
		maxWidth += usageWidth
	} else {
		line = fmt.Sprintf(
			"  %-*s%-*s%-*s%-*s%s",
			paneWidth, "pane",
			typeWidth, "type",
			bindingWidth, "binding",
			statusWidth, "status",
			"cwd",
		)
	}
	return tui.RenderDim(tui.TruncateToWidth(line, maxWidth))
}

func (m Model) renderPaneRow(pane CandidatePane, binding string, selected bool) string {
	paneWidth, typeWidth, bindingWidth, statusWidth, usageWidth, cwdWidth := m.columnWidths()
	name := paneLabel(pane)
	kind := paneTypeLabel(pane)
	statusText, statusStyle := tui.FormatAgentStatus(pane.Status, pane.Activity, pane.Attention, m.spinnerFrame)
	cwd := pane.CWD

	name = tui.TruncateToWidth(name, paneWidth)
	kind = tui.TruncateToWidth(kind, typeWidth)
	binding = tui.TruncateToWidth(binding, bindingWidth)
	if cwdWidth > 0 {
		cwd = tui.TruncateToWidth(cwd, cwdWidth)
	}

	nameCell := fmt.Sprintf("  %-*s", paneWidth, name)
	typeCell := fmt.Sprintf("%-*s", typeWidth, kind)
	bindingCell := fmt.Sprintf("%-*s", bindingWidth, binding)
	cwdCell := fmt.Sprintf("%-*s", cwdWidth, cwd)

	// Build usage text for Codex panes (only for idle/working; needs_input and error omit usage)
	var usageText string
	var usageStyle lipgloss.Style
	if pane.Kind == OccupantAgent && pane.AgentType == "codex" &&
		pane.Status != model.StatusNeedsInput && pane.Status != model.StatusError {
		text, style := formatUsageText(m.codexQuota, usageWidth)
		usageText = text
		usageStyle = style
	}

	if selected {
		typeStyled := tui.RenderSelectedText(typeCell)
		if pane.Kind == OccupantAgent && pane.AgentType != "" {
			typeStyled = tui.RenderSelectedAgentTypeCell(pane.AgentType, typeCell)
		}
		statusCell := renderStatusCell(statusText, statusStyle, true, statusWidth)
		usageCell := renderUsageCell(usageText, usageStyle, true, usageWidth)
		return tui.RenderSelectedText(nameCell) +
			typeStyled +
			tui.RenderSelectedText(bindingCell) +
			statusCell +
			usageCell +
			tui.RenderSelectedText(cwdCell)
	}

	typeStyled := typeCell
	if pane.Kind == OccupantAgent && pane.AgentType != "" {
		typeStyled = tui.RenderAgentTypeCell(pane.AgentType, typeCell)
	} else {
		typeStyled = tui.RenderDim(typeCell)
	}
	statusCell := renderStatusCell(statusText, statusStyle, false, statusWidth)
	usageCell := renderUsageCell(usageText, usageStyle, false, usageWidth)

	if cwdCell != "" {
		cwdCell = tui.RenderDim(cwdCell)
	}
	return nameCell + typeStyled + bindingCell + statusCell + usageCell + cwdCell
}

func (m Model) columnWidths() (paneWidth, typeWidth, bindingWidth, statusWidth, usageWidth, cwdWidth int) {
	width := m.listWidth()
	totalWidth := m.renderWidth()
	paneWidth = 16
	typeWidth = 8
	bindingWidth = 8
	statusWidth = 16
	if m.hasCodexPanes() {
		usageWidth = 22
	}
	if totalWidth >= 110 {
		paneWidth = 20
		statusWidth = 20
		if usageWidth > 0 {
			usageWidth = 32
		}
	}
	if totalWidth >= 140 && usageWidth > 0 {
		statusWidth = 24
		usageWidth = 72
	}
	for cwdWidth = width - 2 - paneWidth - typeWidth - bindingWidth - statusWidth - usageWidth; cwdWidth < 12 && usageWidth > 12; {
		usageWidth--
		cwdWidth++
	}
	for cwdWidth < 12 && usageWidth > 8 {
		usageWidth--
		cwdWidth++
	}
	for cwdWidth < 12 && statusWidth > 12 {
		statusWidth--
		cwdWidth++
	}
	for cwdWidth < 12 && paneWidth > 16 {
		paneWidth--
		cwdWidth++
	}
	if cwdWidth < 12 {
		cwdWidth = 12
	}
	return paneWidth, typeWidth, bindingWidth, statusWidth, usageWidth, cwdWidth
}

func paneTypeLabel(pane CandidatePane) string {
	if pane.Kind == OccupantAgent && pane.AgentType != "" {
		switch pane.AgentType {
		case "claude":
			return "Claude"
		case "codex":
			return "Codex"
		case "opencode":
			return "OpenCode"
		case "copilot":
			return "Copilot"
		default:
			return string(pane.AgentType)
		}
	}
	return "Normal"
}

func formatUsageText(qi *codex.QuotaInfo, maxChars int) (string, lipgloss.Style) {
	if qi == nil || maxChars < 3 {
		return "", lipgloss.NewStyle()
	}

	primaryRemaining := quotaRemainingPct(qi.PrimaryPct)
	primary := formatUsageWindow("5h", primaryRemaining, qi.PrimaryReset)
	primaryStyle := tui.QuotaRemainingStyle(primaryRemaining)

	if lipgloss.Width(primary) > maxChars {
		shorter := fmt.Sprintf("5h %.0f%% left", primaryRemaining)
		if lipgloss.Width(shorter) <= maxChars {
			return shorter, primaryStyle
		}
		shortest := fmt.Sprintf("%.0f%% left", primaryRemaining)
		if lipgloss.Width(shortest) <= maxChars {
			return shortest, primaryStyle
		}
		return "", lipgloss.NewStyle()
	}

	if hasSecondaryQuota(qi) {
		secondary := formatUsageWindow("7d", quotaRemainingPct(qi.SecondaryPct), qi.SecondaryReset)
		combined := primary + " / " + secondary
		if lipgloss.Width(combined) <= maxChars {
			return combined, primaryStyle
		}
	}

	return primary, primaryStyle
}

func formatUsageWindow(label string, pct float64, reset string) string {
	if reset == "" {
		return fmt.Sprintf("%s %.0f%% left", label, pct)
	}
	return fmt.Sprintf("%s %.0f%% left (resets %s)", label, pct, reset)
}

func renderStatusCell(statusText string, statusStyle lipgloss.Style, selected bool, cellWidth int) string {
	if lipgloss.Width(statusText) > cellWidth {
		statusText = tui.TruncateToWidth(statusText, cellWidth)
	}
	if !selected {
		return statusStyle.Width(cellWidth).Render(statusText)
	}
	return tui.WithSelectedBg(statusStyle).Bold(true).Width(cellWidth).Render(statusText)
}

func renderUsageCell(usageText string, usageStyle lipgloss.Style, selected bool, cellWidth int) string {
	if lipgloss.Width(usageText) > cellWidth {
		usageText = tui.TruncateToWidth(usageText, cellWidth)
	}
	if !selected {
		return usageStyle.Width(cellWidth).Render(usageText)
	}
	return tui.WithSelectedBg(usageStyle).Bold(true).Width(cellWidth).Render(usageText)
}

func quotaRemainingPct(usedPct float64) float64 {
	remaining := 100 - usedPct
	if remaining < 0 {
		return 0
	}
	if remaining > 100 {
		return 100
	}
	return math.Round(remaining)
}

func hasSecondaryQuota(qi *codex.QuotaInfo) bool {
	if qi == nil {
		return false
	}
	return qi.SecondaryPct > 0 || qi.SecondaryReset != ""
}
