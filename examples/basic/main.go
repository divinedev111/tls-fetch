// Package main demonstrates using tlsfetch.NewTransport as an http.RoundTripper
// with a standard net/http Client.
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	tlsfetch "github.com/mukuln-official/tls-fetch"
)

func main() {
	tr, err := tlsfetch.NewTransport(tlsfetch.WithProfile(tlsfetch.Chrome131))
	if err != nil {
		log.Fatalf("create transport: %v", err)
	}

	client := &http.Client{Transport: tr}

	resp, err := client.Get("https://tls.peet.ws/api/all")
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
