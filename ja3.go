package tlsfetch

import (
	"fmt"
	"strconv"
	"strings"

	utls "github.com/refraction-networking/utls"
)

// ProfileFromJA3 parses a JA3 fingerprint string and returns a BrowserProfile
// with a ClientHelloSpec built from the parsed fields.
//
// JA3 format: TLSVersion,Ciphers,Extensions,EllipticCurves,EllipticCurvePointFormats
// Each field uses dash-separated decimal values.
func ProfileFromJA3(ja3 string) (BrowserProfile, error) {
	if ja3 == "" {
		return BrowserProfile{}, &ErrInvalidJA3{Input: ja3, Reason: "empty string"}
	}

	parts := strings.Split(ja3, ",")
	if len(parts) != 5 {
		return BrowserProfile{}, &ErrInvalidJA3{
			Input:  ja3,
			Reason: fmt.Sprintf("expected 5 comma-separated fields, got %d", len(parts)),
		}
	}

	// Field 0: TLS version
	tlsVersion, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{
			Input:  ja3,
			Reason: fmt.Sprintf("invalid TLS version %q: %v", parts[0], err),
		}
	}

	// Field 1: Cipher suites
	ciphers, err := parseDashSeparatedUint16(parts[1])
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{
			Input:  ja3,
			Reason: fmt.Sprintf("invalid cipher suites: %v", err),
		}
	}
	ciphers = filterGREASE16(ciphers)

	// Field 2: Extension IDs
	extIDs, err := parseDashSeparatedUint16(parts[2])
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{
			Input:  ja3,
			Reason: fmt.Sprintf("invalid extension IDs: %v", err),
		}
	}

	// Field 3: Elliptic curves (named groups)
	curves, err := parseDashSeparatedUint16(parts[3])
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{
			Input:  ja3,
			Reason: fmt.Sprintf("invalid elliptic curves: %v", err),
		}
	}
	curves = filterGREASE16(curves)

	// Field 4: EC point formats
	pointFormats, err := parseDashSeparatedUint8(parts[4])
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{
			Input:  ja3,
			Reason: fmt.Sprintf("invalid point formats: %v", err),
		}
	}

	namedGroups := make([]utls.CurveID, len(curves))
	for i, c := range curves {
		namedGroups[i] = utls.CurveID(c)
	}

	extensions, err := buildExtensionsFromIDs(extIDs, namedGroups, pointFormats)
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{
			Input:  ja3,
			Reason: fmt.Sprintf("building extensions: %v", err),
		}
	}

	spec := &utls.ClientHelloSpec{
		TLSVersMin:         utls.VersionTLS10,
		TLSVersMax:         uint16(tlsVersion),
		CipherSuites:       ciphers,
		CompressionMethods: []uint8{0}, // no compression
		Extensions:         extensions,
	}

	return BrowserProfile{
		Name:          "custom_ja3",
		ClientHelloID: utls.HelloCustom,
		ClientHelloSpec: spec,
	}, nil
}

// parseDashSeparatedUint16 splits a dash-delimited string of decimal integers
// into a []uint16. An empty string returns an empty slice.
func parseDashSeparatedUint16(s string) ([]uint16, error) {
	if s == "" {
		return []uint16{}, nil
	}
	parts := strings.Split(s, "-")
	out := make([]uint16, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid uint16 %q: %w", p, err)
		}
		out = append(out, uint16(n))
	}
	return out, nil
}

// parseDashSeparatedUint8 splits a dash-delimited string of decimal integers
// into a []uint8. An empty string returns an empty slice.
func parseDashSeparatedUint8(s string) ([]uint8, error) {
	if s == "" {
		return []uint8{}, nil
	}
	parts := strings.Split(s, "-")
	out := make([]uint8, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid uint8 %q: %w", p, err)
		}
		out = append(out, uint8(n))
	}
	return out, nil
}

// isGREASE16 reports whether v is a GREASE uint16 value.
// GREASE values have identical high and low bytes, each with lowest nibble == 0xa.
func isGREASE16(v uint16) bool {
	return ((v >> 8) == v&0xff) && v&0xf == 0xa
}

// filterGREASE16 removes all GREASE values from a []uint16 slice.
func filterGREASE16(in []uint16) []uint16 {
	out := in[:0:len(in)]
	for _, v := range in {
		if !isGREASE16(v) {
			out = append(out, v)
		}
	}
	return out
}

// buildExtensionsFromIDs maps a list of JA3 extension IDs to the corresponding
// uTLS TLSExtension implementations. Named groups and point formats are wired
// into the appropriate extensions when their IDs appear.
func buildExtensionsFromIDs(
	ids []uint16,
	curves []utls.CurveID,
	pointFormats []uint8,
) ([]utls.TLSExtension, error) {
	exts := make([]utls.TLSExtension, 0, len(ids))
	for _, id := range ids {
		if isGREASE16(id) {
			exts = append(exts, &utls.UtlsGREASEExtension{})
			continue
		}
		switch id {
		case 0: // server_name
			exts = append(exts, &utls.SNIExtension{})
		case 5: // status_request
			exts = append(exts, &utls.StatusRequestExtension{})
		case 10: // supported_groups
			exts = append(exts, &utls.SupportedCurvesExtension{Curves: curves})
		case 11: // ec_point_formats
			exts = append(exts, &utls.SupportedPointsExtension{SupportedPoints: pointFormats})
		case 13: // signature_algorithms
			exts = append(exts, &utls.SignatureAlgorithmsExtension{
				SupportedSignatureAlgorithms: []utls.SignatureScheme{
					utls.ECDSAWithP256AndSHA256,
					utls.PSSWithSHA256,
					utls.PKCS1WithSHA256,
					utls.ECDSAWithP384AndSHA384,
					utls.PSSWithSHA384,
					utls.PKCS1WithSHA384,
					utls.PSSWithSHA512,
					utls.PKCS1WithSHA512,
				},
			})
		case 16: // application_layer_protocol_negotiation
			exts = append(exts, &utls.ALPNExtension{
				AlpnProtocols: []string{"h2", "http/1.1"},
			})
		case 18: // signed_certificate_timestamp
			exts = append(exts, &utls.SCTExtension{})
		case 23: // extended_master_secret
			exts = append(exts, &utls.ExtendedMasterSecretExtension{})
		case 27: // compress_certificate
			exts = append(exts, &utls.UtlsCompressCertExtension{})
		case 35: // session_ticket
			exts = append(exts, &utls.SessionTicketExtension{})
		case 43: // supported_versions
			exts = append(exts, &utls.SupportedVersionsExtension{
				Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12},
			})
		case 45: // psk_key_exchange_modes
			exts = append(exts, &utls.PSKKeyExchangeModesExtension{
				Modes: []uint8{utls.PskModeDHE},
			})
		case 51: // key_share
			exts = append(exts, &utls.KeyShareExtension{
				KeyShares: []utls.KeyShare{
					{Group: utls.CurveX25519},
				},
			})
		case 65281: // renegotiation_info (0xff01)
			exts = append(exts, &utls.RenegotiationInfoExtension{
				Renegotiation: utls.RenegotiateOnceAsClient,
			})
		default:
			exts = append(exts, &utls.GenericExtension{Id: id})
		}
	}
	return exts, nil
}
