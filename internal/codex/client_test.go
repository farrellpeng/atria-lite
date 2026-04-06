package codex

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

func TestParseRateLimitsResponse_UnixResetsAt(t *testing.T) {
	now := time.Now()
	primaryReset := now.Add(2*time.Hour + 10*time.Minute).Unix()
	secondaryReset := now.Add(4*24*time.Hour + 12*time.Hour).Unix()

	raw := `{"id":2,"result":{"rateLimits":{"primary":{"usedPercent":3,"resetsAt":` +
		strconv.FormatInt(primaryReset, 10) +
		`,"windowDurationMins":300},"secondary":{"usedPercent":77,"resetsAt":` +
		strconv.FormatInt(secondaryReset, 10) +
		`,"windowDurationMins":10080}}},"jsonrpc":"2.0"}`

	qi, err := parseRateLimitsResponse([]byte(raw))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if qi.PrimaryPct != 3 {
		t.Errorf("PrimaryPct = %v, want 3", qi.PrimaryPct)
	}
	if qi.SecondaryPct != 77 {
		t.Errorf("SecondaryPct = %v, want 77", qi.SecondaryPct)
	}
	if qi.PrimaryReset == "" {
		t.Error("PrimaryReset is empty")
	}
	if qi.SecondaryReset == "" {
		t.Error("SecondaryReset is empty")
	}
}

func TestParseRateLimitsResponse_InvalidJSON(t *testing.T) {
	_, err := parseRateLimitsResponse([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFormatResetTimeAt(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.April, 6, 19, 30, 0, 0, loc)

	tests := []struct {
		name  string
		reset time.Time
		want  string
	}{
		{
			name:  "same day shows time only",
			reset: time.Date(2026, time.April, 6, 21, 47, 0, 0, loc),
			want:  "21:47",
		},
		{
			name:  "future day shows date suffix",
			reset: time.Date(2026, time.April, 8, 15, 48, 0, 0, loc),
			want:  "15:48 on 8 Apr",
		},
		{
			name:  "utc timestamp converted to local time",
			reset: time.Date(2026, time.April, 8, 7, 48, 0, 0, time.UTC),
			want:  "15:48 on 8 Apr",
		},
		{
			name:  "zero time omitted",
			reset: time.Time{},
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatResetTimeAt(tc.reset, now); got != tc.want {
				t.Fatalf("formatResetTimeAt(%v, %v) = %q, want %q", tc.reset, now, got, tc.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.cacheTTL != 60*time.Second {
		t.Errorf("expected cacheTTL 60s, got %v", c.cacheTTL)
	}
}

func TestAvailable(t *testing.T) {
	c := NewClient()
	// Available() should reflect whether findCodexBinary() found codex
	path := findCodexBinary()
	if path == "" && c.Available() {
		t.Error("Available() should be false when codexBin is not found")
	}
	if path != "" && !c.Available() {
		t.Error("Available() should be true when codexBin is found")
	}
}

func TestFindCodexBinary(t *testing.T) {
	// When codex is not on PATH, returns ""
	// When codex is on PATH, returns the path
	path := findCodexBinary()
	if path == "" {
		t.Skip("codex not found")
	}
	// Verify path is executable
	cmd := exec.Command(path, "--version")
	if err := cmd.Run(); err != nil {
		t.Errorf("codex at %s is not executable: %v", path, err)
	}
}

func TestClientFetch_NotAvailable(t *testing.T) {
	c := &Client{codexBin: "", cacheTTL: 60 * time.Second}
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

func TestReadJSONRPCResponseByID_SequentialResponses(t *testing.T) {
	t.Parallel()

	scanner := bufio.NewScanner(strings.NewReader(strings.Join([]string{
		"codex app-server starting",
		`{"id":1,"result":{"ok":true},"jsonrpc":"2.0"}`,
		`{"id":2,"result":{"rateLimits":{"primary":{"usedPercent":42.5,"resetsAt":"2026-04-05T12:00:00Z","windowDurationMins":300}}},"jsonrpc":"2.0"}`,
	}, "\n")))

	ctx := context.Background()

	initResp, err := readJSONRPCResponseByID(ctx, scanner, 1)
	if err != nil {
		t.Fatalf("read init response: %v", err)
	}
	if !strings.Contains(string(initResp), `"id":1`) {
		t.Fatalf("init response = %s, want id=1", initResp)
	}

	rateResp, err := readJSONRPCResponseByID(ctx, scanner, 2)
	if err != nil {
		t.Fatalf("read rate limit response: %v", err)
	}
	if !strings.Contains(string(rateResp), `"id":2`) {
		t.Fatalf("rate limit response = %s, want id=2", rateResp)
	}
}

func TestClientFetch_SequentialJSONRPCResponses(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join(t.TempDir(), "codex")
	script := strings.Join([]string{
		"#!/bin/sh",
		"read _",
		"printf '%s\\n' '{\"id\":1,\"result\":{\"ok\":true},\"jsonrpc\":\"2.0\"}'",
		"read _",
		"printf '%s\\n' '{\"id\":2,\"result\":{\"rateLimits\":{\"primary\":{\"usedPercent\":42.5,\"resetsAt\":\"2099-04-05T12:00:00Z\",\"windowDurationMins\":300},\"secondary\":{\"usedPercent\":18.0,\"resetsAt\":\"2099-04-10T00:00:00Z\",\"windowDurationMins\":10080}}},\"jsonrpc\":\"2.0\"}'",
	}, "\n")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	c := &Client{codexBin: scriptPath, cacheTTL: 60 * time.Second}
	qi := c.Fetch()
	if qi == nil {
		t.Fatal("Fetch() = nil, want quota info")
	}
	if qi.PrimaryPct != 42.5 {
		t.Fatalf("PrimaryPct = %v, want 42.5", qi.PrimaryPct)
	}
	if qi.SecondaryPct != 18.0 {
		t.Fatalf("SecondaryPct = %v, want 18.0", qi.SecondaryPct)
	}
}

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
