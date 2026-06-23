package mux

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"cromedia/core"
)

// MKVMuxer handles Matroska (.mkv) container writing.
type MKVMuxer struct {
	file         *os.File
	ew           *EBMLWriter
	tracks       []core.Track
	clusterTime  uint64
	hasHeader    bool
}

// NewMKVMuxer creates a new MKVMuxer.
func NewMKVMuxer(file *os.File) *MKVMuxer {
	return &MKVMuxer{
		file: file,
		ew:   NewEBMLWriter(file),
	}
}

func (m *MKVMuxer) WriteHeader(tracks []core.Track) error {
	m.tracks = tracks

	// 1. EBML Header (DocType = "matroska")
	ebmlPayload := bytes.NewBuffer(nil)
	ebmlPayload.Write(BuildUIntElement(0x4286, 1)) // EBMLVersion
	ebmlPayload.Write(BuildUIntElement(0x42F7, 1)) // EBMLReadVersion
	ebmlPayload.Write(BuildUIntElement(0x42F2, 4)) // EBMLMaxIDLength
	ebmlPayload.Write(BuildUIntElement(0x42F3, 8)) // EBMLMaxSizeLength
	ebmlPayload.Write(BuildStringElement(0x4282, "matroska")) // DocType
	ebmlPayload.Write(BuildUIntElement(0x4287, 4)) // DocTypeVersion
	ebmlPayload.Write(BuildUIntElement(0x4285, 2)) // DocTypeReadVersion

	if err := m.ew.WriteElement(0x1A45DFA3, ebmlPayload.Bytes()); err != nil { // EBML ID
		return err
	}

	// 2. Open Segment Header
	if err := m.ew.WriteMasterHeader(0x18538067, -1); err != nil {
		return err
	}

	// 3. Segment Info
	infoPayload := bytes.NewBuffer(nil)
	infoPayload.Write(BuildUIntElement(0x2AD7B1, 1000000)) // TimecodeScale = 1ms
	infoPayload.Write(BuildStringElement(0x4D80, "CroMedia")) // MuxingApp
	infoPayload.Write(BuildStringElement(0x5741, "CroMedia")) // WritingApp

	if err := m.ew.WriteElement(0x1549A966, infoPayload.Bytes()); err != nil {
		return err
	}

	// 4. Tracks Element
	tracksPayload := bytes.NewBuffer(nil)
	for i, track := range tracks {
		trackEntry := bytes.NewBuffer(nil)
		trackEntry.Write(BuildUIntElement(0xD7, uint64(i+1))) // TrackNumber
		trackEntry.Write(BuildUIntElement(0x73C5, uint64(track.ID))) // TrackUID
		
		trackTypeVal := uint64(1) // Video
		if track.Type == core.TrackTypeAudio {
			trackTypeVal = 2 // Audio
		} else if track.Type == core.TrackTypeMeta {
			trackTypeVal = 17 // Subtitles
		}
		trackEntry.Write(BuildUIntElement(0x83, trackTypeVal)) // TrackType

		codecID := "V_MPEG4/ISO/AVC"
		if track.Type == core.TrackTypeAudio {
			codecID = "A_AAC"
		} else if track.Type == core.TrackTypeMeta {
			codecID = "S_TEXT/UTF8" // SRT subtitle format in MKV
		}
		
		if track.CodecTag != "" {
			if track.Type == core.TrackTypeVideo {
				if track.CodecTag == "hev1" || track.CodecTag == "h265" {
					codecID = "V_MPEGH/ISO/HEVC"
				} else if track.CodecTag == "vp9" {
					codecID = "V_VP9"
				}
			} else if track.Type == core.TrackTypeAudio {
				if track.CodecTag == "opus" {
					codecID = "A_OPUS"
				}
			}
		}
		trackEntry.Write(BuildStringElement(0x86, codecID)) // CodecID

		// If video, write video settings
		if track.Type == core.TrackTypeVideo {
			videoSettings := bytes.NewBuffer(nil)
			videoSettings.Write(BuildUIntElement(0xB0, uint64(track.Width)))
			videoSettings.Write(BuildUIntElement(0xBA, uint64(track.Height)))
			trackEntry.Write(BuildUIntElement(0xE0, 0)) // Video master
			videoBytes := videoSettings.Bytes()
			videoHeader := append(serializeEBMLID(0xE0), VINTEncode(uint64(len(videoBytes)))...)
			trackEntry.Write(append(videoHeader, videoBytes...))
		}

		entryBytes := trackEntry.Bytes()
		entryHeader := append(serializeEBMLID(0xAE), VINTEncode(uint64(len(entryBytes)))...)
		tracksPayload.Write(append(entryHeader, entryBytes...))
	}

	if err := m.ew.WriteElement(0x1654AE6B, tracksPayload.Bytes()); err != nil {
		return err
	}

	// 5. Start First Cluster
	if err := m.ew.WriteMasterHeader(0x1F43B675, -1); err != nil {
		return err
	}
	clusterTimecode := BuildUIntElement(0xE7, 0)
	if _, err := m.file.Write(clusterTimecode); err != nil {
		return err
	}

	m.hasHeader = true
	return nil
}

func (m *MKVMuxer) WritePacket(pkt *core.Packet) error {
	if !m.hasHeader {
		return fmt.Errorf("must write header before packet")
	}

	trackNumBytes := VINTEncode(uint64(pkt.StreamIndex + 1))
	
	relTimecode := int16(pkt.PTS)
	var timecodeBuf [2]byte
	binary.BigEndian.PutUint16(timecodeBuf[:], uint16(relTimecode))

	var flags byte = 0x00
	if pkt.IsKeyframe {
		flags = 0x80
	}

	blockHeader := append(trackNumBytes, timecodeBuf[0], timecodeBuf[1], flags)
	blockPayload := append(blockHeader, pkt.Data...)

	return m.ew.WriteElement(0xA3, blockPayload)
}

func (m *MKVMuxer) WriteTrailer() error {
	return nil
}

func (m *MKVMuxer) Close() error {
	return m.file.Close()
}
