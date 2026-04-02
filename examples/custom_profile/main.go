// Package main demonstrates loading a browser profile from a JSON file
// using WithProfileFromFile.
//
// Run from the repository root so the relative path resolves correctly:
//
//	go run ./examples/custom_profile
package main

import (
	"fmt"
	"io"
	"log"

	tlsfetch "github.com/mukuln-official/tls-fetch"
)

func main() {
	session, err := tlsfetch.NewSession(
		tlsfetch.WithProfileFromFile("profiles/chrome_131.json"),
	)
	if err != nil {
		log.Fatalf("create session: %v", err)
	}
	defer session.CloseIdleConnections()

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
