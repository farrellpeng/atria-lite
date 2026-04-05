package codex

import "testing"
import "time"

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.cacheTTL != 60*time.Second {
		t.Errorf("expected cacheTTL 60s, got %v", c.cacheTTL)
	}
}

func TestAvailableFalse(t *testing.T) {
	c := NewClient()
	if c.Available() {
		t.Error("Available() should be false when codexBin is empty")
	}
}
