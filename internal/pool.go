package internal

import (
	"net"
	"sync"
	"time"
)

// PoolConfig controls connection pool behavior.
type PoolConfig struct {
	MaxConnsPerHost int
	MaxIdleConns    int
	IdleTimeout     time.Duration
	TTL             time.Duration
}

// PoolStats is a snapshot of pool state.
type PoolStats struct {
	TotalIdle int
	Hosts     int
}

type idleConn struct {
	conn      net.Conn
	idleSince time.Time
}

// Pool is a thread-safe, per-host connection pool with LIFO retrieval.
type Pool struct {
	cfg    PoolConfig
	mu     sync.Mutex
	idle   map[string][]idleConn
	total  int
	closed bool
}

// NewPool creates a connection pool.
func NewPool(cfg PoolConfig) *Pool {
	return &Pool{cfg: cfg, idle: make(map[string][]idleConn)}
}

// Get retrieves the most recently idle connection for host, or nil.
func (p *Pool) Get(host string) net.Conn {
	p.mu.Lock()
	defer p.mu.Unlock()

	stack := p.idle[host]
	for len(stack) > 0 {
		n := len(stack) - 1
		ic := stack[n]
		stack = stack[:n]

		if p.cfg.IdleTimeout > 0 && time.Since(ic.idleSince) > p.cfg.IdleTimeout {
			ic.conn.Close() //nolint:errcheck
			p.total--
			continue
		}

		p.idle[host] = stack
		if len(stack) == 0 {
			delete(p.idle, host)
		}
		p.total--
		return ic.conn
	}

	delete(p.idle, host)
	return nil
}

// Put returns a connection to the pool. Closes it if the pool is full.
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

// Stats returns current pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return PoolStats{TotalIdle: p.total, Hosts: len(p.idle)}
}

// Close closes all pooled connections and prevents future use.
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
