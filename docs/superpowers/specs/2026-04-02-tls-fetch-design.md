# tls-fetch Design Spec

## Overview

A Go HTTP client library with TLS and HTTP/2 fingerprinting that returns standard `net/http` types. Unlike every existing library in this space (bogdanfinn/tls-client, azuretls-client, CycleTLS), tls-fetch does NOT fork `net/http`. It implements a custom `http.RoundTripper` that composes uTLS for TLS fingerprinting with a minimally-forked HTTP/2 transport, while exposing real `*http.Request` and `*http.Response` to consumers.

This means tls-fetch works with all existing Go HTTP middleware, OpenTelemetry, and any code expecting standard library types.

## Why This Exists

Every Go TLS fingerprinting library forks `net/http` into `fhttp`, creating an ecosystem island where standard Go middleware, tooling, and libraries break. Profiles are hardcoded as Go structs, so updating for a new browser release requires code changes and recompilation. No library validates that your configured fingerprint actually matches what you think it does.

tls-fetch fixes all three problems:
1. Standard `net/http` types everywhere
2. Data-driven JSON profiles loadable at runtime
3. Built-in fingerprint validation

## Target Audience

- Go developers building scrapers, monitoring tools, or API clients that need to bypass bot detection
- Anyone currently using bogdanfinn/tls-client or azuretls-client who wants standard library compatibility

## Go Version

Go 1.21+ (enables `log/slog`, `slices`, `maps` packages)

## Architecture

### Approach: Transport-Layer Wrapper

tls-fetch builds a custom `http.RoundTripper` that wraps uTLS for TLS and a minimally-forked HTTP/2 transport, but exposes standard `net/http` types to consumers.

**What gets forked (minimal surface):**
- `h2/` package: forked from `golang.org/x/net/http2`, modified ONLY for SETTINGS frame ordering, pseudo-header ordering, WINDOW_UPDATE value, and PRIORITY frames. All other HTTP/2 logic (HPACK, flow control, multiplexing) stays upstream.

**What does NOT get forked:**
- `net/http` -- real stdlib types used everywhere
- `crypto/tls` -- `refraction-networking/utls` used as a dependency, not forked
- Cookie jar, redirect logic, connection management -- all stdlib

### Transport Flow

```
Consumer code (standard net/http)
        |
        v
+-------------------------+
|   tlsfetch.Transport    |  <- implements http.RoundTripper
|   (orchestrator)        |
+-------------------------+
| 1. Dial TCP + TLS       |  <- uTLS with profile's ClientHelloSpec
| 2. Check ALPN result    |
| 3. Route to protocol    |
+------------+------------+
|  HTTP/1.1  |   HTTP/2   |
|  transport |  transport |
| (stdlib +  | (forked    |
|  ordered   |  h2/ pkg   |
|  headers)  |  w/ custom |
|            |  framing)  |
+------------+------------+
        |
        v
   Connection Pool
   (per-host, TTL, max conns, idle eviction)
```

### HTTP/1.1 Header Ordering

Instead of forking `net/http`, the transport uses a custom `net.Conn` wrapper (`headerOrderConn`) that intercepts `Write()` calls. When the stdlib HTTP/1.1 transport writes headers, the wrapper buffers the raw header bytes, reorders them according to the profile's `HeaderOrder`, and then flushes to the underlying connection. This approach means the stdlib transport writes headers in its default order, but the bytes that actually hit the wire are reordered. The wrapper is only active during the header-writing phase of a request; body writes pass through directly.

## Package Structure

```
tls-fetch/
  tlsfetch.go              # Public entry points: NewClient, NewSession
  client.go                # Client type: wraps http.Client with fingerprinted transport
  session.go               # Session type: stateful client with cookies, headers, helpers
  transport.go             # Core RoundTripper: combines uTLS + HTTP/2 framing
  profile.go               # BrowserProfile type + built-in profiles
  profile_loader.go        # Load profiles from JSON files at runtime
  ja3.go                   # JA3 string parsing to profile conversion
  fingerprint.go           # Calculate + validate JA3/JA4/H2 fingerprint of a profile
  h2/
    transport.go           # Forked HTTP/2 transport (SETTINGS, pseudo-headers, window)
    frame.go               # HTTP/2 frame writing with custom ordering
    priority.go            # Stream priority configuration
  header/
    ordered.go             # OrderedHeader type: preserves insertion order
    writer.go              # Custom HTTP/1.1 header serializer (ordered output)
  proxy/
    proxy.go               # Proxy dialer interface
    http.go                # HTTP CONNECT proxy
    socks5.go              # SOCKS5 proxy
  profiles/
    chrome_131.json        # Data-driven browser profiles
    firefox_128.json
    safari_18.json
    edge_131.json
  internal/
    pool.go                # Connection pool: TTL, max conns, idle eviction
  cmd/
    tlsfetch/
      main.go              # CLI: test fingerprints, make requests, validate profiles
  examples/
    basic/main.go
    session/main.go
    custom_profile/main.go
    proxy/main.go
```

Each package has one responsibility. No file exceeds ~400 lines.

## Core Types

### BrowserProfile

The fundamental configuration unit. Bundles TLS + HTTP/2 + header settings into a single coherent identity.

```go
type BrowserProfile struct {
    Name string

    // TLS
    ClientHello utls.ClientHelloSpec

    // HTTP/2
    Settings      []H2Setting    // ordered key-value pairs
    WindowUpdate  uint32
    Priorities    []H2Priority
    PseudoHeaders [4]string      // e.g. {"m","a","s","p"}

    // HTTP/1.1 defaults
    DefaultHeaders header.Ordered
    HeaderOrder    []string       // for when consumer sets headers by map
}

type H2Setting struct {
    ID    uint16
    Value uint32
}

type H2Priority struct {
    StreamID  uint32
    Exclusive bool
    DependsOn uint32
    Weight    uint8
}
```

### Built-in Profiles

v1.0 ships with 4 validated profiles:
- `Chrome131` -- Chrome 131 on Windows/macOS
- `Firefox128` -- Firefox 128 on Windows/macOS
- `Safari18` -- Safari 18 on macOS
- `Edge131` -- Edge 131 on Windows

Each profile is also available as a JSON file in `profiles/` for reference and as a template for custom profiles.

### JSON Profile Format

```json
{
  "name": "chrome_131",
  "tls": {
    "version": "0x0303",
    "ciphers": ["TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"],
    "extensions": ["..."],
    "curves": ["X25519", "P-256", "P-384"],
    "point_formats": ["uncompressed"],
    "grease": true,
    "randomize_extensions": true
  },
  "h2": {
    "settings": [
      {"id": "HEADER_TABLE_SIZE", "value": 65536},
      {"id": "ENABLE_PUSH", "value": 0},
      {"id": "INITIAL_WINDOW_SIZE", "value": 6291456},
      {"id": "MAX_HEADER_LIST_SIZE", "value": 262144}
    ],
    "window_update": 15663105,
    "pseudo_header_order": ["m", "a", "s", "p"]
  },
  "headers": {
    "default_order": ["User-Agent", "Accept", "Accept-Language", "Accept-Encoding"]
  }
}
```

When a new browser version ships, users add a JSON file and load it at runtime. No Go code changes, no recompilation.

## API Design

### Level 1: Client (low-level, standard net/http)

```go
client, err := tlsfetch.NewClient(
    tlsfetch.WithProfile(tlsfetch.Chrome131),
    tlsfetch.WithProxy("socks5://127.0.0.1:1080"),
    tlsfetch.WithTimeout(30 * time.Second),
)

// Standard net/http usage -- works with any middleware
req, _ := http.NewRequest("GET", "https://example.com", nil)
resp, err := client.Do(req)  // *http.Response
```

The `Client` wraps `http.Client` with the fingerprinted `Transport`. The `Transport` implements `http.RoundTripper` and can be used standalone:

```go
transport, err := tlsfetch.NewTransport(tlsfetch.WithProfile(tlsfetch.Chrome131))

// Use with any http.Client
httpClient := &http.Client{Transport: transport}
```

### Level 2: Session (high-level, stateful)

```go
session := tlsfetch.NewSession(
    tlsfetch.WithProfile(tlsfetch.Chrome131),
)

resp, err := session.Get("https://example.com")
resp, err := session.Post("https://api.com/data", body)
resp, err := session.Put(url, body)
resp, err := session.Delete(url)
resp, err := session.Patch(url, body)
resp, err := session.Head(url)

// Cookies persist automatically across requests
// Ordered headers
session.SetOrderedHeaders(header.Ordered{
    {"User-Agent", "Mozilla/5.0 ..."},
    {"Accept", "text/html"},
})
```

The `Session` wraps `Client` and adds:
- Automatic cookie persistence (stdlib `http.CookieJar`)
- Ordered headers set once, applied to every request
- Convenience methods (Get, Post, etc.) that accept `string`, `[]byte`, `io.Reader`, or any struct (JSON-marshaled)
- Response helpers: `resp.Body` is standard `io.ReadCloser`

### Configuration Options (functional options pattern)

```go
tlsfetch.WithProfile(profile BrowserProfile)
tlsfetch.WithProfileFromFile(path string)     // load JSON at runtime
tlsfetch.WithProfileFromJA3(ja3 string)       // parse JA3 string
tlsfetch.WithProxy(url string)
tlsfetch.WithTimeout(d time.Duration)
tlsfetch.WithLogger(logger *slog.Logger)
tlsfetch.WithPoolConfig(cfg PoolConfig)       // max conns, TTL, idle timeout
tlsfetch.WithInsecureSkipVerify()             // disable cert validation
tlsfetch.WithRedirectPolicy(policy RedirectPolicy)
```

### Fingerprint Validation

```go
report, err := tlsfetch.ValidateProfile(tlsfetch.Chrome131)
// report.JA3Hash  string -- MD5 of JA3 string
// report.JA3      string -- raw JA3 string
// report.JA4      string -- JA4 fingerprint
// report.H2       string -- Akamai HTTP/2 fingerprint
// report.Match    bool   -- true if all match known Chrome 131 fingerprints
// report.Details  []ValidationDetail -- per-component pass/fail
```

### JA3 Parsing

```go
profile, err := tlsfetch.ProfileFromJA3("771,4865-4866-4867-49195-...,0-23-65281-10-11-...,29-23-24,0")
client, err := tlsfetch.NewClient(tlsfetch.WithProfile(profile))
```

## Connection Pool

```go
type PoolConfig struct {
    MaxConnsPerHost int           // default: 10
    MaxIdleConns    int           // default: 100
    IdleTimeout     time.Duration // default: 90s
    TTL             time.Duration // default: 0 (no TTL)
}
```

The pool is internal to the transport. Features:
- Per-host connection grouping
- TTL-based eviction (connections can go stale with long-lived TLS sessions)
- Max connections per host (prevents resource exhaustion under load)
- Idle timeout (release connections not used recently)
- Thread-safe with `sync.Mutex`
- Observable: `transport.PoolStats()` returns active/idle/total counts per host

## Proxy Support

Three proxy types in v1:

- **HTTP CONNECT**: Standard HTTP proxy with CONNECT tunnel for HTTPS
- **SOCKS5**: With and without authentication
- **Direct**: No proxy (default)

Per-client proxy configuration. Proxy dialing happens before TLS, so the fingerprinted TLS handshake goes through the proxy tunnel transparently.

```go
tlsfetch.WithProxy("http://user:pass@proxy.example.com:8080")
tlsfetch.WithProxy("socks5://127.0.0.1:1080")
```

## Structured Logging

Uses `log/slog` from the Go stdlib (1.21+). No custom logger interface needed.

```go
logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

client, _ := tlsfetch.NewClient(
    tlsfetch.WithProfile(tlsfetch.Chrome131),
    tlsfetch.WithLogger(logger),
)

// Logs at Debug level:
// {"level":"DEBUG","msg":"tls handshake","host":"example.com","protocol":"h2","ja3":"abc123..."}
// {"level":"DEBUG","msg":"request","method":"GET","url":"https://example.com","status":200}
```

## CLI Tool

```bash
# Make a request with a profile
tlsfetch curl https://example.com --profile chrome131

# Show your fingerprint
tlsfetch fingerprint --profile chrome131
# JA3:  771,4865-4866-...
# JA4:  t13d1516h2_8daaf6152771_...
# H2:   1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p

# Validate a profile against known fingerprints
tlsfetch validate --profile chrome131
# Chrome 131: PASS (JA3 match, JA4 match, H2 match)

# Load and test a custom JSON profile
tlsfetch curl https://example.com --profile-file ./my_profile.json

# Verbose output showing TLS handshake details
tlsfetch curl https://example.com --profile chrome131 -v
```

## Testing Strategy

### Unit Tests (per package)

| Package | What's Tested |
|---------|---------------|
| `profile.go` | Built-in profiles have valid TLS specs, HTTP/2 settings, no nil fields |
| `profile_loader.go` | JSON parsing, malformed JSON errors, missing required fields rejected |
| `ja3.go` | JA3 string parsing: valid strings, invalid strings, GREASE handling, edge cases |
| `fingerprint.go` | JA3/JA4/H2 calculation matches known values for each built-in profile |
| `header/ordered.go` | Insertion order preserved, Get/Set/Del work, Clone does not alias |
| `header/writer.go` | Headers serialize in exact specified order |
| `h2/transport.go` | SETTINGS frame bytes match expected, pseudo-header order correct |
| `proxy/` | HTTP CONNECT handshake, SOCKS5 auth + no-auth, bad proxy errors |
| `internal/pool.go` | TTL eviction, max conns enforced, idle timeout, concurrent access safe |
| `transport.go` | ALPN routing (h1 vs h2), profile applied correctly, connection reuse |
| `session.go` | Cookie persistence, ordered headers sent, redirects followed |

### Integration Tests

Test against real fingerprint validation services (skipped with `-short`):

```go
func TestChrome131_RealFingerprint(t *testing.T) {
    // Validates against tls.peet.ws that the on-wire fingerprint
    // matches known Chrome 131 JA3/H2 values
}
```

Run for each built-in profile: Chrome 131, Firefox 128, Safari 18, Edge 131.

### Benchmark Tests

```go
BenchmarkRoundTrip          // request throughput
BenchmarkProfileLoad        // JSON profile parsing speed
BenchmarkConnectionPool     // pool get/put under contention
BenchmarkJA3Parse           // JA3 string parsing
```

## Dependencies

- `github.com/refraction-networking/utls` -- TLS fingerprinting (the foundation, not forked)
- `golang.org/x/net/http2` -- forked minimally into `h2/` package
- Go stdlib only for everything else (`net/http`, `log/slog`, `crypto/tls`, `encoding/json`)

No third-party dependencies beyond uTLS. This is a design principle, not an accident.

## v1.0 Scope

**Ships:**
- 4 validated browser profiles (Chrome 131, Firefox 128, Safari 18, Edge 131)
- JSON profile loader for custom/runtime profiles
- JA3 string to profile conversion
- Fingerprint validation (JA3, JA4, H2) with known-value comparison
- Session API (Get/Post/Put/Delete/Patch/Head, auto cookies, ordered headers)
- Client API (standard http.RoundTripper, drop-in compatible)
- HTTP/1.1 ordered headers without forking net/http
- HTTP/2 custom framing (SETTINGS, WINDOW_UPDATE, pseudo-header order)
- Proxy support (HTTP CONNECT, SOCKS5)
- Connection pool with TTL, max conns, idle eviction, stats
- CLI tool for testing fingerprints and making requests
- Structured logging via log/slog
- 4 example programs
- Full unit + integration + benchmark test suite

**NOT in v1.0 scope:**
- HTTP/3 (QUIC) support
- WebSocket with fingerprinting
- JA4 string to profile conversion (JA4 in validation only)
- Android / iOS / additional browser profiles
- Auto-profile updater service
- Retry / rate limiting middleware
- FFI / C shared library for other languages
- Certificate pinning

## Non-Goals

- This is not a web scraping framework. It is an HTTP client library.
- This does not solve JavaScript challenges, CAPTCHAs, or behavioral detection. It solves TLS and HTTP/2 fingerprinting only.
- This does not manage browser-level state (localStorage, IndexedDB, etc.).
