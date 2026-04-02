package header

import (
	"bytes"
	"net"
	"strings"
)

// ReorderHTTP1Headers parses raw HTTP/1.1 request bytes, reorders header lines
// according to order, and returns the reassembled bytes. Headers not present in
// order are appended after the ordered ones in their original relative order.
// The request line and body (everything after \r\n\r\n) are preserved verbatim.
// If order is nil the input is returned unchanged.
func ReorderHTTP1Headers(raw []byte, order []string) []byte {
	if len(order) == 0 {
		return raw
	}

	// Split at first \r\n\r\n to separate head from body.
	sep := []byte("\r\n\r\n")
	sepIdx := bytes.Index(raw, sep)
	if sepIdx == -1 {
		return raw
	}
	head := raw[:sepIdx]
	body := raw[sepIdx+4:]

	// Split head into lines.
	lines := bytes.Split(head, []byte("\r\n"))
	if len(lines) == 0 {
		return raw
	}
	requestLine := lines[0]
	headerLines := lines[1:]

	// Build a lookup from lowercase name to its raw header line.
	// We preserve the original "Name: Value" text for each header.
	type entry struct {
		lowerName string
		raw       []byte
	}
	entries := make([]entry, 0, len(headerLines))
	for _, l := range headerLines {
		if len(l) == 0 {
			continue
		}
		colon := bytes.IndexByte(l, ':')
		if colon == -1 {
			continue
		}
		name := strings.ToLower(string(bytes.TrimSpace(l[:colon])))
		entries = append(entries, entry{lowerName: name, raw: l})
	}

	placed := make([]bool, len(entries))

	var out bytes.Buffer
	out.Write(requestLine)

	// Write ordered headers first.
	for _, name := range order {
		lower := strings.ToLower(name)
		for i, e := range entries {
			if !placed[i] && e.lowerName == lower {
				out.WriteString("\r\n")
				out.Write(e.raw)
				placed[i] = true
			}
		}
	}

	// Append any headers not in the order list, preserving their relative order.
	for i, e := range entries {
		if !placed[i] {
			out.WriteString("\r\n")
			out.Write(e.raw)
		}
	}

	out.Write(sep)
	out.Write(body)
	return out.Bytes()
}

// ReorderConn wraps a net.Conn and intercepts Write calls to reorder HTTP/1.1
// headers. It buffers data until the \r\n\r\n header terminator is seen,
// reorders the headers, and flushes the reordered bytes to the underlying
// connection. After the header block has been flushed, subsequent writes are
// passed through directly.
type ReorderConn struct {
	net.Conn
	order   []string
	buf     []byte
	flushed bool
}

// NewReorderConn wraps conn with a ReorderConn that will reorder headers
// according to order.
func NewReorderConn(conn net.Conn, order []string) *ReorderConn {
	return &ReorderConn{Conn: conn, order: order}
}

// Write buffers data until the end of the HTTP header block is detected, then
// reorders and flushes. After that all writes pass through directly.
func (r *ReorderConn) Write(b []byte) (int, error) {
	if r.flushed {
		return r.Conn.Write(b)
	}

	r.buf = append(r.buf, b...)

	sep := []byte("\r\n\r\n")
	if idx := bytes.Index(r.buf, sep); idx != -1 {
		reordered := ReorderHTTP1Headers(r.buf, r.order)
		r.flushed = true
		r.buf = nil
		_, err := r.Conn.Write(reordered)
		if err != nil {
			return 0, err
		}
	}

	// Return the full length to signal success to the caller even if we are
	// still buffering.
	return len(b), nil
}
