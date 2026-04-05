package codex

import (
	"os/exec"
	"testing"
	"time"
)

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
		t.Log("codex not found on PATH (expected in CI)")
		return
	}
	// Verify path is executable
	cmd := exec.Command(path, "--version")
	if err := cmd.Run(); err != nil {
		t.Errorf("codex at %s is not executable: %v", path, err)
	}
}
