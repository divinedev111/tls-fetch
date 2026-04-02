package internal

import (
	"net"
	"sync"
	"testing"
	"time"
)

// mockConn is a minimal net.Conn implementation that tracks closure.
type mockConn struct {
	mu     sync.Mutex
	closed bool
}

func (m *mockConn) Read(b []byte) (int, error)         { return 0, nil }
func (m *mockConn) Write(b []byte) (int, error)        { return len(b), nil }
func (m *mockConn) Close() error                       { m.mu.Lock(); m.closed = true; m.mu.Unlock(); return nil }
func (m *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (m *mockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func (m *mockConn) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestPool_PutAndGet(t *testing.T) {
	p := NewPool(PoolConfig{
		MaxConnsPerHost: 5,
		MaxIdleConns:    10,
		IdleTimeout:     30 * time.Second,
	})

	conn := &mockConn{}
	p.Put("example.com:443", conn)

	got := p.Get("example.com:443")
	if got == nil {
		t.Fatal("expected a connection, got nil")
	}
	if got != conn {
		t.Fatal("expected the same connection that was put in")
	}
	if conn.isClosed() {
		t.Fatal("connection should not be closed after Get")
	}
}

func TestPool_GetEmpty(t *testing.T) {
	p := NewPool(PoolConfig{
		MaxConnsPerHost: 5,
		MaxIdleConns:    10,
		IdleTimeout:     30 * time.Second,
	})

	got := p.Get("example.com:443")
	if got != nil {
		t.Fatalf("expected nil from empty pool, got %v", got)
	}
}

func TestPool_MaxConnsPerHost(t *testing.T) {
	maxPerHost := 2
	p := NewPool(PoolConfig{
		MaxConnsPerHost: maxPerHost,
		MaxIdleConns:    100,
		IdleTimeout:     30 * time.Second,
	})

	conns := make([]*mockConn, maxPerHost+1)
	for i := range conns {
		conns[i] = &mockConn{}
		p.Put("example.com:443", conns[i])
	}

	// The pool should have closed the conn that was rejected.
	// Exactly one conn should be closed.
	closedCount := 0
	for _, c := range conns {
		if c.isClosed() {
			closedCount++
		}
	}
	if closedCount != 1 {
		t.Fatalf("expected exactly 1 closed conn when exceeding max per host, got %d", closedCount)
	}
}

func TestPool_IdleTimeout(t *testing.T) {
	p := NewPool(PoolConfig{
		MaxConnsPerHost: 5,
		MaxIdleConns:    10,
		IdleTimeout:     50 * time.Millisecond,
	})

	conn := &mockConn{}
	p.Put("example.com:443", conn)

	// Wait for idle timeout to expire.
	time.Sleep(100 * time.Millisecond)

	got := p.Get("example.com:443")
	if got != nil {
		t.Fatal("expected nil after idle timeout, got a connection")
	}
	if !conn.isClosed() {
		t.Fatal("expired connection should have been closed")
	}
}

func TestPool_Stats(t *testing.T) {
	p := NewPool(PoolConfig{
		MaxConnsPerHost: 5,
		MaxIdleConns:    10,
		IdleTimeout:     30 * time.Second,
	})

	p.Put("host1:443", &mockConn{})
	p.Put("host1:443", &mockConn{})
	p.Put("host2:80", &mockConn{})

	stats := p.Stats()

	if stats.TotalIdle != 3 {
		t.Fatalf("expected TotalIdle=3, got %d", stats.TotalIdle)
	}
	if stats.Hosts != 2 {
		t.Fatalf("expected Hosts=2, got %d", stats.Hosts)
	}
}

func TestPool_Close_ClosesAll(t *testing.T) {
	p := NewPool(PoolConfig{
		MaxConnsPerHost: 5,
		MaxIdleConns:    10,
		IdleTimeout:     30 * time.Second,
	})

	conns := []*mockConn{{}, {}, {}}
	p.Put("host1:443", conns[0])
	p.Put("host1:443", conns[1])
	p.Put("host2:443", conns[2])

	p.Close()

	for i, c := range conns {
		if !c.isClosed() {
			t.Errorf("conn[%d] was not closed after pool.Close()", i)
		}
	}

	// Future puts should close the conn immediately.
	extra := &mockConn{}
	p.Put("host1:443", extra)
	if !extra.isClosed() {
		t.Error("put after Close() should close the connection immediately")
	}
}
