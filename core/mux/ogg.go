package mux

import (
	"encoding/binary"
	"fmt"
	"os"

	"cromedia/core"
)

// OggMuxer multiplexes packets into Ogg page structures.
type OggMuxer struct {
	file         *os.File
	tracks       []core.Track
	pageSequence map[int]uint32
}

// NewOggMuxer creates a new OggMuxer.
func NewOggMuxer(file *os.File) *OggMuxer {
	return &OggMuxer{
		file:         file,
		pageSequence: make(map[int]uint32),
	}
}

// WriteHeader registers tracks.
func (m *OggMuxer) WriteHeader(tracks []core.Track) error {
	m.tracks = tracks
	for i := range tracks {
		m.pageSequence[i] = 0
	}
	return nil
}

// WritePacket packages data in Ogg pages with segment headers and CRC.
func (m *OggMuxer) WritePacket(pkt *core.Packet) error {
	if pkt.StreamIndex >= len(m.tracks) {
		return fmt.Errorf("packet StreamIndex out of range")
	}

	serial := uint32(12345 + pkt.StreamIndex)
	seq := m.pageSequence[pkt.StreamIndex]
	m.pageSequence[pkt.StreamIndex] = seq + 1

	// Ogg segmentation logic
	dataLen := len(pkt.Data)
	nSegments := dataLen / 255
	if dataLen%255 != 0 || dataLen == 0 {
		nSegments++
	}

	if nSegments > 255 {
		return fmt.Errorf("packet too large for single Ogg page (%d segments)", nSegments)
	}

	segmentTable := make([]byte, nSegments)
	rem := dataLen
	for i := 0; i < nSegments; i++ {
		if rem >= 255 {
			segmentTable[i] = 255
			rem -= 255
		} else {
			segmentTable[i] = byte(rem)
			rem = 0
		}
	}

	// 1. Build Page Header (27 bytes + segment count + segment table)
	headerSize := 27 + nSegments
	page := make([]byte, headerSize+dataLen)

	copy(page[0:4], "OggS")
	page[4] = 0 // Version
	
	// Flags: 0x02 for first page, 0 for middle
	var flags byte = 0
	if seq == 0 {
		flags = 0x02
	}
	page[5] = flags

	binary.LittleEndian.PutUint64(page[6:14], uint64(pkt.PTS))
	binary.LittleEndian.PutUint32(page[14:18], serial)
	binary.LittleEndian.PutUint32(page[18:22], seq)
	// Checksum field [22:26] is left at 0 initially

	page[26] = byte(nSegments)
	copy(page[27:27+nSegments], segmentTable)
	copy(page[headerSize:], pkt.Data)

	// Calculate and write Ogg CRC-32
	crc := computeOggCRC(page)
	binary.LittleEndian.PutUint32(page[22:26], crc)

	if _, err := m.file.Write(page); err != nil {
		return err
	}

	return nil
}

// WriteTrailer is a no-op.
func (m *OggMuxer) WriteTrailer() error {
	return nil
}

// Close closes the file.
func (m *OggMuxer) Close() error {
	return m.file.Close()
}

var oggCRCTable [256]uint32

func init() {
	for i := 0; i < 256; i++ {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if (r & 0x80000000) != 0 {
				r = (r << 1) ^ 0x04C11DB7
			} else {
				r <<= 1
			}
		}
		oggCRCTable[i] = r
	}
}

func computeOggCRC(data []byte) uint32 {
	var crc uint32 = 0
	for _, b := range data {
		crc = (crc << 8) ^ oggCRCTable[byte(crc>>24)^b]
	}
	return crc
}
