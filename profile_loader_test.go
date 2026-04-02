package tlsfetch

import (
	"strings"
	"testing"
)

func TestLoadProfileFromFile_Chrome131(t *testing.T) {
	p, err := LoadProfileFromFile("profiles/chrome_131.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name != "Chrome131" {
		t.Errorf("Name: got %q, want %q", p.Name, "Chrome131")
	}
	if len(p.H2Settings) != 4 {
		t.Errorf("H2Settings count: got %d, want 4", len(p.H2Settings))
	}
	if p.H2WindowUpdate != 15663105 {
		t.Errorf("H2WindowUpdate: got %d, want 15663105", p.H2WindowUpdate)
	}
	wantPseudo := [4]string{":method", ":authority", ":scheme", ":path"}
	if p.PseudoHeaderOrder != wantPseudo {
		t.Errorf("PseudoHeaderOrder: got %v, want %v", p.PseudoHeaderOrder, wantPseudo)
	}
}

func TestLoadProfileFromFile_NotFound(t *testing.T) {
	_, err := LoadProfileFromFile("profiles/does_not_exist.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestLoadProfileFromFile_InvalidJSON(t *testing.T) {
	_, err := LoadProfileFromJSON([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadProfileFromFile_MissingName(t *testing.T) {
	data := `{
		"tls": {"client_hello_id": "HelloChrome_131"},
		"h2": {"settings": [], "window_update": 0, "pseudo_header_order": []},
		"headers": {"order": []}
	}`
	_, err := LoadProfileFromJSON([]byte(data))
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention 'name', got: %v", err)
	}
}
