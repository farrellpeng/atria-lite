package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type QuotaInfo struct {
	PrimaryPct     float64
	PrimaryReset   string
	SecondaryPct   float64
	SecondaryReset string
	FetchedAt      time.Time
}

type Client struct {
	codexBin string
	cache    *QuotaInfo
	cacheTTL time.Duration
	mu       sync.RWMutex
}

func NewClient() *Client {
	return &Client{codexBin: findCodexBinary(), cacheTTL: 60 * time.Second}
}

func (c *Client) Available() bool {
	return c.codexBin != ""
}

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
	stdin, err := cmd.StdinPipe()
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
	defer func() {
		stdin.Close()
		cmd.Wait()
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
			// Check if this line has id=1 or id=2
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
	fmt.Fprintf(stdin, "{\"id\":1,\"method\":\"initialize\",\"params\":{\"clientInfo\":{\"name\":\"atria-lite\",\"version\":\"1.0.0\"}}}\n")

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
	fmt.Fprintf(stdin, "{\"id\":2,\"method\":\"account/rateLimits/read\"}\n")

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
	target := fmt.Sprintf("\"id\":%d", id)
	return bytes.Contains(line, []byte(target))
}

func (c *Client) cachedOrNil() *QuotaInfo {
	c.mu.RLock()
	if c.cache == nil {
		c.mu.RUnlock()
		return nil
	}
	if time.Since(c.cache.FetchedAt) > c.cacheTTL {
		c.mu.RUnlock()
		c.mu.Lock()
		c.cache = nil
		c.mu.Unlock()
		return nil
	}
	c.mu.RUnlock()
	return c.cache
}

func (c *Client) Cached() *QuotaInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache
}

func findCodexBinary() string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	// 1. Look in PATH
	if path, err := exec.LookPath("codex"); err == nil {
		return path
	}
	// 2. Check nvm paths
	if nvmPath := findNvmCodex(); nvmPath != "" {
		return nvmPath
	}
	// 3. Common fallback paths
	for _, p := range []string{
		home + "/.local/bin/codex",
		home + "/.volta/bin/codex",
		home + "/.asdf/shims/codex",
		home + "/bin/codex",
		"/usr/local/bin/codex",
		"/opt/homebrew/bin/codex",
		"/usr/bin/codex",
	} {
		if info, err := os.Stat(p); err == nil && info.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

// --- JSON-RPC types and parsing ---

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
	UsedPercent        float64 `json:"usedPercent"`
	ResetsAt           string  `json:"resetsAt"`
	WindowDurationMins int     `json:"windowDurationMins"`
}

func parseRateLimitsResponse(data []byte) (*QuotaInfo, error) {
	var resp rateLimitsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	qi := &QuotaInfo{
		PrimaryPct:  resp.Result.RateLimits.Primary.UsedPercent,
		PrimaryReset: timeUntilReset(resp.Result.RateLimits.Primary.ResetsAt),
		FetchedAt:   time.Now(),
	}
	if sec := resp.Result.RateLimits.Secondary; sec.UsedPercent > 0 {
		qi.SecondaryPct = sec.UsedPercent
		qi.SecondaryReset = timeUntilReset(sec.ResetsAt)
	}
	return qi, nil
}

// timeUntilReset converts ISO8601 reset timestamp to "2h10m" format.
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
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours() / 24)
	remainingHours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, remainingHours)
}

// --- binary discovery ---

func findNvmCodex() string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	pattern := home + "/.nvm/versions/node/*/bin/codex"
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return ""
	}
	// Sort descending so newest is first
	sort.Slice(matches, func(i, j int) bool { return matches[i] > matches[j] })
	return matches[0]
}
