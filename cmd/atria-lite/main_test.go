package main

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sethdeckard/atria/internal/lite"
)

func TestRunHelpAndMonitorBehavior(t *testing.T) {
	var monitorCalls []lite.MonitorContext
	oldRunMonitorUI := runMonitorUI
	oldRunLiteStart := runLiteStart
	runMonitorUI = func(ctx lite.MonitorContext) error {
		monitorCalls = append(monitorCalls, ctx)
		return nil
	}
	runLiteStart = lite.Start
	t.Cleanup(func() {
		runMonitorUI = oldRunMonitorUI
		runLiteStart = oldRunLiteStart
	})

	validContext, err := lite.EncodeMonitorContext(lite.MonitorContext{
		SelfPaneID:    1,
		StarterPaneID: 2,
		WindowID:      3,
		TabID:         4,
	})
	if err != nil {
		t.Fatalf("EncodeMonitorContext() error = %v", err)
	}
	contextMissingSelf, err := lite.EncodeMonitorContext(lite.MonitorContext{
		StarterPaneID: 2,
		WindowID:      3,
		TabID:         4,
	})
	if err != nil {
		t.Fatalf("EncodeMonitorContext() missing self error = %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		selfPaneID string
		wantCode   int
		wantCalls  int
		wantCtx    *lite.MonitorContext
		wantStdout string
		wantStderr string
		stderrLike string
	}{
		{
			name:       "top level short help",
			args:       []string{"-h"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "top level help",
			args:       []string{"--help"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "start help",
			args:       []string{"start", "-h"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "start long help",
			args:       []string{"start", "--help"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "monitor help",
			args:       []string{"monitor", "--help"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "monitor short help",
			args:       []string{"monitor", "-h"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:      "monitor valid context",
			args:      []string{"monitor", "--context-base64", validContext},
			wantCode:  0,
			wantCalls: 1,
			wantCtx: &lite.MonitorContext{
				SelfPaneID:    1,
				StarterPaneID: 2,
				WindowID:      3,
				TabID:         4,
			},
		},
		{
			name:       "monitor fills self pane id from env when context is missing it",
			args:       []string{"monitor", "--context-base64", contextMissingSelf},
			selfPaneID: "99",
			wantCode:   0,
			wantCalls:  1,
			wantCtx: &lite.MonitorContext{
				SelfPaneID:    99,
				StarterPaneID: 2,
				WindowID:      3,
				TabID:         4,
			},
		},
		{
			name: "monitor fills self pane id from env when window id is zero",
			args: func() []string {
				zeroWindowContext, err := lite.EncodeMonitorContext(lite.MonitorContext{
					StarterPaneID: 2,
					WindowID:      0,
					TabID:         4,
				})
				if err != nil {
					t.Fatalf("EncodeMonitorContext() zero window error = %v", err)
				}
				return []string{"monitor", "--context-base64", zeroWindowContext}
			}(),
			selfPaneID: "99",
			wantCode:   0,
			wantCalls:  1,
			wantCtx: &lite.MonitorContext{
				SelfPaneID:    99,
				StarterPaneID: 2,
				WindowID:      0,
				TabID:         4,
			},
		},
		{
			name: "monitor fills self pane id from env when starter pane id is zero",
			args: func() []string {
				zeroStarterContext, err := lite.EncodeMonitorContext(lite.MonitorContext{
					StarterPaneID: 0,
					WindowID:      0,
					TabID:         4,
				})
				if err != nil {
					t.Fatalf("EncodeMonitorContext() zero starter error = %v", err)
				}
				return []string{"monitor", "--context-base64", zeroStarterContext}
			}(),
			selfPaneID: "99",
			wantCode:   0,
			wantCalls:  1,
			wantCtx: &lite.MonitorContext{
				SelfPaneID:    99,
				StarterPaneID: 0,
				WindowID:      0,
				TabID:         4,
			},
		},
		{
			name:       "monitor invalid context",
			args:       []string{"monitor", "--context-base64", "e30"},
			wantCode:   2,
			stderrLike: "self pane id is required",
		},
		{
			name:       "monitor unknown flag",
			args:       []string{"monitor", "--bad-flag"},
			wantCode:   2,
			stderrLike: "flag provided but not defined: -bad-flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitorCalls = nil
			if tt.selfPaneID != "" {
				t.Setenv("WEZTERM_PANE", tt.selfPaneID)
			}
			code, stdout, stderr := captureRun(t, tt.args)
			if code != tt.wantCode {
				t.Fatalf("run() exit code = %d, want %d", code, tt.wantCode)
			}
			if tt.wantStdout != "" && !strings.Contains(stdout, tt.wantStdout) {
				t.Fatalf("stdout %q does not contain %q", stdout, tt.wantStdout)
			}
			if tt.wantStdout == "" && stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr, tt.wantStderr) {
				t.Fatalf("stderr %q does not contain %q", stderr, tt.wantStderr)
			}
			if tt.stderrLike != "" && !strings.Contains(stderr, tt.stderrLike) {
				t.Fatalf("stderr %q does not contain %q", stderr, tt.stderrLike)
			}
			if len(monitorCalls) != tt.wantCalls {
				t.Fatalf("runMonitorUI call count = %d, want %d", len(monitorCalls), tt.wantCalls)
			}
			if tt.wantCtx != nil && !reflect.DeepEqual(monitorCalls[0], *tt.wantCtx) {
				t.Fatalf("runMonitorUI ctx = %#v, want %#v", monitorCalls[0], *tt.wantCtx)
			}
		})
	}
}

func TestRunStartUsesCurrentExecutableForMonitorPane(t *testing.T) {
	var gotOpts []lite.StartOptions
	oldRunLiteStart := runLiteStart
	runLiteStart = func(opts lite.StartOptions) error {
		gotOpts = append(gotOpts, opts)
		return nil
	}
	t.Cleanup(func() {
		runLiteStart = oldRunLiteStart
	})

	code, stdout, stderr := captureRun(t, []string{"start"})
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0, stderr=%q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if len(gotOpts) != 1 {
		t.Fatalf("runLiteStart() calls = %d, want 1", len(gotOpts))
	}
	if len(gotOpts[0].MonitorCommand) != 2 {
		t.Fatalf("MonitorCommand = %v, want 2 args", gotOpts[0].MonitorCommand)
	}
	if gotOpts[0].MonitorCommand[1] != "monitor" {
		t.Fatalf("MonitorCommand[1] = %q, want %q", gotOpts[0].MonitorCommand[1], "monitor")
	}
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	if gotOpts[0].MonitorCommand[0] != exePath {
		t.Fatalf("MonitorCommand[0] = %q, want %q", gotOpts[0].MonitorCommand[0], exePath)
	}
}

func captureRun(t *testing.T, args []string) (int, string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() stdout = %v", err)
	}
	defer stdoutR.Close()

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() stderr = %v", err)
	}
	defer stderrR.Close()

	os.Stdout = stdoutW
	os.Stderr = stderrW

	code := run(args)

	if err := stdoutW.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	if err := stderrW.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}

	var stdoutBuf bytes.Buffer
	if _, err := io.Copy(&stdoutBuf, stdoutR); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	var stderrBuf bytes.Buffer
	if _, err := io.Copy(&stderrBuf, stderrR); err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	return code, stdoutBuf.String(), stderrBuf.String()
}
