package tlsfetch

import utls "github.com/refraction-networking/utls"

type H2Setting struct {
	ID    uint16
	Value uint32
}

type H2Priority struct {
	StreamID  uint32
	Exclusive bool
	DependsOn uint32
	Weight    uint8
}

type BrowserProfile struct {
	Name              string
	ClientHelloID     utls.ClientHelloID
	ClientHelloSpec   *utls.ClientHelloSpec
	H2Settings        []H2Setting
	H2WindowUpdate    uint32
	H2Priorities      []H2Priority
	PseudoHeaderOrder [4]string
	HeaderOrder       []string
}
