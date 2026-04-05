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

func TestTimeUntilReset(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"expired", -1 * time.Second, "now"},
		{"sub-minute", 30 * time.Second, "<1m"},
		{"exactly 1 minute", 1*time.Minute + 5*time.Second, "1m"},
		{"10 minutes", 10*time.Minute + 5*time.Second, "10m"},
		{"sub-hour", 45*time.Minute + 30*time.Second, "45m"},
		{"1 hour", 1*time.Hour + 5*time.Second, "1h0m"},
		{"1 hour 30 minutes", 1*time.Hour + 30*time.Minute + time.Second, "1h30m"},
		{"sub-day", 23*time.Hour + 59*time.Minute + time.Second, "23h59m"},
		{"exactly 24 hours", 24*time.Hour + 5*time.Second, "1d0h"},
		{"1 day 6 hours", 30*time.Hour + 5*time.Second, "1d6h"},
		{"2 days", 48*time.Hour + 5*time.Second, "2d0h"},
		{"2 days 12 hours", 60*time.Hour + 5*time.Second, "2d12h"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			target := now.Add(tc.offset)
			got := timeUntilReset(target.Format(time.RFC3339))
			if got != tc.want {
				t.Errorf("timeUntilReset(%s) = %q, want %q", tc.offset, got, tc.want)
			}
		})
	}

	// empty/invalid input
	if got := timeUntilReset(""); got != "" {
		t.Errorf("timeUntilReset(\"\") = %q, want \"\"", got)
	}
	if got := timeUntilReset("not-iso"); got != "" {
		t.Errorf("timeUntilReset(\"not-iso\") = %q, want \"\"", got)
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
