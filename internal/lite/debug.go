package lite

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

var liteDebugMu sync.Mutex

func liteDebugf(format string, args ...any) {
	logPath := os.Getenv("ATRIA_LITE_DEBUG")
	if logPath == "" {
		return
	}

	liteDebugMu.Lock()
	defer liteDebugMu.Unlock()

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	line := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339Nano), line)
}

func summarizePaneInfos(panes []wezterm.PaneInfo) []string {
	out := make([]string, 0, len(panes))
	for _, pane := range panes {
		out = append(out, fmt.Sprintf("pane=%d tab=%d title=%q cols=%d", pane.PaneID, pane.TabID, pane.Title, pane.Cols))
	}
	return out
}

func summarizeCandidates(panes []CandidatePane) []string {
	out := make([]string, 0, len(panes))
	for _, pane := range panes {
		out = append(out, fmt.Sprintf("pane=%d tab=%d kind=%s agent=%s title=%q", pane.PaneID, pane.TabID, pane.Kind, pane.AgentType, pane.Title))
	}
	return out
}

func summarizeCandidatePtr(pane *CandidatePane) string {
	if pane == nil {
		return "<nil>"
	}
	return fmt.Sprintf("pane=%d tab=%d kind=%s agent=%s title=%q", pane.PaneID, pane.TabID, pane.Kind, pane.AgentType, pane.Title)
}
