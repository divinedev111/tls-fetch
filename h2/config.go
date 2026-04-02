package h2

// Config holds HTTP/2 fingerprint parameters for a connection.
type Config struct {
	Settings          []Setting
	WindowUpdate      uint32
	Priorities        []Priority
	PseudoHeaderOrder [4]string
}

// Setting is a single HTTP/2 SETTINGS parameter.
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
