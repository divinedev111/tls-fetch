package tlsfetch

import (
	"encoding/json"
	"fmt"
	"os"

	utls "github.com/refraction-networking/utls"
)

// jsonH2Setting is the JSON representation of an H2 setting.
type jsonH2Setting struct {
	ID    uint16 `json:"id"`
	Value uint32 `json:"value"`
}

// jsonProfile is the JSON shape for a browser profile file.
type jsonProfile struct {
	Name string `json:"name"`
	TLS  struct {
		ClientHelloID string `json:"client_hello_id"`
	} `json:"tls"`
	H2 struct {
		Settings          []jsonH2Setting `json:"settings"`
		WindowUpdate      uint32          `json:"window_update"`
		PseudoHeaderOrder []string        `json:"pseudo_header_order"`
	} `json:"h2"`
	Headers struct {
		Order []string `json:"order"`
	} `json:"headers"`
}

// clientHelloIDMap maps the string names used in JSON to utls.ClientHelloID values.
var clientHelloIDMap = map[string]utls.ClientHelloID{
	"HelloChrome_131":  utls.HelloChrome_131,
	"HelloChrome_Auto": utls.HelloChrome_Auto,
	"HelloFirefox_120": utls.HelloFirefox_120,
	"HelloFirefox_Auto": utls.HelloFirefox_Auto,
	"HelloSafari_Auto": utls.HelloSafari_Auto,
	"HelloSafari_16_0": utls.HelloSafari_16_0,
	"HelloEdge_Auto":   utls.HelloEdge_Auto,
	"HelloEdge_85":     utls.HelloEdge_85,
	"HelloEdge_106":    utls.HelloEdge_106,
	"HelloCustom":      utls.HelloCustom,
}

// LoadProfileFromFile reads a JSON file at path and returns a BrowserProfile.
func LoadProfileFromFile(path string) (BrowserProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BrowserProfile{}, fmt.Errorf("tlsfetch: load profile from file: %w", err)
	}
	return LoadProfileFromJSON(data)
}

// LoadProfileFromJSON parses a JSON-encoded profile and returns a BrowserProfile.
func LoadProfileFromJSON(data []byte) (BrowserProfile, error) {
	var jp jsonProfile
	if err := json.Unmarshal(data, &jp); err != nil {
		return BrowserProfile{}, fmt.Errorf("tlsfetch: load profile from JSON: %w", err)
	}

	if jp.Name == "" {
		return BrowserProfile{}, &ErrInvalidProfile{Reason: "name field is required"}
	}

	helloID, ok := clientHelloIDMap[jp.TLS.ClientHelloID]
	if !ok {
		return BrowserProfile{}, &ErrInvalidProfile{
			Name:   jp.Name,
			Reason: fmt.Sprintf("unknown ClientHelloID %q", jp.TLS.ClientHelloID),
		}
	}

	settings := make([]H2Setting, len(jp.H2.Settings))
	for i, s := range jp.H2.Settings {
		settings[i] = H2Setting{ID: s.ID, Value: s.Value}
	}

	var pseudo [4]string
	for i, v := range jp.H2.PseudoHeaderOrder {
		if i >= 4 {
			break
		}
		pseudo[i] = v
	}

	return BrowserProfile{
		Name:              jp.Name,
		ClientHelloID:     helloID,
		H2Settings:        settings,
		H2WindowUpdate:    jp.H2.WindowUpdate,
		PseudoHeaderOrder: pseudo,
		HeaderOrder:       jp.Headers.Order,
	}, nil
}
