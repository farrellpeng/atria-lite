package wezterm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sethdeckard/atria/internal/terminal"
)

// Client implements terminal.Backend using the wezterm CLI.
// Communication uses WezTerm's Unix socket (auto-discovered via WEZTERM_UNIX_SOCKET).
type Client struct {
	weztermPath string
}

// PaneInfo is a structured snapshot of a WezTerm pane.
type PaneInfo struct {
	WindowID  int
	TabID     int
	PaneID    int
	Workspace string
	Title     string
	CWD       string
	Cols      int
	TTYName   string
	IsSelf    bool
	// IsActive is kept as a compatibility alias for IsSelf.
	// It does not mean the pane is currently focused in the WezTerm UI.
	IsActive bool
}

// SplitPaneOptions controls how SplitPane arranges a new pane.
type SplitPaneOptions struct {
	PaneID     int
	Direction  string // "top", "right", "bottom", "left"
	TopLevel   bool
	Percent    int
	CWD        string
	MovePaneID int
	Command    []string
}

// NewClient creates a new WezTerm Client. Empty weztermPath defaults to "wezterm".
func NewClient(weztermPath string) *Client {
	if weztermPath == "" {
		weztermPath = "wezterm"
	}
	return &Client{weztermPath: weztermPath}
}

// run executes wezterm cli with the given arguments and returns stdout.
func (c *Client) run(args ...string) ([]byte, error) {
	fullArgs := append([]string{"cli"}, args...)
	cmd := exec.Command(c.weztermPath, fullArgs...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("wezterm cli %v failed: %s", args, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("wezterm cli %v failed: %w", args, err)
	}
	return out, nil
}

// listEntry represents a single pane from wezterm cli list --format json.
type listEntry struct {
	WindowID  int    `json:"window_id"`
	TabID     int    `json:"tab_id"`
	PaneID    int    `json:"pane_id"`
	Workspace string `json:"workspace"`
	Title     string `json:"title"`
	CWD       string `json:"cwd"`
	Size      struct {
		Cols int `json:"cols"`
	} `json:"size"`
	TTYName string `json:"tty_name"`
}

func (e listEntry) toPaneInfo(selfPaneID int) PaneInfo {
	isSelf := selfPaneID != 0 && e.PaneID == selfPaneID
	return PaneInfo{
		WindowID:  e.WindowID,
		TabID:     e.TabID,
		PaneID:    e.PaneID,
		Workspace: e.Workspace,
		Title:     e.Title,
		CWD:       normalizeCWD(e.CWD),
		Cols:      e.Size.Cols,
		TTYName:   e.TTYName,
		IsSelf:    isSelf,
		IsActive:  isSelf,
	}
}

// parseListOutput parses the flat JSON array from wezterm cli list.
func parseListOutput(data []byte) ([]listEntry, error) {
	var entries []listEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse wezterm cli list: %w", err)
	}
	return entries, nil
}

// normalizeCWD strips the file:// URI prefix that WezTerm may use for CWD values.
func normalizeCWD(raw string) string {
	if !strings.HasPrefix(raw, "file://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		// Fallback: strip prefix manually.
		return strings.TrimPrefix(raw, "file://")
	}
	return u.Path
}

// CurrentPaneIDFromEnv returns the pane id from WEZTERM_PANE.
func CurrentPaneIDFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv("WEZTERM_PANE"))
	if raw == "" {
		return 0, fmt.Errorf("WEZTERM_PANE is not set")
	}
	id, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse WEZTERM_PANE %q: %w", raw, err)
	}
	return id, nil
}

// Available checks if wezterm is installed and its CLI can reach a running
// instance. Unlike Kitty, wezterm cli auto-discovers the Unix socket without
// needing WEZTERM_UNIX_SOCKET, so this succeeds as long as any WezTerm
// instance is reachable — enabling the "enabled but inactive" state when
// Atria runs outside WezTerm.
func (c *Client) Available() error {
	path, err := exec.LookPath(c.weztermPath)
	if err != nil {
		return fmt.Errorf("wezterm not found in PATH")
	}
	c.weztermPath = path

	// Probe with list to verify connectivity. wezterm cli auto-discovers
	// the socket, so this works from any terminal as long as WezTerm is running.
	if _, err := c.run("list", "--format", "json"); err != nil {
		return fmt.Errorf("wezterm cli probe failed: %w", err)
	}
	return nil
}

// ListSessions returns all WezTerm panes as terminal sessions.
func (c *Client) ListSessions() ([]terminal.Session, error) {
	panes, err := c.ListPanes()
	if err != nil {
		return nil, err
	}
	sessions := make([]terminal.Session, 0, len(panes))
	for _, e := range panes {
		sessions = append(sessions, terminal.Session{
			ID:   strconv.Itoa(e.PaneID),
			Name: e.Title,
			TTY:  e.TTYName,
		})
	}
	return sessions, nil
}

// ListPanes returns structured information about all WezTerm panes.
func (c *Client) ListPanes() ([]PaneInfo, error) {
	out, err := c.run("list", "--format", "json")
	if err != nil {
		return nil, err
	}
	entries, err := parseListOutput(out)
	if err != nil {
		return nil, err
	}
	activePaneID, err := CurrentPaneIDFromEnv()
	if err != nil {
		activePaneID = 0
	}
	panes := make([]PaneInfo, 0, len(entries))
	for _, entry := range entries {
		panes = append(panes, entry.toPaneInfo(activePaneID))
	}
	return panes, nil
}

// ListWindowPanes returns structured pane info for a single window.
func (c *Client) ListWindowPanes(windowID int) ([]PaneInfo, error) {
	panes, err := c.ListPanes()
	if err != nil {
		return nil, err
	}
	out := make([]PaneInfo, 0, len(panes))
	for _, pane := range panes {
		if pane.WindowID == windowID {
			out = append(out, pane)
		}
	}
	return out, nil
}

// NewSession launches a new window in WezTerm and returns its pane ID.
func (c *Client) NewSession() (string, error) {
	out, err := c.run("spawn")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SendText sends literal text to a WezTerm pane via stdin to avoid shell escaping.
func (c *Client) SendText(sessionID, text string) error {
	fullArgs := []string{"cli", "send-text", "--pane-id", sessionID, "--no-paste"}
	cmd := exec.Command(c.weztermPath, fullArgs...)
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wezterm cli send-text failed: %s", string(out))
	}
	return nil
}

// RunCommand sends a command string followed by Enter to a WezTerm pane.
func (c *Client) RunCommand(sessionID, cmd string) error {
	if err := c.SendText(sessionID, cmd); err != nil {
		return err
	}
	return c.SendText(sessionID, "\r")
}

// FocusSession activates the WezTerm pane with the given ID.
func (c *Client) FocusSession(sessionID string) error {
	_, err := c.run("activate-pane", "--pane-id", sessionID)
	return err
}

// ActivatePane activates the WezTerm pane with the given ID.
func (c *Client) ActivatePane(sessionID string) error {
	return c.FocusSession(sessionID)
}

// ActivateTab activates the WezTerm tab with the given ID.
func (c *Client) ActivateTab(tabID int) error {
	if tabID <= 0 {
		return fmt.Errorf("tabID must be positive")
	}
	_, err := c.run("activate-tab", "--tab-id", strconv.Itoa(tabID))
	return err
}

// SplitPane creates a new pane with the requested layout.
func (c *Client) SplitPane(opts SplitPaneOptions) (int, error) {
	if opts.Percent < 0 || opts.Percent > 100 {
		return 0, fmt.Errorf("percent must be between 0 and 100")
	}
	args := []string{"split-pane"}
	if opts.PaneID != 0 {
		args = append(args, "--pane-id", strconv.Itoa(opts.PaneID))
	}
	switch opts.Direction {
	case "":
	case "top":
		args = append(args, "--top")
	case "right":
		args = append(args, "--right")
	case "bottom":
		args = append(args, "--bottom")
	case "left":
		args = append(args, "--left")
	default:
		return 0, fmt.Errorf("unsupported split direction: %s", opts.Direction)
	}
	if opts.TopLevel {
		args = append(args, "--top-level")
	}
	if opts.Percent > 0 {
		args = append(args, "--percent", strconv.Itoa(opts.Percent))
	}
	if opts.CWD != "" {
		args = append(args, "--cwd", opts.CWD)
	}
	if opts.MovePaneID > 0 {
		args = append(args, "--move-pane-id", strconv.Itoa(opts.MovePaneID))
	}
	if len(opts.Command) > 0 {
		args = append(args, "--")
		args = append(args, opts.Command...)
	}
	out, err := c.run(args...)
	if err != nil {
		return 0, err
	}
	paneID, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse split-pane pane id: %w", err)
	}
	return paneID, nil
}

// MovePaneToNewTab moves a pane to a new tab in the specified window.
func (c *Client) MovePaneToNewTab(paneID, windowID int) error {
	if paneID < 0 {
		return fmt.Errorf("paneID must be non-negative")
	}
	if windowID < 0 {
		return fmt.Errorf("windowID must be non-negative")
	}

	args := []string{"move-pane-to-new-tab"}
	if paneID > 0 {
		args = append(args, "--pane-id", strconv.Itoa(paneID))
	}
	if windowID > 0 {
		args = append(args, "--window-id", strconv.Itoa(windowID))
	}
	_, err := c.run(args...)
	return err
}

// AdjustPaneSize resizes a pane toward the given direction.
func (c *Client) AdjustPaneSize(paneID int, direction string, amount int) error {
	if paneID <= 0 {
		return fmt.Errorf("paneID must be positive")
	}
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	switch direction {
	case "Left", "Right", "Up", "Down", "Next", "Prev":
	default:
		return fmt.Errorf("unsupported direction: %s", direction)
	}
	_, err := c.run("adjust-pane-size", "--pane-id", strconv.Itoa(paneID), "--amount", strconv.Itoa(amount), direction)
	return err
}

// ReadScreen captures the visible screen text from a WezTerm pane.
func (c *Client) ReadScreen(sessionID string, lines int) (string, error) {
	out, err := c.run("get-text", "--pane-id", sessionID)
	if err != nil {
		return "", err
	}
	return terminal.TrimScreenTail(string(out), lines), nil
}

func trimToLastN(text string, n int) string {
	return terminal.TrimScreenTail(text, n)
}

// GetVar reads a variable from a WezTerm pane. Supported: "path".
func (c *Client) GetVar(sessionID, varName string) (string, error) {
	if varName != "path" {
		return "", fmt.Errorf("unsupported variable: %s", varName)
	}
	panes, err := c.ListPanes()
	if err != nil {
		return "", err
	}
	for _, pane := range panes {
		if strconv.Itoa(pane.PaneID) == sessionID {
			return pane.CWD, nil
		}
	}
	return "", fmt.Errorf("pane %s not found", sessionID)
}

// MonitorOutput is not supported by the WezTerm backend. Screen reads are the
// primary status detection mechanism.
func (c *Client) MonitorOutput(sessionID, logPath, patterns string) (int, error) {
	return 0, fmt.Errorf("wezterm backend does not support output monitoring")
}
