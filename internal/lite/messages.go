package lite

import "github.com/sethdeckard/atria/internal/terminal/wezterm"

type refreshTickMsg struct{}

type statusTickMsg struct{}

type spinnerTickMsg struct{}

type windowPanesLoadedMsg struct {
	panes []wezterm.PaneInfo
}

type windowPanesLoadFailedMsg struct {
	err error
}

type candidatePanesLoadedMsg struct {
	panes []CandidatePane
}

type paneStatusesLoadedMsg struct {
	panes []CandidatePane
}

type paneStatusesLoadFailedMsg struct {
	err error
}

type slotActionCompletedMsg struct {
	bindings   []SlotBinding
	statusText string
}

type slotActionFailedMsg struct {
	err error
}
