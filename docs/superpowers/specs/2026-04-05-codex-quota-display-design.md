# Codex Quota Display Design

## Summary

Display Codex token usage quota in Atria's monitor panel for Codex agent sessions. Data is sourced from the Codex app-server JSON-RPC API (`account/rateLimits/read`), providing 5-hour and 7-day window usage percentages with reset countdowns.

## Motivation

Codex's idle status bar shows `gpt-5.3-codex default · 73% left · ~/projects/foo` — a single percentage. The app-server API provides richer data: dual-window (5h/7d) utilization, reset timestamps, and window duration. Displaying this in Atria's status column lets users track quota without leaving the TUI.

Reference implementation: [agent-quota.wezterm](https://github.com/M-Marbouh/agent-quota.wezterm) — a WezTerm status bar plugin that fetches both Claude and Codex quota. Atria will implement a Go-native equivalent for Codex.

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

### Model changes: `internal/model/types.go`

Add `QuotaInfo` struct and `Quota` field to `AgentSession`:

```go
type QuotaInfo struct {
    PrimaryPct      float64
    PrimaryReset    string
    SecondaryPct    float64
    SecondaryReset  string
    FetchedAt       time.Time
}

type AgentSession struct {
    // ... existing fields ...
    Quota *QuotaInfo `json:"-"` // non-nil only for Codex agents
}
```

Defined in the `model` package to avoid circular dependencies.

### Codex binary discovery

1. `os/exec.LookPath("codex")`
2. `$HOME/.nvm/versions/node/*/bin/codex` (pick highest version)
3. Common paths: `~/.local/bin/codex`, `~/.volta/bin/codex`, `/usr/local/bin/codex`, `/opt/homebrew/bin/codex`
4. Not found → `NewClient()` returns nil, all quota logic skipped

## TUI Display

### Status column integration

Quota info is appended to the existing status text for Codex agents only.

| Session status | Display example                      |
|----------------|--------------------------------------|
| idle           | `● idle · 73% (2h10m)`              |
| working        | `✶ working... · 73% (2h10m)`        |
| needs_input    | `⚠ needs input` (no quota appended) |
| error          | `✗ error` (no quota appended)        |

Rules:
- Only Codex agents with non-nil `Quota` show quota
- `needs_input` and `error` states omit quota to avoid cluttering critical info
- Primary (5h) percentage always shown; secondary (7d) shown when space permits
- Color coding: green (<50%), yellow (50-79%), red (>=80%) — matches agent-quota convention

### Rendering location

Modified in `internal/tui/status.go` `FormatAgentStatus()` and `internal/tui/projectlist.go` row formatting functions.

## Refresh Strategy

### Timing

- Quota is account-level, not session-level — single global fetch for all Codex sessions
- 60s refresh interval (configurable via `cacheTTL`)
- First Codex session appearance triggers immediate fetch (no 60s wait)
- Last Codex session removal stops the refresh loop

### tea.Cmd integration

```
App.Update() receives tickMsg
  → check if any Codex sessions exist
  → if yes: dispatch codexFetchCmd()
  → write result to all Codex sessions' Quota field
  → schedule next quota tick (60s)
```

The `codexFetchCmd` calls `Client.Fetch()` in a goroutine (via `exec.CommandContext`) and returns a `quotaMsg` with the result.

## What does NOT change

- Existing `ReadScreen` / `ClassifyScreen` / `ClassifyOutput` logic untouched
- `gpt-\S+-codex` idle pattern still used for status detection
- Quota is purely additive display, no impact on core monitoring
- No new external dependencies (codex binary is already required for Codex sessions)

## Testing

- `internal/codex/client_test.go`: mock `exec.Cmd`, test JSON-RPC protocol parsing, caching, timeout
- `internal/terminal/monitor_test.go`: confirm existing Codex patterns unaffected
- `internal/tui/projectlist_test.go`: verify quota rendering in status column
