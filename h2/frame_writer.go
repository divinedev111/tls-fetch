package h2

import (
	"encoding/binary"
	"io"
)

const (
	framePriority     = 0x02
	frameSettings     = 0x04
	frameWindowUpdate = 0x08
)

func writeHTTP2Preface(w io.Writer) error {
	_, err := io.WriteString(w, "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
	return err
}

func writeFrameHeader(w io.Writer, length uint32, frameType uint8, flags uint8, streamID uint32) error {
	var hdr [9]byte
	hdr[0] = byte(length >> 16)
	hdr[1] = byte(length >> 8)
	hdr[2] = byte(length)
	hdr[3] = frameType
	hdr[4] = flags
	binary.BigEndian.PutUint32(hdr[5:9], streamID&0x7FFFFFFF)
	_, err := w.Write(hdr[:])
	return err
}

func writeSettingsFrame(w io.Writer, settings []Setting) error {
	payloadLen := uint32(len(settings) * 6)
	if err := writeFrameHeader(w, payloadLen, frameSettings, 0x00, 0); err != nil {
		return err
	}
	for _, s := range settings {
		var entry [6]byte
		binary.BigEndian.PutUint16(entry[0:2], s.ID)
		binary.BigEndian.PutUint32(entry[2:6], s.Value)
		if _, err := w.Write(entry[:]); err != nil {
			return err
		}
	}
	return nil
}

func writeWindowUpdateFrame(w io.Writer, streamID uint32, increment uint32) error {
	if err := writeFrameHeader(w, 4, frameWindowUpdate, 0x00, streamID); err != nil {
		return err
	}
	var payload [4]byte
	binary.BigEndian.PutUint32(payload[:], increment&0x7FFFFFFF)
	_, err := w.Write(payload[:])
	return err
}

func writePriorityFrame(w io.Writer, p Priority) error {
	if err := writeFrameHeader(w, 5, framePriority, 0x00, p.StreamID); err != nil {
		return err
	}
	var payload [5]byte
	dep := p.DependsOn & 0x7FFFFFFF
	if p.Exclusive {
		dep |= 0x80000000
	}
	binary.BigEndian.PutUint32(payload[0:4], dep)
	payload[4] = p.Weight
	_, err := w.Write(payload[:])
	return err
}

// WriteInitialFrames writes the HTTP/2 connection preface followed by
// SETTINGS, WINDOW_UPDATE, and PRIORITY frames.
func WriteInitialFrames(w io.Writer, cfg Config) error {
	if err := writeHTTP2Preface(w); err != nil {
		return err
	}
	if err := writeSettingsFrame(w, cfg.Settings); err != nil {
		return err
	}
	if cfg.WindowUpdate > 0 {
		if err := writeWindowUpdateFrame(w, 0, cfg.WindowUpdate); err != nil {
			return err
		}
	}
	for _, p := range cfg.Priorities {
		if err := writePriorityFrame(w, p); err != nil {
			return err
		}
	}
	return nil
}
