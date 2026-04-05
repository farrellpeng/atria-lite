package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/sethdeckard/atria/internal/model"
)

// FormatAgentStatus returns the shared Atria status text and color styling.
func FormatAgentStatus(status model.AgentStatus, activity, attention string, spinnerFrame int) (string, lipgloss.Style) {
	switch status {
	case model.StatusNeedsInput:
		text := "\u26a0 " + attention
		if text == "\u26a0 " {
			text = "\u26a0 needs input"
		}
		return text, statusNeedsInputStyle
	case model.StatusWorking:
		spin := spinnerFrames[spinnerFrame%len(spinnerFrames)]
		text := spin + " "
		if activity != "" {
			text += activity
		} else {
			text += "working..."
		}
		return text, statusWorkingStyle
	case model.StatusIdle:
		text := "\u25cf idle"
		if activity != "" {
			text = "\u25cf " + activity
		}
		return text, statusIdleStyle
	case model.StatusError:
		text := "\u2717 error"
		if attention != "" {
			text = "\u2717 " + attention
		}
		return text, statusErrorStyle
	default:
		spin := spinnerFrames[spinnerFrame%len(spinnerFrames)]
		return spin + " working...", statusWorkingStyle
	}
}

// RenderSelectedStatusCell applies the selected-row background while
// preserving the status-specific foreground color.
func RenderSelectedStatusCell(style lipgloss.Style, text string) string {
	return WithSelectedBg(style).Bold(true).Render(text)
}
