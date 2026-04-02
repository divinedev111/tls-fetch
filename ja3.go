package tlsfetch

import (
	"fmt"
	"strconv"
	"strings"

	utls "github.com/refraction-networking/utls"
)

// ProfileFromJA3 parses a JA3 fingerprint string into a BrowserProfile.
//
// Format: TLSVersion,Ciphers,Extensions,EllipticCurves,PointFormats
// Fields are dash-separated decimal values.
func ProfileFromJA3(ja3 string) (BrowserProfile, error) {
	if ja3 == "" {
		return BrowserProfile{}, &ErrInvalidJA3{Input: ja3, Reason: "empty string"}
	}

	parts := strings.Split(ja3, ",")
	if len(parts) != 5 {
		return BrowserProfile{}, &ErrInvalidJA3{
			Input:  ja3,
			Reason: fmt.Sprintf("expected 5 fields, got %d", len(parts)),
		}
	}

	tlsVersion, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{
			Input:  ja3,
			Reason: fmt.Sprintf("invalid TLS version %q: %v", parts[0], err),
		}
	}

	ciphers, err := parseDashUint16(parts[1])
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{Input: ja3, Reason: fmt.Sprintf("ciphers: %v", err)}
	}
	ciphers = filterGREASE(ciphers)

	extIDs, err := parseDashUint16(parts[2])
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{Input: ja3, Reason: fmt.Sprintf("extensions: %v", err)}
	}

	curves, err := parseDashUint16(parts[3])
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{Input: ja3, Reason: fmt.Sprintf("curves: %v", err)}
	}
	curves = filterGREASE(curves)

	pointFormats, err := parseDashUint8(parts[4])
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{Input: ja3, Reason: fmt.Sprintf("point formats: %v", err)}
	}

	namedGroups := make([]utls.CurveID, len(curves))
	for i, c := range curves {
		namedGroups[i] = utls.CurveID(c)
	}

	extensions, err := buildExtensions(extIDs, namedGroups, pointFormats)
	if err != nil {
		return BrowserProfile{}, &ErrInvalidJA3{Input: ja3, Reason: fmt.Sprintf("extensions: %v", err)}
	}

	spec := &utls.ClientHelloSpec{
		TLSVersMin:         utls.VersionTLS10,
		TLSVersMax:         uint16(tlsVersion),
		CipherSuites:       ciphers,
		CompressionMethods: []uint8{0},
		Extensions:         extensions,
	}

	return BrowserProfile{
		Name:            "custom_ja3",
		ClientHelloID:   utls.HelloCustom,
		ClientHelloSpec: spec,
	}, nil
}

func parseDashUint16(s string) ([]uint16, error) {
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

func parseDashUint8(s string) ([]uint8, error) {
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

func isGREASE(v uint16) bool {
	return ((v >> 8) == v&0xff) && v&0xf == 0xa
}

func filterGREASE(in []uint16) []uint16 {
	out := make([]uint16, 0, len(in))
	for _, v := range in {
		if !isGREASE(v) {
			out = append(out, v)
		}
	}
	return out
}

func buildExtensions(ids []uint16, curves []utls.CurveID, pointFormats []uint8) ([]utls.TLSExtension, error) {
	exts := make([]utls.TLSExtension, 0, len(ids))
	for _, id := range ids {
		if isGREASE(id) {
			exts = append(exts, &utls.UtlsGREASEExtension{})
			continue
		}
		switch id {
		case 0:
			exts = append(exts, &utls.SNIExtension{})
		case 5:
			exts = append(exts, &utls.StatusRequestExtension{})
		case 10:
			exts = append(exts, &utls.SupportedCurvesExtension{Curves: curves})
		case 11:
			exts = append(exts, &utls.SupportedPointsExtension{SupportedPoints: pointFormats})
		case 13:
			exts = append(exts, &utls.SignatureAlgorithmsExtension{
				SupportedSignatureAlgorithms: []utls.SignatureScheme{
					utls.ECDSAWithP256AndSHA256, utls.PSSWithSHA256, utls.PKCS1WithSHA256,
					utls.ECDSAWithP384AndSHA384, utls.PSSWithSHA384, utls.PKCS1WithSHA384,
					utls.PSSWithSHA512, utls.PKCS1WithSHA512,
				},
			})
		case 16:
			exts = append(exts, &utls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}})
		case 18:
			exts = append(exts, &utls.SCTExtension{})
		case 23:
			exts = append(exts, &utls.ExtendedMasterSecretExtension{})
		case 27:
			exts = append(exts, &utls.UtlsCompressCertExtension{})
		case 35:
			exts = append(exts, &utls.SessionTicketExtension{})
		case 43:
			exts = append(exts, &utls.SupportedVersionsExtension{
				Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12},
			})
		case 45:
			exts = append(exts, &utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}})
		case 51:
			exts = append(exts, &utls.KeyShareExtension{
				KeyShares: []utls.KeyShare{{Group: utls.CurveX25519}},
			})
		case 65281:
			exts = append(exts, &utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient})
		default:
			exts = append(exts, &utls.GenericExtension{Id: id})
		}
	}
	return exts, nil
}
