// Command tlsfetch is a CLI tool for TLS fingerprint testing and validation.
//
// Usage:
//
//	tlsfetch curl <url> [--profile chrome131] [--proxy socks5://...] [-v]
//	tlsfetch fingerprint --profile chrome131
//	tlsfetch validate --profile chrome131
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	tlsfetch "github.com/mukuln-official/tls-fetch"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "curl":
		runCurl(os.Args[2:])
	case "fingerprint":
		runFingerprint(os.Args[2:])
	case "validate":
		runValidate(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  tlsfetch curl <url> [--profile chrome131] [--proxy socks5://...] [-v]")
	fmt.Fprintln(os.Stderr, "  tlsfetch fingerprint --profile chrome131")
	fmt.Fprintln(os.Stderr, "  tlsfetch validate --profile chrome131")
}

// profileFromName maps a lowercase profile name string to its BrowserProfile.
func profileFromName(name string) (tlsfetch.BrowserProfile, error) {
	switch strings.ToLower(name) {
	case "chrome131":
		return tlsfetch.Chrome131, nil
	case "firefox128":
		return tlsfetch.Firefox128, nil
	case "safari18":
		return tlsfetch.Safari18, nil
	case "edge131":
		return tlsfetch.Edge131, nil
	default:
		return tlsfetch.BrowserProfile{}, fmt.Errorf("unknown profile %q; valid: chrome131, firefox128, safari18, edge131", name)
	}
}

// runCurl handles the `tlsfetch curl` subcommand.
func runCurl(args []string) {
	fs := flag.NewFlagSet("curl", flag.ExitOnError)
	profileName := fs.String("profile", "chrome131", "browser profile to use (chrome131|firefox128|safari18|edge131)")
	proxyURL := fs.String("proxy", "", "proxy URL (e.g. socks5://127.0.0.1:1080)")
	verbose := fs.Bool("v", false, "verbose output: show fingerprint info and status")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: tlsfetch curl <url> [--profile chrome131] [--proxy socks5://...] [-v]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "curl: missing <url>")
		fs.Usage()
		os.Exit(1)
	}

	targetURL := fs.Arg(0)

	profile, err := profileFromName(*profileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "curl: %v\n", err)
		os.Exit(1)
	}

	opts := []tlsfetch.Option{tlsfetch.WithProfile(profile)}
	if *proxyURL != "" {
		opts = append(opts, tlsfetch.WithProxy(*proxyURL))
	}

	if *verbose {
		fp := tlsfetch.CalculateH2Fingerprint(profile)
		fmt.Fprintf(os.Stderr, "Profile:        %s\n", profile.Name)
		fmt.Fprintf(os.Stderr, "H2 Fingerprint: %s\n", fp)
		fmt.Fprintln(os.Stderr)
	}

	session, err := tlsfetch.NewSession(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "curl: create session: %v\n", err)
		os.Exit(1)
	}

	resp, err := session.Get(targetURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "curl: request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if *verbose {
		fmt.Fprintf(os.Stderr, "Status: %s\n\n", resp.Status)
	}

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		fmt.Fprintf(os.Stderr, "curl: read response: %v\n", err)
		os.Exit(1)
	}
}

// runFingerprint handles the `tlsfetch fingerprint` subcommand.
func runFingerprint(args []string) {
	fs := flag.NewFlagSet("fingerprint", flag.ExitOnError)
	profileName := fs.String("profile", "chrome131", "browser profile (chrome131|firefox128|safari18|edge131)")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: tlsfetch fingerprint --profile chrome131")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	profile, err := profileFromName(*profileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fingerprint: %v\n", err)
		os.Exit(1)
	}

	fp := tlsfetch.CalculateH2Fingerprint(profile)
	out := struct {
		Profile       string `json:"profile"`
		H2Fingerprint string `json:"h2_fingerprint"`
	}{
		Profile:       profile.Name,
		H2Fingerprint: fp,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "fingerprint: encode JSON: %v\n", err)
		os.Exit(1)
	}
}

// runValidate handles the `tlsfetch validate` subcommand.
func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	profileName := fs.String("profile", "chrome131", "browser profile (chrome131|firefox128|safari18|edge131)")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: tlsfetch validate --profile chrome131")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	profile, err := profileFromName(*profileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate: %v\n", err)
		os.Exit(1)
	}

	report := tlsfetch.ValidateProfile(profile)
	fmt.Printf("Name:           %s\n", report.Name)
	fmt.Printf("H2 Fingerprint: %s\n", report.H2Fingerprint)
	fmt.Printf("Pseudo-Headers: %s\n", report.PseudoHeaders)
	fmt.Printf("Settings Count: %d\n", report.SettingsCount)
}
