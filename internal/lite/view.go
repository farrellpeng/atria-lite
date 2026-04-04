package lite

import (
	"fmt"
	"strings"
)

func (m Model) View() string {
	var lines []string
	lines = append(lines, "Atria Lite Monitor")
	lines = append(lines, "")

	switch m.mode {
	case ModeReplacePrompt:
		lines = append(lines, m.viewReplacePrompt()...)
	case ModeNormalPanePicker:
		lines = append(lines, m.viewNormalPanePicker()...)
	default:
		lines = append(lines, m.viewAgentList()...)
	}

	lines = append(lines, "")
	lines = append(lines, m.viewSlotSummary()...)
	lines = append(lines, "")
	lines = append(lines, m.viewFooter())

	return strings.Join(lines, "\n")
}

func (m Model) viewAgentList() []string {
	agents := m.agentPanes()
	lines := []string{"Agents"}
	if len(agents) == 0 {
		return append(lines, "  No agent panes in this window.")
	}

	for i, pane := range agents {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}

		binding := "unbound"
		if slot, ok := m.slotForPane(pane.PaneID); ok {
			binding = string(slot)
		}

		lines = append(lines, fmt.Sprintf("%s %s [%s]", cursor, paneLabel(pane), binding))
	}
	return lines
}

func (m Model) viewReplacePrompt() []string {
	lines := []string{
		"Replace Prompt",
		fmt.Sprintf("  All 3 slots are full. %s would require a replacement.", paneLabel(m.replacePane)),
	}
	if m.replaceTarget != "" {
		lines = append(lines, fmt.Sprintf("  Default target: %s", m.replaceTarget))
	}
	lines = append(lines, "  Press esc to go back.")
	return lines
}

func (m Model) viewNormalPanePicker() []string {
	normals := m.normalPanes()
	lines := []string{"Normal Panes"}
	if len(normals) == 0 {
		return append(lines, "  No normal panes available.")
	}

	for _, pane := range normals {
		lines = append(lines, fmt.Sprintf("  - %s", paneLabel(pane)))
	}
	lines = append(lines, "  Press esc to return.")
	return lines
}

func (m Model) viewSlotSummary() []string {
	lines := []string{"Slots"}
	for _, slot := range slotOrder {
		if binding, ok := m.bindingForSlot(slot); ok {
			lines = append(lines, fmt.Sprintf("  %s: pane %d (%s)", slot, binding.PaneID, binding.Kind))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s: empty", slot))
	}
	return lines
}

func (m Model) viewFooter() string {
	help := "enter load agent | n normal panes | r refresh | esc back"
	if m.statusText == "" {
		return help
	}
	return fmt.Sprintf("%s\n%s", m.statusText, help)
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
