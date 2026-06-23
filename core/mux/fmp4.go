package mux

import (
	"encoding/binary"
	"fmt"
	"os"

	"cromedia/core"
)

// FMP4Muxer handles Fragmented MP4 (fMP4) writing.
type FMP4Muxer struct {
	file        *os.File
	tracks      []core.Track
	seqNum      uint32
	trackSampleCount map[int]int
}

// NewFMP4Muxer creates a new FMP4Muxer.
func NewFMP4Muxer(file *os.File) *FMP4Muxer {
	return &FMP4Muxer{
		file:             file,
		seqNum:           1,
		trackSampleCount: make(map[int]int),
	}
}

func (m *FMP4Muxer) WriteHeader(tracks []core.Track) error {
	m.tracks = tracks

	// 1. Write ftyp
	ftyp := []byte{
		0, 0, 0, 24,
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
		0, 0, 2, 0, // minor version
		'i', 's', 'o', 'm',
		'i', 's', 'o', '2',
	}
	if _, err := m.file.Write(ftyp); err != nil {
		return err
	}

	// 2. Build moov box (minimum metadata, without mdat offset mapping since it's fragmented)
	// Build mvex box inside moov
	var tracksBytes []byte
	for _, track := range tracks {
		// Minimum track header
		tkhd := make([]byte, 92)
		binary.BigEndian.PutUint32(tkhd[0:4], 92)
		copy(tkhd[4:8], "tkhd")
		tkhd[8] = 0 // version
		binary.BigEndian.PutUint32(tkhd[20:24], uint32(track.ID))
		binary.BigEndian.PutUint32(tkhd[84:88], track.Width)
		binary.BigEndian.PutUint32(tkhd[88:92], track.Height)

		// mdia box
		mdhd := make([]byte, 32)
		binary.BigEndian.PutUint32(mdhd[0:4], 32)
		copy(mdhd[4:8], "mdhd")
		binary.BigEndian.PutUint32(mdhd[20:24], track.Timescale)

		hdlr := make([]byte, 32)
		binary.BigEndian.PutUint32(hdlr[0:4], 32)
		copy(hdlr[4:8], "hdlr")
		copy(hdlr[16:20], string(track.Type))

		minf := []byte{
			0, 0, 0, 36,
			'm', 'i', 'n', 'f',
			0, 0, 0, 28, // stbl (minimum)
			's', 't', 'b', 'l',
			0, 0, 0, 20, // stsd
			's', 't', 's', 'd',
			0, 0, 0, 0, // version + flags
			0, 0, 0, 0, // entry count
		}

		mdiaPayload := append(mdhd, hdlr...)
		mdiaPayload = append(mdiaPayload, minf...)
		mdia := make([]byte, 8)
		binary.BigEndian.PutUint32(mdia[0:4], uint32(len(mdiaPayload)+8))
		copy(mdia[4:8], "mdia")
		mdia = append(mdia, mdiaPayload...)

		trakPayload := append(tkhd, mdia...)
		trak := make([]byte, 8)
		binary.BigEndian.PutUint32(trak[0:4], uint32(len(trakPayload)+8))
		copy(trak[4:8], "trak")
		trak = append(trak, trakPayload...)

		tracksBytes = append(tracksBytes, trak...)
	}

	// mvex box
	mvexPayload := []byte{}
	for _, track := range tracks {
		trex := make([]byte, 32)
		binary.BigEndian.PutUint32(trex[0:4], 32)
		copy(trex[4:8], "trex")
		binary.BigEndian.PutUint32(trex[12:16], uint32(track.ID))
		binary.BigEndian.PutUint32(trex[16:20], 1) // default sample description index
		binary.BigEndian.PutUint32(trex[20:24], 1000) // default duration
		binary.BigEndian.PutUint32(trex[24:28], 1024) // default size
		mvexPayload = append(mvexPayload, trex...)
	}
	mvex := make([]byte, 8)
	binary.BigEndian.PutUint32(mvex[0:4], uint32(len(mvexPayload)+8))
	copy(mvex[4:8], "mvex")
	mvex = append(mvex, mvexPayload...)

	mvhd := make([]byte, 108)
	binary.BigEndian.PutUint32(mvhd[0:4], 108)
	copy(mvhd[4:8], "mvhd")
	binary.BigEndian.PutUint32(mvhd[20:24], 1000) // timescale
	binary.BigEndian.PutUint32(mvhd[84:88], uint32(len(tracks)+1)) // next track ID

	moovPayload := append(mvhd, tracksBytes...)
	moovPayload = append(moovPayload, mvex...)

	moov := make([]byte, 8)
	binary.BigEndian.PutUint32(moov[0:4], uint32(len(moovPayload)+8))
	copy(moov[4:8], "moov")
	moov = append(moov, moovPayload...)

	_, err := m.file.Write(moov)
	return err
}

func (m *FMP4Muxer) WritePacket(pkt *core.Packet) error {
	if pkt.StreamIndex >= len(m.tracks) {
		return fmt.Errorf("packet StreamIndex out of range")
	}
	track := m.tracks[pkt.StreamIndex]

	// Create moof box
	mfhd := make([]byte, 16)
	binary.BigEndian.PutUint32(mfhd[0:4], 16)
	copy(mfhd[4:8], "mfhd")
	binary.BigEndian.PutUint32(mfhd[12:16], m.seqNum)
	m.seqNum++

	tfhd := make([]byte, 16)
	binary.BigEndian.PutUint32(tfhd[0:4], 16)
	copy(tfhd[4:8], "tfhd")
	binary.BigEndian.PutUint32(tfhd[8:12], 0x020000) // default-base-is-moof
	binary.BigEndian.PutUint32(tfhd[12:16], uint32(track.ID))

	// Single sample track run (trun)
	trun := make([]byte, 32)
	binary.BigEndian.PutUint32(trun[0:4], 32)
	copy(trun[4:8], "trun")
	binary.BigEndian.PutUint32(trun[8:12], 0x000301) // data-offset-present | sample-size-present | sample-duration-present
	binary.BigEndian.PutUint32(trun[12:16], 1) // sample count

	// Data offset: moof size + 8 (mdat header size)
	// We'll patch this offset after compiling moof size
	
	binary.BigEndian.PutUint32(trun[20:24], uint32(pkt.Duration))
	binary.BigEndian.PutUint32(trun[24:28], uint32(len(pkt.Data)))

	trafPayload := append(tfhd, trun...)
	traf := make([]byte, 8)
	binary.BigEndian.PutUint32(traf[0:4], uint32(len(trafPayload)+8))
	copy(traf[4:8], "traf")
	traf = append(traf, trafPayload...)

	moofPayload := append(mfhd, traf...)
	moof := make([]byte, 8)
	binary.BigEndian.PutUint32(moof[0:4], uint32(len(moofPayload)+8))
	copy(moof[4:8], "moof")
	moof = append(moof, moofPayload...)

	// Patch trun data offset to point to mdat start
	// Offset is relative to first byte of moof box
	binary.BigEndian.PutUint32(moof[len(moof)-16:len(moof)-12], uint32(len(moof))+8)

	// Write moof
	if _, err := m.file.Write(moof); err != nil {
		return err
	}

	// Write mdat
	mdatHeader := make([]byte, 8)
	binary.BigEndian.PutUint32(mdatHeader[0:4], uint32(len(pkt.Data)+8))
	copy(mdatHeader[4:8], "mdat")
	if _, err := m.file.Write(mdatHeader); err != nil {
		return err
	}

	if _, err := m.file.Write(pkt.Data); err != nil {
		return err
	}

	return nil
}

func (m *FMP4Muxer) WriteTrailer() error {
	return nil
}

func (m *FMP4Muxer) Close() error {
	return m.file.Close()
}
