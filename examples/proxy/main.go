// Package main demonstrates routing traffic through a SOCKS5 proxy
// using WithProxy.
//
// Requires a running SOCKS5 proxy at 127.0.0.1:1080.
// If no proxy is available the request will fail; that is expected.
package main

import (
	"fmt"
	"io"
	"log"

	tlsfetch "github.com/mukuln-official/tls-fetch"
)

func main() {
	session, err := tlsfetch.NewSession(
		tlsfetch.WithProfile(tlsfetch.Chrome131),
		tlsfetch.WithProxy("socks5://127.0.0.1:1080"),
	)
	if err != nil {
		log.Fatalf("create session: %v", err)
	}
	defer session.CloseIdleConnections()

	resp, err := session.Get("https://httpbin.org/ip")
	if err != nil {
		log.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("read body: %v", err)
	}

	fmt.Printf("Status: %s\n\n%s\n", resp.Status, body)
}
