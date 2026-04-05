package lite

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sethdeckard/atria/internal/tui"
)

func (m Model) View() string {
	sections := []string{tui.RenderTitleBar("agents", m.renderWidth())}

	switch m.mode {
	case ModeReplacePrompt:
		sections = append(sections, strings.Join(m.viewReplacePrompt(), "\n"))
	case ModeNormalPanePicker:
		sections = append(sections, strings.Join(m.viewNormalPanePicker(), "\n"))
	default:
		sections = append(sections, strings.Join(m.viewAgentList(), "\n"))
	}

	sections = append(sections, strings.Join(m.viewSlotSummary(), "\n"))
	sections = append(sections, m.viewFooter())

	return strings.Join(sections, "\n\n")
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
		line := m.renderPaneRow(paneLabel(pane), paneTypeLabel(pane), binding, pane.CWD)
		if i == m.cursor {
			lines = append(lines, tui.RenderSelectedText(line))
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) viewReplacePrompt() []string {
	lines := []string{
		"  Replace",
		fmt.Sprintf("  All 3 slots are full. %s would require a replacement.", paneLabel(m.replacePane)),
	}
	for _, slot := range allowedReplaceSlots(m.bindings, m.replacePane) {
		lines = append(lines, fmt.Sprintf("  Press %s to replace %s.", strings.TrimPrefix(string(slot), "slot"), slot))
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
		line := m.renderPaneRow(paneLabel(pane), paneTypeLabel(pane), "unbound", pane.CWD)
		if i == m.cursor {
			lines = append(lines, tui.RenderSelectedText(line))
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) viewSlotSummary() []string {
	lines := []string{tui.RenderDim("  slots")}
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
	help := " enter:load  n:normal panes  r:refresh"
	switch m.mode {
	case ModeReplacePrompt:
		help = " 1/2/3:replace  esc:back  r:refresh"
	case ModeNormalPanePicker:
		help = " enter:load  esc:back  r:refresh"
	}
	if m.statusText == "" {
		return tui.RenderFooter(help)
	}
	return fmt.Sprintf("%s\n%s", tui.RenderDim("  "+m.statusText), tui.RenderFooter(help))
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

func (m Model) renderColumnHeaders() string {
	paneWidth, typeWidth, bindingWidth, cwdWidth := m.columnWidths()
	line := fmt.Sprintf(
		"  %-*s%-*s%-*s%s",
		paneWidth, "pane",
		typeWidth, "type",
		bindingWidth, "binding",
		"cwd",
	)
	maxWidth := 2 + paneWidth + typeWidth + bindingWidth + cwdWidth
	return tui.RenderDim(tui.TruncateToWidth(line, maxWidth))
}

func (m Model) renderPaneRow(name, kind, binding, cwd string) string {
	paneWidth, typeWidth, bindingWidth, cwdWidth := m.columnWidths()
	name = tui.TruncateToWidth(name, paneWidth-1)
	kind = tui.TruncateToWidth(kind, typeWidth-1)
	binding = tui.TruncateToWidth(binding, bindingWidth-1)
	if cwdWidth > 0 {
		cwd = tui.TruncateToWidth(cwd, cwdWidth)
	}

	line := fmt.Sprintf(
		"  %-*s%-*s%-*s%s",
		paneWidth, name,
		typeWidth, kind,
		bindingWidth, binding,
		cwd,
	)
	return padLiteLine(line, m.renderWidth())
}

func (m Model) columnWidths() (paneWidth, typeWidth, bindingWidth, cwdWidth int) {
	width := m.renderWidth()
	paneWidth = 22
	typeWidth = 10
	bindingWidth = 11
	cwdWidth = width - 2 - paneWidth - typeWidth - bindingWidth
	if cwdWidth < 12 {
		cwdWidth = 12
	}
	return paneWidth, typeWidth, bindingWidth, cwdWidth
}

func paneTypeLabel(pane CandidatePane) string {
	if pane.Kind == OccupantAgent && pane.AgentType != "" {
		return string(pane.AgentType)
	}
	return string(pane.Kind)
}

func padLiteLine(line string, width int) string {
	if pad := width - lipgloss.Width(line); pad > 0 {
		return line + strings.Repeat(" ", pad)
	}
	return line
}
