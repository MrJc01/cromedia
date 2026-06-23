package mux

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"cromedia/core"
)

// WAVMuxer multiplexes PCM audio into a RIFF/WAV file.
type WAVMuxer struct {
	file          *os.File
	track         core.Track
	totalDataSize int64
}

// NewWAVMuxer creates a new WAVMuxer.
func NewWAVMuxer(file *os.File) *WAVMuxer {
	return &WAVMuxer{file: file}
}

// WriteHeader writes the dummy 44-byte WAV header.
func (m *WAVMuxer) WriteHeader(tracks []core.Track) error {
	if len(tracks) == 0 {
		return fmt.Errorf("no tracks specified for WAV muxer")
	}
	m.track = tracks[0]

	if _, err := m.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Write 44 dummy bytes for header
	dummy := make([]byte, 44)
	if _, err := m.file.Write(dummy); err != nil {
		return err
	}

	m.totalDataSize = 0
	return nil
}

// WritePacket writes raw PCM samples.
func (m *WAVMuxer) WritePacket(pkt *core.Packet) error {
	n, err := m.file.Write(pkt.Data)
	if err != nil {
		return err
	}
	m.totalDataSize += int64(n)
	return nil
}

// WriteTrailer updates file size offsets in the header.
func (m *WAVMuxer) WriteTrailer() error {
	if _, err := m.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+m.totalDataSize))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16) // Subchunk1Size
	binary.LittleEndian.PutUint16(header[20:22], 1)  // AudioFormat (PCM = 1)
	
	// Default channels to stereo if not specified
	channels := uint16(2)
	binary.LittleEndian.PutUint16(header[22:24], channels)
	
	sampleRate := m.track.Timescale
	if sampleRate == 0 {
		sampleRate = 44100
	}
	binary.LittleEndian.PutUint32(header[24:28], sampleRate)

	bitsPerSample := uint16(16)
	blockAlign := channels * (bitsPerSample / 8)
	byteRate := sampleRate * uint32(blockAlign)

	binary.LittleEndian.PutUint32(header[28:32], byteRate)
	binary.LittleEndian.PutUint16(header[32:34], blockAlign)
	binary.LittleEndian.PutUint16(header[34:36], bitsPerSample)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(m.totalDataSize))

	if _, err := m.file.Write(header); err != nil {
		return err
	}

	return nil
}

// Close closes the underlying file.
func (m *WAVMuxer) Close() error {
	return m.file.Close()
}
