package h2

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestWriteHTTP2Preface(t *testing.T) {
	var buf bytes.Buffer
	if err := writeHTTP2Preface(&buf); err != nil {
		t.Fatalf("writeHTTP2Preface: %v", err)
	}
	want := "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	if got := buf.String(); got != want {
		t.Errorf("preface:\n got  %q\n want %q", got, want)
	}
}

func TestWriteSettingsFrame(t *testing.T) {
	// Chrome 131 settings: 4 settings, each 6 bytes = 24 byte payload.
	settings := []Setting{
		{ID: 1, Value: 65536},
		{ID: 2, Value: 0},
		{ID: 4, Value: 6291456},
		{ID: 6, Value: 262144},
	}

	var buf bytes.Buffer
	if err := writeSettingsFrame(&buf, settings); err != nil {
		t.Fatalf("writeSettingsFrame: %v", err)
	}

	data := buf.Bytes()

	// Total frame: 9-byte header + 24-byte payload = 33 bytes.
	if len(data) != 33 {
		t.Fatalf("frame length: got %d, want 33", len(data))
	}

	// Frame header: 3-byte length (big-endian) = 24.
	payloadLen := int(data[0])<<16 | int(data[1])<<8 | int(data[2])
	if payloadLen != 24 {
		t.Errorf("payload length field: got %d, want 24", payloadLen)
	}

	// Frame type: 0x04 (SETTINGS).
	if data[3] != 0x04 {
		t.Errorf("frame type: got 0x%02x, want 0x04", data[3])
	}

	// Flags: 0x00 (not an ACK).
	if data[4] != 0x00 {
		t.Errorf("flags: got 0x%02x, want 0x00", data[4])
	}

	// Stream ID: 0.
	streamID := binary.BigEndian.Uint32(data[5:9]) & 0x7FFFFFFF
	if streamID != 0 {
		t.Errorf("stream ID: got %d, want 0", streamID)
	}

	// First setting: ID=1 (HEADER_TABLE_SIZE), Value=65536.
	firstID := binary.BigEndian.Uint16(data[9:11])
	firstVal := binary.BigEndian.Uint32(data[11:15])
	if firstID != 1 {
		t.Errorf("first setting ID: got %d, want 1", firstID)
	}
	if firstVal != 65536 {
		t.Errorf("first setting value: got %d, want 65536", firstVal)
	}
}

func TestWriteWindowUpdateFrame(t *testing.T) {
	var buf bytes.Buffer
	if err := writeWindowUpdateFrame(&buf, 0, 15663105); err != nil {
		t.Fatalf("writeWindowUpdateFrame: %v", err)
	}

	data := buf.Bytes()

	// 9-byte header + 4-byte payload = 13.
	if len(data) != 13 {
		t.Fatalf("frame length: got %d, want 13", len(data))
	}

	// Payload length = 4.
	payloadLen := int(data[0])<<16 | int(data[1])<<8 | int(data[2])
	if payloadLen != 4 {
		t.Errorf("payload length field: got %d, want 4", payloadLen)
	}

	// Frame type: 0x08 (WINDOW_UPDATE).
	if data[3] != 0x08 {
		t.Errorf("frame type: got 0x%02x, want 0x08", data[3])
	}

	// Stream ID in header: 0.
	streamID := binary.BigEndian.Uint32(data[5:9]) & 0x7FFFFFFF
	if streamID != 0 {
		t.Errorf("stream ID: got %d, want 0", streamID)
	}

	// Payload: increment value.
	increment := binary.BigEndian.Uint32(data[9:13]) & 0x7FFFFFFF
	if increment != 15663105 {
		t.Errorf("increment: got %d, want 15663105", increment)
	}
}

func TestWritePriorityFrame(t *testing.T) {
	p := Priority{
		StreamID:  3,
		Exclusive: false,
		DependsOn: 0,
		Weight:    200,
	}

	var buf bytes.Buffer
	if err := writePriorityFrame(&buf, p); err != nil {
		t.Fatalf("writePriorityFrame: %v", err)
	}

	data := buf.Bytes()

	// 9-byte header + 5-byte payload = 14.
	if len(data) != 14 {
		t.Fatalf("frame length: got %d, want 14", len(data))
	}

	// Payload length = 5.
	payloadLen := int(data[0])<<16 | int(data[1])<<8 | int(data[2])
	if payloadLen != 5 {
		t.Errorf("payload length field: got %d, want 5", payloadLen)
	}

	// Frame type: 0x02 (PRIORITY).
	if data[3] != 0x02 {
		t.Errorf("frame type: got 0x%02x, want 0x02", data[3])
	}

	// Stream ID in frame header = 3.
	streamID := binary.BigEndian.Uint32(data[5:9]) & 0x7FFFFFFF
	if streamID != 3 {
		t.Errorf("stream ID: got %d, want 3", streamID)
	}

	// Payload: 4-byte dependency (exclusive bit = 0) + 1-byte weight.
	dep := binary.BigEndian.Uint32(data[9:13])
	exclusiveBit := dep >> 31
	depStream := dep & 0x7FFFFFFF
	if exclusiveBit != 0 {
		t.Errorf("exclusive bit: got %d, want 0", exclusiveBit)
	}
	if depStream != 0 {
		t.Errorf("depends-on: got %d, want 0", depStream)
	}
	if data[13] != 200 {
		t.Errorf("weight: got %d, want 200", data[13])
	}
}

func TestWritePriorityFrame_Exclusive(t *testing.T) {
	p := Priority{
		StreamID:  9,
		Exclusive: true,
		DependsOn: 7,
		Weight:    0,
	}

	var buf bytes.Buffer
	if err := writePriorityFrame(&buf, p); err != nil {
		t.Fatalf("writePriorityFrame: %v", err)
	}

	data := buf.Bytes()

	// Payload dependency field should have exclusive bit set.
	dep := binary.BigEndian.Uint32(data[9:13])
	exclusiveBit := dep >> 31
	depStream := dep & 0x7FFFFFFF
	if exclusiveBit != 1 {
		t.Errorf("exclusive bit: got %d, want 1", exclusiveBit)
	}
	if depStream != 7 {
		t.Errorf("depends-on: got %d, want 7", depStream)
	}
	if data[13] != 0 {
		t.Errorf("weight: got %d, want 0", data[13])
	}
}

func TestWriteInitialFrames(t *testing.T) {
	cfg := Config{
		Settings: []Setting{
			{ID: 1, Value: 65536},
			{ID: 4, Value: 6291456},
		},
		WindowUpdate: 15663105,
		Priorities: []Priority{
			{StreamID: 3, Exclusive: false, DependsOn: 0, Weight: 200},
		},
	}

	var buf bytes.Buffer
	if err := WriteInitialFrames(&buf, cfg); err != nil {
		t.Fatalf("WriteInitialFrames: %v", err)
	}

	data := buf.Bytes()

	// Should start with the preface.
	preface := "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	if len(data) < len(preface) {
		t.Fatalf("data too short: %d bytes", len(data))
	}
	if string(data[:len(preface)]) != preface {
		t.Errorf("preface mismatch")
	}

	// After preface: SETTINGS frame (9 + 12) + WINDOW_UPDATE (13) + PRIORITY (14).
	rest := data[len(preface):]
	expectedLen := (9 + 12) + 13 + 14
	if len(rest) != expectedLen {
		t.Errorf("frames length: got %d, want %d", len(rest), expectedLen)
	}

	// First frame type should be SETTINGS (0x04).
	if rest[3] != 0x04 {
		t.Errorf("first frame type: got 0x%02x, want 0x04", rest[3])
	}
}

func TestWriteInitialFrames_NoPriorities(t *testing.T) {
	cfg := Config{
		Settings: []Setting{
			{ID: 1, Value: 65536},
		},
		WindowUpdate: 10000,
	}

	var buf bytes.Buffer
	if err := WriteInitialFrames(&buf, cfg); err != nil {
		t.Fatalf("WriteInitialFrames: %v", err)
	}

	data := buf.Bytes()
	preface := "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	rest := data[len(preface):]

	// SETTINGS (9 + 6) + WINDOW_UPDATE (13), no PRIORITY frames.
	expectedLen := (9 + 6) + 13
	if len(rest) != expectedLen {
		t.Errorf("frames length: got %d, want %d", len(rest), expectedLen)
	}
}

func TestWriteInitialFrames_NoWindowUpdate(t *testing.T) {
	cfg := Config{
		Settings: []Setting{
			{ID: 1, Value: 65536},
		},
		WindowUpdate: 0,
	}

	var buf bytes.Buffer
	if err := WriteInitialFrames(&buf, cfg); err != nil {
		t.Fatalf("WriteInitialFrames: %v", err)
	}

	data := buf.Bytes()
	preface := "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	rest := data[len(preface):]

	// SETTINGS only (9 + 6), no WINDOW_UPDATE, no PRIORITYs.
	expectedLen := 9 + 6
	if len(rest) != expectedLen {
		t.Errorf("frames length: got %d, want %d", len(rest), expectedLen)
	}
}
