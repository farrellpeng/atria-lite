package tui

import (
	"fmt"
	"strings"

	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sethdeckard/atria/internal/model"
)

type chatEntry struct {
	Timestamp time.Time
	Direction string // "sent" | "received"
	Text      string
}

type completionState struct {
	active        bool
	projectDir    string
	items         []string
	cursor        int
	filter        string
}

type chatView struct {
	input        textarea.Model
	entries      []chatEntry
	streamHeight int
	ready        bool
	completion   *completionState
}

func newChatView() chatView {
	ta := textarea.New()
	ta.Placeholder = "Type a prompt..."
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.Focus()
	ta.CharLimit = 0

	return chatView{
		input: ta,
	}
}

func (c *chatView) setSize(width, height int) {
	// header(2) + hint(1) + textarea(3+1border) = 7 lines overhead
	overhead := 7
	c.streamHeight = height - overhead
	if c.streamHeight < 7 {
		c.streamHeight = 7
	}
	c.input.SetWidth(width - 2)
	c.ready = true
}

func (c *chatView) addEntry(entry chatEntry) {
	c.entries = append(c.entries, entry)
}

func (c *chatView) setProjectDir(projectDir string) {
	if c.completion == nil {
		c.completion = &completionState{}
	}
	c.completion.projectDir = projectDir
}

func (c *chatView) triggerCompletion() {
	if c.completion == nil {
		c.completion = &completionState{}
	}
	c.completion.active = true
	// Build a simple file list from the project dir
	c.completion.items = []string{"file1.txt", "file2.go"}
	c.completion.cursor = 0
}

func (c *chatView) nextCompletion() {
	if c.completion == nil || !c.completion.active || len(c.completion.items) == 0 {
		return
	}
	c.completion.cursor = (c.completion.cursor + 1) % len(c.completion.items)
}

func (c *chatView) prevCompletion() {
	if c.completion == nil || !c.completion.active || len(c.completion.items) == 0 {
		return
	}
	c.completion.cursor--
	if c.completion.cursor < 0 {
		c.completion.cursor = len(c.completion.items) - 1
	}
}

func (c *chatView) applyCompletion() {
	if c.completion == nil || !c.completion.active || len(c.completion.items) == 0 {
		return
	}
	selected := c.completion.items[c.completion.cursor]
	c.input.SetValue(c.input.Value() + selected)
	c.completion.active = false
}

func (c *chatView) cancelCompletion() {
	if c.completion == nil {
		return
	}
	c.completion.active = false
}

func (c *chatView) refreshCompletion() {
	// Refresh completion list based on current input filter
	if c.completion == nil || !c.completion.active {
		return
	}
	// Simple refresh: rebuild items (could filter based on @ prefix)
	c.completion.items = []string{"file1.txt", "file2.go"}
	if c.completion.cursor >= len(c.completion.items) {
		c.completion.cursor = 0
	}
}

func (c *chatView) renderHeader(session *model.AgentSession, project *model.Project, width int) string {
	if session == nil || project == nil {
		return titleStyle.Render("  No agent selected")
	}
	agentStr := strings.ToUpper(string(session.Type)[:1]) + string(session.Type)[1:]
	name := project.DisplayName()
	dir := contractHome(project.Dir)
	right := brandingStyle.Render("atria  ")
	rightW := lipgloss.Width(right)
	maxLeft := width - rightW - 1 // -1 for min gap

	leftText := "  " + name + " \u00b7 " + agentStr + " \u00b7 " + dir
	if lipgloss.Width(leftText) > maxLeft {
		// Truncate dir from the left: …/tail
		prefix := "  " + name + " \u00b7 " + agentStr + " \u00b7 "
		availForDir := maxLeft - lipgloss.Width(prefix) - 1 // -1 for …
		if availForDir >= 4 {
			dirRunes := []rune(dir)
			truncDir := ""
			for i := len(dirRunes) - 1; i >= 0; i-- {
				candidate := "\u2026" + string(dirRunes[i]) + truncDir
				if lipgloss.Width(candidate) > availForDir {
					break
				}
				truncDir = string(dirRunes[i]) + truncDir
			}
			leftText = prefix + "\u2026" + truncDir
		} else {
			// Drop dir entirely
			leftText = "  " + name + " \u00b7 " + agentStr
		}
		// Final clamp if name itself is too long
		if lipgloss.Width(leftText) > maxLeft && maxLeft > 4 {
			truncated := "  "
			for _, r := range leftText[2:] {
				if lipgloss.Width(truncated+string(r)) > maxLeft-1 {
					truncated += "\u2026"
					break
				}
				truncated += string(r)
			}
			leftText = truncated
		}
	}
	left := titleStyle.Render(leftText)
	leftW := lipgloss.Width(left)
	gap := width - leftW - rightW
	var line string
	if gap < 0 {
		line = left
	} else {
		line = left + strings.Repeat(" ", gap) + right
	}

	sepWidth := width - 2
	if sepWidth < 1 {
		sepWidth = 1
	}
	sep := dimStyle.Render("  " + strings.Repeat("\u2500", sepWidth))

	return line + "\n" + sep
}

func (c *chatView) renderStreamBox(session *model.AgentSession, width, spinnerFrame int) string {
	var sb strings.Builder

	boxWidth := width - 1
	if boxWidth < 6 {
		boxWidth = 6
	}
	innerWidth := boxWidth - 4 // "│ " + content + " │"
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Top border with live status
	statusText := ""
	if session != nil {
		st, _ := formatStatus(session, spinnerFrame)
		statusText = " " + st + " "
	}
	rightText := " esc:back "
	rightLen := lipgloss.Width(rightText)
	maxStatusWidth := boxWidth - 2 - rightLen - 1 // -2 for ┌┐, -1 for min fill
	if statusText != "" && lipgloss.Width(statusText) > maxStatusWidth {
		// Truncate status text (rune-safe)
		truncated := " "
		for _, r := range statusText[1:] { // skip leading space already added
			if lipgloss.Width(truncated+string(r)+"\u2026 ") > maxStatusWidth {
				truncated += "\u2026 "
				break
			}
			truncated += string(r)
		}
		statusText = truncated
	}
	fillLen := boxWidth - 2 - lipgloss.Width(statusText) - rightLen
	if fillLen < 0 {
		fillLen = 0
	}
	topBorder := "\u250c" + statusText + strings.Repeat("\u2500", fillLen) + rightText + "\u2510"
	sb.WriteString(dimStyle.Render(" " + topBorder))
	sb.WriteString("\n")

	// Compute how many lines for chat entries
	chatLines := 0
	maxChatEntries := 5
	entries := c.entries
	if len(entries) > maxChatEntries {
		entries = entries[len(entries)-maxChatEntries:]
	}
	if len(entries) > 0 {
		chatLines = len(entries) + 1 // +1 for dashed separator
	}

	contentLines := c.streamHeight - 2 // top + bottom borders
	if contentLines < 1 {
		contentLines = 1
	}
	streamLines := contentLines - chatLines
	if streamLines < 1 {
		streamLines = 1
	}

	// Render stream content (bottom N lines of LastScreen)
	if session == nil || strings.TrimSpace(session.LastScreen) == "" {
		placeholder := "no output"
		pad := innerWidth - lipgloss.Width(placeholder)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(dimStyle.Render(" \u2502") + " " + dimStyle.Render(placeholder) + strings.Repeat(" ", pad) + " " + dimStyle.Render("\u2502"))
		sb.WriteString("\n")
		for i := 1; i < streamLines; i++ {
			sb.WriteString(dimStyle.Render(" \u2502") + strings.Repeat(" ", innerWidth+2) + dimStyle.Render("\u2502"))
			sb.WriteString("\n")
		}
	} else {
		lines := strings.Split(sanitizeBoxText(session.LastScreen), "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > streamLines {
			lines = lines[len(lines)-streamLines:]
		}
		for _, line := range lines {
			sb.WriteString(c.renderBoxLine(line, innerWidth))
			sb.WriteString("\n")
		}
		for i := len(lines); i < streamLines; i++ {
			sb.WriteString(dimStyle.Render(" \u2502") + strings.Repeat(" ", innerWidth+2) + dimStyle.Render("\u2502"))
			sb.WriteString("\n")
		}
	}

	// Chat entries section
	if len(entries) > 0 {
		// Dashed separator
		dashes := strings.Repeat("\u2500 ", innerWidth/2)
		if lipgloss.Width(dashes) > innerWidth {
			dashes = dashes[:innerWidth]
		}
		sb.WriteString(dimStyle.Render(" \u2502") + " " + dimStyle.Render(dashes))
		pad := innerWidth - lipgloss.Width(dashes)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(strings.Repeat(" ", pad) + " " + dimStyle.Render("\u2502"))
		sb.WriteString("\n")

		for _, e := range entries {
			ts := e.Timestamp.Format("15:04")
			// Collapse newlines to spaces for display
			entryText := strings.ReplaceAll(e.Text, "\n", " ")
			var prefix string
			switch e.Direction {
			case "sent":
				prefix = fmt.Sprintf("[%s] > ", ts)
			case "received":
				prefix = fmt.Sprintf("[%s]   ", ts)
			default:
				prefix = fmt.Sprintf("[%s]   ", ts)
			}
			// Truncate plain text to fit innerWidth, then style
			plain := truncateToWidth(prefix+entryText, innerWidth)
			var styled string
			switch e.Direction {
			case "sent":
				styled = chatSentStyle.Render(plain)
			case "received":
				styled = chatReceivedStyle.Render(plain)
			default:
				styled = plain
			}
			textWidth := lipgloss.Width(styled)
			pad := innerWidth - textWidth
			if pad < 0 {
				pad = 0
			}
			sb.WriteString(dimStyle.Render(" \u2502") + " " + styled + strings.Repeat(" ", pad) + " " + dimStyle.Render("\u2502"))
			sb.WriteString("\n")
		}
	}

	// Bottom border
	bottomBorder := "\u2514" + strings.Repeat("\u2500", boxWidth-2) + "\u2518"
	sb.WriteString(dimStyle.Render(" " + bottomBorder))

	return sb.String()
}

// renderBoxLine renders a single line inside box borders, truncating if needed.
func (c *chatView) renderBoxLine(line string, innerWidth int) string {
	line = sanitizeBoxText(line)
	line = truncateToWidth(line, innerWidth)
	lineWidth := lipgloss.Width(line)
	pad := innerWidth - lineWidth
	if pad < 0 {
		pad = 0
	}
	return dimStyle.Render(" \u2502") + " " + line + strings.Repeat(" ", pad) + " " + dimStyle.Render("\u2502")
}

func (c *chatView) render(session *model.AgentSession, project *model.Project, width, spinnerFrame int) string {
	header := c.renderHeader(session, project, width)
	streamBox := c.renderStreamBox(session, width, spinnerFrame)
	inputHint := dimStyle.Render("  Enter: newline  Ctrl+D: send  Esc: back")
	inputView := c.input.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		streamBox,
		inputHint,
		inputView,
	)
}

// renderInline renders the chat entries and input for embedding in the stream panel.
// contentHeight: available lines for output display
// Returns rendered string.
func (c *chatView) renderInline(session *model.AgentSession, contentHeight, innerWidth, spinnerFrame int) string {
	var sb strings.Builder

	borderStyle := dimStyle
	if session != nil {
		switch session.Status {
		case model.StatusWorking:
			borderStyle = statusWorkingStyle
		case model.StatusIdle:
			borderStyle = statusIdleStyle
		case model.StatusNeedsInput:
			borderStyle = statusNeedsInputStyle
		case model.StatusError:
			borderStyle = statusErrorStyle
		}
	}

	// 1. Output content (last N lines from LastScreen)
	// Reserve lines for: 1 separator + up to 3 entries + 1 hint + 1 textarea = 6 fixed lines
	outputLines := contentHeight - 6
	if outputLines < 2 {
		outputLines = 2
	}
	if session == nil || strings.TrimSpace(session.LastScreen) == "" {
		placeholder := "no output"
		pad := innerWidth - lipgloss.Width(placeholder)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(borderStyle.Render(" \u2502") + " " + dimStyle.Render(placeholder) + strings.Repeat(" ", pad) + " " + borderStyle.Render("\u2502"))
		sb.WriteString("\n")
		for i := 1; i < outputLines; i++ {
			sb.WriteString(borderStyle.Render(" \u2502") + strings.Repeat(" ", innerWidth+2) + borderStyle.Render("\u2502"))
			sb.WriteString("\n")
		}
	} else {
		lines := strings.Split(sanitizeBoxText(session.LastScreen), "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > outputLines {
			lines = lines[len(lines)-outputLines:]
		}
		for _, line := range lines {
			line = truncateToWidth(line, innerWidth)
			lineWidth := lipgloss.Width(line)
			pad := innerWidth - lineWidth
			if pad < 0 {
				pad = 0
			}
			sb.WriteString(borderStyle.Render(" \u2502") + " " + line + strings.Repeat(" ", pad) + " " + borderStyle.Render("\u2502"))
			sb.WriteString("\n")
		}
		for i := len(lines); i < outputLines; i++ {
			sb.WriteString(borderStyle.Render(" \u2502") + strings.Repeat(" ", innerWidth+2) + borderStyle.Render("\u2502"))
			sb.WriteString("\n")
		}
	}

	// 2. Dashed separator
	dashes := strings.Repeat("\u2500 ", innerWidth/2)
	if lipgloss.Width(dashes) > innerWidth {
		dashes = dashes[:innerWidth]
	}
	sb.WriteString(dimStyle.Render(" \u2502") + " " + dimStyle.Render(dashes))
	pad := innerWidth - lipgloss.Width(dashes)
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(strings.Repeat(" ", pad) + " " + dimStyle.Render("\u2502"))
	sb.WriteString("\n")

	// 3. Chat entries (last 3)
	entries := c.entries
	if len(entries) > 3 {
		entries = entries[len(entries)-3:]
	}
	for _, e := range entries {
		ts := e.Timestamp.Format("15:04")
		entryText := strings.ReplaceAll(e.Text, "\n", " ")
		var prefix string
		switch e.Direction {
		case "sent":
			prefix = fmt.Sprintf("[%s] > ", ts)
		default:
			prefix = fmt.Sprintf("[%s]   ", ts)
		}
		plain := truncateToWidth(prefix+entryText, innerWidth)
		var styled string
		switch e.Direction {
		case "sent":
			styled = chatSentStyle.Render(plain)
		case "received":
			styled = chatReceivedStyle.Render(plain)
		default:
			styled = plain
		}
		textWidth := lipgloss.Width(styled)
		pad = innerWidth - textWidth
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(dimStyle.Render(" \u2502") + " " + styled + strings.Repeat(" ", pad) + " " + dimStyle.Render("\u2502"))
		sb.WriteString("\n")
	}

	// 4. Hint line
	hint := " Ctrl+D:send  Tab:completion  Esc:close"
	hint = truncateToWidth(hint, innerWidth)
	pad = innerWidth - lipgloss.Width(hint)
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(dimStyle.Render(" \u2502") + " " + dimStyle.Render(hint) + strings.Repeat(" ", pad) + " " + dimStyle.Render("\u2502"))
	sb.WriteString("\n")

	// 5. Textarea (rendered inline as plain text since Bubble Tea's textarea.View()
	// doesn't produce bordered output suitable for inline embedding)
	if c.input.Value() != "" || c.input.Focused() {
		text := c.input.Value()
		if text == "" {
			text = c.input.Placeholder
		}
		text = truncateToWidth(text, innerWidth)
		textWidth := lipgloss.Width(text)
		pad = innerWidth - textWidth
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(dimStyle.Render(" \u2502") + " " + text + strings.Repeat(" ", pad) + " " + dimStyle.Render("\u2502"))
		sb.WriteString("\n")
	} else {
		placeholder := truncateToWidth(c.input.Placeholder, innerWidth)
		placeholderWidth := lipgloss.Width(placeholder)
		pad = innerWidth - placeholderWidth
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(dimStyle.Render(" \u2502") + " " + dimStyle.Render(placeholder) + strings.Repeat(" ", pad) + " " + dimStyle.Render("\u2502"))
		sb.WriteString("\n")
	}

	return sb.String()
}
