# tls-fetch

Go HTTP client with TLS fingerprinting. Standard `net/http` types. No forks.

## Why

- **Standard types.** `NewTransport` returns an `http.RoundTripper`. Drop it into any existing `http.Client` with no code changes.
- **JSON profiles.** Browser fingerprints are plain JSON files you can edit, version-control, and share without recompiling.
- **Fingerprint validation.** Built-in tooling computes and validates JA3, Akamai H2, and pseudo-header order before you send a single request.

## Install

```
go get github.com/mukuln-official/tls-fetch
```

## Quick Start

```go
session, err := tlsfetch.NewSession(tlsfetch.WithProfile(tlsfetch.Chrome131))
if err != nil {
    log.Fatal(err)
}
defer session.CloseIdleConnections()

resp, err := session.Get("https://tls.peet.ws/api/all")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()

body, _ := io.ReadAll(resp.Body)
fmt.Println(string(body))
```

## Features

### RoundTripper

Use `NewTransport` wherever `http.RoundTripper` is accepted:

```go
tr, err := tlsfetch.NewTransport(tlsfetch.WithProfile(tlsfetch.Chrome131))
client := &http.Client{Transport: tr}
resp, err := client.Get("https://example.com")
```

### Session

`NewSession` wraps `http.Client` with a cookie jar and ordered default headers:

```go
session, _ := tlsfetch.NewSession(tlsfetch.WithProfile(tlsfetch.Firefox128))
session.SetOrderedHeaders(header.Ordered{
    {"User-Agent", "Mozilla/5.0 ..."},
    {"Accept-Language", "en-US,en;q=0.9"},
})
resp, _ := session.Post("https://example.com/login", map[string]string{"user": "alice"})
```

Available methods: `Get`, `Post`, `Put`, `Delete`, `Patch`, `Head`.

### Custom JSON Profiles

Load a fingerprint profile from a JSON file at runtime:

```go
session, err := tlsfetch.NewSession(
    tlsfetch.WithProfileFromFile("profiles/chrome_131.json"),
)
```

See `profiles/` for the bundled examples. The JSON schema matches `BrowserProfile`.

### Proxy Support

Route all traffic through a SOCKS5 or HTTP proxy:

```go
session, err := tlsfetch.NewSession(
    tlsfetch.WithProfile(tlsfetch.Chrome131),
    tlsfetch.WithProxy("socks5://127.0.0.1:1080"),
)
```

### Fingerprint Validation

Compute and validate profiles without making any network requests:

```go
fp := tlsfetch.CalculateH2Fingerprint(tlsfetch.Chrome131)
// "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p"

report := tlsfetch.ValidateProfile(tlsfetch.Chrome131)
fmt.Println(report.Name, report.H2Fingerprint, report.SettingsCount)
```

### CLI

```
go install github.com/mukuln-official/tls-fetch/cmd/tlsfetch@latest
```

```
# Make a fingerprinted request
tlsfetch curl https://tls.peet.ws/api/all --profile chrome131 -v

# Print H2 fingerprint as JSON
tlsfetch fingerprint --profile firefox128

# Show profile details
tlsfetch validate --profile safari18
```

## Built-in Profiles

| Constant      | Browser        | H2 Fingerprint                                      |
|---------------|----------------|-----------------------------------------------------|
| `Chrome131`   | Chrome 131     | `1:65536;2:0;4:6291456;6:262144\|15663105\|0\|m,a,s,p`  |
| `Firefox128`  | Firefox 128    | `1:65536;4:131072;5:16384\|12517377\|6\|m,p,a,s`        |
| `Safari18`    | Safari 18      | `4:4194304;3:100\|10485760\|0\|m,s,p,a`                 |
| `Edge131`     | Edge 131       | `1:65536;2:0;4:6291456;6:262144\|15663105\|0\|m,a,s,p`  |

## Architecture

tls-fetch is a transport-layer wrapper. `NewTransport` creates an `http.RoundTripper` that dials TLS using [uTLS](https://github.com/refraction-networking/utls) (for ClientHello fingerprinting), then routes the negotiated connection to either a custom HTTP/2 transport that sends fingerprinted SETTINGS and WINDOW_UPDATE frames, or the stdlib HTTP/1.1 path with a header-reordering net.Conn shim. No goroutine pools, no global state, no forked standard library.

## Comparison

| Feature                   | tls-fetch | bogdanfinn/tls-client | azuretls-client | CycleTLS |
|---------------------------|-----------|-----------------------|-----------------|----------|
| Standard net/http Types   | Yes       | No                    | No              | No       |
| JSON Profiles             | Yes       | No                    | No              | No       |
| Fingerprint Validation    | Yes       | No                    | No              | No       |
| HTTP/2 Fingerprint        | Yes       | Yes                   | Yes             | No       |
| Proxy Support             | Yes       | Yes                   | Yes             | Yes      |
| Cookie Jar                | Yes       | Yes                   | Yes             | Yes      |

## Contributing

Contributions welcome. Please open an issue first.

## License

MIT — see [LICENSE](LICENSE).
