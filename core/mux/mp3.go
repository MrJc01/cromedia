package mux

import (
	"os"

	"cromedia/core"
)

// MP3Muxer writes raw MPEG Audio frames.
type MP3Muxer struct {
	file *os.File
}

// NewMP3Muxer creates a new MP3Muxer.
func NewMP3Muxer(file *os.File) *MP3Muxer {
	return &MP3Muxer{file: file}
}

// WriteHeader is a no-op for MP3 raw streams.
func (m *MP3Muxer) WriteHeader(tracks []core.Track) error {
	return nil
}

// WritePacket writes the raw MPEG audio frame.
func (m *MP3Muxer) WritePacket(pkt *core.Packet) error {
	_, err := m.file.Write(pkt.Data)
	return err
}

// WriteTrailer is a no-op.
func (m *MP3Muxer) WriteTrailer() error {
	return nil
}

// Close closes the file.
func (m *MP3Muxer) Close() error {
	return m.file.Close()
}
