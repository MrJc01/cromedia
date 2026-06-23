package demux

import (
	"encoding/binary"
	"io"
	"os"
	"testing"
)

func TestFLVDemuxer(t *testing.T) {
	// Construct a mock FLV file in memory
	var data []byte

	// 1. FLV Header
	data = append(data, 'F', 'L', 'V', 1)
	data = append(data, 5) // Audio + Video present
	data = append(data, 0, 0, 0, 9) // Header size = 9

	// 2. PreviousTagSize0 (4 bytes, always 0)
	data = append(data, 0, 0, 0, 0)

	// 3. FLV Video Tag (Type 9)
	// Timestamp = 1000 ms. Extended = 0.
	data = append(data, 9) // Type Video
	data = append(data, 0, 0, 5) // Data size = 5
	data = append(data, 0, 3, 0xE8) // TS = 1000 (0x3E8)
	data = append(data, 0) // TS Extended = 0
	data = append(data, 0, 0, 0) // StreamID = 0
	data = append(data, 0x17) // 1 = Keyframe, 7 = AVC
	data = append(data, 0x01, 0x02, 0x03, 0x04) // Dummy AVC payload
	
	// PreviousTagSize1 (Data size 5 + Header 11 = 16)
	var pts1 [4]byte
	binary.BigEndian.PutUint32(pts1[:], 16)
	data = append(data, pts1[:]...)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "flv_test_*.flv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write mock FLV: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek back: %v", err)
	}

	demuxer := NewFLVDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	tr := tracks[0]
	if tr.ID != 9 {
		t.Errorf("Expected track ID 9, got %d", tr.ID)
	}
	if tr.CodecTag != "avc1" {
		t.Errorf("Expected codec tag avc1, got %s", tr.CodecTag)
	}

	if len(tr.Samples) != 1 {
		t.Fatalf("Expected 1 sample, got %d", len(tr.Samples))
	}

	s := tr.Samples[0]
	if s.Time != 1000 {
		t.Errorf("Expected sample time 1000, got %d", s.Time)
	}
	if !s.IsKeyframe {
		t.Errorf("Expected sample to be detected as keyframe")
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	if pkt.PTS != 1000 {
		t.Errorf("Expected packet PTS 1000, got %d", pkt.PTS)
	}

	expectedPayload := []byte{0x17, 0x01, 0x02, 0x03, 0x04}
	if string(pkt.Data) != string(expectedPayload) {
		t.Errorf("Expected packet data %v, got %v", expectedPayload, pkt.Data)
	}
}
