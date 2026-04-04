package lite

import (
	"reflect"
	"testing"

	"github.com/sethdeckard/atria/internal/model"
)

func TestPlanInitialLayout(t *testing.T) {
	tests := []struct {
		name         string
		panes        []CandidatePane
		want         []SlotBinding
		wantOverflow []int
	}{
		{
			name: "one normal no agent goes to slot1",
			panes: []CandidatePane{
				{PaneID: 10, Kind: OccupantNormal},
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantNormal},
			},
		},
		{
			name: "one agent one normal uses slot1+slot2",
			panes: []CandidatePane{
				{PaneID: 10, Kind: OccupantNormal},
				{PaneID: 11, Kind: OccupantAgent, AgentType: model.AgentCodex},
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 10, Kind: OccupantNormal},
			},
		},
		{
			name: "two agents one normal uses slot1+slot2+slot3",
			panes: []CandidatePane{
				{PaneID: 10, Kind: OccupantNormal},
				{PaneID: 11, Kind: OccupantAgent, AgentType: model.AgentClaude},
				{PaneID: 12, Kind: OccupantAgent, AgentType: model.AgentCodex},
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 10, Kind: OccupantNormal},
			},
		},
		{
			name: "three agents uses all slots",
			panes: []CandidatePane{
				{PaneID: 11, Kind: OccupantAgent, AgentType: model.AgentClaude},
				{PaneID: 12, Kind: OccupantAgent, AgentType: model.AgentCodex},
				{PaneID: 13, Kind: OccupantAgent, AgentType: model.AgentOpenCode},
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
			},
		},
		{
			name: "extra panes overflow to new tabs",
			panes: []CandidatePane{
				{PaneID: 10, Kind: OccupantNormal},
				{PaneID: 11, Kind: OccupantAgent, AgentType: model.AgentClaude},
				{PaneID: 12, Kind: OccupantAgent, AgentType: model.AgentCodex},
				{PaneID: 13, Kind: OccupantAgent, AgentType: model.AgentOpenCode},
				{PaneID: 14, Kind: OccupantAgent, AgentType: model.AgentCopilot},
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
			},
			wantOverflow: []int{10, 14},
		},
		{
			name: "multiple normals keep only first normal",
			panes: []CandidatePane{
				{PaneID: 40, Kind: OccupantNormal},
				{PaneID: 10, Kind: OccupantNormal},
				{PaneID: 30, Kind: OccupantNormal},
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 40, Kind: OccupantNormal},
			},
			wantOverflow: []int{10, 30},
		},
		{
			name: "preserves caller left-to-right order instead of pane ids",
			panes: []CandidatePane{
				{PaneID: 30, Kind: OccupantAgent, AgentType: model.AgentClaude},
				{PaneID: 10, Kind: OccupantAgent, AgentType: model.AgentCodex},
				{PaneID: 20, Kind: OccupantNormal},
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 30, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 20, Kind: OccupantNormal},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, overflow := PlanInitialLayout(tt.panes)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
			if !reflect.DeepEqual(overflow, tt.wantOverflow) {
				t.Fatalf("overflow mismatch\nwant: %#v\ngot:  %#v", tt.wantOverflow, overflow)
			}
		})
	}
}

func TestPlanAgentLoadShiftsNormalPaneRight(t *testing.T) {
	tests := []struct {
		name       string
		bindings   []SlotBinding
		pane       CandidatePane
		want       []SlotBinding
		wantPrompt bool
	}{
		{
			name: "normal in slot1 shifts right for first agent",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantNormal},
			},
			pane: CandidatePane{PaneID: 11, Kind: OccupantAgent, AgentType: model.AgentClaude},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 10, Kind: OccupantNormal},
			},
		},
		{
			name: "normal in slot2 shifts right for second agent",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 11, Kind: OccupantNormal},
			},
			pane: CandidatePane{PaneID: 12, Kind: OccupantAgent, AgentType: model.AgentCodex},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 11, Kind: OccupantNormal},
			},
		},
		{
			name: "full layout prompts instead of auto replacing",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 12, Kind: OccupantNormal},
			},
			pane: CandidatePane{PaneID: 13, Kind: OccupantAgent, AgentType: model.AgentOpenCode},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 12, Kind: OccupantNormal},
			},
			wantPrompt: true,
		},
		{
			name: "same pane returns unchanged without prompt",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
			},
			pane: CandidatePane{PaneID: 10, Kind: OccupantAgent, AgentType: model.AgentClaude},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, prompt := PlanAgentLoad(tt.bindings, tt.pane)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
			if prompt != tt.wantPrompt {
				t.Fatalf("replacePrompt mismatch\nwant: %v\ngot:  %v", tt.wantPrompt, prompt)
			}
		})
	}
}

func TestPlanLoadReplacesSamePaneAcrossKinds(t *testing.T) {
	tests := []struct {
		name       string
		load       func([]SlotBinding, CandidatePane) ([]SlotBinding, bool)
		bindings   []SlotBinding
		pane       CandidatePane
		want       []SlotBinding
		wantPrompt bool
	}{
		{
			name: "slot1 normal becomes agent",
			load: PlanAgentLoad,
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantNormal},
			},
			pane: CandidatePane{PaneID: 10, Kind: OccupantAgent, AgentType: model.AgentClaude},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
			},
		},
		{
			name: "slot1 agent becomes normal",
			load: PlanNormalLoad,
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
			},
			pane: CandidatePane{PaneID: 10, Kind: OccupantNormal},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantNormal},
			},
		},
		{
			name: "slot1 agent changes to normal while another agent remains",
			load: PlanNormalLoad,
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 11, Kind: OccupantAgent},
			},
			pane: CandidatePane{PaneID: 10, Kind: OccupantNormal},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 10, Kind: OccupantNormal},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, prompt := tt.load(tt.bindings, tt.pane)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
			if prompt != tt.wantPrompt {
				t.Fatalf("replacePrompt mismatch\nwant: %v\ngot:  %v", tt.wantPrompt, prompt)
			}
		})
	}
}

func TestPlanNormalLoadUsesRightmostActiveSlot(t *testing.T) {
	tests := []struct {
		name       string
		bindings   []SlotBinding
		pane       CandidatePane
		want       []SlotBinding
		wantPrompt bool
	}{
		{
			name:     "no agent uses slot1",
			bindings: nil,
			pane:     CandidatePane{PaneID: 10, Kind: OccupantNormal},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantNormal},
			},
		},
		{
			name: "one agent uses slot2",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
			},
			pane: CandidatePane{PaneID: 10, Kind: OccupantNormal},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 10, Kind: OccupantNormal},
			},
		},
		{
			name: "existing normal is replaced in the rightmost active slot",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantNormal},
			},
			pane: CandidatePane{PaneID: 10, Kind: OccupantNormal},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 10, Kind: OccupantNormal},
			},
		},
		{
			name: "two agents use slot3",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
			},
			pane: CandidatePane{PaneID: 10, Kind: OccupantNormal},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 10, Kind: OccupantNormal},
			},
		},
		{
			name: "full layout prompts instead of auto replacing",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
			},
			pane: CandidatePane{PaneID: 10, Kind: OccupantNormal},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 13, Kind: OccupantAgent},
			},
			wantPrompt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, prompt := PlanNormalLoad(tt.bindings, tt.pane)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
			if prompt != tt.wantPrompt {
				t.Fatalf("replacePrompt mismatch\nwant: %v\ngot:  %v", tt.wantPrompt, prompt)
			}
		})
	}
}

func TestNormalizeAndShrinkKeepCompactOrder(t *testing.T) {
	bindings := []SlotBinding{
		{Slot: Slot3, PaneID: 30, Kind: OccupantNormal},
		{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 20, Kind: OccupantNormal},
		{Slot: Slot2, PaneID: 11, Kind: OccupantAgent},
	}

	wantNormalized := []SlotBinding{
		{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 11, Kind: OccupantAgent},
		{Slot: Slot3, PaneID: 30, Kind: OccupantNormal},
	}
	if got := normalizeBindings(bindings); !reflect.DeepEqual(got, wantNormalized) {
		t.Fatalf("normalizeBindings mismatch\nwant: %#v\ngot:  %#v", wantNormalized, got)
	}

	wantShrunk := []SlotBinding{
		{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
		{Slot: Slot2, PaneID: 11, Kind: OccupantAgent},
	}
	live := map[int]bool{10: true, 11: true}
	if got := ShrinkBindings(bindings, live); !reflect.DeepEqual(got, wantShrunk) {
		t.Fatalf("ShrinkBindings mismatch\nwant: %#v\ngot:  %#v", wantShrunk, got)
	}
}

func TestNormalizeBindingsDeduplicatesPaneIDs(t *testing.T) {
	tests := []struct {
		name     string
		bindings []SlotBinding
		want     []SlotBinding
	}{
		{
			name: "agent wins over normal for same pane id",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantNormal},
				{Slot: Slot2, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 20, Kind: OccupantNormal},
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 20, Kind: OccupantNormal},
			},
		},
		{
			name: "leftmost agent wins for duplicate agent pane ids",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 20, Kind: OccupantNormal},
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 20, Kind: OccupantNormal},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeBindings(tt.bindings); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeBindings mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}

func TestShrinkLayoutDropsEmptyTrailingSlot(t *testing.T) {
	tests := []struct {
		name     string
		bindings []SlotBinding
		live     map[int]bool
		want     []SlotBinding
	}{
		{
			name: "drops empty trailing slot",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 12, Kind: OccupantNormal},
			},
			live: map[int]bool{
				10: true,
				11: true,
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 11, Kind: OccupantAgent},
			},
		},
		{
			name: "compacts middle hole",
			bindings: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 11, Kind: OccupantAgent},
				{Slot: Slot3, PaneID: 12, Kind: OccupantNormal},
			},
			live: map[int]bool{
				10: true,
				12: true,
			},
			want: []SlotBinding{
				{Slot: Slot1, PaneID: 10, Kind: OccupantAgent},
				{Slot: Slot2, PaneID: 12, Kind: OccupantNormal},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShrinkBindings(tt.bindings, tt.live)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("bindings mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}
