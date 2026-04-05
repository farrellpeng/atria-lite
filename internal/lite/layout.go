package lite

var slotOrder = []SlotID{Slot1, Slot2, Slot3}

// PlanInitialLayout expects panes in current-window left-to-right order and
// preserves that encounter order when choosing which agent panes occupy slots.
func PlanInitialLayout(panes []CandidatePane) (bindings []SlotBinding, overflow []int) {
	if len(panes) == 0 {
		return nil, nil
	}

	agents := make([]CandidatePane, 0, len(panes))
	normals := make([]CandidatePane, 0, 1)
	for _, pane := range panes {
		switch pane.Kind {
		case OccupantAgent:
			agents = append(agents, pane)
		case OccupantNormal:
			normals = append(normals, pane)
		}
	}

	selected := make([]CandidatePane, 0, 3)
	if len(agents) > 0 {
		for _, pane := range agents {
			if len(selected) == len(slotOrder) {
				break
			}
			selected = append(selected, pane)
		}
	}
	targetSlots := len(slotOrder)
	if len(agents) < minimumStartupSlots {
		targetSlots = minimumStartupSlots
	}
	if len(selected) < len(slotOrder) && len(normals) > 0 {
		for _, pane := range normals {
			if len(selected) == targetSlots {
				break
			}
			selected = append(selected, pane)
		}
	}

	selectedIDs := make(map[int]bool, len(selected))
	for _, pane := range selected {
		selectedIDs[pane.PaneID] = true
	}
	for _, pane := range panes {
		if !selectedIDs[pane.PaneID] {
			overflow = append(overflow, pane.PaneID)
		}
	}

	bindings = make([]SlotBinding, 0, len(selected))
	for i, pane := range selected {
		bindings = append(bindings, SlotBinding{
			Slot:   slotOrder[i],
			PaneID: pane.PaneID,
			Kind:   pane.Kind,
		})
	}

	return bindings, overflow
}

func PlanAgentLoad(bindings []SlotBinding, pane CandidatePane) ([]SlotBinding, bool) {
	current := normalizeBindings(bindings)
	if idx, found := findPaneIndex(current, pane.PaneID); found {
		if current[idx].Kind == OccupantAgent {
			return current, false
		}
		current = append(append([]SlotBinding(nil), current[:idx]...), current[idx+1:]...)
	}
	if len(current) == len(slotOrder) {
		return current, true
	}

	insertAt := leadingAgents(current)

	next := make([]SlotBinding, 0, len(current)+1)
	next = append(next, current[:insertAt]...)
	next = append(next, SlotBinding{PaneID: pane.PaneID, Kind: OccupantAgent})
	next = append(next, current[insertAt:]...)
	return reindex(next), false
}

func PlanNormalLoad(bindings []SlotBinding, pane CandidatePane) ([]SlotBinding, bool) {
	current := normalizeBindings(bindings)
	if idx, found := findPaneIndex(current, pane.PaneID); found {
		if current[idx].Kind == OccupantNormal {
			return current, false
		}
		current = append(append([]SlotBinding(nil), current[:idx]...), current[idx+1:]...)
	}
	if len(current) == len(slotOrder) {
		return current, true
	}

	next := append(append([]SlotBinding(nil), current...), SlotBinding{PaneID: pane.PaneID, Kind: OccupantNormal})
	return reindex(next), false
}

func ShrinkBindings(bindings []SlotBinding, livePaneIDs map[int]bool) []SlotBinding {
	current := normalizeBindings(bindings)
	shrunk := make([]SlotBinding, 0, len(current))
	for _, binding := range current {
		if livePaneIDs[binding.PaneID] {
			shrunk = append(shrunk, binding)
		}
	}
	return reindex(shrunk)
}

func normalizeBindings(bindings []SlotBinding) []SlotBinding {
	agents := make([]SlotBinding, 0, len(bindings))
	normals := make([]SlotBinding, 0, len(bindings))
	seenPaneIDs := make(map[int]bool, len(bindings))

	for _, slot := range slotOrder {
		for _, binding := range bindings {
			if binding.Slot != slot || binding.Kind != OccupantAgent {
				continue
			}
			if seenPaneIDs[binding.PaneID] {
				break
			}
			if len(agents) < len(slotOrder) {
				agents = append(agents, SlotBinding{PaneID: binding.PaneID, Kind: OccupantAgent})
				seenPaneIDs[binding.PaneID] = true
			}
			break
		}
	}

	for _, slot := range slotOrder {
		for _, binding := range bindings {
			if binding.Slot != slot || binding.Kind != OccupantNormal || seenPaneIDs[binding.PaneID] {
				continue
			}
			if len(agents)+len(normals) >= len(slotOrder) {
				break
			}
			normals = append(normals, SlotBinding{PaneID: binding.PaneID, Kind: OccupantNormal})
			seenPaneIDs[binding.PaneID] = true
			break
		}
	}

	next := make([]SlotBinding, 0, len(slotOrder))
	next = append(next, agents...)
	next = append(next, normals...)
	return reindex(next)
}

func leadingAgents(bindings []SlotBinding) int {
	count := 0
	for _, binding := range bindings {
		if binding.Kind != OccupantAgent {
			break
		}
		count++
	}
	return count
}

func reindex(bindings []SlotBinding) []SlotBinding {
	next := make([]SlotBinding, 0, len(bindings))
	for i, binding := range bindings {
		if i >= len(slotOrder) {
			break
		}
		next = append(next, SlotBinding{
			Slot:   slotOrder[i],
			PaneID: binding.PaneID,
			Kind:   binding.Kind,
		})
	}
	return next
}

func findPaneIndex(bindings []SlotBinding, paneID int) (int, bool) {
	for i, binding := range bindings {
		if binding.PaneID == paneID {
			return i, true
		}
	}
	return 0, false
}
