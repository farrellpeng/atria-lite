package lite

import (
	"reflect"
	"strings"
	"testing"
)

func TestEncodeMonitorContextRoundTrip(t *testing.T) {
	original := MonitorContext{
		SelfPaneID:    101,
		StarterPaneID: 202,
		WindowID:      303,
		TabID:         404,
		SlotBindings: []SlotBinding{
			{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			{Slot: Slot2, PaneID: 22, Kind: OccupantNormal},
		},
		WorkspacePaneIDs: []int{7, 8, 9},
	}

	encoded, err := EncodeMonitorContext(original)
	if err != nil {
		t.Fatalf("EncodeMonitorContext() error = %v", err)
	}
	if strings.ContainsAny(encoded, "+/=\n\r\t ") {
		t.Fatalf("encoded context is not CLI-safe: %q", encoded)
	}

	decoded, err := DecodeMonitorContext(encoded)
	if err != nil {
		t.Fatalf("DecodeMonitorContext() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round trip mismatch\nwant: %#v\ngot:  %#v", original, decoded)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestMonitorContextValidate(t *testing.T) {
	t.Run("requires core ids", func(t *testing.T) {
		tests := []struct {
			name string
			ctx  MonitorContext
		}{
			{
				name: "self pane id",
				ctx:  MonitorContext{StarterPaneID: 2, WindowID: 3, TabID: 4},
			},
			{
				name: "starter pane id",
				ctx:  MonitorContext{SelfPaneID: 1, StarterPaneID: -1, WindowID: 3, TabID: 4},
			},
			{
				name: "tab id",
				ctx:  MonitorContext{SelfPaneID: 1, StarterPaneID: 2, WindowID: 3},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if err := tt.ctx.Validate(); err == nil {
					t.Fatalf("Validate() error = nil, want non-nil")
				}
			})
		}
	})

	t.Run("allows zero window id for wezterm cli contexts", func(t *testing.T) {
		ctx := MonitorContext{
			SelfPaneID:    1,
			StarterPaneID: 2,
			WindowID:      0,
			TabID:         4,
		}
		if err := ctx.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("allows zero starter pane id for wezterm cli contexts", func(t *testing.T) {
		ctx := MonitorContext{
			SelfPaneID:    1,
			StarterPaneID: 0,
			WindowID:      0,
			TabID:         4,
		}
		if err := ctx.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})
}
