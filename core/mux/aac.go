package mux

import (
	"os"

	"cromedia/core"
)

// AACMuxer writes raw ADTS AAC frames.
type AACMuxer struct {
	file *os.File
}

// NewAACMuxer creates a new AACMuxer.
func NewAACMuxer(file *os.File) *AACMuxer {
	return &AACMuxer{file: file}
}

// WriteHeader is a no-op for raw ADTS.
func (m *AACMuxer) WriteHeader(tracks []core.Track) error {
	return nil
}

// WritePacket writes the raw ADTS AAC frame.
func (m *AACMuxer) WritePacket(pkt *core.Packet) error {
	_, err := m.file.Write(pkt.Data)
	return err
}

// WriteTrailer is a no-op.
func (m *AACMuxer) WriteTrailer() error {
	return nil
}

// Close closes the file.
func (m *AACMuxer) Close() error {
	return m.file.Close()
}
