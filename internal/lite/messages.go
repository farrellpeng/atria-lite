package lite

import "github.com/sethdeckard/atria/internal/terminal/wezterm"

type windowPanesLoadedMsg struct {
	panes []wezterm.PaneInfo
}

type windowPanesLoadFailedMsg struct {
	err error
}

type candidatePanesLoadedMsg struct {
	panes []CandidatePane
}

type slotActionCompletedMsg struct {
	bindings   []SlotBinding
	statusText string
}

type slotActionFailedMsg struct {
	err error
}
