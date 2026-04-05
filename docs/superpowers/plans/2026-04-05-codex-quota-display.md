# Codex Quota Display 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在 atria-lite 的 monitor 面板中为 Codex agent pane 显示 token 用量配额（5h/7d 窗口百分比和重置倒计时）。

**架构：** 新增 `internal/codex/client.go` 包，通过 `codex app-server --listen stdio://` 的 JSON-RPC API 获取配额数据。配额为账户级别，全局缓存，60s 刷新一次。显示在 lite 的 status 列中。

**技术栈：** Go, os/exec stdio pipes, JSON-RPC, lipgloss

---

## 文件清单

| 文件 | 职责 |
|------|------|
| `internal/codex/client.go` (新建) | codex app-server JSON-RPC 客户端，二进制发现，缓存 |
| `internal/codex/client_test.go` (新建) | client 单元测试 |
| `internal/lite/messages.go` | 新增 `codexQuotaTickMsg`、`codexQuotaMsg` 消息类型 |
| `internal/lite/commands.go` | 新增 `quotaTickCmd`、`fetchCodexQuota` 命令 |
| `internal/lite/model.go` | 新增 `codexClient`、`codexQuota` 字段；`hasCodexPanes`、`ensureQuotaTick`；Update handler |
| `internal/tui/styles.go` | 新增 quota 百分比颜色样式 |
| `internal/lite/view.go` | 新增 `renderStatusCell`、`formatQuotaSuffix`、`hasCodexQuota`；修改 `columnWidths`、`renderPaneRow` |

---

## 任务 1：codex client 包骨架

**文件：**
- 创建：`internal/codex/client.go`
- 创建：`internal/codex/client_test.go`

- [ ] **步骤 1：创建 client.go 骨架**

```go
package codex

import (
    "sync"
    "time"
)

type QuotaInfo struct {
    PrimaryPct      float64
    PrimaryReset    string
    SecondaryPct    float64
    SecondaryReset  string
    FetchedAt       time.Time
}

type Client struct {
    codexBin string
    cache    *QuotaInfo
    cacheTTL time.Duration
    mu       sync.RWMutex
}

func NewClient() *Client {
    return &Client{cacheTTL: 60 * time.Second}
}

func (c *Client) Available() bool {
    return c.codexBin != ""
}

func (c *Client) Fetch() *QuotaInfo {
    return nil // stub
}

func (c *Client) Cached() *QuotaInfo {
    return nil // stub
}
```

- [ ] **步骤 2：运行测试验证编译通过**

运行：`go build ./internal/codex/...`
预期：PASS，无输出

- [ ] **步骤 3：Commit**

```bash
git add internal/codex/client.go
git commit -m "stub: codex client package with QuotaInfo and Client types"
```

---

## 任务 2：codex 二进制发现

**文件：**
- 修改：`internal/codex/client.go` — 添加二进制发现逻辑

- [ ] **步骤 1：编写二进制发现的测试用例**

```go
package codex

import (
    "os"
    "os/exec"
    "testing"
)

func TestFindCodexBinary(t *testing.T) {
    // 在 PATH 中找不到 codex 时返回 ""
    // 在 PATH 中找到 codex 时返回路径
    path := findCodexBinary()
    if path == "" {
        t.Log("codex not found on PATH (expected in CI)")
        return
    }
    // 验证路径可执行
    cmd := exec.Command(path, "--version")
    if err := cmd.Run(); err != nil {
        t.Errorf("codex at %s is not executable: %v", path, err)
    }
}
```

- [ ] **步骤 2：运行测试**

运行：`go test ./internal/codex/... -v -run TestFind`
预期：PASS 或 SKIP（codex 未安装时 SKIP）

- [ ] **步骤 3：实现 findCodexBinary**

在 `NewClient()` 中调用发现逻辑，设置 `c.codexBin`。

```go
func findCodexBinary() string {
    // 1. os/exec.LookPath("codex")
    if path, err := exec.LookPath("codex"); err == nil {
        return path
    }
    // 2. nvm paths: $HOME/.nvm/versions/node/*/bin/codex
    if nvmPath := findNvmCodex(); nvmPath != "" {
        return nvmPath
    }
    // 3. common fallback paths
    home := os.Getenv("HOME")
    for _, p := range []string{
        home + "/.local/bin/codex",
        home + "/.volta/bin/codex",
        home + "/.asdf/shims/codex",
        home + "/bin/codex",
        "/usr/local/bin/codex",
        "/opt/homebrew/bin/codex",
        "/usr/bin/codex",
    } {
        if info, err := os.Stat(p); err == nil && info.Mode()&0111 != 0 {
            return p
        }
    }
    return ""
}

func findNvmCodex() string {
    home := os.Getenv("HOME")
    pattern := home + "/.nvm/versions/node/*/bin/codex"
    matches, _ := filepath.Glob(pattern)
    if len(matches) == 0 {
        return ""
    }
    // pick highest version
    var best string
    for _, m := range matches {
        if best == "" || m > best {
            best = m
        }
    }
    return best
}
```

- [ ] **步骤 4：运行测试**

运行：`go test ./internal/codex/... -v -run TestFind`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/codex/client.go internal/codex/client_test.go
git commit -m "codex: add binary discovery with nvm and fallback paths"
```

---

## 任务 3：JSON-RPC 协议实现

**文件：**
- 修改：`internal/codex/client.go` — 添加 Fetch 实现

- [ ] **步骤 1：编写 JSON-RPC 测试（mock 进程输出）**

```go
package codex

import (
    "encoding/json"
    "testing"
    "time"
)

func TestParseRateLimitsResponse(t *testing.T) {
    raw := `{"id":2,"result":{"rateLimits":{"primary":{"usedPercent":42.5,"resetsAt":"2026-04-05T12:00:00Z","windowDurationMins":300},"secondary":{"usedPercent":18.0,"resetsAt":"2026-04-10T00:00:00Z"}}},"jsonrpc":"2.0"}`
    qi, err := parseRateLimitsResponse([]byte(raw))
    if err != nil {
        t.Fatalf("parse error: %v", err)
    }
    if qi.PrimaryPct != 42.5 {
        t.Errorf("PrimaryPct = %v, want 42.5", qi.PrimaryPct)
    }
    if qi.SecondaryPct != 18.0 {
        t.Errorf("SecondaryPct = %v, want 18.0", qi.SecondaryPct)
    }
    if qi.PrimaryReset == "" {
        t.Error("PrimaryReset is empty")
    }
}

func TestParseRateLimitsResponse_MissingSecondary(t *testing.T) {
    raw := `{"id":2,"result":{"rateLimits":{"primary":{"usedPercent":99.1,"resetsAt":"2026-04-05T14:30:00Z","windowDurationMins":300}}},"jsonrpc":"2.0"}`
    qi, err := parseRateLimitsResponse([]byte(raw))
    if err != nil {
        t.Fatalf("parse error: %v", err)
    }
    if qi.PrimaryPct != 99.1 {
        t.Errorf("PrimaryPct = %v, want 99.1", qi.PrimaryPct)
    }
    if qi.SecondaryPct != 0 {
        t.Errorf("SecondaryPct = %v, want 0", qi.SecondaryPct)
    }
}

func TestParseRateLimitsResponse_InvalidJSON(t *testing.T) {
    _, err := parseRateLimitsResponse([]byte("not json"))
    if err == nil {
        t.Error("expected error for invalid JSON")
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/codex/... -v -run TestParse`
预期：FAIL — `parseRateLimitsResponse` not defined

- [ ] **步骤 3：实现 parseRateLimitsResponse 和 timeUntilReset**

```go
type rateLimitsResponse struct {
    ID     int `json:"id"`
    Result struct {
        RateLimits struct {
            Primary   rateLimitWindow `json:"primary"`
            Secondary rateLimitWindow `json:"secondary,omitempty"`
        } `json:"rateLimits"`
    } `json:"result"`
}

type rateLimitWindow struct {
    UsedPercent   float64 `json:"usedPercent"`
    ResetsAt     string  `json:"resetsAt"`
    WindowDurationMins int `json:"windowDurationMins"`
}

func parseRateLimitsResponse(data []byte) (*QuotaInfo, error) {
    var resp rateLimitsResponse
    if err := json.Unmarshal(data, &resp); err != nil {
        return nil, err
    }
    qi := &QuotaInfo{
        PrimaryPct:   resp.Result.RateLimits.Primary.UsedPercent,
        PrimaryReset:  timeUntilReset(resp.Result.RateLimits.Primary.ResetsAt),
        FetchedAt:    time.Now(),
    }
    if sec := resp.Result.RateLimits.Secondary; sec.UsedPercent > 0 {
        qi.SecondaryPct = sec.UsedPercent
        qi.SecondaryReset = timeUntilReset(sec.ResetsAt)
    }
    return qi, nil
}

// timeUntilReset converts ISO8601 reset timestamp to "2h10m" format
func timeUntilReset(iso string) string {
    if iso == "" {
        return ""
    }
    t, err := time.Parse(time.RFC3339, iso)
    if err != nil {
        return ""
    }
    d := time.Until(t)
    if d <= 0 {
        return "now"
    }
    if d < time.Hour {
        return fmt.Sprintf("%dm", int(d.Minutes()))
    }
    if d < 24*time.Hour {
        return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
    }
    return fmt.Sprintf("%dd%dh", int(d.Hours()/24), int(d.Hours())%24)
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/codex/... -v -run TestParse`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/codex/client.go internal/codex/client_test.go
git commit -m "codex: add JSON-RPC rate limits parsing and time formatting"
```

---

## 任务 4：Fetch 端到端

**文件：**
- 修改：`internal/codex/client.go` — 实现完整的 Fetch 逻辑

**重要设计决策：** 使用单个长期 scanner 顺序消费 stdout，收到响应行后立即停止，避免 Scanner 缓冲区导致数据丢失。按 `{"id":N}` 中的 id 过滤响应。

- [ ] **步骤 1：编写 Fetch 集成测试**

```go
package codex

import (
    "os/exec"
    "testing"
)

func TestClientFetch_NotAvailable(t *testing.T) {
    c := &Client{codexBin: "", cacheTTL: 60 * 0}
    if c.Available() {
        t.Skip("codex available, skipping not-available test")
    }
    result := c.Fetch()
    if result != nil {
        t.Errorf("Fetch() = %v, want nil when not available", result)
    }
}

func TestClientFetch_Integration(t *testing.T) {
    path := findCodexBinary()
    if path == "" {
        t.Skip("codex not found")
    }
    c := NewClient()
    if !c.Available() {
        t.Skip("codex not available")
    }
    result := c.Fetch()
    // result may be nil if app-server fails or returns error
    t.Logf("Fetch() = %+v", result)
}
```

- [ ] **步骤 2：运行测试**

运行：`go test ./internal/codex/... -v -run TestClientFetch`
预期：可能 SKIP（codex 未安装）或 PASS

- [ ] **步骤 3：实现 Fetch（完整 JSON-RPC 协议）**

关键实现点：
- 使用单个 `bufio.Scanner` 贯穿整个 Fetch 生命周期，每次 Scan 读一行
- `readResponse(id)` 函数循环 Scan 直到找到匹配 id 的 JSON 行或超时/结束
- 首次调用用 15s 超时（给 app-server 冷启动留余地），后续调用用 8s
- 任何错误路径都先尝试返回 fresh cache

```go
func (c *Client) Fetch() *QuotaInfo {
    if c.codexBin == "" {
        return nil
    }

    // Use 15s for first fetch (app-server cold start), 8s thereafter
    timeout := 15 * time.Second
    c.mu.RLock()
    if c.cache != nil {
        timeout = 8 * time.Second
    }
    c.mu.RUnlock()

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, c.codexBin, "app-server", "--listen", "stdio://")
    stdin, err := cmd.StdoutPipe()
    if err != nil {
        return c.cachedOrNil()
    }
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return c.cachedOrNil()
    }

    if err := cmd.Start(); err != nil {
        return c.cachedOrNil()
    }

    // Deferred cleanup: close stdin to unblock app-server, then wait
    done := make(chan struct{})
    defer func() {
        stdin.Close()
        cmd.Wait()
        close(done)
    }()

    // Run scanner reading in a goroutine so we can select on ctx
    respCh := make(chan []byte, 1)
    errCh := make(chan error, 1)
    go func() {
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            line := scanner.Bytes()
            if len(line) == 0 || line[0] != '{' {
                continue
            }
            // Check if this line has the id we're waiting for
            if hasID(line, 1) || hasID(line, 2) {
                select {
                case respCh <- line:
                default:
                }
                return
            }
        }
        if err := scanner.Err(); err != nil {
            errCh <- err
        }
    }()

    // Send initialize
    fmt.Fprintf(stdin, `{"id":1,"method":"initialize","params":{"clientInfo":{"name":"atria-lite","version":"1.0.0"}}}\n`)

    // Wait for initialize response (id=1)
    select {
    case <-respCh:
        // ok
    case <-errCh:
        return c.cachedOrNil()
    case <-ctx.Done():
        return c.cachedOrNil()
    }

    // Send rateLimits request
    fmt.Fprintf(stdin, `{"id":2,"method":"account/rateLimits/read"}\n`)

    // Wait for rateLimits response (id=2)
    select {
    case raw := <-respCh:
        qi, err := parseRateLimitsResponse(raw)
        if err != nil || qi == nil {
            return c.cachedOrNil()
        }
        c.mu.Lock()
        c.cache = qi
        c.mu.Unlock()
        return qi
    case <-errCh:
        return c.cachedOrNil()
    case <-ctx.Done():
        return c.cachedOrNil()
    }
}

// hasID returns true if the JSON line contains the given id.
// Lightweight check: look for `"id":N` pattern without full parsing.
func hasID(line []byte, id int) bool {
    target := fmt.Sprintf(`"id":%d`, id)
    return bytes.Contains(line, []byte(target))
}

func (c *Client) cachedOrNil() *QuotaInfo {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.cache
}
```

**关于 Scanner 缓冲区的说明：** `bufio.Scanner` 默认最大 token size 是 64KB。JSON-RPC 响应行通常远小于此值。如果 `codex app-server` 输出了非 JSON 行（如启动信息），这些行会在循环中被跳过（`line[0] != '{'`），不会进入缓冲区。当 id 匹配的行出现时立即返回，不会有数据丢失问题。

- [ ] **步骤 4：运行测试**

运行：`go test ./internal/codex/... -v`
预期：编译通过（集成测试可能 SKIP）

- [ ] **步骤 5：Commit**

```bash
git add internal/codex/client.go internal/codex/client_test.go
git commit -m "codex: implement Fetch with JSON-RPC over stdio"
```

---

## 任务 4b：缓存回退

**文件：**
- 修改：`internal/codex/client.go` — 实现 Cached() 并在 Fetch 失败时回退

- [ ] **步骤 1：编写缓存测试**

```go
func TestCached(t *testing.T) {
    c := NewClient()
    // No cache initially
    if c.Cached() != nil {
        t.Error("Cached() on empty client should return nil")
    }
    if c.cachedOrNil() != nil {
        t.Error("cachedOrNil() on empty client should return nil")
    }

    // Set cache manually
    c.mu.Lock()
    c.cache = &QuotaInfo{PrimaryPct: 42.5, FetchedAt: time.Now()}
    c.mu.Unlock()

    if c.Cached() == nil {
        t.Error("Cached() should return the cached value")
    }
    if c.Cached().PrimaryPct != 42.5 {
        t.Errorf("PrimaryPct = %.1f, want 42.5", c.Cached().PrimaryPct)
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/codex/... -v -run TestCached`
预期：FAIL — `Cached()` not defined

- [ ] **步骤 3：实现 Cached()**

```go
func (c *Client) Cached() *QuotaInfo {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.cache
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/codex/... -v -run TestCached`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/codex/client.go internal/codex/client_test.go
git commit -m "codex: add Cached() for cache access"
```

---

## 任务 5：lite 集成——消息和命令

**文件：**
- 修改：`internal/lite/messages.go`
- 修改：`internal/lite/commands.go`

- [ ] **步骤 1：添加消息类型**

在 `internal/lite/messages.go` 添加：

```go
type codexQuotaTickMsg struct{}

type codexQuotaMsg struct {
    quota *codex.QuotaInfo
}
```

`lite` 直接 import `codex` 不会形成循环依赖（`codex` 不依赖 `lite` 或 `model`），因此使用具体类型保留编译期类型安全。

- [ ] **步骤 2：运行测试验证编译通过**

运行：`go build ./internal/lite/...`
预期：PASS

- [ ] **步骤 3：添加命令**

在 `internal/lite/commands.go` 添加：

```go
const liteQuotaInterval = 60 * time.Second

func quotaTickCmd() tea.Cmd {
    return tea.Tick(liteQuotaInterval, func(time.Time) tea.Msg {
        return codexQuotaTickMsg{}
    })
}

func fetchCodexQuota(client *codex.Client) tea.Cmd {
    if client == nil || !client.Available() {
        return nil
    }
    return func() tea.Msg {
        return codexQuotaMsg{quota: client.Fetch()}
    }
}
```

- [ ] **步骤 4：运行测试验证编译通过**

运行：`go build ./internal/lite/...`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/lite/messages.go internal/lite/commands.go
git commit -m "lite: add codexQuotaTickMsg, codexQuotaMsg, and fetch commands"
```

---

## 任务 6：lite 集成——Model 变更

**文件：**
- 修改：`internal/lite/model.go`

- [ ] **步骤 1：添加字段和方法**

在 `Model` struct 中添加：

```go
type Model struct {
    // ... existing fields ...
    codexClient *codex.Client   // nil if codex binary not found (Available() == false)
    codexQuota  *codex.QuotaInfo // global account-level quota cache
}
```

在 `NewModel()` 中：

```go
func NewModel(client windowPaneClient, ctx MonitorContext) Model {
    bindings := normalizeBindings(ctx.SlotBindings)
    ctx.SlotBindings = bindings
    ctx.WorkspacePaneIDs = bindingPaneIDs(bindings)

    return Model{
        client: client,
        ctx: ctx,
        mode: ModeList,
        bindings: bindings,
        missingPaneGrace: make(map[int]int),
        codexClient: codex.NewClient(),
    }
}
```

添加 helper 方法：

```go
func (m Model) hasCodexPanes() bool {
    for _, pane := range m.panes {
        if pane.Kind == OccupantAgent && pane.AgentType == model.AgentCodex {
            return true
        }
    }
    return false
}

func (m *Model) ensureQuotaTick() tea.Cmd {
    if m.codexClient == nil || !m.codexClient.Available() {
        return nil
    }
    if !m.hasCodexPanes() || m.codexQuota != nil {
        return nil
    }
    return fetchCodexQuota(m.codexClient)
}

func (m Model) hasCodexQuota() bool {
    return m.codexQuota != nil
}
```

- [ ] **步骤 2：添加 Update handler**

在 `Update()` 的 switch 中添加：

```go
case codexQuotaTickMsg:
    if m.codexClient != nil && m.codexClient.Available() && m.hasCodexPanes() {
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

- [ ] **步骤 3：在 pane 加载消息中触发首次 fetch**

在 `candidatePanesLoadedMsg` handler 的三个返回路径中全部添加 `m.ensureQuotaTick()`：

```go
case candidatePanesLoadedMsg:
    // ... existing handling ...
    // Path 1: autoloaded pane
    if autoloadedPane != nil {
        if cmd := syncWorkspaceBindings(...); cmd != nil {
            return m, tea.Batch(cmd, refreshTickCmd(), m.ensureStatusTick(), m.ensureSpinnerTick(), m.ensureQuotaTick())
        }
        m.applyBindings(nextBindings)
        m.statusText = fmt.Sprintf("Loaded %s", paneLabel(*autoloadedPane))
    } else if recoverWorkspace != nil {
        // Path 2: recover workspace
        if cmd := syncWorkspaceBindings(...); cmd != nil {
            return m, tea.Batch(cmd, refreshTickCmd(), m.ensureStatusTick(), m.ensureSpinnerTick(), m.ensureQuotaTick())
        }
    }
    // Path 3: normal (no autoload, no recovery)
    return m, tea.Batch(refreshTickCmd(), m.ensureStatusTick(), m.ensureSpinnerTick(), m.ensureQuotaTick())
```

在 `paneStatusesLoadedMsg` handler 中添加：

```go
case paneStatusesLoadedMsg:
    m.panes = msg.panes
    m.syncReplacePrompt()
    m.clampCursor()
    return m, tea.Batch(m.ensureStatusTick(), m.ensureSpinnerTick(), m.ensureQuotaTick())
```

- [ ] **步骤 4：运行测试验证编译通过**

运行：`go build ./internal/lite/...`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/lite/model.go
git commit -m "lite: integrate codex client, quota tick loop, and hasCodexPanes"
```

---

## 任务 7：TUI 样式——quota 百分比颜色

**文件：**
- 修改：`internal/tui/styles.go`

- [ ] **步骤 1：添加 quota 百分比样式**

```go
// QuotaPercentageStyle returns green/yellow/red based on percentage
func QuotaPercentageStyle(pct float64) lipgloss.Style {
    switch {
    case pct >= 80:
        return lipgloss.NewStyle().
            Foreground(lipgloss.AdaptiveColor{Light: "#cc0000", Dark: "#ff4444"}) // red
    case pct >= 50:
        return lipgloss.NewStyle().
            Foreground(lipgloss.AdaptiveColor{Light: "#b8860b", Dark: "#e0af68"}) // yellow
    default:
        return lipgloss.NewStyle().
            Foreground(lipgloss.AdaptiveColor{Light: "#2d7d46", Dark: "#9ece6a"}) // green
    }
}
```

- [ ] **步骤 2：运行测试验证编译通过**

运行：`go build ./internal/tui/...`
预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add internal/tui/styles.go
git commit -m "tui: add QuotaPercentageStyle with green/yellow/red thresholds"
```

---

## 任务 8：Display——quota suffix builder 和 status cell helper

**文件：**
- 修改：`internal/lite/view.go`

- [ ] **步骤 1：编写测试**

```go
package lite

import (
    "testing"

    "github.com/charmbracelet/lipgloss"
)

func TestFormatQuotaSuffix(t *testing.T) {
    qi := &codex.QuotaInfo{
        PrimaryPct:     42.5,
        PrimaryReset:   "2h10m",
        SecondaryPct:   18.0,
        SecondaryReset: "4d12h",
    }

    text, style := formatQuotaSuffix(qi, 50)
    if text == "" {
        t.Error("text is empty")
    }
    if style == (lipgloss.Style{}) {
        t.Error("style is zero")
    }
    t.Logf("suffix: %q", text)
}

func TestFormatQuotaSuffix_TooNarrowForPrimary(t *testing.T) {
    qi := &codex.QuotaInfo{PrimaryPct: 42.5, PrimaryReset: "2h10m"}
    text, _ := formatQuotaSuffix(qi, 3) // too narrow for " · 42%"
    if text != "" {
        t.Errorf("text = %q, want empty", text)
    }
}

func TestFormatQuotaSuffix_DropsSecondary(t *testing.T) {
    qi := &codex.QuotaInfo{
        PrimaryPct:   42.5,
        PrimaryReset: "2h10m",
        SecondaryPct: 18.0,
    }
    text, _ := formatQuotaSuffix(qi, 15) // fits primary but not secondary
    // should contain primary, may not contain secondary
    if text == "" {
        t.Error("text is empty")
    }
}

func TestRenderStatusCell(t *testing.T) {
    // Test selected=false, no quota
    result := renderStatusCell("● idle", lipgloss.NewStyle(), "", lipgloss.NewStyle(), false, 20)
    if result == "" {
        t.Error("result is empty")
    }

    // Test selected=true, with quota
    result = renderStatusCell("● idle", lipgloss.NewStyle(), " · 42% (2h)", lipgloss.NewStyle(), true, 30)
    if result == "" {
        t.Error("result is empty")
    }

    // Test truncation
    result = renderStatusCell("● idle", lipgloss.NewStyle(), " · 42% (2h)", lipgloss.NewStyle(), false, 10)
    if result == "" {
        t.Error("result is empty after truncation")
    }
    if lipgloss.Width(result) != 10 {
        t.Errorf("width = %d, want 10", lipgloss.Width(result))
    }
}
```

注：测试需要在 test 文件顶部添加 import：

```go
import "github.com/sethdeckard/atria/internal/codex"
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/lite/... -v -run "TestFormatQuotaSuffix|TestRenderStatusCell"`
预期：FAIL — functions not defined

- [ ] **步骤 3：实现 formatQuotaSuffix**

```go
// formatQuotaSuffix builds the quota suffix text and returns its style.
// Returns ("", zero) if maxChars is insufficient for the minimum suffix.
func formatQuotaSuffix(qi *codex.QuotaInfo, maxChars int) (string, lipgloss.Style) {
    if qi == nil || maxChars < 8 {
        return "", lipgloss.NewStyle()
    }

    // Build primary: " · 42% (2h10m)"
    primary := fmt.Sprintf(" \u00b7 %.0f%% (%s)", qi.PrimaryPct, qi.PrimaryReset)
    primaryStyle := tui.QuotaPercentageStyle(qi.PrimaryPct)

    if maxChars < lipgloss.Width(primary) {
        // Try just the percentage: " · 42%"
        shorter := fmt.Sprintf(" \u00b7 %.0f%%", qi.PrimaryPct)
        if maxChars >= lipgloss.Width(shorter) {
            return shorter, primaryStyle
        }
        // Too narrow even for percentage
        return "", lipgloss.NewStyle()
    }

    // Add secondary if it fits: " · 42% (2h10m) · 18% (4d)"
    if qi.SecondaryPct > 0 && qi.SecondaryReset != "" {
        secondary := fmt.Sprintf(" \u00b7 %.0f%% (%s)", qi.SecondaryPct, qi.SecondaryReset)
        combined := primary + secondary
        if maxChars >= lipgloss.Width(combined) {
            return combined, primaryStyle
        }
    }

    return primary, primaryStyle
}
```

- [ ] **步骤 4：实现 renderStatusCell**

```go
// renderStatusCell renders a fixed-width status cell with optional quota suffix.
// Handles both unselected and selected rendering paths.
func renderStatusCell(statusText string, statusStyle lipgloss.Style,
    quotaSuffix string, quotaStyle lipgloss.Style,
    selected bool, cellWidth int) string {

    // Build combined text and truncate to cellWidth
    combined := statusText + quotaSuffix
    if lipgloss.Width(combined) > cellWidth {
        // Truncate statusText to fit, keeping quotaSuffix fully
        avail := cellWidth - lipgloss.Width(quotaSuffix)
        if avail < 0 {
            avail = 0
        }
        statusText = tui.TruncateToWidth(statusText, avail)
        combined = statusText + quotaSuffix
    }

    if !selected {
        // Unselected: status + quota, each with their own style
        statusPart := statusStyle.Width(cellWidth - lipgloss.Width(quotaSuffix)).Render(statusText)
        return statusPart + quotaStyle.Render(quotaSuffix)
    }

    // Selected: wrap both parts with selected background, preserving foreground
    statusPart := tui.WithSelectedBg(statusStyle).Bold(true).Width(cellWidth - lipgloss.Width(quotaSuffix)).Render(statusText)
    return statusPart + tui.WithSelectedBg(quotaStyle).Bold(true).Render(quotaSuffix)
}
```

将 `internal/tui/styles.go` 中现有的 `withSelectedBg` 导出（首字母大写）：

```go
// Before:
func withSelectedBg(s lipgloss.Style) lipgloss.Style {

// After:
func WithSelectedBg(s lipgloss.Style) lipgloss.Style {
```

`withSelectedBg` 目前只在 `RenderSelectedStatusCell` 和 `RenderSelectedAgentTypeCell` 内部使用，导出它不会破坏现有调用。
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test ./internal/lite/... -v -run "TestFormatQuotaSuffix|TestRenderStatusCell"`
预期：PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/lite/view.go internal/tui/styles.go
git commit -m "lite: add formatQuotaSuffix and renderStatusCell helpers"
```

---

## 任务 9：Display——集成到 renderPaneRow 和 columnWidths

**文件：**
- 修改：`internal/lite/view.go`

- [ ] **步骤 1：修改 columnWidths**

在 `columnWidths()` 的 return 前添加扩展 pass：

```go
func (m Model) columnWidths() (paneWidth, typeWidth, bindingWidth, statusWidth, cwdWidth int) {
    // ... existing logic up to the final return ...
    cwdWidth = width - 2 - paneWidth - typeWidth - bindingWidth - statusWidth
    for cwdWidth < 12 && statusWidth > 18 {
        statusWidth--
        cwdWidth++
    }
    for cwdWidth < 12 && paneWidth > 16 {
        paneWidth--
        cwdWidth++
    }
    if cwdWidth < 12 {
        cwdWidth = 12
    }

    // Expansion pass: give status more room when quota is available
    if m.hasCodexQuota() && m.codexQuota != nil {
        targetStatus := 32
        if width >= 110 {
            targetStatus = 36
        }
        for statusWidth < targetStatus && cwdWidth > 12 {
            statusWidth++
            cwdWidth--
        }
    }

    return paneWidth, typeWidth, bindingWidth, statusWidth, cwdWidth
}
```

- [ ] **步骤 2：修改 renderPaneRow**

在 `renderPaneRow` 中，将 status cell 构建部分改为：

```go
// Build quota suffix for Codex panes (only for idle/working; needs_input and error omit quota)
var quotaSuffix string
var quotaStyle lipgloss.Style
if pane.Kind == OccupantAgent && pane.AgentType == model.AgentCodex &&
    pane.Status != model.StatusNeedsInput && pane.Status != model.StatusError {
    suffix, style := formatQuotaSuffix(m.codexQuota, statusWidth-4)
    quotaSuffix = suffix
    quotaStyle = style
}

if selected {
    // ... nameCell, typeCell, bindingCell ...
    statusCell := renderStatusCell(statusText, statusStyle, quotaSuffix, quotaStyle, true, statusWidth)
    return tui.RenderSelectedText(nameCell) +
        typeStyled +
        tui.RenderSelectedText(bindingCell) +
        statusCell +
        tui.RenderSelectedText(cwdCell)
}

// ... non-selected path ...
statusCell := renderStatusCell(statusText, statusStyle, quotaSuffix, quotaStyle, false, statusWidth)
```

注：删除原有的 `fmt.Sprintf("%-*s", statusWidth, statusText)` 和单独的 `statusStyle.Render(statusCell)` 调用。

- [ ] **步骤 3：运行测试验证编译通过**

运行：`go build ./internal/lite/... && go test ./internal/lite/... -v`
预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/lite/view.go
git commit -m "lite: integrate quota display into renderPaneRow and columnWidths"
```

---

## 任务 10：端到端验证

- [ ] **步骤 1：运行完整构建和测试**

运行：`go build ./... && go test ./...`
预期：全部 PASS

- [ ] **步骤 2：运行 linter**

运行：`golangci-lint run ./internal/codex/... ./internal/lite/...`
预期：PASS

- [ ] **步骤 3：验证功能逻辑**

确保：
1. `codex.NewClient()` 在 codex 不存在时返回可用 Client（Available()=false）
2. `Fetch()` 在 Available()=false 时返回 nil
3. `hasCodexPanes()` 正确识别 Codex pane
4. quota suffix 在 statusWidth 不足时优雅降级
5. 选中行和未选中行都有正确的背景色

- [ ] **步骤 4：Commit**

```bash
git add -A && git commit -m "codex quota display: complete implementation"
```
