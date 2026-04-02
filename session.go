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
// and supports ordered default headers. It wraps a Client and attaches
// session-level headers to every outgoing request.
type Session struct {
	client  *Client
	headers header.Ordered
	jar     http.CookieJar
}

// NewSession creates a Session with TLS fingerprinting from the given options.
// At least one profile source must be provided.
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

	return &Session{
		client: c,
		jar:    jar,
	}, nil
}

// SetOrderedHeaders sets the default headers that will be merged into every
// request. Per-request headers take precedence over session headers.
func (s *Session) SetOrderedHeaders(headers header.Ordered) {
	s.headers = headers
}

// Get issues a GET to the specified URL with session headers and cookies.
func (s *Session) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	s.applyHeaders(req)
	return s.client.Do(req)
}

// Post issues a POST to the specified URL with the given body.
// body encoding: string -> StringReader, []byte -> BytesReader,
// io.Reader -> passthrough, anything else -> json.Marshal.
func (s *Session) Post(url string, body interface{}) (*http.Response, error) {
	r, contentType, err := encodeBody(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, r)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	s.applyHeaders(req)
	return s.client.Do(req)
}

// Put issues a PUT to the specified URL with the given body.
func (s *Session) Put(url string, body interface{}) (*http.Response, error) {
	r, contentType, err := encodeBody(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, url, r)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	s.applyHeaders(req)
	return s.client.Do(req)
}

// Delete issues a DELETE to the specified URL.
func (s *Session) Delete(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	s.applyHeaders(req)
	return s.client.Do(req)
}

// Patch issues a PATCH to the specified URL with the given body.
func (s *Session) Patch(url string, body interface{}) (*http.Response, error) {
	r, contentType, err := encodeBody(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPatch, url, r)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	s.applyHeaders(req)
	return s.client.Do(req)
}

// Head issues a HEAD to the specified URL.
func (s *Session) Head(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	s.applyHeaders(req)
	return s.client.Do(req)
}

// CloseIdleConnections closes idle connections in the underlying client.
func (s *Session) CloseIdleConnections() {
	s.client.CloseIdleConnections()
}

// applyHeaders merges session-level ordered headers into the request.
// Existing request headers are not overwritten.
func (s *Session) applyHeaders(req *http.Request) {
	for _, pair := range s.headers {
		if req.Header.Get(pair[0]) == "" {
			req.Header.Set(pair[0], pair[1])
		}
	}
}

// encodeBody converts a body value into an io.Reader and optional Content-Type.
func encodeBody(body interface{}) (io.Reader, string, error) {
	if body == nil {
		return nil, "", nil
	}
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
