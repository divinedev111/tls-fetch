package tlsfetch

import (
	"testing"
)

func TestParseJA3_ValidString(t *testing.T) {
	// Standard JA3 with SNI, extended_master_secret, renegotiation_info, supported_groups,
	// ec_point_formats, signature_algorithms, supported_versions, psk_key_exchange_modes, key_share.
	ja3 := "771,4865-4866-4867-49195-49199,0-23-65281-10-11-13-43-45-51,29-23-24,0"
	p, err := ProfileFromJA3(ja3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ClientHelloSpec == nil {
		t.Fatal("ClientHelloSpec is nil")
	}
	if len(p.ClientHelloSpec.CipherSuites) == 0 {
		t.Error("CipherSuites is empty")
	}
}

func TestParseJA3_InvalidFormat(t *testing.T) {
	cases := []struct {
		name string
		ja3  string
	}{
		{"empty", ""},
		{"too few fields", "771,4865"},
		{"non-numeric version", "abc,4865-4866,0-23,29-23,0"},
		{"too many fields", "771,4865,0,29,0,extra"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ProfileFromJA3(tc.ja3)
			if err == nil {
				t.Errorf("expected error for input %q, got nil", tc.ja3)
			}
		})
	}
}

func TestParseJA3_GREASEStripped(t *testing.T) {
	// 2570 == 0x0a0a which is a GREASE value; 4865, 4866 are real ciphers.
	// 2570 in the curves field is also GREASE.
	ja3 := "771,2570-4865-4866,0-23,2570-29-23,0"
	p, err := ProfileFromJA3(ja3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ClientHelloSpec == nil {
		t.Fatal("ClientHelloSpec is nil")
	}

	for _, c := range p.ClientHelloSpec.CipherSuites {
		if isGREASE(c) {
			t.Errorf("found GREASE value 0x%04x in CipherSuites", c)
		}
	}
}
