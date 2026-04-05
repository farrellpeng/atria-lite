package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sethdeckard/atria/internal/lite"
	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

type usageError struct {
	msg string
}

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func (e usageError) Error() string {
	return e.msg
}

func main() {
	os.Exit(run(os.Args[1:]))
}

var runMonitorUI = func(ctx lite.MonitorContext) error {
	client := wezterm.NewClient("")
	model := lite.NewModel(client, ctx)
	_, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}
	if isHelpArg(args[0]) && len(args) == 1 {
		printUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "start":
		if err := runStart(args[1:]); err != nil {
			return reportError(err)
		}
		return 0
	case "monitor":
		if err := runMonitor(args[1:]); err != nil {
			return reportError(err)
		}
		return 0
	default:
		printUsage(os.Stderr)
		return 2
	}
}

func runStart(args []string) error {
	if len(args) == 1 && isHelpArg(args[0]) {
		printUsage(os.Stdout)
		return nil
	}
	if len(args) > 0 {
		return usageError{msg: "start does not accept arguments"}
	}
	return lite.Start(lite.StartOptions{})
}

func runMonitor(args []string) error {
	if len(args) == 1 && isHelpArg(args[0]) {
		printUsage(os.Stdout)
		return nil
	}

	fs := flag.NewFlagSet("monitor", flag.ContinueOnError)
	var flagOutput bytes.Buffer
	fs.SetOutput(&flagOutput)

	var encodedContext string
	fs.StringVar(&encodedContext, "context-base64", "", "")

	if err := fs.Parse(args); err != nil {
		msg := strings.TrimSpace(flagOutput.String())
		if msg == "" {
			msg = err.Error()
		}
		return usageError{msg: msg}
	}
	if fs.NArg() > 0 {
		return usageError{msg: "monitor does not accept positional arguments"}
	}
	if encodedContext == "" {
		return usageError{msg: "--context-base64 is required"}
	}
	ctx, err := lite.DecodeMonitorContext(encodedContext)
	if err != nil {
		return usageError{msg: err.Error()}
	}
	if ctx.SelfPaneID == 0 && ctx.StarterPaneID != 0 && ctx.WindowID != 0 && ctx.TabID != 0 {
		selfPaneID, err := wezterm.CurrentPaneIDFromEnv()
		if err != nil {
			return usageError{msg: err.Error()}
		}
		ctx.SelfPaneID = selfPaneID
	}
	if err := ctx.Validate(); err != nil {
		return usageError{msg: err.Error()}
	}
	return runMonitorUI(ctx)
}

func reportError(err error) int {
	var usageErr usageError
	if errors.As(err, &usageErr) {
		fmt.Fprintf(os.Stderr, "error: %v\n\n%s", err, usageText())
		return 2
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, usageText())
}

func usageText() string {
	return `Usage:
  atria-lite start
  atria-lite monitor --context-base64 <value>

Commands:
  start    Start the lite WezTerm monitor layout
  monitor  Decode a MonitorContext passed via CLI
`
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help"
}
