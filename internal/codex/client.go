package codex

import (
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
	return nil // stub
}

func (c *Client) Cached() *QuotaInfo {
	return nil // stub
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
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours()/24), int(d.Hours())%24)
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
