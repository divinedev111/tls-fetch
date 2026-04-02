package tlsfetch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"

	"github.com/mukuln-official/tls-fetch/header"
)

// Session is a high-level HTTP client that persists cookies across requests
// and supports ordered default headers.
type Session struct {
	client  *Client
	headers header.Ordered
	jar     http.CookieJar
}

// NewSession creates a Session with TLS fingerprinting from the given options.
func NewSession(opts ...Option) (*Session, error) {
	c, err := NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("tlsfetch: new session: %w", err)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("tlsfetch: cookie jar: %w", err)
	}
	c.HTTP.Jar = jar

	return &Session{client: c, jar: jar}, nil
}

// SetOrderedHeaders sets the default headers merged into every request.
// Per-request headers take precedence.
func (s *Session) SetOrderedHeaders(headers header.Ordered) {
	s.headers = headers
}

// Get issues a GET to the specified URL.
func (s *Session) Get(url string) (*http.Response, error) {
	return s.do(http.MethodGet, url, nil)
}

// Post issues a POST with the given body.
// Body: string, []byte, io.Reader, or any struct (JSON-marshaled).
func (s *Session) Post(url string, body any) (*http.Response, error) {
	return s.do(http.MethodPost, url, body)
}

// Put issues a PUT with the given body.
func (s *Session) Put(url string, body any) (*http.Response, error) {
	return s.do(http.MethodPut, url, body)
}

// Delete issues a DELETE to the specified URL.
func (s *Session) Delete(url string) (*http.Response, error) {
	return s.do(http.MethodDelete, url, nil)
}

// Patch issues a PATCH with the given body.
func (s *Session) Patch(url string, body any) (*http.Response, error) {
	return s.do(http.MethodPatch, url, body)
}

// Head issues a HEAD to the specified URL.
func (s *Session) Head(url string) (*http.Response, error) {
	return s.do(http.MethodHead, url, nil)
}

// CloseIdleConnections closes idle connections in the underlying client.
func (s *Session) CloseIdleConnections() {
	s.client.CloseIdleConnections()
}

// do is the shared request path for all HTTP methods.
func (s *Session) do(method, url string, body any) (*http.Response, error) {
	var r io.Reader
	var contentType string

	if body != nil {
		var err error
		r, contentType, err = encodeBody(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	s.applyHeaders(req)
	return s.client.Do(req)
}

func (s *Session) applyHeaders(req *http.Request) {
	for _, pair := range s.headers {
		if req.Header.Get(pair[0]) == "" {
			req.Header.Set(pair[0], pair[1])
		}
	}
}

func encodeBody(body any) (io.Reader, string, error) {
	switch v := body.(type) {
	case string:
		return strings.NewReader(v), "", nil
	case []byte:
		return bytes.NewReader(v), "", nil
	case io.Reader:
		return v, "", nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, "", fmt.Errorf("tlsfetch: marshal body: %w", err)
		}
		return bytes.NewReader(data), "application/json", nil
	}
}
