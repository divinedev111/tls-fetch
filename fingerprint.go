package tlsfetch

import (
	"fmt"
	"strings"
)

type ValidationReport struct {
	Name          string
	H2Fingerprint string
	PseudoHeaders string
	SettingsCount int
}

// CalculateH2Fingerprint returns an Akamai-style HTTP/2 fingerprint.
// Format: SETTINGS|WINDOW_UPDATE|PRIORITY_COUNT|PSEUDO_HEADER_ORDER
func CalculateH2Fingerprint(p BrowserProfile) string {
	settingsParts := make([]string, len(p.H2Settings))
	for i, s := range p.H2Settings {
		settingsParts[i] = fmt.Sprintf("%d:%d", s.ID, s.Value)
	}
	settings := strings.Join(settingsParts, ";")

	windowUpdate := fmt.Sprintf("%d", p.H2WindowUpdate)

	priorityCount := fmt.Sprintf("%d", len(p.H2Priorities))

	phParts := make([]string, 4)
	for i, ph := range p.PseudoHeaderOrder {
		phParts[i] = pseudoHeaderShorthand(ph)
	}
	pseudoHeaders := strings.Join(phParts, ",")

	return fmt.Sprintf("%s|%s|%s|%s", settings, windowUpdate, priorityCount, pseudoHeaders)
}

func pseudoHeaderShorthand(ph string) string {
	switch ph {
	case ":method":
		return "m"
	case ":authority":
		return "a"
	case ":scheme":
		return "s"
	case ":path":
		return "p"
	default:
		return ph
	}
}

// ValidateProfile computes derived fingerprint fields for a BrowserProfile.
func ValidateProfile(p BrowserProfile) ValidationReport {
	h2fp := CalculateH2Fingerprint(p)
	phParts := make([]string, 4)
	for i, ph := range p.PseudoHeaderOrder {
		phParts[i] = pseudoHeaderShorthand(ph)
	}
	return ValidationReport{
		Name:          p.Name,
		H2Fingerprint: h2fp,
		PseudoHeaders: strings.Join(phParts, ","),
		SettingsCount: len(p.H2Settings),
	}
}
