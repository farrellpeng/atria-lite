package wezterm

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeWeztermStub(t *testing.T, stdout string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.log")
	scriptPath := filepath.Join(dir, "wezterm")
	script := "#!/bin/sh\n" +
		"printf '%s\n' \"$@\" > \"" + argsPath + "\"\n" +
		"cat <<'EOF'\n" + stdout + "\nEOF\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return scriptPath, argsPath
}

func readArgsLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read args log: %v", err)
	}
	if len(data) == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("")
	if c.weztermPath != "wezterm" {
		t.Errorf("expected weztermPath %q, got %q", "wezterm", c.weztermPath)
	}
}

func TestNewClientCustomPath(t *testing.T) {
	c := NewClient("/usr/local/bin/wezterm")
	if c.weztermPath != "/usr/local/bin/wezterm" {
		t.Errorf("expected weztermPath %q, got %q", "/usr/local/bin/wezterm", c.weztermPath)
	}
}

func TestParseListOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantID    int
		wantTitle string
		wantCWD   string
		wantTTY   string
	}{
		{
			name: "single pane",
			input: `[{"window_id": 0, "tab_id": 0, "pane_id": 1, "workspace": "default",
				"title": "claude", "cwd": "/home/user/project", "tty_name": "/dev/pts/0"}]`,
			wantLen:   1,
			wantID:    1,
			wantTitle: "claude",
			wantCWD:   "/home/user/project",
			wantTTY:   "/dev/pts/0",
		},
		{
			name: "multiple panes",
			input: `[
				{"window_id": 0, "tab_id": 0, "pane_id": 1, "workspace": "default",
				 "title": "claude", "cwd": "/tmp", "tty_name": "/dev/pts/0"},
				{"window_id": 0, "tab_id": 1, "pane_id": 2, "workspace": "default",
				 "title": "codex", "cwd": "/home", "tty_name": "/dev/pts/1"},
				{"window_id": 1, "tab_id": 2, "pane_id": 3, "workspace": "default",
				 "title": "zsh", "cwd": "/var", "tty_name": "/dev/pts/2"}
			]`,
			wantLen:   3,
			wantID:    1,
			wantTitle: "claude",
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantLen: 0,
		},
		{
			name:      "missing optional fields",
			input:     `[{"pane_id": 5, "title": "shell", "cwd": "", "tty_name": ""}]`,
			wantLen:   1,
			wantID:    5,
			wantTitle: "shell",
		},
		{
			name: "CWD with file:// URI",
			input: `[{"pane_id": 1, "title": "claude", "cwd": "file:///Users/test/project",
				"tty_name": "/dev/ttys001"}]`,
			wantLen: 1,
			wantID:  1,
			wantCWD: "file:///Users/test/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseListOutput([]byte(tt.input))
			if err != nil {
				t.Fatalf("parseListOutput() error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("parseListOutput() returned %d entries, want %d", len(got), tt.wantLen)
			}
			if tt.wantLen == 0 {
				return
			}
			if got[0].PaneID != tt.wantID {
				t.Errorf("PaneID = %d, want %d", got[0].PaneID, tt.wantID)
			}
			if tt.wantTitle != "" && got[0].Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", got[0].Title, tt.wantTitle)
			}
			if tt.wantCWD != "" && got[0].CWD != tt.wantCWD {
				t.Errorf("CWD = %q, want %q", got[0].CWD, tt.wantCWD)
			}
			if tt.wantTTY != "" && got[0].TTYName != tt.wantTTY {
				t.Errorf("TTYName = %q, want %q", got[0].TTYName, tt.wantTTY)
			}
		})
	}
}

func TestParseListOutputInvalidJSON(t *testing.T) {
	_, err := parseListOutput([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCurrentPaneIDFromEnv(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "42")
	got, err := CurrentPaneIDFromEnv()
	if err != nil {
		t.Fatalf("CurrentPaneIDFromEnv() error: %v", err)
	}
	if got != 42 {
		t.Fatalf("CurrentPaneIDFromEnv() = %d, want 42", got)
	}
}

func TestCurrentPaneIDFromEnvInvalid(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "not-an-int")
	_, err := CurrentPaneIDFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid WEZTERM_PANE")
	}
}

func TestListPanesStructuresFields(t *testing.T) {
	scriptPath, _ := writeWeztermStub(t, `[
		{"window_id": 7, "tab_id": 1, "pane_id": 11, "workspace": "default", "title": "claude", "cwd": "file:///tmp/a", "tty_name": "/dev/pts/1"},
		{"window_id": 7, "tab_id": 2, "pane_id": 22, "workspace": "work", "title": "codex", "cwd": "/tmp/b", "tty_name": "/dev/pts/2"}
	]`)
	t.Setenv("WEZTERM_PANE", "22")

	c := NewClient(scriptPath)
	panes, err := c.ListPanes()
	if err != nil {
		t.Fatalf("ListPanes() error: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("ListPanes() len = %d, want 2", len(panes))
	}

	first := panes[0]
	if first.WindowID != 7 {
		t.Fatalf("first.WindowID = %d, want 7", first.WindowID)
	}
	if first.TabID != 1 {
		t.Fatalf("first.TabID = %d, want 1", first.TabID)
	}
	if first.Workspace != "default" {
		t.Fatalf("first.Workspace = %q, want %q", first.Workspace, "default")
	}
	if first.Title != "claude" {
		t.Fatalf("first.Title = %q, want %q", first.Title, "claude")
	}
	if first.TTYName != "/dev/pts/1" {
		t.Fatalf("first.TTYName = %q, want %q", first.TTYName, "/dev/pts/1")
	}
	if first.CWD != "/tmp/a" {
		t.Fatalf("first.CWD = %q, want %q", first.CWD, "/tmp/a")
	}
	if first.IsSelf {
		t.Fatal("expected first pane to be self=false")
	}
	if first.IsActive {
		t.Fatal("expected first pane to be inactive")
	}

	second := panes[1]
	if second.WindowID != 7 {
		t.Fatalf("second.WindowID = %d, want 7", second.WindowID)
	}
	if second.TabID != 2 {
		t.Fatalf("second.TabID = %d, want 2", second.TabID)
	}
	if second.Workspace != "work" {
		t.Fatalf("second.Workspace = %q, want %q", second.Workspace, "work")
	}
	if second.Title != "codex" {
		t.Fatalf("second.Title = %q, want %q", second.Title, "codex")
	}
	if second.TTYName != "/dev/pts/2" {
		t.Fatalf("second.TTYName = %q, want %q", second.TTYName, "/dev/pts/2")
	}
	if second.CWD != "/tmp/b" {
		t.Fatalf("second.CWD = %q, want %q", second.CWD, "/tmp/b")
	}
	if !second.IsSelf {
		t.Fatal("expected second pane to be self")
	}
	if !second.IsActive {
		t.Fatal("expected second pane to be active")
	}
}

func TestListPanesWithoutCurrentPaneEnv(t *testing.T) {
	scriptPath, _ := writeWeztermStub(t, `[
		{"window_id": 3, "tab_id": 4, "pane_id": 5, "workspace": "default", "title": "shell", "cwd": "file:///tmp/project", "tty_name": "/dev/pts/9"}
	]`)
	t.Setenv("WEZTERM_PANE", "")

	c := NewClient(scriptPath)
	panes, err := c.ListPanes()
	if err != nil {
		t.Fatalf("ListPanes() error: %v", err)
	}
	if len(panes) != 1 {
		t.Fatalf("ListPanes() len = %d, want 1", len(panes))
	}

	pane := panes[0]
	if pane.WindowID != 3 {
		t.Fatalf("WindowID = %d, want 3", pane.WindowID)
	}
	if pane.TabID != 4 {
		t.Fatalf("TabID = %d, want 4", pane.TabID)
	}
	if pane.Workspace != "default" {
		t.Fatalf("Workspace = %q, want %q", pane.Workspace, "default")
	}
	if pane.Title != "shell" {
		t.Fatalf("Title = %q, want %q", pane.Title, "shell")
	}
	if pane.TTYName != "/dev/pts/9" {
		t.Fatalf("TTYName = %q, want %q", pane.TTYName, "/dev/pts/9")
	}
	if pane.CWD != "/tmp/project" {
		t.Fatalf("CWD = %q, want %q", pane.CWD, "/tmp/project")
	}
	if pane.IsSelf {
		t.Fatal("expected IsSelf=false when WEZTERM_PANE is unset")
	}
	if pane.IsActive {
		t.Fatal("expected IsActive=false when WEZTERM_PANE is unset")
	}
}

func TestListSessionsWithoutCurrentPaneEnv(t *testing.T) {
	scriptPath, _ := writeWeztermStub(t, `[
		{"window_id": 3, "tab_id": 4, "pane_id": 5, "workspace": "default", "title": "shell", "cwd": "/tmp/project", "tty_name": "/dev/pts/9"}
	]`)
	t.Setenv("WEZTERM_PANE", "")

	c := NewClient(scriptPath)
	sessions, err := c.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("ListSessions() len = %d, want 1", len(sessions))
	}

	session := sessions[0]
	if session.ID != "5" {
		t.Fatalf("ID = %q, want %q", session.ID, "5")
	}
	if session.Name != "shell" {
		t.Fatalf("Name = %q, want %q", session.Name, "shell")
	}
	if session.TTY != "/dev/pts/9" {
		t.Fatalf("TTY = %q, want %q", session.TTY, "/dev/pts/9")
	}
}

func TestListWindowPanesFiltersByWindow(t *testing.T) {
	scriptPath, _ := writeWeztermStub(t, `[
		{"window_id": 7, "tab_id": 1, "pane_id": 11, "workspace": "default", "title": "claude", "cwd": "file:///tmp/a", "tty_name": "/dev/pts/1"},
		{"window_id": 8, "tab_id": 2, "pane_id": 22, "workspace": "default", "title": "codex", "cwd": "/tmp/b", "tty_name": "/dev/pts/2"},
		{"window_id": 7, "tab_id": 3, "pane_id": 33, "workspace": "default", "title": "shell", "cwd": "/tmp/c", "tty_name": "/dev/pts/3"}
	]`)
	t.Setenv("WEZTERM_PANE", "33")

	c := NewClient(scriptPath)
	panes, err := c.ListWindowPanes(7)
	if err != nil {
		t.Fatalf("ListWindowPanes() error: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("ListWindowPanes() len = %d, want 2", len(panes))
	}

	gotIDs := []int{panes[0].PaneID, panes[1].PaneID}
	wantIDs := []int{11, 33}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("pane ids = %v, want %v", gotIDs, wantIDs)
	}
	if panes[0].CWD != "/tmp/a" {
		t.Fatalf("pane 11 cwd = %q, want %q", panes[0].CWD, "/tmp/a")
	}
	if !panes[1].IsActive {
		t.Fatal("expected pane 33 to be marked active")
	}
}

func TestSplitPaneUsesTopLevelAndPercent(t *testing.T) {
	scriptPath, argsPath := writeWeztermStub(t, "123\n")
	c := NewClient(scriptPath)

	got, err := c.SplitPane(SplitPaneOptions{
		PaneID:    9,
		Direction: "top",
		TopLevel:  true,
		Percent:   35,
		CWD:       "/tmp/project",
		Command:   []string{"bash", "-l"},
	})
	if err != nil {
		t.Fatalf("SplitPane() error: %v", err)
	}
	if got != 123 {
		t.Fatalf("SplitPane() = %d, want 123", got)
	}

	gotArgs := readArgsLog(t, argsPath)
	wantArgs := []string{
		"cli",
		"split-pane",
		"--pane-id", "9",
		"--top",
		"--top-level",
		"--percent", "35",
		"--cwd", "/tmp/project",
		"--",
		"bash",
		"-l",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("SplitPane args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestSplitPaneOmitsPercentWhenZero(t *testing.T) {
	scriptPath, argsPath := writeWeztermStub(t, "123\n")
	c := NewClient(scriptPath)

	if _, err := c.SplitPane(SplitPaneOptions{Direction: "top", Percent: 0}); err != nil {
		t.Fatalf("SplitPane() error: %v", err)
	}

	gotArgs := readArgsLog(t, argsPath)
	wantArgs := []string{"cli", "split-pane", "--top"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("SplitPane args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestSplitPaneValidatesPercent(t *testing.T) {
	c := NewClient("wezterm")
	for _, tc := range []struct {
		name    string
		percent int
	}{
		{name: "negative", percent: -1},
		{name: "too large", percent: 101},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.SplitPane(SplitPaneOptions{Percent: tc.percent})
			if err == nil {
				t.Fatalf("expected error for percent %d", tc.percent)
			}
		})
	}
}

func TestMovePaneToNewTabUsesWindowID(t *testing.T) {
	scriptPath, argsPath := writeWeztermStub(t, "")
	c := NewClient(scriptPath)

	if err := c.MovePaneToNewTab(55, 77); err != nil {
		t.Fatalf("MovePaneToNewTab() error: %v", err)
	}

	gotArgs := readArgsLog(t, argsPath)
	wantArgs := []string{
		"cli",
		"move-pane-to-new-tab",
		"--pane-id", "55",
		"--window-id", "77",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("MovePaneToNewTab args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestMovePaneToNewTabValidatesIDs(t *testing.T) {
	c := NewClient("wezterm")
	tests := []struct {
		name     string
		paneID   int
		windowID int
	}{
		{name: "zero pane", paneID: 0, windowID: 1},
		{name: "negative pane", paneID: -1, windowID: 1},
		{name: "zero window", paneID: 1, windowID: 0},
		{name: "negative window", paneID: 1, windowID: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.MovePaneToNewTab(tt.paneID, tt.windowID)
			if err == nil {
				t.Fatalf("expected error for paneID=%d windowID=%d", tt.paneID, tt.windowID)
			}
		})
	}
}

func TestNormalizeCWD(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain path", "/Users/test/project", "/Users/test/project"},
		{"file URI", "file:///Users/test/project", "/Users/test/project"},
		{"file URI with hostname", "file://localhost/Users/test/project", "/Users/test/project"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCWD(tt.input)
			if got != tt.want {
				t.Errorf("normalizeCWD(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTrimToLastN(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"fewer lines than n", "a\nb\nc", 5, "a\nb\nc"},
		{"exact lines", "a\nb\nc", 3, "a\nb\nc"},
		{"more lines than n", "a\nb\nc\nd\ne", 3, "c\nd\ne"},
		{"single line", "hello", 3, "hello"},
		{"empty string", "", 3, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimToLastN(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("trimToLastN(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestMonitorOutputUnsupported(t *testing.T) {
	c := NewClient("")
	pid, err := c.MonitorOutput("1", "/tmp/log", "pattern")
	if err == nil {
		t.Fatal("expected error from MonitorOutput")
	}
	if pid != 0 {
		t.Errorf("expected pid 0, got %d", pid)
	}
}

func TestGetVarUnsupported(t *testing.T) {
	c := NewClient("")
	_, err := c.GetVar("1", "title")
	if err == nil {
		t.Fatal("expected error for unsupported variable")
	}
}
