package lite

import (
	"fmt"
	"strings"

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
	paneWidth, typeWidth, bindingWidth, statusWidth, cwdWidth := m.columnWidths()
	line := fmt.Sprintf(
		"  %-*s%-*s%-*s%-*s%s",
		paneWidth, "pane",
		typeWidth, "type",
		bindingWidth, "binding",
		statusWidth, "status",
		"cwd",
	)
	maxWidth := 2 + paneWidth + typeWidth + bindingWidth + statusWidth + cwdWidth
	return tui.RenderDim(tui.TruncateToWidth(line, maxWidth))
}

func (m Model) renderPaneRow(pane CandidatePane, binding string, selected bool) string {
	paneWidth, typeWidth, bindingWidth, statusWidth, cwdWidth := m.columnWidths()
	name := paneLabel(pane)
	kind := paneTypeLabel(pane)
	statusText, statusStyle := tui.FormatAgentStatus(pane.Status, pane.Activity, pane.Attention, m.spinnerFrame)
	cwd := pane.CWD

	name = tui.TruncateToWidth(name, paneWidth-1)
	kind = tui.TruncateToWidth(kind, typeWidth-1)
	binding = tui.TruncateToWidth(binding, bindingWidth-1)
	statusText = tui.TruncateToWidth(statusText, statusWidth-1)
	if cwdWidth > 0 {
		cwd = tui.TruncateToWidth(cwd, cwdWidth)
	}

	nameCell := fmt.Sprintf("  %-*s", paneWidth, name)
	typeCell := fmt.Sprintf("%-*s", typeWidth, kind)
	bindingCell := fmt.Sprintf("%-*s", bindingWidth, binding)
	statusCell := fmt.Sprintf("%-*s", statusWidth, statusText)
	cwdCell := cwd

	if selected {
		typeStyled := tui.RenderSelectedText(typeCell)
		if pane.Kind == OccupantAgent && pane.AgentType != "" {
			typeStyled = tui.RenderSelectedAgentTypeCell(pane.AgentType, typeCell)
		}
		selectedStatus := tui.RenderSelectedText(statusCell)
		if pane.Kind == OccupantAgent && pane.AgentType != "" {
			selectedStatus = tui.RenderSelectedStatusCell(statusStyle, statusCell)
		}
		return tui.RenderSelectedText(nameCell) +
			typeStyled +
			tui.RenderSelectedText(bindingCell) +
			selectedStatus +
			tui.RenderSelectedText(cwdCell)
	}

	typeStyled := typeCell
	if pane.Kind == OccupantAgent && pane.AgentType != "" {
		typeStyled = tui.RenderAgentTypeCell(pane.AgentType, typeCell)
	} else {
		typeStyled = tui.RenderDim(typeCell)
	}
	if pane.Kind == OccupantAgent && pane.AgentType != "" {
		statusCell = statusStyle.Render(statusCell)
	} else {
		statusCell = tui.RenderDim(statusCell)
	}

	if cwdCell != "" {
		cwdCell = tui.RenderDim(cwdCell)
	}
	return nameCell + typeStyled + bindingCell + statusCell + cwdCell
}

func (m Model) columnWidths() (paneWidth, typeWidth, bindingWidth, statusWidth, cwdWidth int) {
	width := m.renderWidth()
	paneWidth = 18
	typeWidth = 10
	bindingWidth = 10
	statusWidth = 24
	if width >= 110 {
		paneWidth = 24
		statusWidth = 28
	}
	cwdWidth = width - 2 - paneWidth - typeWidth - bindingWidth - statusWidth
	for cwdWidth < 12 && statusWidth > 18 {
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
	return paneWidth, typeWidth, bindingWidth, statusWidth, cwdWidth
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
