package proxy

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

const (
	socks5Version = 0x05

	// Authentication methods.
	authNone     = 0x00
	authUserPass = 0x02
	authNoAccept = 0xFF

	// Command codes.
	cmdConnect = 0x01

	// Address types.
	atypIPv4   = 0x01
	atypDomain = 0x03
	atypIPv6   = 0x04

	// Sub-negotiation version for username/password auth (RFC 1929).
	authSubVer = 0x01
)

// socks5Dialer tunnels connections through a SOCKS5 proxy.
type socks5Dialer struct {
	addr string
	user *url.Userinfo
}

// DialContext performs the SOCKS5 handshake with the proxy at s.addr and
// returns a connection tunneled to the target addr.
func (s *socks5Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var nd net.Dialer
	conn, err := nd.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return nil, fmt.Errorf("proxy: dial SOCKS5 proxy %s: %w", s.addr, err)
	}

	if err := s.handshake(conn, addr); err != nil {
		conn.Close()
		return nil, fmt.Errorf("proxy: SOCKS5 handshake: %w", err)
	}
	return conn, nil
}

func (s *socks5Dialer) handshake(conn net.Conn, target string) error {
	// --- Step 1: Greeting ---
	methods := []byte{authNone}
	if s.user != nil {
		methods = []byte{authUserPass}
	}
	greeting := append([]byte{socks5Version, byte(len(methods))}, methods...)
	if _, err := conn.Write(greeting); err != nil {
		return fmt.Errorf("write greeting: %w", err)
	}

	// Server method selection.
	resp := make([]byte, 2)
	if _, err := readFull(conn, resp); err != nil {
		return fmt.Errorf("read method selection: %w", err)
	}
	if resp[0] != socks5Version {
		return fmt.Errorf("unexpected SOCKS version %d", resp[0])
	}
	if resp[1] == authNoAccept {
		return fmt.Errorf("proxy rejected all authentication methods")
	}

	// --- Step 2: Authentication (if required) ---
	switch resp[1] {
	case authNone:
		// No authentication needed.
	case authUserPass:
		if s.user == nil {
			return fmt.Errorf("proxy requires authentication but no credentials provided")
		}
		if err := s.doUserPassAuth(conn); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported authentication method %d", resp[1])
	}

	// --- Step 3: CONNECT request ---
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return fmt.Errorf("invalid target address %q: %w", target, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port %q: %w", portStr, err)
	}

	req, err := buildConnectRequest(host, uint16(port))
	if err != nil {
		return err
	}
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("write CONNECT request: %w", err)
	}

	// --- Step 4: Read reply ---
	return readReply(conn)
}

// doUserPassAuth performs RFC 1929 username/password sub-negotiation.
func (s *socks5Dialer) doUserPassAuth(conn net.Conn) error {
	username := s.user.Username()
	password, _ := s.user.Password()

	msg := make([]byte, 0, 3+len(username)+len(password))
	msg = append(msg, authSubVer, byte(len(username)))
	msg = append(msg, []byte(username)...)
	msg = append(msg, byte(len(password)))
	msg = append(msg, []byte(password)...)

	if _, err := conn.Write(msg); err != nil {
		return fmt.Errorf("write auth: %w", err)
	}

	res := make([]byte, 2)
	if _, err := readFull(conn, res); err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}
	if res[1] != 0x00 {
		return fmt.Errorf("authentication failed (status %d)", res[1])
	}
	return nil
}

// buildConnectRequest assembles the SOCKS5 CONNECT message for host:port.
func buildConnectRequest(host string, port uint16) ([]byte, error) {
	// Determine address type.
	var addrBytes []byte
	var atyp byte

	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			atyp = atypIPv4
			addrBytes = ip4
		} else {
			atyp = atypIPv6
			addrBytes = ip.To16()
		}
	} else {
		// Domain name.
		if len(host) > 255 {
			return nil, fmt.Errorf("domain name too long: %d bytes", len(host))
		}
		atyp = atypDomain
		addrBytes = append([]byte{byte(len(host))}, []byte(host)...)
	}

	// VER CMD RSV ATYP [ADDR] PORT
	req := []byte{socks5Version, cmdConnect, 0x00, atyp}
	req = append(req, addrBytes...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	req = append(req, portBytes...)
	return req, nil
}

// readReply reads and validates the SOCKS5 reply.
func readReply(conn net.Conn) error {
	// VER REP RSV ATYP
	hdr := make([]byte, 4)
	if _, err := readFull(conn, hdr); err != nil {
		return fmt.Errorf("read reply header: %w", err)
	}
	if hdr[0] != socks5Version {
		return fmt.Errorf("unexpected SOCKS version in reply: %d", hdr[0])
	}
	if hdr[1] != 0x00 {
		return fmt.Errorf("SOCKS5 CONNECT failed with code %d", hdr[1])
	}

	// Consume the bound address so the stream is positioned at the tunnel.
	switch hdr[3] {
	case atypIPv4:
		buf := make([]byte, 4+2)
		_, err := readFull(conn, buf)
		return err
	case atypIPv6:
		buf := make([]byte, 16+2)
		_, err := readFull(conn, buf)
		return err
	case atypDomain:
		lenBuf := make([]byte, 1)
		if _, err := readFull(conn, lenBuf); err != nil {
			return err
		}
		buf := make([]byte, int(lenBuf[0])+2)
		_, err := readFull(conn, buf)
		return err
	default:
		return fmt.Errorf("unknown address type in reply: %d", hdr[3])
	}
}

// readFull reads exactly len(buf) bytes from conn into buf.
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
