package h2

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/http2"
)

// Transport manages HTTP/2 connections with custom initial framing.
// PseudoHeaderOrder is recorded for fingerprint calculation but not yet
// enforced on the wire (requires a forked HPACK encoder).
type Transport struct {
	config Config
	mu     sync.Mutex
	conns  map[string]*http2.ClientConn
}

// NewTransport returns an HTTP/2 transport that applies cfg on new connections.
func NewTransport(cfg Config) *Transport {
	return &Transport{config: cfg, conns: make(map[string]*http2.ClientConn)}
}

// RoundTrip sends req over a pre-dialed connection. Not http.RoundTripper —
// it takes a pre-established net.Conn alongside the request.
func (t *Transport) RoundTrip(req *http.Request, tlsConn net.Conn) (*http.Response, error) {
	addr := req.URL.Host
	if addr == "" {
		return nil, fmt.Errorf("h2: request has no host")
	}

	t.mu.Lock()
	cc, ok := t.conns[addr]
	t.mu.Unlock()

	if ok {
		resp, err := cc.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
		t.mu.Lock()
		delete(t.conns, addr)
		t.mu.Unlock()
	}

	cc, err := t.newClientConn(tlsConn)
	if err != nil {
		return nil, fmt.Errorf("h2: new client conn: %w", err)
	}

	t.mu.Lock()
	t.conns[addr] = cc
	t.mu.Unlock()

	return cc.RoundTrip(req)
}

func (t *Transport) newClientConn(conn net.Conn) (*http2.ClientConn, error) {
	if err := WriteInitialFrames(conn, t.config); err != nil {
		return nil, err
	}

	h2t := &http2.Transport{AllowHTTP: true, DisableCompression: true}
	return h2t.NewClientConn(conn)
}

// CloseIdleConnections closes all cached HTTP/2 client connections.
func (t *Transport) CloseIdleConnections() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for addr, cc := range t.conns {
		cc.Close()
		delete(t.conns, addr)
	}
}
