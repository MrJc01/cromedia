package mux

import (
	"encoding/binary"
	"fmt"
	"os"

	"cromedia/core"
)

// FLVMuxer multiplexes audio and video packets into an FLV container.
type FLVMuxer struct {
	file   *os.File
	tracks []core.Track
}

// NewFLVMuxer creates a new FLVMuxer.
func NewFLVMuxer(file *os.File) *FLVMuxer {
	return &FLVMuxer{file: file}
}

// WriteHeader writes the FLV signature, header properties, and initial tag size.
func (m *FLVMuxer) WriteHeader(tracks []core.Track) error {
	m.tracks = tracks

	var typeFlags byte = 0
	for _, t := range tracks {
		if t.Type == core.TrackTypeVideo {
			typeFlags |= 0x01
		} else if t.Type == core.TrackTypeAudio {
			typeFlags |= 0x04
		}
	}

	header := []byte{
		'F', 'L', 'V',
		1,         // Version
		typeFlags, // Audio/Video flags
		0, 0, 0, 9, // DataOffset = 9
	}
	if _, err := m.file.Write(header); err != nil {
		return err
	}

	// Write PreviousTagSize0 (always 0)
	var pts0 [4]byte
	if _, err := m.file.Write(pts0[:]); err != nil {
		return err
	}

	return nil
}

// WritePacket writes a media frame inside an FLV tag.
func (m *FLVMuxer) WritePacket(pkt *core.Packet) error {
	if pkt.StreamIndex >= len(m.tracks) {
		return fmt.Errorf("packet StreamIndex out of range")
	}
	track := m.tracks[pkt.StreamIndex]

	var tagType byte = 9 // Video default
	if track.Type == core.TrackTypeAudio {
		tagType = 8
	}

	dataSize := uint32(len(pkt.Data))
	ts := pkt.PTS // expected to be in milliseconds for FLV

	tagHeader := make([]byte, 11)
	tagHeader[0] = tagType
	tagHeader[1] = byte(dataSize >> 16)
	tagHeader[2] = byte(dataSize >> 8)
	tagHeader[3] = byte(dataSize)

	tagHeader[4] = byte(ts >> 16)
	tagHeader[5] = byte(ts >> 8)
	tagHeader[6] = byte(ts)
	tagHeader[7] = byte(ts >> 24) // Timestamp extended

	// Stream ID is always 0
	tagHeader[8] = 0
	tagHeader[9] = 0
	tagHeader[10] = 0

	if _, err := m.file.Write(tagHeader); err != nil {
		return err
	}
	if _, err := m.file.Write(pkt.Data); err != nil {
		return err
	}

	// Write PreviousTagSize
	var pts [4]byte
	binary.BigEndian.PutUint32(pts[:], dataSize+11)
	if _, err := m.file.Write(pts[:]); err != nil {
		return err
	}

	return nil
}

// WriteTrailer is a no-op for FLV.
func (m *FLVMuxer) WriteTrailer() error {
	return nil
}

// Close closes the file.
func (m *FLVMuxer) Close() error {
	return m.file.Close()
}
