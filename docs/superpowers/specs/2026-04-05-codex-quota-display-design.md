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
- Timeout (8s) or parse failure → return last cached data if fresh, else nil
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
    codexBin string           // resolved codex binary path
    cache    *QuotaInfo
    cacheTTL time.Duration    // default 60s
    mu       sync.RWMutex
}

func NewClient() *Client                    // finds codex binary, returns nil if not found
func (c *Client) Fetch() (*QuotaInfo, error) // runs JSON-RPC, returns quota data
func (c *Client) Cached() *QuotaInfo        // returns cached data if within TTL
```

`QuotaInfo` is defined in the `codex` package. The lite Model holds a `*codex.QuotaInfo` field for the global account-level quota.

### Codex binary discovery

1. `os/exec.LookPath("codex")`
2. `$HOME/.nvm/versions/node/*/bin/codex` (pick highest version)
3. Common paths: `~/.local/bin/codex`, `~/.volta/bin/codex`, `/usr/local/bin/codex`, `/opt/homebrew/bin/codex`
4. Not found → `NewClient()` returns nil, all quota logic skipped

## atria-lite Integration

### Model changes: `internal/lite/model.go`

The `Model` struct gains two new fields:

```go
type Model struct {
    // ... existing fields ...
    codexClient *codex.Client  // nil if codex binary not found
    codexQuota  *codex.QuotaInfo // global account-level quota cache
}
```

`NewModel()` calls `codex.NewClient()` and stores the result. If nil, all quota logic is skipped.

### Message type: `internal/lite/messages.go`

```go
type codexQuotaMsg struct {
    quota *codex.QuotaInfo
}
```

### Command: `internal/lite/commands.go`

New `tea.Cmd` for periodic quota refresh:

```go
const liteQuotaInterval = 60 * time.Second

func quotaTickCmd() tea.Cmd {
    return tea.Tick(liteQuotaInterval, func(time.Time) tea.Msg {
        return codexQuotaTickMsg{}
    })
}

func fetchCodexQuota(client *codex.Client) tea.Cmd {
    if client == nil {
        return nil
    }
    return func() tea.Msg {
        quota, _ := client.Fetch()
        return codexQuotaMsg{quota: quota}
    }
}
```

### Update handling: `internal/lite/model.go`

```go
case codexQuotaTickMsg:
    return m, fetchCodexQuota(m.codexClient)

case codexQuotaMsg:
    if msg.quota != nil {
        m.codexQuota = msg.quota
    }
    return m, quotaTickCmd()
```

Startup: when `Init()` returns, also return `fetchCodexQuota(m.codexClient)` for immediate first fetch.

### Display: `internal/lite/view.go`

Modified `renderPaneRow` — when the pane is a Codex agent and `m.codexQuota` is non-nil, append quota to status text.

The quota string is built in a helper:

```go
func formatQuotaSuffix(quota *codex.QuotaInfo) string
```

Returns something like `" · 73% (2h10m)"`. Coloring is handled by the existing `statusStyle` mechanism.

Display rules:

| Pane status    | Display example                      |
|----------------|--------------------------------------|
| idle           | `● idle · 73% (2h10m)`              |
| working        | `✶ working... · 73% (2h10m)`        |
| needs_input    | `⚠ needs input` (no quota appended) |
| error          | `✗ error` (no quota appended)        |

- Only Codex panes show quota
- `needs_input` and `error` omit quota
- Primary (5h) always shown; secondary (7d) shown when space permits
- Color coding: green (<50%), yellow (50-79%), red (>=80%)

## What does NOT change

- Existing `ReadScreen` / `ClassifyScreen` / `ClassifyOutput` logic untouched
- `gpt-\S+-codex` idle pattern still used for status detection
- Main atria TUI (`internal/tui/`) not affected
- No new external dependencies (codex binary is already required for Codex sessions)

## Testing

- `internal/codex/client_test.go`: mock `exec.Cmd`, test JSON-RPC protocol parsing, caching, timeout
- `internal/lite/model_test.go`: verify quota field handling, tick scheduling
- `internal/lite/view_test.go`: verify quota rendering in pane rows
