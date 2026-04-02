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

var Chrome131 = BrowserProfile{
	Name:          "chrome_131",
	ClientHelloID: utls.HelloChrome_131,
	H2Settings: []H2Setting{
		{ID: 1, Value: 65536},
		{ID: 2, Value: 0},
		{ID: 4, Value: 6291456},
		{ID: 6, Value: 262144},
	},
	H2WindowUpdate: 15663105,
	PseudoHeaderOrder: [4]string{
		":method", ":authority", ":scheme", ":path",
	},
	HeaderOrder: []string{
		"Host", "Connection", "Content-Length", "Cache-Control",
		"sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
		"Upgrade-Insecure-Requests", "User-Agent", "Accept",
		"Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-User",
		"Sec-Fetch-Dest", "Accept-Encoding", "Accept-Language", "Cookie",
	},
}

var Firefox128 = BrowserProfile{
	Name:          "firefox_128",
	ClientHelloID: utls.HelloFirefox_120,
	H2Settings: []H2Setting{
		{ID: 1, Value: 65536},
		{ID: 4, Value: 131072},
		{ID: 5, Value: 16384},
	},
	H2WindowUpdate: 12517377,
	H2Priorities: []H2Priority{
		{StreamID: 3, Exclusive: false, DependsOn: 0, Weight: 200},
		{StreamID: 5, Exclusive: false, DependsOn: 0, Weight: 100},
		{StreamID: 7, Exclusive: false, DependsOn: 0, Weight: 0},
		{StreamID: 9, Exclusive: false, DependsOn: 7, Weight: 0},
		{StreamID: 11, Exclusive: false, DependsOn: 3, Weight: 0},
		{StreamID: 13, Exclusive: false, DependsOn: 0, Weight: 240},
	},
	PseudoHeaderOrder: [4]string{
		":method", ":path", ":authority", ":scheme",
	},
	HeaderOrder: []string{
		"Host", "User-Agent", "Accept", "Accept-Language",
		"Accept-Encoding", "Content-Type", "Content-Length",
		"Connection", "Cookie", "Upgrade-Insecure-Requests",
		"Sec-Fetch-Dest", "Sec-Fetch-Mode", "Sec-Fetch-Site",
		"Sec-Fetch-User", "TE",
	},
}

var Safari18 = BrowserProfile{
	Name:          "safari_18",
	ClientHelloID: utls.HelloSafari_Auto,
	H2Settings: []H2Setting{
		{ID: 4, Value: 4194304},
		{ID: 3, Value: 100},
	},
	H2WindowUpdate: 10485760,
	PseudoHeaderOrder: [4]string{
		":method", ":scheme", ":path", ":authority",
	},
	HeaderOrder: []string{
		"Host", "Accept", "Accept-Language",
		"Connection", "Accept-Encoding", "User-Agent",
	},
}

var Edge131 = BrowserProfile{
	Name:          "edge_131",
	ClientHelloID: utls.HelloEdge_Auto,
	H2Settings: []H2Setting{
		{ID: 1, Value: 65536},
		{ID: 2, Value: 0},
		{ID: 4, Value: 6291456},
		{ID: 6, Value: 262144},
	},
	H2WindowUpdate: 15663105,
	PseudoHeaderOrder: [4]string{
		":method", ":authority", ":scheme", ":path",
	},
	HeaderOrder: []string{
		"Host", "Connection", "Content-Length", "Cache-Control",
		"sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
		"Upgrade-Insecure-Requests", "User-Agent", "Accept",
		"Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-User",
		"Sec-Fetch-Dest", "Accept-Encoding", "Accept-Language", "Cookie",
	},
}
