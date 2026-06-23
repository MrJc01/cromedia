package mux

import (
	"fmt"
	"os"

	"cromedia/core"
)

// TSMuxer handles writing MPEG-TS (.ts) container files.
type TSMuxer struct {
	file         *os.File
	tracks       []core.Track
	cc           map[uint16]byte // continuity counters per PID
	pcrVal       uint64
}

// NewTSMuxer creates a new TSMuxer.
func NewTSMuxer(file *os.File) *TSMuxer {
	return &TSMuxer{
		file:   file,
		cc:     make(map[uint16]byte),
		pcrVal: 0,
	}
}

func (m *TSMuxer) WriteHeader(tracks []core.Track) error {
	m.tracks = tracks

	// Write PAT
	if err := m.writePAT(); err != nil {
		return err
	}

	// Write PMT
	if err := m.writePMT(); err != nil {
		return err
	}

	return nil
}

func (m *TSMuxer) WritePacket(pkt *core.Packet) error {
	if pkt.StreamIndex >= len(m.tracks) {
		return fmt.Errorf("packet StreamIndex out of range")
	}
	track := m.tracks[pkt.StreamIndex]

	pid := uint16(0x100 + pkt.StreamIndex)

	// Build PES packet header
	pesHeader := make([]byte, 14)
	pesHeader[0] = 0x00
	pesHeader[1] = 0x00
	pesHeader[2] = 0x01
	
	// Stream ID
	if track.Type == core.TrackTypeVideo {
		pesHeader[3] = 0xE0 // Video stream 0
	} else {
		pesHeader[3] = 0xC0 // Audio stream 0
	}

	// Packet length (0 for variable video size in TS, but audio should be filled)
	pesLen := 0
	if track.Type == core.TrackTypeAudio {
		pesLen = len(pkt.Data) + 8
	}
	pesHeader[4] = byte(pesLen >> 8)
	pesHeader[5] = byte(pesLen)

	pesHeader[6] = 0x80 // Standard flags
	pesHeader[7] = 0x80 // PTS present, DTS not present (simplified)
	pesHeader[8] = 0x05 // Header data length (5 bytes for PTS)

	// Encode PTS (90kHz clock)
	pts90 := uint64(pkt.PTS)
	pesHeader[9] = byte(((pts90 >> 29) & 0x07) << 1) | 0x21
	pesHeader[10] = byte(pts90 >> 22)
	pesHeader[11] = byte(((pts90 >> 14) & 0xFF) << 1) | 0x01
	pesHeader[12] = byte(pts90 >> 7)
	pesHeader[13] = byte((pts90 & 0x7F) << 1) | 0x01

	pesPayload := append(pesHeader, pkt.Data...)

	// Split pesPayload into TS packets of 188 bytes
	offset := 0
	isStart := true

	for offset < len(pesPayload) {
		tsPack := make([]byte, 188)
		tsPack[0] = 0x47 // Sync byte
		
		// Payload unit start indicator + PID
		flagsAndPID := pid
		if isStart {
			flagsAndPID |= 0x4000
			isStart = false
		}
		tsPack[1] = byte(flagsAndPID >> 8)
		tsPack[2] = byte(flagsAndPID)

		// Continuity counter
		cc := m.cc[pid]
		tsPack[3] = 0x10 | (cc & 0x0F) // payload only, no adaptation field by default
		m.cc[pid] = (cc + 1) & 0x0F

		payloadSpace := 184
		remBytes := len(pesPayload) - offset

		if remBytes < payloadSpace {
			// Needs padding (adaptation field)
			tsPack[3] = 0x30 | (tsPack[3] & 0x0F) // adaptation + payload
			padLen := payloadSpace - remBytes
			
			// Fill adaptation field
			tsPack[4] = byte(padLen - 1)
			if padLen > 1 {
				tsPack[5] = 0x00 // flags
				for i := 6; i < 5+padLen; i++ {
					tsPack[i] = 0xFF
				}
			}
			copy(tsPack[4+padLen:], pesPayload[offset:])
			offset += remBytes
		} else {
			copy(tsPack[4:], pesPayload[offset:offset+payloadSpace])
			offset += payloadSpace
		}

		if _, err := m.file.Write(tsPack); err != nil {
			return err
		}
	}

	return nil
}

func (m *TSMuxer) WriteTrailer() error {
	return nil
}

func (m *TSMuxer) Close() error {
	return m.file.Close()
}

func (m *TSMuxer) writePAT() error {
	pat := make([]byte, 188)
	pat[0] = 0x47
	pat[1] = 0x40 // payload unit start, PID = 0
	pat[2] = 0x00
	pat[3] = 0x10 // payload only, CC = 0

	pat[4] = 0x00 // Pointer field
	pat[5] = 0x00 // Table ID
	pat[6] = 0xB0 // Section syntax indicator
	pat[7] = 0x0D // Section length
	pat[8] = 0x00 // Transport stream ID
	pat[9] = 0x01
	pat[10] = 0xC1 // Version, current/next
	pat[11] = 0x00 // Section number
	pat[12] = 0x00 // Last section number

	// Program 1: Program map PID = 0x1000
	pat[13] = 0x00
	pat[14] = 0x01 // Program number = 1
	pat[15] = 0xE0 // Reserved + Program map PID
	pat[16] = 0x10 // PID = 0x1000
	pat[17] = 0x00

	// CRC32 of PAT payload (simplified dummy check)
	pat[18] = 0x2A
	pat[19] = 0xB1
	pat[20] = 0x04
	pat[21] = 0xB2

	// Padding
	for i := 22; i < 188; i++ {
		pat[i] = 0xFF
	}

	_, err := m.file.Write(pat)
	return err
}

func (m *TSMuxer) writePMT() error {
	pmt := make([]byte, 188)
	pmt[0] = 0x47
	pmt[1] = 0x50 // unit start, PID = 0x1000
	pmt[2] = 0x00
	pmt[3] = 0x10 // CC = 0

	pmt[4] = 0x00 // Pointer field
	pmt[5] = 0x02 // Table ID
	pmt[6] = 0xB0 // Section syntax indicator
	
	// Section length: 12 + 5 * numStreams
	secLen := 12 + 5*len(m.tracks)
	pmt[7] = byte(secLen)
	
	pmt[8] = 0x00 // Program number
	pmt[9] = 0x01
	pmt[10] = 0xC1 // Version
	pmt[11] = 0x00 // Section
	pmt[12] = 0x00 // Last section

	pmt[13] = 0xE1 // PCR PID
	pmt[14] = 0x00 // PCR PID = 0x100 (Video track usually)
	pmt[15] = 0xF0 // Program info length
	pmt[16] = 0x00

	offset := 17
	for i, track := range m.tracks {
		if track.Type == core.TrackTypeVideo {
			pmt[offset] = 0x1B // H.264 Video Stream Type
		} else {
			pmt[offset] = 0x0F // AAC Audio Stream Type
		}
		pmt[offset+1] = byte(0xE0 | (0x100+i)>>8)
		pmt[offset+2] = byte(0x100 + i)
		pmt[offset+3] = 0xF0
		pmt[offset+4] = 0x00 // Info length = 0
		offset += 5
	}

	// Padding
	for i := offset; i < 188; i++ {
		pmt[i] = 0xFF
	}

	_, err := m.file.Write(pmt)
	return err
}
