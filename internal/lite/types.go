package lite

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/sethdeckard/atria/internal/model"
)

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

func EncodeMonitorContext(ctx MonitorContext) (string, error) {
	raw, err := json.Marshal(ctx)
	if err != nil {
		return "", fmt.Errorf("marshal monitor context: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeMonitorContext(s string) (MonitorContext, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return MonitorContext{}, fmt.Errorf("decode monitor context: %w", err)
	}

	var ctx MonitorContext
	if err := json.Unmarshal(raw, &ctx); err != nil {
		return MonitorContext{}, fmt.Errorf("unmarshal monitor context: %w", err)
	}
	return ctx, nil
}
