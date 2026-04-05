package codex

import (
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
