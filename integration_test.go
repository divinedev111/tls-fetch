//go:build !short

package tlsfetch_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	tlsfetch "github.com/mukuln-official/tls-fetch"
)

// tlsPeetResponse is a partial shape of the tls.peet.ws /api/all response.
type tlsPeetResponse struct {
	Tls struct {
		JA3 string `json:"ja3"`
	} `json:"tls"`
	Http2 struct {
		AkamaiFingerprint string `json:"akamai_fingerprint"`
	} `json:"http2"`
}

func TestIntegration_Chrome131_Fingerprint(t *testing.T) {
	t.Parallel()

	session, err := tlsfetch.NewSession(tlsfetch.WithProfile(tlsfetch.Chrome131))
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.CloseIdleConnections()

	resp, err := session.Get("https://tls.peet.ws/api/all")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var result tlsPeetResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal: %v (body: %s)", err, body)
	}

	if result.Tls.JA3 == "" {
		t.Error("expected non-empty JA3 hash")
	}
	if result.Http2.AkamaiFingerprint == "" {
		t.Error("expected non-empty H2 fingerprint")
	}

	t.Logf("Chrome131 JA3: %s", result.Tls.JA3)
	t.Logf("Chrome131 H2:  %s", result.Http2.AkamaiFingerprint)
}

func TestIntegration_Firefox128_Fingerprint(t *testing.T) {
	t.Parallel()

	session, err := tlsfetch.NewSession(tlsfetch.WithProfile(tlsfetch.Firefox128))
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.CloseIdleConnections()

	resp, err := session.Get("https://tls.peet.ws/api/all")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var result tlsPeetResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal: %v (body: %s)", err, body)
	}

	if result.Tls.JA3 == "" {
		t.Error("expected non-empty JA3 hash")
	}
	if result.Http2.AkamaiFingerprint == "" {
		t.Error("expected non-empty H2 fingerprint")
	}

	t.Logf("Firefox128 JA3: %s", result.Tls.JA3)
	t.Logf("Firefox128 H2:  %s", result.Http2.AkamaiFingerprint)
}

func TestIntegration_StandardHTTPClient_Compatible(t *testing.T) {
	t.Parallel()

	tr, err := tlsfetch.NewTransport(tlsfetch.WithProfile(tlsfetch.Chrome131))
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}

	client := &http.Client{Transport: tr}

	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	t.Logf("httpbin.org/get status: %s", resp.Status)
}
