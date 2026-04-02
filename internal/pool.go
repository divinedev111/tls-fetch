// Package internal provides shared internal utilities for tls-fetch.
package internal

import (
	"net"
	"sync"
	"time"
)

// PoolConfig holds configuration for the connection pool.
type PoolConfig struct {
	// MaxConnsPerHost limits how many idle connections are kept per host.
	// Connections put when the pool for a host is at capacity are closed.
	MaxConnsPerHost int
	// MaxIdleConns limits the total number of idle connections across all hosts.
	// Connections put when this global limit is reached are closed.
	MaxIdleConns int
	// IdleTimeout is how long a connection may remain idle before it is
	// considered expired and closed on the next Get.
	IdleTimeout time.Duration
	// TTL is the absolute lifetime of a connection (unused by pool directly;
	// the caller is responsible for honoring it before calling Put).
	TTL time.Duration
}

// PoolStats contains a snapshot of pool state.
type PoolStats struct {
	// TotalIdle is the total number of idle connections across all hosts.
	TotalIdle int
	// Hosts is the number of distinct hosts with at least one idle connection.
	Hosts int
}

// idleConn wraps a net.Conn with the time it was returned to the pool.
type idleConn struct {
	conn      net.Conn
	idleSince time.Time
}

// Pool is a per-host connection pool. It is safe for concurrent use.
//
// Retrieval order is LIFO (most-recently-idle first) to keep connections
// warm and avoid idle-timeout races.
type Pool struct {
	cfg    PoolConfig
	mu     sync.Mutex
	idle   map[string][]idleConn // host → stack of idle conns
	total  int                   // total idle conns across all hosts
	closed bool
}

// NewPool creates a ready-to-use Pool with the given configuration.
func NewPool(cfg PoolConfig) *Pool {
	return &Pool{
		cfg:  cfg,
		idle: make(map[string][]idleConn),
	}
}

// Get retrieves the most-recently-idle connection for host. Expired
// connections are closed and skipped. Returns nil when no live connection is
// available.
func (p *Pool) Get(host string) net.Conn {
	p.mu.Lock()
	defer p.mu.Unlock()

	stack := p.idle[host]
	for len(stack) > 0 {
		// Pop from the end (LIFO).
		n := len(stack) - 1
		ic := stack[n]
		stack = stack[:n]

		if p.cfg.IdleTimeout > 0 && time.Since(ic.idleSince) > p.cfg.IdleTimeout {
			// Expired — close and try the next one.
			ic.conn.Close() //nolint:errcheck
			p.total--
			continue
		}

		// Live connection found.
		p.idle[host] = stack
		if len(stack) == 0 {
			delete(p.idle, host)
		}
		p.total--
		return ic.conn
	}

	// Drained without finding a live connection.
	delete(p.idle, host)
	return nil
}

// Put returns conn to the pool for host. If the pool is closed, at capacity
// for that host, or at the global idle-connection limit, conn is closed
// immediately.
func (p *Pool) Put(host string, conn net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		conn.Close() //nolint:errcheck
		return
	}

	perHostFull := p.cfg.MaxConnsPerHost > 0 && len(p.idle[host]) >= p.cfg.MaxConnsPerHost
	globalFull := p.cfg.MaxIdleConns > 0 && p.total >= p.cfg.MaxIdleConns

	if perHostFull || globalFull {
		conn.Close() //nolint:errcheck
		return
	}

	p.idle[host] = append(p.idle[host], idleConn{conn: conn, idleSince: time.Now()})
	p.total++
}

// Stats returns a snapshot of current pool state.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return PoolStats{
		TotalIdle: p.total,
		Hosts:     len(p.idle),
	}
}

// Close closes all pooled connections and prevents future use. Any connection
// Put after Close is closed immediately.
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for host, stack := range p.idle {
		for _, ic := range stack {
			ic.conn.Close() //nolint:errcheck
		}
		delete(p.idle, host)
	}
	p.total = 0
	p.closed = true
}
