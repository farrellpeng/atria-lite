# Codex Quota Display Design (atria-lite)

## Summary

Display Codex token usage quota in atria-lite's monitor panel for Codex agent panes. Data is sourced from the Codex app-server JSON-RPC API (`account/rateLimits/read`), providing 5-hour and 7-day window usage percentages with reset countdowns.

**Scope: atria-lite only.** This feature is implemented exclusively in `internal/lite/` and `cmd/atria-lite/`. The main atria TUI (`internal/tui/`) is not affected.

## Motivation

Codex's idle status bar shows `gpt-5.3-codex default · 73% left · ~/projects/foo` — a single percentage. The app-server API provides richer data: dual-window (5h/7d) utilization, reset timestamps, and window duration. Displaying this in atria-lite's status column lets users track quota without leaving the monitor.

Reference implementation: [agent-quota.wezterm](https://github.com/M-Marbouh/agent-quota.wezterm) — a WezTerm status bar plugin that fetches both Claude and Codex quota. Atria-lite will implement a Go-native equivalent for Codex.

## Data Source

### Codex app-server JSON-RPC

Protocol: JSON-RPC over stdio (JSONL).

1. Launch `codex app-server --listen stdio://`
2. Send `initialize` (id=1) with client info
3. Send `account/rateLimits/read` (id=2)
4. Parse response: `rateLimits.primary.usedPercent`, `rateLimits.primary.resetsAt` (Unix timestamp), `rateLimits.secondary.*`

No python3 dependency — Go communicates directly via `os/exec` stdin/stdout pipes.

### Error handling

- codex binary not found → silent skip, no error
- Timeout (15s first fetch, 8s subsequent) or parse failure → `Fetch()` returns last cached data internally; on fresh failure, returns nil
- Rate limit is account-level, not session-level

## Architecture

### New package: `internal/codex/client.go`

```go
type QuotaInfo struct {
    PrimaryPct      float64    // 5h window usage %
    PrimaryReset    string     // "2h10m" human-readable countdown
    SecondaryPct    float64    // 7d window usage % (0 = no data)
    SecondaryReset  string     // 7d countdown
    FetchedAt       time.Time
}

type Client struct {
    codexBin string           // resolved codex binary path, "" means no-op
    cache    *QuotaInfo
    cacheTTL time.Duration    // default 60s
    mu       sync.RWMutex
}

func NewClient() *Client                      // finds codex binary; empty codexBin if not found
func (c *Client) Available() bool             // returns codexBin != ""
func (c *Client) Fetch() *QuotaInfo           // runs JSON-RPC, returns quota (cached on error)
```

`Client` is always non-nil. When codex binary is not found, `Available()` returns false and `Fetch()` returns nil. This avoids nil-checks at every call site.

### Codex binary discovery

1. `os/exec.LookPath("codex")`
2. `$HOME/.nvm/versions/node/*/bin/codex` (pick highest version)
3. Common paths: `~/.local/bin/codex`, `~/.volta/bin/codex`, `/usr/local/bin/codex`, `/opt/homebrew/bin/codex`
4. Not found → `codexBin` is empty, `Available()` returns false, all quota logic skipped

## atria-lite Integration

### Model changes: `internal/lite/model.go`

The `Model` struct gains two new fields:

```go
type Model struct {
    // ... existing fields ...
    codexClient *codex.Client   // always non-nil; Available() checks binary
    codexQuota  *codex.QuotaInfo // global account-level quota cache
}
```

`NewModel()` creates `codex.NewClient()`. No changes to `cmd/atria-lite/main.go` needed.

### Init change: `internal/lite/model.go`

Init does NOT fetch quota immediately — pane data isn't available yet. The first quota fetch happens after the first `candidatePanesLoadedMsg` or `paneStatusesLoadedMsg` arrives and reveals a Codex pane. This follows the existing pattern where status ticks and spinner ticks only start after agent panes are discovered.

```go
func (m Model) Init() tea.Cmd {
    return refreshWindowPanes(m.client, m.ctx)
}
```

### Message types: `internal/lite/messages.go`

```go
type codexQuotaTickMsg struct{}

type codexQuotaMsg struct {
    quota *codex.QuotaInfo
}
```

### Command: `internal/lite/commands.go`

```go
const liteQuotaInterval = 60 * time.Second

func quotaTickCmd() tea.Cmd {
    return tea.Tick(liteQuotaInterval, func(time.Time) tea.Msg {
        return codexQuotaTickMsg{}
    })
}

func fetchCodexQuota(client *codex.Client) tea.Cmd {
    if !client.Available() {
        return nil
    }
    return func() tea.Msg {
        return codexQuotaMsg{quota: client.Fetch()}
    }
}
```

### Update handling: `internal/lite/model.go`

```go
case codexQuotaTickMsg:
    if m.codexClient.Available() && m.hasCodexPanes() {
        return m, fetchCodexQuota(m.codexClient)
    }
    return m, nil

case codexQuotaMsg:
    if msg.quota != nil {
        m.codexQuota = msg.quota
    }
    if m.hasCodexPanes() {
        return m, quotaTickCmd()
    }
    return m, nil
```

Quota polling is gated by `hasCodexPanes()`, matching the existing pattern for status ticks (`hasAgentPanes()`) and spinner ticks (`hasWorkingPanes()`). The polling loop starts only when Codex panes are present and stops when the last one disappears.

First quota fetch is triggered in `candidatePanesLoadedMsg` and `paneStatusesLoadedMsg` handlers (where pane data first becomes available), by appending `m.ensureQuotaTick()` to the returned `tea.Batch`:

```go
func (m *Model) ensureQuotaTick() tea.Cmd {
    if m.codexClient.Available() && m.hasCodexPanes() && m.codexQuota == nil {
        return fetchCodexQuota(m.codexClient) // immediate first fetch
    }
    return nil
}

func (m Model) hasCodexPanes() bool {
    for _, pane := range m.panes {
        if pane.Kind == OccupantAgent && pane.AgentType == model.AgentCodex {
            return true
        }
    }
    return false
}
```

### Display: `internal/lite/view.go`

#### Column width adjustment

`columnWidths()` is modified to give the status column more room. Target values: 32 (narrow) / 36 (wide), but these are targets, not guarantees — the existing CWD-minimum-12 shrink logic still applies. If total width is insufficient, `statusWidth` falls back toward the current 24/28 values.

Implementation: add a second expansion pass after the existing shrink loops. When any Codex pane has quota, expand `statusWidth` up to the target and shrink CWD accordingly (minimum CWD stays at 12). When no Codex pane has quota, keep existing widths.

```go
if m.hasCodexQuota() {
    targetStatus := 32
    if width >= 110 {
        targetStatus = 36
    }
    for statusWidth < targetStatus && cwdWidth > 12 {
        statusWidth++
        cwdWidth--
    }
}
```

#### Status cell rendering: width-aware helper

The current `renderPaneRow` builds status as a fixed-width plain-text cell, then applies a single style. Quota requires two independently-colored segments within the same cell. A new helper handles both unselected and selected paths:

```go
// renderStatusCell renders a fixed-width status cell with optional quota suffix.
// It handles both unselected and selected rendering paths.
//   - unselected: statusStyle + quotaStyle within the cell width
//   - selected:   selectedStatusStyle + selectedQuotaStyle within the cell width
func renderStatusCell(statusText string, statusStyle lipgloss.Style,
    quotaSuffix string, quotaStyle lipgloss.Style,
    selected bool, cellWidth int) string
```

Logic:
1. Build the combined plain text: `statusText + quotaSuffix`
2. Truncate combined text to `cellWidth` using `lipgloss.Width` (ANSI-aware)
3. For unselected: render as `statusStyle.Width(cellWidth).Render(statusText) + quotaStyle.Render(quotaSuffix)`, padded to `cellWidth`
4. For selected: same structure but both styles wrapped with `withSelectedBg()` to get the purple highlight, preserving foreground colors

The helper replaces the current inline status cell logic in `renderPaneRow`:

```go
// Before:
statusCell := fmt.Sprintf("%-*s", statusWidth, statusText)
// ...
statusCell = statusStyle.Render(statusCell)

// After:
statusCell := renderStatusCell(statusText, statusStyle,
    quotaSuffix, quotaStyle, false, statusWidth)
```

And for selected rows:
```go
// Before:
selectedStatus := tui.RenderSelectedStatusCell(statusStyle, statusCell)

// After:
statusCell := renderStatusCell(statusText, statusStyle,
    quotaSuffix, quotaStyle, true, statusWidth)
```

#### Quota suffix builder

```go
func formatQuotaSuffix(quota *codex.QuotaInfo, maxChars int) (text string, style lipgloss.Style)
```

Returns `" · 73% (2h10m)"` and the corresponding percentage-based style. If `maxChars` is insufficient for primary+secondary, drops secondary. If insufficient for primary, returns empty string (graceful degradation — no quota shown).

Display rules:

| Pane status    | Display example                      |
|----------------|--------------------------------------|
| idle           | `● idle · 73% (2h10m)`              |
| working        | `✶ working... · 73% (2h10m)`        |
| needs_input    | `⚠ needs input` (no quota appended) |
| error          | `✗ error` (no quota appended)        |

- Only Codex panes show quota
- `needs_input` and `error` omit quota
- Primary (5h) always shown; secondary (7d) shown when status column has remaining space
- Color coding: green (<50%), yellow (50-79%), red (>=80%)

**Note on multiple Codex panes:** quota is account-level, so all Codex panes show the same data. This is correct and expected — users running multiple Codex sessions need to know the shared limit.

## What does NOT change

- Existing `ReadScreen` / `ClassifyScreen` / `ClassifyOutput` logic untouched
- `gpt-\S+-codex` idle pattern still used for status detection
- Main atria TUI (`internal/tui/`) not affected
- `cmd/atria-lite/main.go` not affected (NewModel creates codex.Client internally)
- No new external dependencies (codex binary is already required for Codex sessions)

## Testing

- `internal/codex/client_test.go`: binary discovery, JSON-RPC protocol parsing, caching, timeout, missing secondary field, percentage at 0/100, concurrent Fetch calls
- `internal/lite/model_test.go`: quota field handling, tick scheduling, Init batch commands
- `internal/lite/view_test.go`: quota rendering in pane rows, truncation at narrow widths, no quota for non-Codex panes
