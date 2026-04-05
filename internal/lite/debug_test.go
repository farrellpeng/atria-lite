package lite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

func TestLiteDebugfWritesWhenEnvSet(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "lite-debug.log")
	t.Setenv("ATRIA_LITE_DEBUG", logPath)

	liteDebugf("workspace=%v", []int{1, 2})

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "workspace=[1 2]") {
		t.Fatalf("log = %q, want workspace entry", string(data))
	}
}

func TestWaitForWorkspaceAnchorPollsUntilPaneSettlesBelowMonitor(t *testing.T) {
	client := &stubWindowPaneClient{
		paneSequence: [][]wezterm.PaneInfo{
			{
				{PaneID: 200, WindowID: 7, TabID: 70, TopRow: 0, Rows: 10},
				{PaneID: 11, WindowID: 7, TabID: 70, TopRow: 0, Rows: 20},
			},
			{
				{PaneID: 200, WindowID: 7, TabID: 70, TopRow: 0, Rows: 10},
				{PaneID: 11, WindowID: 7, TabID: 70, TopRow: 11, Rows: 20},
			},
		},
	}

	originalSleep := sleepForWorkspaceSettle
	defer func() { sleepForWorkspaceSettle = originalSleep }()
	sleepCalls := 0
	sleepForWorkspaceSettle = func(time.Duration) {
		sleepCalls++
	}

	waitForWorkspaceAnchor(client, MonitorContext{
		SelfPaneID: 200,
		WindowID:   7,
		TabID:      70,
	}, 11)

	if client.listWindowCalls != 2 {
		t.Fatalf("ListWindowPanes() calls = %d, want 2", client.listWindowCalls)
	}
	if sleepCalls != 1 {
		t.Fatalf("sleep calls = %d, want 1", sleepCalls)
	}
}
