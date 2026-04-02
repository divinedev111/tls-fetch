// Package proxy provides Dialer implementations for HTTP CONNECT and SOCKS5
// proxies, plus a direct (no-proxy) dialer.
package proxy

import (
	"context"
	"fmt"
	"net"
	"net/url"
)

// Dialer dials a network connection, optionally via a proxy.
type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// FromURL returns the appropriate Dialer for the given proxy URL.
//
//   - nil URL → directDialer (no proxy)
//   - http:// or https:// → httpDialer (HTTP CONNECT tunnel)
//   - socks5:// → socks5Dialer
//   - any other scheme → error
func FromURL(u *url.URL) (Dialer, error) {
	if u == nil {
		return &directDialer{}, nil
	}
	switch u.Scheme {
	case "http", "https":
		return &httpDialer{addr: hostport(u), user: u.User}, nil
	case "socks5":
		return &socks5Dialer{addr: hostport(u), user: u.User}, nil
	default:
		return nil, fmt.Errorf("proxy: unsupported scheme %q", u.Scheme)
	}
}

// hostport returns host:port from a *url.URL, defaulting port 80 for http and
// 1080 for socks5 when the URL has no explicit port.
func hostport(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	switch u.Scheme {
	case "https":
		return u.Hostname() + ":443"
	case "socks5":
		return u.Hostname() + ":1080"
	default:
		return u.Hostname() + ":80"
	}
}

// directDialer dials directly without any proxy.
type directDialer struct{}

func (d *directDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var nd net.Dialer
	return nd.DialContext(ctx, network, addr)
}
