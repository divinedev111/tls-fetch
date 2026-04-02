package header_test

import (
	"net/http"
	"testing"

	"github.com/mukuln-official/tls-fetch/header"
)

func TestOrdered_PreservesInsertionOrder(t *testing.T) {
	o := header.Ordered{
		{"Accept", "text/html"},
		{"User-Agent", "go-test"},
		{"Accept-Encoding", "gzip"},
	}

	keys := o.Keys()
	want := []string{"Accept", "User-Agent", "Accept-Encoding"}
	if len(keys) != len(want) {
		t.Fatalf("got %d keys, want %d", len(keys), len(want))
	}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("keys[%d] = %q, want %q", i, k, want[i])
		}
	}
}

func TestOrdered_Get_Found(t *testing.T) {
	o := header.Ordered{
		{"Content-Type", "application/json"},
	}
	if v := o.Get("content-type"); v != "application/json" {
		t.Errorf("Get = %q, want %q", v, "application/json")
	}
}

func TestOrdered_Get_NotFound(t *testing.T) {
	o := header.Ordered{}
	if v := o.Get("X-Missing"); v != "" {
		t.Errorf("Get on missing = %q, want empty string", v)
	}
}

func TestOrdered_Set_Existing(t *testing.T) {
	o := header.Ordered{
		{"Accept", "text/html"},
		{"User-Agent", "go-test"},
	}
	o.Set("accept", "application/json")

	if v := o.Get("Accept"); v != "application/json" {
		t.Errorf("after Set, Get = %q, want %q", v, "application/json")
	}
	// Should still be at position 0, not appended
	if o[0][0] != "Accept" {
		t.Errorf("Set should replace in place; got %q at index 0", o[0][0])
	}
	if len(o) != 2 {
		t.Errorf("Set on existing key should not grow slice; got len=%d", len(o))
	}
}

func TestOrdered_Set_New(t *testing.T) {
	o := header.Ordered{
		{"Accept", "text/html"},
	}
	o.Set("X-New", "value")

	if len(o) != 2 {
		t.Fatalf("Set new key should append; got len=%d", len(o))
	}
	if o[1][0] != "X-New" || o[1][1] != "value" {
		t.Errorf("appended pair = %v, want [X-New value]", o[1])
	}
}

func TestOrdered_Del(t *testing.T) {
	o := header.Ordered{
		{"Accept", "text/html"},
		{"User-Agent", "go-test"},
		{"Accept-Encoding", "gzip"},
	}
	o.Del("user-agent")

	if len(o) != 2 {
		t.Fatalf("Del should shrink slice; got len=%d", len(o))
	}
	if v := o.Get("User-Agent"); v != "" {
		t.Errorf("deleted key still present: %q", v)
	}
	// Remaining keys should be in original order
	if o[0][0] != "Accept" || o[1][0] != "Accept-Encoding" {
		t.Errorf("remaining order wrong: %v", o)
	}
}

func TestOrdered_Clone_NoAlias(t *testing.T) {
	orig := header.Ordered{
		{"Accept", "text/html"},
		{"User-Agent", "go-test"},
	}
	clone := orig.Clone()
	clone.Set("Accept", "application/json")

	// Original should be unchanged
	if v := orig.Get("Accept"); v != "text/html" {
		t.Errorf("original mutated after clone.Set; got %q", v)
	}
}

func TestOrdered_ToHTTPHeader(t *testing.T) {
	o := header.Ordered{
		{"Content-Type", "application/json"},
		{"X-Custom", "hello"},
		{"X-Custom", "world"},
	}
	h := o.ToHTTPHeader()

	if _, ok := h["Content-Type"]; !ok {
		t.Error("Content-Type missing from http.Header")
	}
	// net/http canonicalizes keys
	if h.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", h.Get("Content-Type"))
	}
	// Multiple values for same key
	if len(h[http.CanonicalHeaderKey("X-Custom")]) != 2 {
		t.Errorf("expected 2 values for X-Custom, got %v", h["X-Custom"])
	}
}
