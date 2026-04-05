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
	CWD       string
	Kind      OccupantKind
	AgentType model.AgentType
}

type SlotBinding struct {
	Slot   SlotID       `json:"slot"`
	PaneID int          `json:"pane_id"`
	Kind   OccupantKind `json:"kind"`
}

type MonitorContext struct {
	SelfPaneID       int           `json:"self_pane_id"`
	StarterPaneID    int           `json:"starter_pane_id"`
	WindowID         int           `json:"window_id"`
	TabID            int           `json:"tab_id"`
	SlotBindings     []SlotBinding `json:"slot_bindings"`
	WorkspacePaneIDs []int         `json:"workspace_pane_ids"`
}

func (ctx MonitorContext) Validate() error {
	switch {
	case ctx.SelfPaneID == 0:
		return fmt.Errorf("self pane id is required")
	case ctx.StarterPaneID < 0:
		return fmt.Errorf("starter pane id must be non-negative")
	case ctx.WindowID < 0:
		return fmt.Errorf("window id must be non-negative")
	case ctx.TabID == 0:
		return fmt.Errorf("tab id is required")
	default:
		return nil
	}
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
