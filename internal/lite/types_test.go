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
}
