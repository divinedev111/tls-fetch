package tlsfetch

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/mukuln-official/tls-fetch/h2"
	"github.com/mukuln-official/tls-fetch/header"
	"github.com/mukuln-official/tls-fetch/internal"
	"github.com/mukuln-official/tls-fetch/proxy"
	utls "github.com/refraction-networking/utls"
)

// Transport is an http.RoundTripper that performs TLS handshakes with
// browser-specific ClientHello fingerprints and routes HTTP/2 traffic
// through a custom h2.Transport that sends fingerprinted initial frames.
type Transport struct {
	profile            BrowserProfile
	dialer             proxy.Dialer
	pool               *internal.Pool
	h2t                *h2.Transport
	logger             *slog.Logger
	insecureSkipVerify bool

	// mu guards the one-shot http1 transport cache (not strictly needed
	// since each H1 request gets its own transport, but kept for safety).
	mu sync.Mutex
}

// NewTransport creates a Transport from the given options.
// At least one profile source (WithProfile, WithProfileFromFile, or
// WithProfileFromJA3) must be provided.
func NewTransport(opts ...Option) (*Transport, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	profile, err := resolveProfile(cfg)
	if err != nil {
		return nil, err
	}

	var dialer proxy.Dialer
	if cfg.proxyURL != "" {
		u, err := url.Parse(cfg.proxyURL)
		if err != nil {
			return nil, fmt.Errorf("tlsfetch: invalid proxy URL: %w", err)
		}
		dialer, err = proxy.FromURL(u)
		if err != nil {
			return nil, fmt.Errorf("tlsfetch: proxy dialer: %w", err)
		}
	} else {
		dialer, _ = proxy.FromURL(nil)
	}

	h2cfg := h2.Config{
		Settings:          toH2Settings(profile.H2Settings),
		WindowUpdate:      profile.H2WindowUpdate,
		Priorities:        toH2Priorities(profile.H2Priorities),
		PseudoHeaderOrder: profile.PseudoHeaderOrder,
	}

	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Transport{
		profile:            *profile,
		dialer:             dialer,
		pool:               internal.NewPool(cfg.pool),
		h2t:                h2.NewTransport(h2cfg),
		logger:             logger,
		insecureSkipVerify: cfg.insecureSkipVerify,
	}, nil
}

// RoundTrip executes a single HTTP request with TLS fingerprinting.
// It implements the http.RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL == nil {
		return nil, fmt.Errorf("tlsfetch: request has no URL")
	}
	if req.URL.Host == "" {
		return nil, fmt.Errorf("tlsfetch: request has no host")
	}

	addr := req.URL.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		// No port; default to 443 for https.
		addr = net.JoinHostPort(addr, "443")
	}

	serverName := req.URL.Hostname()

	ctx := req.Context()

	t.logger.Debug("dialing TLS",
		"host", serverName,
		"addr", addr,
		"profile", t.profile.Name,
	)

	tlsConn, err := t.dialTLS(ctx, addr, serverName)
	if err != nil {
		return nil, fmt.Errorf("tlsfetch: dial TLS %s: %w", addr, err)
	}

	alpn := tlsConn.ConnectionState().NegotiatedProtocol

	t.logger.Debug("TLS handshake complete",
		"alpn", alpn,
		"host", serverName,
	)

	switch alpn {
	case "h2":
		return t.roundTripH2(req, tlsConn)
	default:
		return t.roundTripH1(req, tlsConn)
	}
}

// PoolStats returns a snapshot of the connection pool state.
func (t *Transport) PoolStats() internal.PoolStats {
	return t.pool.Stats()
}

// CloseIdleConnections closes all idle connections in both the pool and
// the h2 transport.
func (t *Transport) CloseIdleConnections() {
	t.pool.Close()
	t.h2t.CloseIdleConnections()
}

// resolveProfile picks a BrowserProfile from the config, checking in order:
// direct profile, file path, JA3 string.
func resolveProfile(cfg *config) (*BrowserProfile, error) {
	if cfg.profile != nil {
		return cfg.profile, nil
	}
	if cfg.profilePath != "" {
		p, err := LoadProfileFromFile(cfg.profilePath)
		if err != nil {
			return nil, fmt.Errorf("tlsfetch: load profile from file: %w", err)
		}
		return &p, nil
	}
	if cfg.ja3String != "" {
		p, err := ProfileFromJA3(cfg.ja3String)
		if err != nil {
			return nil, fmt.Errorf("tlsfetch: profile from JA3: %w", err)
		}
		return &p, nil
	}
	return nil, fmt.Errorf("tlsfetch: no browser profile configured (use WithProfile, WithProfileFromFile, or WithProfileFromJA3)")
}

// dialTLS dials a TCP connection through the configured proxy (or directly),
// then performs a TLS handshake using the profile's uTLS ClientHello.
func (t *Transport) dialTLS(ctx context.Context, addr, serverName string) (*utls.UConn, error) {
	rawConn, err := t.dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	tlsCfg := &utls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: t.insecureSkipVerify,
	}

	var uconn *utls.UConn
	if t.profile.ClientHelloSpec != nil {
		uconn = utls.UClient(rawConn, tlsCfg, utls.HelloCustom)
		if err := uconn.ApplyPreset(t.profile.ClientHelloSpec); err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("apply ClientHelloSpec: %w", err)
		}
	} else {
		uconn = utls.UClient(rawConn, tlsCfg, t.profile.ClientHelloID)
	}

	if err := uconn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("TLS handshake: %w", err)
	}

	return uconn, nil
}

// roundTripH2 delegates the request to the h2.Transport with the pre-dialed
// TLS connection.
func (t *Transport) roundTripH2(req *http.Request, tlsConn *utls.UConn) (*http.Response, error) {
	// h2.Transport.RoundTrip accepts net.Conn; *utls.UConn satisfies it.
	return t.h2t.RoundTrip(req, tlsConn)
}

// roundTripH1 sends the request over HTTP/1.1 using a one-shot stdlib
// http.Transport that returns our pre-dialed, header-reordering connection.
func (t *Transport) roundTripH1(req *http.Request, tlsConn *utls.UConn) (*http.Response, error) {
	var reorderConn net.Conn = tlsConn
	if len(t.profile.HeaderOrder) > 0 {
		reorderConn = header.NewReorderConn(tlsConn, t.profile.HeaderOrder)
	}

	// used tracks whether the pre-dialed conn has been handed out.
	// The stdlib transport calls DialTLSContext once; we give it ours.
	var once sync.Once
	used := false

	stdTransport := &http.Transport{
		// Disable keep-alives so the stdlib transport doesn't try to reuse.
		DisableKeepAlives: true,
		// Provide our pre-dialed TLS connection.
		DialTLSContext: func(_ context.Context, network, addr string) (net.Conn, error) {
			var conn net.Conn
			once.Do(func() {
				conn = reorderConn
				used = true
			})
			if conn != nil {
				return conn, nil
			}
			return nil, fmt.Errorf("tlsfetch: H1 transport already consumed the pre-dialed connection")
		},
		// Force HTTP/1.1 — do not negotiate h2 on this transport.
		TLSClientConfig: &tls.Config{
			NextProtos: []string{"http/1.1"},
		},
		ForceAttemptHTTP2: false,
	}
	defer stdTransport.CloseIdleConnections()

	resp, err := stdTransport.RoundTrip(req)
	if err != nil {
		if !used {
			tlsConn.Close()
		}
		return nil, fmt.Errorf("tlsfetch: H1 round trip: %w", err)
	}
	return resp, nil
}

// toH2Settings converts the package-level H2Setting slice to the h2 sub-package type.
func toH2Settings(settings []H2Setting) []h2.Setting {
	out := make([]h2.Setting, len(settings))
	for i, s := range settings {
		out[i] = h2.Setting{ID: s.ID, Value: s.Value}
	}
	return out
}

// toH2Priorities converts the package-level H2Priority slice to the h2 sub-package type.
func toH2Priorities(priorities []H2Priority) []h2.Priority {
	out := make([]h2.Priority, len(priorities))
	for i, p := range priorities {
		out[i] = h2.Priority{
			StreamID:  p.StreamID,
			Exclusive: p.Exclusive,
			DependsOn: p.DependsOn,
			Weight:    p.Weight,
		}
	}
	return out
}
