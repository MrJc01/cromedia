package mux

import (
	"os"

	"cromedia/core"
)

// AnnexBMuxer writes raw Annex B streams (H.264/H.265) with start codes.
type AnnexBMuxer struct {
	file *os.File
}

// NewAnnexBMuxer creates a new AnnexBMuxer.
func NewAnnexBMuxer(file *os.File) *AnnexBMuxer {
	return &AnnexBMuxer{file: file}
}

func (m *AnnexBMuxer) WriteHeader(tracks []core.Track) error {
	// If any track has CodecPrivate (e.g. SPS/PPS), write it first as Annex B
	for _, track := range tracks {
		if len(track.CodecPrivate) > 0 {
			startCode := []byte{0, 0, 0, 1}
			if _, err := m.file.Write(startCode); err != nil {
				return err
			}
			if _, err := m.file.Write(track.CodecPrivate); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *AnnexBMuxer) WritePacket(pkt *core.Packet) error {
	// Add 4-byte start code if packet doesn't already start with one
	hasStartCode := len(pkt.Data) >= 3 && pkt.Data[0] == 0 && pkt.Data[1] == 0 && (pkt.Data[2] == 1 || (pkt.Data[2] == 0 && pkt.Data[3] == 1))
	if !hasStartCode {
		startCode := []byte{0, 0, 0, 1}
		if _, err := m.file.Write(startCode); err != nil {
			return err
		}
	}
	_, err := m.file.Write(pkt.Data)
	return err
}

func (m *AnnexBMuxer) WriteTrailer() error {
	return nil
}

func (m *AnnexBMuxer) Close() error {
	return m.file.Close()
}
