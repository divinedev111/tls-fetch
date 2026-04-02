package header_test

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mukuln-official/tls-fetch/header"
)

func TestReorderHeaders_BasicReorder(t *testing.T) {
	raw := []byte("GET / HTTP/1.1\r\nAccept-Encoding: gzip\r\nContent-Type: text/html\r\nAccept: */*\r\n\r\n")
	order := []string{"Accept", "Content-Type", "Accept-Encoding"}

	out := header.ReorderHTTP1Headers(raw, order)
	lines := strings.Split(string(out), "\r\n")

	// lines[0] = request line, lines[1..3] = headers, lines[4] = empty, lines[5] = body (empty)
	if lines[1] != "Accept: */*" {
		t.Errorf("line 1 = %q, want %q", lines[1], "Accept: */*")
	}
	if lines[2] != "Content-Type: text/html" {
		t.Errorf("line 2 = %q, want %q", lines[2], "Content-Type: text/html")
	}
	if lines[3] != "Accept-Encoding: gzip" {
		t.Errorf("line 3 = %q, want %q", lines[3], "Accept-Encoding: gzip")
	}
}

func TestReorderHeaders_PreservesRequestLine(t *testing.T) {
	raw := []byte("POST /api HTTP/1.1\r\nContent-Type: application/json\r\n\r\n")
	out := header.ReorderHTTP1Headers(raw, []string{"Content-Type"})
	first := strings.SplitN(string(out), "\r\n", 2)[0]
	if first != "POST /api HTTP/1.1" {
		t.Errorf("request line = %q, want %q", first, "POST /api HTTP/1.1")
	}
}

func TestReorderHeaders_UnknownHeadersAppended(t *testing.T) {
	raw := []byte("GET / HTTP/1.1\r\nX-Custom: foo\r\nAccept: */*\r\nX-Other: bar\r\n\r\n")
	order := []string{"Accept"}

	out := header.ReorderHTTP1Headers(raw, order)
	lines := strings.Split(string(out), "\r\n")

	// lines[1] = Accept (ordered), then unknowns in original relative order
	if lines[1] != "Accept: */*" {
		t.Errorf("line 1 = %q, want Accept", lines[1])
	}
	// X-Custom and X-Other should follow in their original relative order
	if lines[2] != "X-Custom: foo" {
		t.Errorf("line 2 = %q, want X-Custom", lines[2])
	}
	if lines[3] != "X-Other: bar" {
		t.Errorf("line 3 = %q, want X-Other", lines[3])
	}
}

func TestReorderHeaders_EmptyOrder_NoChange(t *testing.T) {
	raw := []byte("GET / HTTP/1.1\r\nAccept: */*\r\nUser-Agent: go\r\n\r\n")
	out := header.ReorderHTTP1Headers(raw, nil)
	if !bytes.Equal(out, raw) {
		t.Errorf("nil order should be passthrough\ngot:  %q\nwant: %q", out, raw)
	}
}

func TestReorderHeaders_BodyNotModified(t *testing.T) {
	body := `{"key":"value","extra":"\r\n\r\nfake boundary"}`
	raw := []byte("POST /api HTTP/1.1\r\nContent-Type: application/json\r\n\r\n" + body)
	out := header.ReorderHTTP1Headers(raw, []string{"Content-Type"})

	idx := bytes.Index(out, []byte("\r\n\r\n"))
	if idx == -1 {
		t.Fatal("no header terminator in output")
	}
	gotBody := string(out[idx+4:])
	if gotBody != body {
		t.Errorf("body modified\ngot:  %q\nwant: %q", gotBody, body)
	}
}

// --- ReorderConn tests ---

// pipeConn wraps the write end of a net.Pipe so we can capture what was written.
type pipeConn struct {
	net.Conn
}

func TestReorderConn_ReordersHeaders(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	order := []string{"Accept", "User-Agent"}
	rc := header.NewReorderConn(client, order)

	raw := "GET / HTTP/1.1\r\nUser-Agent: go\r\nAccept: */*\r\n\r\n"

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := server.Read(buf)
		done <- buf[:n]
	}()

	if _, err := rc.Write([]byte(raw)); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	select {
	case got := <-done:
		lines := strings.Split(string(got), "\r\n")
		if lines[1] != "Accept: */*" {
			t.Errorf("line 1 = %q, want Accept", lines[1])
		}
		if lines[2] != "User-Agent: go" {
			t.Errorf("line 2 = %q, want User-Agent", lines[2])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for reordered data")
	}
}

func TestReorderConn_PassthroughAfterHeaders(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	order := []string{"Accept"}
	rc := header.NewReorderConn(client, order)

	headers := "GET / HTTP/1.1\r\nAccept: */*\r\n\r\n"
	extraData := "some body data after headers"

	allReceived := make(chan []byte, 1)
	go func() {
		var buf []byte
		tmp := make([]byte, 4096)
		// Read until we have the full expected content
		for !bytes.Contains(buf, []byte(extraData)) {
			n, err := server.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if err != nil {
				break
			}
		}
		allReceived <- buf
	}()

	if _, err := rc.Write([]byte(headers)); err != nil {
		t.Fatalf("Write headers error: %v", err)
	}
	if _, err := rc.Write([]byte(extraData)); err != nil {
		t.Fatalf("Write body error: %v", err)
	}

	select {
	case got := <-allReceived:
		if !bytes.Contains(got, []byte(extraData)) {
			t.Errorf("passthrough data not received; got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for passthrough data")
	}
}
