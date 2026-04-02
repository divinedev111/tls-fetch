// Package header provides an insertion-order-preserving HTTP header type
// and utilities for reordering headers in raw HTTP/1.1 requests.
package header

import (
	"net/http"
	"strings"
)

// HeaderPair is a name-value pair representing a single HTTP header.
type HeaderPair [2]string

// Ordered is a slice of HeaderPairs that preserves insertion order.
// Name matching for all methods is case-insensitive.
type Ordered []HeaderPair

// Get returns the value of the first header with the given name, or an empty
// string if no match is found.
func (o Ordered) Get(name string) string {
	for _, p := range o {
		if strings.EqualFold(p[0], name) {
			return p[1]
		}
	}
	return ""
}

// Set replaces the value of the first matching header in place. If no header
// with that name exists, a new pair is appended.
func (o *Ordered) Set(name, value string) {
	for i, p := range *o {
		if strings.EqualFold(p[0], name) {
			(*o)[i][1] = value
			return
		}
	}
	*o = append(*o, HeaderPair{name, value})
}

// Del removes all headers with the given name.
func (o *Ordered) Del(name string) {
	out := (*o)[:0]
	for _, p := range *o {
		if !strings.EqualFold(p[0], name) {
			out = append(out, p)
		}
	}
	*o = out
}

// Clone returns a deep copy of the Ordered slice. Mutating the clone does not
// affect the original.
func (o Ordered) Clone() Ordered {
	c := make(Ordered, len(o))
	copy(c, o)
	return c
}

// ToHTTPHeader converts the Ordered slice to a standard net/http Header map.
// Keys are canonicalized. Multiple values for the same key are preserved.
func (o Ordered) ToHTTPHeader() http.Header {
	h := make(http.Header, len(o))
	for _, p := range o {
		h.Add(p[0], p[1])
	}
	return h
}

// Keys returns the header names in insertion order.
func (o Ordered) Keys() []string {
	keys := make([]string, len(o))
	for i, p := range o {
		keys[i] = p[0]
	}
	return keys
}
