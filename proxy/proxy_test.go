package proxy

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"testing"
)

// TestFromURL_HTTP verifies that an http:// URL returns an httpDialer.
func TestFromURL_HTTP(t *testing.T) {
	u, _ := url.Parse("http://proxy.example.com:8080")
	d, err := FromURL(u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := d.(*httpDialer); !ok {
		t.Fatalf("expected *httpDialer, got %T", d)
	}
}

// TestFromURL_SOCKS5 verifies that a socks5:// URL returns a socks5Dialer.
func TestFromURL_SOCKS5(t *testing.T) {
	u, _ := url.Parse("socks5://proxy.example.com:1080")
	d, err := FromURL(u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := d.(*socks5Dialer); !ok {
		t.Fatalf("expected *socks5Dialer, got %T", d)
	}
}

// TestFromURL_UnsupportedScheme verifies that an ftp:// URL returns an error.
func TestFromURL_UnsupportedScheme(t *testing.T) {
	u, _ := url.Parse("ftp://proxy.example.com")
	_, err := FromURL(u)
	if err == nil {
		t.Fatal("expected error for unsupported scheme, got nil")
	}
}

// TestFromURL_Nil verifies that a nil URL returns a directDialer.
func TestFromURL_Nil(t *testing.T) {
	d, err := FromURL(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := d.(*directDialer); !ok {
		t.Fatalf("expected *directDialer, got %T", d)
	}
}

// ------------------------------------------------------------------
// Integration-style tests using in-process fake proxy servers.
// ------------------------------------------------------------------

// TestHTTPDialer_Connect verifies that httpDialer correctly issues a
// CONNECT tunnel through a local fake HTTP proxy.
func TestHTTPDialer_Connect(t *testing.T) {
	// Start a fake HTTP proxy that accepts CONNECT and pipes.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	const target = "backend.example.com:443"
	connectedTo := make(chan string, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		br := bufio.NewReader(conn)
		// Read request line.
		line, _ := br.ReadString('\n')
		connectedTo <- line
		// Drain remaining headers.
		for {
			h, _ := br.ReadString('\n')
			if h == "\r\n" || h == "\n" || h == "" {
				break
			}
		}
		// Respond with 200 Connection Established.
		fmt.Fprintf(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		// Echo back whatever is written through the tunnel.
		io.Copy(conn, br)
	}()

	d := &httpDialer{addr: ln.Addr().String()}
	conn, err := d.DialContext(t.Context(), "tcp", target)
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	defer conn.Close()

	requestLine := <-connectedTo
	expected := fmt.Sprintf("CONNECT %s HTTP/1.1\r\n", target)
	if requestLine != expected {
		t.Errorf("request line = %q, want %q", requestLine, expected)
	}
}

// TestHTTPDialer_Connect_BasicAuth verifies that Basic auth credentials
// are forwarded in the Proxy-Authorization header.
func TestHTTPDialer_Connect_BasicAuth(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	authHeader := make(chan string, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		br := bufio.NewReader(conn)
		// Read all headers, capture Proxy-Authorization.
		for {
			line, _ := br.ReadString('\n')
			if line == "\r\n" || line == "\n" || line == "" {
				break
			}
			if len(line) > 20 && line[:20] == "Proxy-Authorization:" {
				authHeader <- line
			}
		}
		fmt.Fprintf(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	}()

	user := url.UserPassword("alice", "secret")
	d := &httpDialer{addr: ln.Addr().String(), user: user}
	conn, err := d.DialContext(t.Context(), "tcp", "backend.example.com:443")
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	conn.Close()

	got := <-authHeader
	creds := base64.StdEncoding.EncodeToString([]byte("alice:secret"))
	expected := fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", creds)
	if got != expected {
		t.Errorf("auth header = %q, want %q", got, expected)
	}
}

// TestSOCKS5Dialer_NoAuth verifies the SOCKS5 handshake with no authentication.
func TestSOCKS5Dialer_NoAuth(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	const target = "backend.example.com"
	const targetPort = 443

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Greeting: VER=5, NMETHODS=1, METHODS=[0x00]
		buf := make([]byte, 3)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		// Reply: VER=5, METHOD=0x00 (no auth)
		conn.Write([]byte{0x05, 0x00})

		// Read CONNECT request: VER RSV ATYP ...
		hdr := make([]byte, 4)
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return
		}
		// ATYP should be 0x03 (domain)
		if hdr[3] != 0x03 {
			return
		}
		lenBuf := make([]byte, 1)
		io.ReadFull(conn, lenBuf)
		domainBuf := make([]byte, int(lenBuf[0])+2) // domain + port
		io.ReadFull(conn, domainBuf)

		// Reply: success
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	}()

	d := &socks5Dialer{addr: ln.Addr().String()}
	conn, err := d.DialContext(t.Context(), "tcp", fmt.Sprintf("%s:%d", target, targetPort))
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	conn.Close()
}

// TestSOCKS5Dialer_UserPassAuth verifies the SOCKS5 handshake with
// username/password authentication.
func TestSOCKS5Dialer_UserPassAuth(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	authReceived := make(chan [2]string, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Greeting
		hdr := make([]byte, 2)
		io.ReadFull(conn, hdr)
		nMethods := int(hdr[1])
		methods := make([]byte, nMethods)
		io.ReadFull(conn, methods)
		// Respond: choose method 0x02 (user/pass)
		conn.Write([]byte{0x05, 0x02})

		// Username/password sub-negotiation (RFC 1929)
		verBuf := make([]byte, 1)
		io.ReadFull(conn, verBuf) // VER=1
		ulenBuf := make([]byte, 1)
		io.ReadFull(conn, ulenBuf)
		uname := make([]byte, int(ulenBuf[0]))
		io.ReadFull(conn, uname)
		plenBuf := make([]byte, 1)
		io.ReadFull(conn, plenBuf)
		passwd := make([]byte, int(plenBuf[0]))
		io.ReadFull(conn, passwd)
		authReceived <- [2]string{string(uname), string(passwd)}
		// Auth success
		conn.Write([]byte{0x01, 0x00})

		// CONNECT request
		hdr4 := make([]byte, 4)
		io.ReadFull(conn, hdr4)
		lenB := make([]byte, 1)
		io.ReadFull(conn, lenB)
		rest := make([]byte, int(lenB[0])+2)
		io.ReadFull(conn, rest)

		// Reply: success
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	}()

	user := url.UserPassword("bob", "pass123")
	d := &socks5Dialer{addr: ln.Addr().String(), user: user}
	conn, err := d.DialContext(t.Context(), "tcp", "backend.example.com:443")
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	conn.Close()

	got := <-authReceived
	if got[0] != "bob" || got[1] != "pass123" {
		t.Errorf("auth = %v, want [bob pass123]", got)
	}
}
