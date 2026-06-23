package mux

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"cromedia/core"
)

// EBMLWriter provides helper methods to write EBML structured data.
type EBMLWriter struct {
	w *os.File
}

// NewEBMLWriter creates an EBMLWriter.
func NewEBMLWriter(w *os.File) *EBMLWriter {
	return &EBMLWriter{w: w}
}

// VINTEncode encodes a value as a variable-length integer (VINT) with the size marker.
func VINTEncode(val uint64) []byte {
	if val < 0x7F {
		return []byte{byte(0x80 | val)}
	} else if val < 0x3FFF {
		return []byte{byte(0x40 | (val >> 8)), byte(val)}
	} else if val < 0x1FFFFF {
		return []byte{byte(0x20 | (val >> 16)), byte(val >> 8), byte(val)}
	} else if val < 0x0FFFFFFF {
		return []byte{byte(0x10 | (val >> 24)), byte(val >> 16), byte(val >> 8), byte(val)}
	} else {
		// 8-byte fallback
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, val)
		buf[0] |= 0x01
		return buf
	}
}

// WriteElement writes a full EBML element: ID + Size + Data.
func (ew *EBMLWriter) WriteElement(id uint32, data []byte) error {
	idBytes := serializeEBMLID(id)
	sizeBytes := VINTEncode(uint64(len(data)))
	if _, err := ew.w.Write(idBytes); err != nil {
		return err
	}
	if _, err := ew.w.Write(sizeBytes); err != nil {
		return err
	}
	if _, err := ew.w.Write(data); err != nil {
		return err
	}
	return nil
}

// WriteMasterHeader writes just the EBML ID and the size bytes (for open containers).
func (ew *EBMLWriter) WriteMasterHeader(id uint32, size int64) error {
	idBytes := serializeEBMLID(id)
	var sizeBytes []byte
	if size < 0 {
		// Unknown size: VINT all ones for 8 bytes
		sizeBytes = []byte{0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	} else {
		sizeBytes = VINTEncode(uint64(size))
	}
	if _, err := ew.w.Write(idBytes); err != nil {
		return err
	}
	if _, err := ew.w.Write(sizeBytes); err != nil {
		return err
	}
	return nil
}

func serializeEBMLID(id uint32) []byte {
	if id <= 0xFF {
		return []byte{byte(id)}
	} else if id <= 0xFFFF {
		return []byte{byte(id >> 8), byte(id)}
	} else if id <= 0xFFFFFF {
		return []byte{byte(id >> 16), byte(id >> 8), byte(id)}
	} else {
		return []byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}
	}
}

// BuildUIntElement helper to encode uint EBML element
func BuildUIntElement(id uint32, val uint64) []byte {
	var valBytes []byte
	if val <= 0xFF {
		valBytes = []byte{byte(val)}
	} else if val <= 0xFFFF {
		valBytes = make([]byte, 2)
		binary.BigEndian.PutUint16(valBytes, uint16(val))
	} else if val <= 0xFFFFFFFF {
		valBytes = make([]byte, 4)
		binary.BigEndian.PutUint32(valBytes, uint32(val))
	} else {
		valBytes = make([]byte, 8)
		binary.BigEndian.PutUint64(valBytes, val)
	}
	
	idBytes := serializeEBMLID(id)
	sizeBytes := VINTEncode(uint64(len(valBytes)))
	return append(append(idBytes, sizeBytes...), valBytes...)
}

// BuildStringElement helper to encode string EBML element
func BuildStringElement(id uint32, val string) []byte {
	idBytes := serializeEBMLID(id)
	sizeBytes := VINTEncode(uint64(len(val)))
	return append(append(idBytes, sizeBytes...), []byte(val)...)
}

// WebMMuxer handles writing WebM containers.
type WebMMuxer struct {
	file         *os.File
	ew           *EBMLWriter
	tracks       []core.Track
	clusterTime  uint64
	hasHeader    bool
}

// NewWebMMuxer creates a WebMMuxer.
func NewWebMMuxer(file *os.File) *WebMMuxer {
	return &WebMMuxer{
		file: file,
		ew:   NewEBMLWriter(file),
	}
}

func (m *WebMMuxer) WriteHeader(tracks []core.Track) error {
	m.tracks = tracks

	// 1. EBML Header
	ebmlPayload := bytes.NewBuffer(nil)
	ebmlPayload.Write(BuildUIntElement(0x4286, 1)) // EBMLVersion
	ebmlPayload.Write(BuildUIntElement(0x42F7, 1)) // EBMLReadVersion
	ebmlPayload.Write(BuildUIntElement(0x42F2, 4)) // EBMLMaxIDLength
	ebmlPayload.Write(BuildUIntElement(0x42F3, 8)) // EBMLMaxSizeLength
	ebmlPayload.Write(BuildStringElement(0x4282, "webm")) // DocType
	ebmlPayload.Write(BuildUIntElement(0x4287, 2)) // DocTypeVersion
	ebmlPayload.Write(BuildUIntElement(0x4285, 2)) // DocTypeReadVersion

	if err := m.ew.WriteElement(0x1A45DFA3, ebmlPayload.Bytes()); err != nil { // EBML ID
		return err
	}

	// 2. Open Segment Header (unknown/infinite size)
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
		}
		trackEntry.Write(BuildUIntElement(0x83, trackTypeVal)) // TrackType

		codecID := "V_VP8"
		if track.Type == core.TrackTypeAudio {
			codecID = "A_VORBIS"
		}
		if track.CodecTag != "" {
			if track.Type == core.TrackTypeVideo {
				if track.CodecTag == "avc1" || track.CodecTag == "h264" {
					codecID = "V_MPEG4/ISO/AVC"
				} else if track.CodecTag == "hev1" || track.CodecTag == "h265" {
					codecID = "V_MPEGH/ISO/HEVC"
				}
			} else {
				if track.CodecTag == "opus" {
					codecID = "A_OPUS"
				} else if track.CodecTag == "mp4a" || track.CodecTag == "aac" {
					codecID = "A_AAC"
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
			// Replace last size tag for Video master element:
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

func (m *WebMMuxer) WritePacket(pkt *core.Packet) error {
	if !m.hasHeader {
		return fmt.Errorf("must write header before packet")
	}

	// Write a SimpleBlock: ID 0xA3
	// SimpleBlock structure:
	// - Track Number: VINT (usually StreamIndex + 1)
	// - Timecode: int16 (relative to Cluster Timecode, in ms)
	// - Flags: 1 byte (Keyframe = 0x80, otherwise 0x00)
	// - Frame payload
	trackNumBytes := VINTEncode(uint64(pkt.StreamIndex + 1))
	
	relTimecode := int16(pkt.PTS) // Assuming PTS is in ms already for simplicity
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

func (m *WebMMuxer) WriteTrailer() error {
	return nil
}

func (m *WebMMuxer) Close() error {
	return m.file.Close()
}
