package lite

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sethdeckard/atria/internal/model"
	"github.com/sethdeckard/atria/internal/terminal"
	"github.com/sethdeckard/atria/internal/terminal/wezterm"
)

const defaultMonitorPercent = 35
const monitorSelfPanePlaceholder = "__SELF_PANE_ID__"

type StartOptions struct {
	WezTermPath    string
	MonitorPercent int
	MonitorCommand []string
}

type wezTermRuntime interface {
	ListPanes() ([]wezterm.PaneInfo, error)
	ListWindowPanes(windowID int) ([]wezterm.PaneInfo, error)
	SplitPane(opts wezterm.SplitPaneOptions) (int, error)
	MovePaneToNewTab(paneID, windowID int) error
	ActivatePane(sessionID string) error
}

var (
	currentPaneIDFromEnv = wezterm.CurrentPaneIDFromEnv
	newWezTermRuntime    = func(path string) wezTermRuntime { return wezTermClientRuntime{Client: wezterm.NewClient(path)} }
)

type wezTermClientRuntime struct {
	*wezterm.Client
}

func (r wezTermClientRuntime) ActivatePane(sessionID string) error {
	return r.FocusSession(sessionID)
}

func Start(opts StartOptions) error {
	starterPaneID, err := currentPaneIDFromEnv()
	if err != nil {
		return fmt.Errorf("resolve current wezterm pane: %w", err)
	}
	return startWithRuntime(newWezTermRuntime(opts.WezTermPath), starterPaneID, opts)
}

func startWithRuntime(runtime wezTermRuntime, starterPaneID int, opts StartOptions) error {
	starterPane, err := findStarterPane(runtime, starterPaneID)
	if err != nil {
		return err
	}

	windowPanes, err := runtime.ListWindowPanes(starterPane.WindowID)
	if err != nil {
		return fmt.Errorf("list window panes for window %d: %w", starterPane.WindowID, err)
	}
	if err := requireWindowPanes(windowPanes, starterPane.WindowID, starterPaneID); err != nil {
		return err
	}

	bindings, overflow := PlanInitialLayout(classifyCandidates(windowPanes, starterPaneID))

	for _, paneID := range overflow {
		windowPanes, err := runtime.ListWindowPanes(starterPane.WindowID)
		if err != nil {
			return fmt.Errorf("recheck window panes for window %d: %w", starterPane.WindowID, err)
		}
		if err := requireWindowPanes(windowPanes, starterPane.WindowID, starterPaneID, paneID); err != nil {
			return err
		}
		if err := runtime.MovePaneToNewTab(paneID, starterPane.WindowID); err != nil {
			return fmt.Errorf("move pane %d to new tab in window %d: %w", paneID, starterPane.WindowID, err)
		}
	}

	windowPanes, err = runtime.ListWindowPanes(starterPane.WindowID)
	if err != nil {
		return fmt.Errorf("recheck window panes before monitor split for window %d: %w", starterPane.WindowID, err)
	}
	if err := requireWindowPanes(windowPanes, starterPane.WindowID, starterPaneID); err != nil {
		return err
	}

	bindings = ShrinkBindings(bindings, paneIDSet(windowPanes))
	ctx := MonitorContext{
		SelfPaneID:       -1,
		StarterPaneID:    starterPaneID,
		WindowID:         starterPane.WindowID,
		TabID:            starterPane.TabID,
		SlotBindings:     bindings,
		WorkspacePaneIDs: bindingPaneIDs(bindings),
	}
	jsonTemplate, err := monitorContextJSONTemplate(ctx)
	if err != nil {
		return fmt.Errorf("build monitor context template: %w", err)
	}

	monitorPaneID, err := runtime.SplitPane(wezterm.SplitPaneOptions{
		PaneID:    starterPaneID,
		Direction: "top",
		TopLevel:  true,
		Percent:   monitorPercent(opts.MonitorPercent),
		Command:   buildMonitorBootstrapCommand(opts.MonitorCommand, jsonTemplate),
	})
	if err != nil {
		return fmt.Errorf("start monitor pane: %w", err)
	}
	if err := runtime.ActivatePane(strconv.Itoa(monitorPaneID)); err != nil {
		return fmt.Errorf("activate monitor pane %d: %w", monitorPaneID, err)
	}

	return nil
}

func findStarterPane(runtime wezTermRuntime, starterPaneID int) (wezterm.PaneInfo, error) {
	panes, err := runtime.ListPanes()
	if err != nil {
		return wezterm.PaneInfo{}, fmt.Errorf("list wezterm panes: %w", err)
	}
	for _, pane := range panes {
		if pane.PaneID == starterPaneID {
			return pane, nil
		}
	}
	return wezterm.PaneInfo{}, fmt.Errorf("starter pane %d was not found", starterPaneID)
}

func classifyCandidates(panes []wezterm.PaneInfo, starterPaneID int) []CandidatePane {
	candidates := make([]CandidatePane, 0, len(panes))
	for _, pane := range panes {
		if pane.PaneID == starterPaneID {
			continue
		}

		agentType := terminal.DetectAgent(pane.Title)
		kind := OccupantNormal
		if agentType != model.AgentType("") {
			kind = OccupantAgent
		}

		candidates = append(candidates, CandidatePane{
			PaneID:    pane.PaneID,
			WindowID:  pane.WindowID,
			TabID:     pane.TabID,
			Title:     pane.Title,
			Kind:      kind,
			AgentType: agentType,
		})
	}
	return candidates
}

func requireWindowPanes(panes []wezterm.PaneInfo, windowID int, requiredPaneIDs ...int) error {
	live := paneIDSet(panes)
	for _, paneID := range requiredPaneIDs {
		if !live[paneID] {
			return fmt.Errorf("pane %d is no longer present in window %d", paneID, windowID)
		}
	}
	return nil
}

func paneIDSet(panes []wezterm.PaneInfo) map[int]bool {
	live := make(map[int]bool, len(panes))
	for _, pane := range panes {
		live[pane.PaneID] = true
	}
	return live
}

func bindingPaneIDs(bindings []SlotBinding) []int {
	ids := make([]int, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.PaneID)
	}
	return ids
}

func monitorPercent(percent int) int {
	if percent == 0 {
		return defaultMonitorPercent
	}
	return percent
}

func buildMonitorBootstrapCommand(prefix []string, jsonTemplate string) []string {
	if len(prefix) == 0 {
		prefix = []string{"atria-lite", "monitor"}
	}

	command := []string{
		"bash",
		"-lc",
		monitorBootstrapScript(),
		"atria-lite-monitor-bootstrap",
		jsonTemplate,
	}
	command = append(command, prefix...)
	return command
}

func monitorContextJSONTemplate(ctx MonitorContext) (string, error) {
	raw, err := json.Marshal(struct {
		SelfPaneID       any           `json:"self_pane_id"`
		StarterPaneID    int           `json:"starter_pane_id"`
		WindowID         int           `json:"window_id"`
		TabID            int           `json:"tab_id"`
		SlotBindings     []SlotBinding `json:"slot_bindings"`
		WorkspacePaneIDs []int         `json:"workspace_pane_ids"`
	}{
		SelfPaneID:       monitorSelfPanePlaceholder,
		StarterPaneID:    ctx.StarterPaneID,
		WindowID:         ctx.WindowID,
		TabID:            ctx.TabID,
		SlotBindings:     ctx.SlotBindings,
		WorkspacePaneIDs: ctx.WorkspacePaneIDs,
	})
	if err != nil {
		return "", err
	}

	template := strings.Replace(string(raw), `"`+monitorSelfPanePlaceholder+`"`, monitorSelfPanePlaceholder, 1)
	return template, nil
}

func monitorBootstrapScript() string {
	return strings.Join([]string{
		`self_pane_id="${WEZTERM_PANE:-}"`,
		`if [ -z "$self_pane_id" ]; then`,
		`  printf 'WEZTERM_PANE is not set\n' >&2`,
		`  exit 1`,
		`fi`,
		`json_template="$1"`,
		`shift`,
		fmt.Sprintf(`json="${json_template//%s/$self_pane_id}"`, monitorSelfPanePlaceholder),
		`blob="$(printf '%s' "$json" | base64 | tr -d '\n=' | tr '+/' '-_')"`,
		`exec "$@" --context-base64 "$blob"`,
	}, "\n")
}
