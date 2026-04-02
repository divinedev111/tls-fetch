package h2

// Config holds HTTP/2 connection parameters used for fingerprinting.
// Settings, WindowUpdate, and Priorities control the initial frames
// sent on a new HTTP/2 connection. PseudoHeaderOrder controls the
// order of pseudo-headers in HEADERS frames (customisation requires
// a forked HPACK encoder and is not yet wired up).
type Config struct {
	Settings          []Setting
	WindowUpdate      uint32
	Priorities        []Priority
	PseudoHeaderOrder [4]string
}

// Setting is a single HTTP/2 SETTINGS parameter (id + value).
type Setting struct {
	ID    uint16
	Value uint32
}

// Priority describes an HTTP/2 PRIORITY frame.
type Priority struct {
	StreamID  uint32
	Exclusive bool
	DependsOn uint32
	Weight    uint8
}
