package tlsfetch

import (
	"testing"
)

func TestBuiltinProfiles_NotNil(t *testing.T) {
	profiles := []BrowserProfile{Chrome131, Firefox128, Safari18, Edge131}
	names := []string{"chrome_131", "firefox_128", "safari_18", "edge_131"}

	for i, p := range profiles {
		t.Run(names[i], func(t *testing.T) {
			if p.Name == "" {
				t.Errorf("Name is empty")
			}
			if p.ClientHelloID.Client == "" {
				t.Errorf("ClientHelloID.Client is empty")
			}
			if len(p.H2Settings) == 0 {
				t.Errorf("H2Settings is empty")
			}
			if p.H2WindowUpdate == 0 {
				t.Errorf("H2WindowUpdate is zero")
			}
			var zeroPseudo [4]string
			if p.PseudoHeaderOrder == zeroPseudo {
				t.Errorf("PseudoHeaderOrder is zero-value")
			}
			if len(p.HeaderOrder) == 0 {
				t.Errorf("HeaderOrder is empty")
			}
		})
	}
}

func TestChrome131_CorrectH2Settings(t *testing.T) {
	p := Chrome131

	wantSettings := []H2Setting{
		{ID: 1, Value: 65536},
		{ID: 2, Value: 0},
		{ID: 4, Value: 6291456},
		{ID: 6, Value: 262144},
	}

	if len(p.H2Settings) != len(wantSettings) {
		t.Fatalf("H2Settings count: got %d, want %d", len(p.H2Settings), len(wantSettings))
	}
	for i, want := range wantSettings {
		got := p.H2Settings[i]
		if got.ID != want.ID || got.Value != want.Value {
			t.Errorf("H2Settings[%d]: got {%d, %d}, want {%d, %d}",
				i, got.ID, got.Value, want.ID, want.Value)
		}
	}

	if p.H2WindowUpdate != 15663105 {
		t.Errorf("H2WindowUpdate: got %d, want 15663105", p.H2WindowUpdate)
	}

	wantPseudo := [4]string{":method", ":authority", ":scheme", ":path"}
	if p.PseudoHeaderOrder != wantPseudo {
		t.Errorf("PseudoHeaderOrder: got %v, want %v", p.PseudoHeaderOrder, wantPseudo)
	}
}
