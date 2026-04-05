package codex

import (
	"os/exec"
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
