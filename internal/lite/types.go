package lite

import "github.com/sethdeckard/atria/internal/model"

type SlotID string

const (
	Slot1 SlotID = "slot1"
	Slot2 SlotID = "slot2"
	Slot3 SlotID = "slot3"
)

type OccupantKind string

const (
	OccupantAgent  OccupantKind = "agent"
	OccupantNormal OccupantKind = "normal"
)

type CandidatePane struct {
	PaneID    int
	WindowID  int
	TabID     int
	Title     string
	Kind      OccupantKind
	AgentType model.AgentType
}

type SlotBinding struct {
	Slot   SlotID
	PaneID int
	Kind   OccupantKind
}

type MonitorContext struct {
	SelfPaneID       int
	StarterPaneID    int
	WindowID         int
	TabID            int
	SlotBindings     []SlotBinding
	WorkspacePaneIDs []int
}
