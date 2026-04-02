package tlsfetch

import (
	"testing"
)

func TestNewTransport_RequiresProfile(t *testing.T) {
	_, err := NewTransport()
	if err == nil {
		t.Fatal("expected error when no profile is configured, got nil")
	}
}

func TestNewTransport_WithProfile(t *testing.T) {
	tr, err := NewTransport(WithProfile(Chrome131))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestResolveProfile_FromOption(t *testing.T) {
	cfg := defaultConfig()
	p := Chrome131
	cfg.profile = &p

	got, err := resolveProfile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "chrome_131" {
		t.Fatalf("expected profile name Chrome131, got %q", got.Name)
	}
}

func TestResolveProfile_NoProfile(t *testing.T) {
	cfg := defaultConfig()
	_, err := resolveProfile(cfg)
	if err == nil {
		t.Fatal("expected error when no profile source is set, got nil")
	}
}
