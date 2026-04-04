package lite

var slotOrder = []SlotID{Slot1, Slot2, Slot3}

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
	if len(selected) < len(slotOrder) && len(normals) > 0 {
		selected = append(selected, normals[0])
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
	if hasPane(current, pane.PaneID) {
		return current, false
	}
	if len(current) == len(slotOrder) {
		return current, true
	}

	insertAt := len(current)
	if len(current) > 0 && current[len(current)-1].Kind == OccupantNormal {
		insertAt = len(current) - 1
	}

	next := make([]SlotBinding, 0, len(current)+1)
	next = append(next, current[:insertAt]...)
	next = append(next, SlotBinding{PaneID: pane.PaneID, Kind: OccupantAgent})
	next = append(next, current[insertAt:]...)
	return reindex(next), false
}

func PlanNormalLoad(bindings []SlotBinding, pane CandidatePane) ([]SlotBinding, bool) {
	current := normalizeBindings(bindings)
	if hasPane(current, pane.PaneID) {
		return current, false
	}
	if len(current) == len(slotOrder) {
		return current, true
	}

	if len(current) > 0 && current[len(current)-1].Kind == OccupantNormal {
		next := append([]SlotBinding(nil), current...)
		next[len(next)-1] = SlotBinding{
			Slot:   current[len(current)-1].Slot,
			PaneID: pane.PaneID,
			Kind:   OccupantNormal,
		}
		return next, false
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
	var normal *SlotBinding

	for _, slot := range slotOrder {
		for _, binding := range bindings {
			if binding.Slot != slot {
				continue
			}
			switch binding.Kind {
			case OccupantAgent:
				if len(agents) < len(slotOrder) {
					agents = append(agents, SlotBinding{PaneID: binding.PaneID, Kind: OccupantAgent})
				}
			case OccupantNormal:
				if normal == nil {
					nb := SlotBinding{PaneID: binding.PaneID, Kind: OccupantNormal}
					normal = &nb
				}
			}
			break
		}
	}

	next := make([]SlotBinding, 0, len(slotOrder))
	next = append(next, agents...)
	if len(next) < len(slotOrder) && normal != nil {
		next = append(next, *normal)
	}
	return reindex(next)
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

func hasPane(bindings []SlotBinding, paneID int) bool {
	for _, binding := range bindings {
		if binding.PaneID == paneID {
			return true
		}
	}
	return false
}
