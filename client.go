package tlsfetch

import (
	"fmt"
	"net/http"
)

// Client wraps an http.Client backed by a fingerprinting Transport.
type Client struct {
	// HTTP is the underlying stdlib client. It can be used directly for
	// advanced use cases (e.g. custom CheckRedirect), but callers should
	// prefer the convenience methods on Client instead.
	HTTP      *http.Client
	transport *Transport
}

// NewClient creates a Client with TLS fingerprinting from the given options.
// At least one profile source must be provided.
func NewClient(opts ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	tr, err := NewTransport(opts...)
	if err != nil {
		return nil, fmt.Errorf("tlsfetch: new client: %w", err)
	}

	httpClient := &http.Client{
		Transport: tr,
		Timeout:   cfg.timeout,
	}

	if !cfg.followRedirects {
		httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return &Client{
		HTTP:      httpClient,
		transport: tr,
	}, nil
}

// Do sends an HTTP request and returns an HTTP response.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.HTTP.Do(req)
}

// Get issues a GET to the specified URL.
func (c *Client) Get(url string) (*http.Response, error) {
	return c.HTTP.Get(url)
}

// CloseIdleConnections closes idle connections in the transport and pool.
func (c *Client) CloseIdleConnections() {
	c.transport.CloseIdleConnections()
}
