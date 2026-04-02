// Package main demonstrates using tlsfetch.NewSession with ordered headers.
package main

import (
	"fmt"
	"io"
	"log"

	tlsfetch "github.com/mukuln-official/tls-fetch"
	"github.com/mukuln-official/tls-fetch/header"
)

func main() {
	session, err := tlsfetch.NewSession(tlsfetch.WithProfile(tlsfetch.Chrome131))
	if err != nil {
		log.Fatalf("create session: %v", err)
	}
	defer session.CloseIdleConnections()

	session.SetOrderedHeaders(header.Ordered{
		{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"},
		{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
		{"Accept-Language", "en-US,en;q=0.9"},
		{"Accept-Encoding", "gzip, deflate, br"},
	})

	resp, err := session.Get("https://tls.peet.ws/api/all")
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
