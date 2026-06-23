package demux

import (
	"io"
	"os"
	"testing"

	"cromedia/core"
)

func makeVINT(val uint64) []byte {
	var length int
	if val < 0x7F {
		length = 1
	} else if val < 0x3FFF {
		length = 2
	} else if val < 0x1FFFFF {
		length = 3
	} else if val < 0x0FFFFFFF {
		length = 4
	} else {
		length = 8
	}

	buf := make([]byte, length)
	var prefix byte = 0x80 >> (uint(length) - 1)
	temp := val
	for i := length - 1; i >= 0; i-- {
		buf[i] = byte(temp & 0xFF)
		temp >>= 8
	}
	buf[0] |= prefix
	return buf
}

func makeElement(id uint32, payload []byte) []byte {
	var idBytes []byte
	if id <= 0xFF {
		idBytes = []byte{byte(id)}
	} else if id <= 0xFFFF {
		idBytes = []byte{byte(id >> 8), byte(id)}
	} else if id <= 0xFFFFFF {
		idBytes = []byte{byte(id >> 16), byte(id >> 8), byte(id)}
	} else {
		idBytes = []byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}
	}
	return append(idBytes, append(makeVINT(uint64(len(payload))), payload...)...)
}

func TestWebMDemuxer(t *testing.T) {
	// 1. EBML Header
	ebmlHeader := makeElement(0x1A45DFA3, []byte("ebml"))

	// 2. Info -> TimecodeScale (1,000,000 ns = 1ms)
	tcScale := makeElement(0x2AD7B1, []byte{0x0F, 0x42, 0x40})
	info := makeElement(0x1549A966, tcScale)

	// 3. Tracks -> TrackEntry -> Number, Type, CodecID, Video (Width, Height)
	trNum := makeElement(0xD7, []byte{1})
	trType := makeElement(0x83, []byte{1}) // Video
	codecID := makeElement(0x86, []byte("V_VP8"))

	vidWidth := makeElement(0xB0, []byte{0x01, 0x40})  // 320
	vidHeight := makeElement(0xBA, []byte{0x00, 0xF0}) // 240
	var videoBytes []byte
	videoBytes = append(videoBytes, vidWidth...)
	videoBytes = append(videoBytes, vidHeight...)
	video := makeElement(0xE0, videoBytes)

	var trEntryBytes []byte
	trEntryBytes = append(trEntryBytes, trNum...)
	trEntryBytes = append(trEntryBytes, trType...)
	trEntryBytes = append(trEntryBytes, codecID...)
	trEntryBytes = append(trEntryBytes, video...)
	trEntry := makeElement(0xAE, trEntryBytes)

	tracksEl := makeElement(0x1654AE6B, trEntry)

	// 4. Cluster -> Timecode, SimpleBlock
	clusterTimecode := makeElement(0xE7, []byte{10}) // timecode = 10

	// SimpleBlock payload: Track 1, relTimecode 5 (absTimecode = 15), keyframe flag 0x80, data "vp8data"
	blockHeader := []byte{0x81, 0, 5, 0x80}
	blockPayload := append(blockHeader, []byte("vp8data")...)
	simpleBlock := makeElement(0xA3, blockPayload)

	var clusterBytes []byte
	clusterBytes = append(clusterBytes, clusterTimecode...)
	clusterBytes = append(clusterBytes, simpleBlock...)
	cluster := makeElement(0x1F43B675, clusterBytes)

	// Segment (wrapping Info, Tracks, and Cluster)
	var segmentBytes []byte
	segmentBytes = append(segmentBytes, info...)
	segmentBytes = append(segmentBytes, tracksEl...)
	segmentBytes = append(segmentBytes, cluster...)
	segment := makeElement(0x18538067, segmentBytes)

	// Assemble complete WebM mock file
	var fileBytes []byte
	fileBytes = append(fileBytes, ebmlHeader...)
	fileBytes = append(fileBytes, segment...)

	// Create temp file
	tmpFile, err := os.CreateTemp("", "webm_test_*.webm")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(fileBytes); err != nil {
		t.Fatalf("Failed to write mock WebM: %v", err)
	}

	// Seek to beginning
	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}

	demuxer := NewWebMDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	tr := tracks[0]
	if tr.ID != 1 {
		t.Errorf("Expected Track ID 1, got %d", tr.ID)
	}
	if tr.CodecTag != "V_VP8" {
		t.Errorf("Expected Codec ID 'V_VP8', got '%s'", tr.CodecTag)
	}
	if tr.Width != 320 || tr.Height != 240 {
		t.Errorf("Expected 320x240, got %dx%d", tr.Width, tr.Height)
	}

	if len(tr.Samples) != 1 {
		t.Fatalf("Expected 1 sample, got %d", len(tr.Samples))
	}

	s := tr.Samples[0]
	if s.Time != 15 { // Cluster Timecode (10) + SimpleBlock relTimecode (5)
		t.Errorf("Expected sample time 15, got %d", s.Time)
	}
	if !s.IsKeyframe {
		t.Error("Expected sample to be marked as keyframe")
	}

	// Read packet
	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	if string(pkt.Data) != "vp8data" {
		t.Errorf("Expected packet data 'vp8data', got '%s'", string(pkt.Data))
	}
	if pkt.PTS != 15 {
		t.Errorf("Expected packet PTS 15, got %d", pkt.PTS)
	}

	// Read next packet (should EOF)
	_, err = demuxer.ReadPacket()
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}
}

func TestWebMDemuxerMultipleTracks(t *testing.T) {
	// EBML Header
	ebmlHeader := makeElement(0x1A45DFA3, []byte("ebml"))

	// Info
	tcScale := makeElement(0x2AD7B1, []byte{0x0F, 0x42, 0x40}) // 1,000,000 ns
	info := makeElement(0x1549A966, tcScale)

	// Tracks
	// Track 1 (Video)
	trNum1 := makeElement(0xD7, []byte{1})
	trType1 := makeElement(0x83, []byte{1}) // Video
	codecID1 := makeElement(0x86, []byte("V_VP8"))
	var trEntry1Bytes []byte
	trEntry1Bytes = append(trEntry1Bytes, trNum1...)
	trEntry1Bytes = append(trEntry1Bytes, trType1...)
	trEntry1Bytes = append(trEntry1Bytes, codecID1...)
	trEntry1 := makeElement(0xAE, trEntry1Bytes)

	// Track 2 (Audio)
	trNum2 := makeElement(0xD7, []byte{2})
	trType2 := makeElement(0x83, []byte{2}) // Audio
	codecID2 := makeElement(0x86, []byte("A_VORBIS"))
	var trEntry2Bytes []byte
	trEntry2Bytes = append(trEntry2Bytes, trNum2...)
	trEntry2Bytes = append(trEntry2Bytes, trType2...)
	trEntry2Bytes = append(trEntry2Bytes, codecID2...)
	trEntry2 := makeElement(0xAE, trEntry2Bytes)

	// Track 3 (Subtitle)
	trNum3 := makeElement(0xD7, []byte{3})
	trType3 := makeElement(0x83, []byte{17}) // Subtitle
	codecID3 := makeElement(0x86, []byte("S_TEXT/UTF8"))
	var trEntry3Bytes []byte
	trEntry3Bytes = append(trEntry3Bytes, trNum3...)
	trEntry3Bytes = append(trEntry3Bytes, trType3...)
	trEntry3Bytes = append(trEntry3Bytes, codecID3...)
	trEntry3 := makeElement(0xAE, trEntry3Bytes)

	var tracksElBytes []byte
	tracksElBytes = append(tracksElBytes, trEntry1...)
	tracksElBytes = append(tracksElBytes, trEntry2...)
	tracksElBytes = append(tracksElBytes, trEntry3...)
	tracksEl := makeElement(0x1654AE6B, tracksElBytes)

	// Cluster
	clusterTimecode := makeElement(0xE7, []byte{10})

	// SimpleBlock 1 (Track 1, relTimecode 0 -> absTimecode 10)
	block1Header := []byte{0x81, 0, 0, 0x80}
	block1Payload := append(block1Header, []byte("vp8_1")...)
	simpleBlock1 := makeElement(0xA3, block1Payload)

	// SimpleBlock 2 (Track 2, relTimecode 5 -> absTimecode 15)
	block2Header := []byte{0x82, 0, 5, 0x80}
	block2Payload := append(block2Header, []byte("vorbis_1")...)
	simpleBlock2 := makeElement(0xA3, block2Payload)

	// SimpleBlock 3 (Track 3, relTimecode 10 -> absTimecode 20)
	block3Header := []byte{0x83, 0, 10, 0x80}
	block3Payload := append(block3Header, []byte("subtitle_1")...)
	simpleBlock3 := makeElement(0xA3, block3Payload)

	var clusterBytes []byte
	clusterBytes = append(clusterBytes, clusterTimecode...)
	clusterBytes = append(clusterBytes, simpleBlock1...)
	clusterBytes = append(clusterBytes, simpleBlock2...)
	clusterBytes = append(clusterBytes, simpleBlock3...)
	cluster := makeElement(0x1F43B675, clusterBytes)

	var segmentBytes []byte
	segmentBytes = append(segmentBytes, info...)
	segmentBytes = append(segmentBytes, tracksEl...)
	segmentBytes = append(segmentBytes, cluster...)
	segment := makeElement(0x18538067, segmentBytes)

	var fileBytes []byte
	fileBytes = append(fileBytes, ebmlHeader...)
	fileBytes = append(fileBytes, segment...)

	tmpFile, err := os.CreateTemp("", "webm_multitrack_test_*.webm")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(fileBytes); err != nil {
		t.Fatalf("Failed to write mock WebM: %v", err)
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}

	demuxer := NewWebMDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 3 {
		t.Fatalf("Expected 3 tracks, got %d", len(tracks))
	}

	if tracks[0].Type != core.TrackTypeVideo || tracks[0].ID != 1 {
		t.Errorf("Expected track 0 to be video with ID 1, got %s with ID %d", tracks[0].Type, tracks[0].ID)
	}
	if tracks[1].Type != core.TrackTypeAudio || tracks[1].ID != 2 {
		t.Errorf("Expected track 1 to be audio with ID 2, got %s with ID %d", tracks[1].Type, tracks[1].ID)
	}
	if tracks[2].Type != core.TrackTypeMeta || tracks[2].ID != 3 {
		t.Errorf("Expected track 2 to be meta with ID 3, got %s with ID %d", tracks[2].Type, tracks[2].ID)
	}

	// Verify time-interleaved packet reading
	// Order should be: time 10 (track 0), time 15 (track 1), time 20 (track 2)
	p1, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket 1 failed: %v", err)
	}
	if p1.StreamIndex != 0 || p1.PTS != 10 || string(p1.Data) != "vp8_1" {
		t.Errorf("Packet 1 incorrect: %+v", p1)
	}

	p2, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket 2 failed: %v", err)
	}
	if p2.StreamIndex != 1 || p2.PTS != 15 || string(p2.Data) != "vorbis_1" {
		t.Errorf("Packet 2 incorrect: %+v", p2)
	}

	p3, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket 3 failed: %v", err)
	}
	if p3.StreamIndex != 2 || p3.PTS != 20 || string(p3.Data) != "subtitle_1" {
		t.Errorf("Packet 3 incorrect: %+v", p3)
	}

	_, err = demuxer.ReadPacket()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

