package tlsfetch

import (
	"testing"
)

func TestNewSession_RequiresProfile(t *testing.T) {
	_, err := NewSession()
	if err == nil {
		t.Fatal("expected error when no profile is configured, got nil")
	}
}

func TestNewSession_WithProfile(t *testing.T) {
	s, err := NewSession(WithProfile(Chrome131))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil session")
	}
}

func TestNewClient_WithProfile(t *testing.T) {
	c, err := NewClient(WithProfile(Chrome131))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.HTTP == nil {
		t.Fatal("expected non-nil underlying http.Client")
	}
}
