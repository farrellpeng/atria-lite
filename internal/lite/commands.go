package lite

import tea "github.com/charmbracelet/bubbletea"

func refreshWindowPanes(client windowPaneClient, windowID int) tea.Cmd {
	if client == nil || windowID == 0 {
		return nil
	}

	return func() tea.Msg {
		panes, err := client.ListWindowPanes(windowID)
		if err != nil {
			return windowPanesLoadFailedMsg{err: err}
		}
		return windowPanesLoadedMsg{panes: panes}
	}
}
