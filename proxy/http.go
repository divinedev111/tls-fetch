package proxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// httpDialer tunnels connections through an HTTP proxy using the CONNECT method.
type httpDialer struct {
	addr string
	user *url.Userinfo
}

// DialContext connects to the HTTP proxy at h.addr and issues a CONNECT
// request to reach the target addr. On a 200 response the underlying TCP
// connection is returned as the tunnel.
func (h *httpDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var nd net.Dialer
	conn, err := nd.DialContext(ctx, "tcp", h.addr)
	if err != nil {
		return nil, fmt.Errorf("proxy: dial HTTP proxy %s: %w", h.addr, err)
	}

	// Build and send the CONNECT request.
	req, err := http.NewRequestWithContext(ctx, http.MethodConnect, "http://"+addr, nil)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("proxy: build CONNECT request: %w", err)
	}
	req.Host = addr

	if h.user != nil {
		username := h.user.Username()
		password, _ := h.user.Password()
		creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		req.Header.Set("Proxy-Authorization", "Basic "+creds)
	}

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("proxy: write CONNECT request: %w", err)
	}

	// Read and validate the proxy response.
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("proxy: read CONNECT response: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("proxy: CONNECT to %s returned status %d", addr, resp.StatusCode)
	}

	// If the reader has buffered data that belong to the tunnel, wrap the conn
	// so those bytes are delivered first.
	if br.Buffered() > 0 {
		return &bufferedConn{Conn: conn, r: br}, nil
	}
	return conn, nil
}

// bufferedConn wraps a net.Conn with a bufio.Reader that may contain bytes
// already read from the underlying connection (e.g. proxy response body).
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}
