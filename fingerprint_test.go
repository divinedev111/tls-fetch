package tlsfetch

import "testing"

func TestCalculateH2Fingerprint_Chrome131(t *testing.T) {
	got := CalculateH2Fingerprint(Chrome131)
	want := "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p"
	if got != want {
		t.Errorf("Chrome131 fingerprint:\n got  %q\n want %q", got, want)
	}
}

func TestCalculateH2Fingerprint_Firefox128(t *testing.T) {
	got := CalculateH2Fingerprint(Firefox128)
	want := "1:65536;4:131072;5:16384|12517377|6|m,p,a,s"
	if got != want {
		t.Errorf("Firefox128 fingerprint:\n got  %q\n want %q", got, want)
	}
}

func TestPseudoHeaderShorthand(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{":method", "m"},
		{":authority", "a"},
		{":scheme", "s"},
		{":path", "p"},
		{":unknown", ":unknown"},
	}
	for _, c := range cases {
		got := pseudoHeaderShorthand(c.input)
		if got != c.want {
			t.Errorf("pseudoHeaderShorthand(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestValidateProfile_Chrome131(t *testing.T) {
	r := ValidateProfile(Chrome131)
	if r.Name != "chrome_131" {
		t.Errorf("Name: got %q, want %q", r.Name, "chrome_131")
	}
	if r.H2Fingerprint == "" {
		t.Error("H2Fingerprint is empty")
	}
}
