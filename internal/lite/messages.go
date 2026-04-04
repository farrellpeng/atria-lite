package lite

import "github.com/sethdeckard/atria/internal/terminal/wezterm"

type windowPanesLoadedMsg struct {
	panes []wezterm.PaneInfo
}

type windowPanesLoadFailedMsg struct {
	err error
}
