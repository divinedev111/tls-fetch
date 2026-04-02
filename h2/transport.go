package h2

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/http2"
)

// Transport manages HTTP/2 connections with custom initial framing.
//
// It writes custom SETTINGS, WINDOW_UPDATE, and PRIORITY frames before
// handing the connection to the standard http2.ClientConn for request
// round-tripping.
//
// Note: PseudoHeaderOrder in Config is recorded for fingerprint calculation
// but not yet enforced on the wire. The standard http2 encoder hardcodes
// the order as :authority, :method, :path, :scheme. Custom ordering
// requires a forked HPACK encoder (planned for a future version).
type Transport struct {
	config Config
	mu     sync.Mutex
	conns  map[string]*http2.ClientConn
}

// NewTransport returns an HTTP/2 transport that applies cfg when
// establishing new connections.
func NewTransport(cfg Config) *Transport {
	return &Transport{
		config: cfg,
		conns:  make(map[string]*http2.ClientConn),
	}
}

// RoundTrip sends req over the provided pre-dialed connection.
// It reuses existing HTTP/2 client connections keyed by host, falling
// back to creating a new one if none exists or the existing one has
// become unusable.
//
// This is NOT the http.RoundTripper interface --- it accepts a
// pre-established net.Conn alongside the request.
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
		// Connection went bad; remove and create a new one.
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

// newClientConn writes our custom initial frames then delegates to
// http2.Transport.NewClientConn. The standard library will also write
// its own preface and SETTINGS frame; the server processes both and
// uses the latest value for each setting.
func (t *Transport) newClientConn(conn net.Conn) (*http2.ClientConn, error) {
	if err := WriteInitialFrames(conn, t.config); err != nil {
		return nil, err
	}

	h2t := &http2.Transport{
		AllowHTTP:          true,
		DisableCompression: true,
	}
	cc, err := h2t.NewClientConn(conn)
	if err != nil {
		return nil, err
	}
	return cc, nil
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
