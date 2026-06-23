package mux

import (
	"fmt"
	"os"

	"cromedia/core"
)

// FLACMuxer multiplexes FLAC audio frames.
type FLACMuxer struct {
	file *os.File
}

// NewFLACMuxer creates a new FLACMuxer.
func NewFLACMuxer(file *os.File) *FLACMuxer {
	return &FLACMuxer{file: file}
}

// WriteHeader writes "fLaC" signature and STREAMINFO block.
func (m *FLACMuxer) WriteHeader(tracks []core.Track) error {
	if len(tracks) == 0 {
		return fmt.Errorf("no tracks specified for FLAC muxer")
	}
	tr := tracks[0]

	// 1. Write fLaC signature
	if _, err := m.file.Write([]byte("fLaC")); err != nil {
		return err
	}

	// 2. Build and write STREAMINFO block (34 bytes payload + 4 bytes header)
	header := []byte{
		0x80,       // Last metadata block, block type = 0 (STREAMINFO)
		0, 0, 34,   // length = 34
	}
	if _, err := m.file.Write(header); err != nil {
		return err
	}

	payload := make([]byte, 34)
	// Min/Max Block Size: 4096
	payload[0] = 0x10
	payload[1] = 0x00
	payload[2] = 0x10
	payload[3] = 0x00

	sampleRate := tr.Timescale
	if sampleRate == 0 {
		sampleRate = 44100
	}
	channels := uint32(2)
	bitsPerSample := uint32(16)

	// Packing:
	// Sample rate: 20 bits
	// Channels: 3 bits (0 = 1 channel, 1 = 2 channels)
	// Bits per sample: 5 bits (0 = 1 bit? 15 = 16 bits)
	payload[10] = byte(sampleRate >> 12)
	payload[11] = byte((sampleRate >> 4) & 0xFF)
	payload[12] = byte(((sampleRate & 0x0F) << 4) | (((channels - 1) & 0x07) << 1) | (((bitsPerSample - 1) >> 4) & 0x01))
	payload[13] = byte(((bitsPerSample - 1) & 0x0F) << 4)

	if _, err := m.file.Write(payload); err != nil {
		return err
	}

	return nil
}

// WritePacket writes the raw FLAC audio frame.
func (m *FLACMuxer) WritePacket(pkt *core.Packet) error {
	_, err := m.file.Write(pkt.Data)
	return err
}

// WriteTrailer is a no-op.
func (m *FLACMuxer) WriteTrailer() error {
	return nil
}

// Close closes the file.
func (m *FLACMuxer) Close() error {
	return m.file.Close()
}
