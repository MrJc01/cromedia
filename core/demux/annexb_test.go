package demux

import (
	"io"
	"os"
	"testing"
)

func TestAnnexBDemuxer(t *testing.T) {
	var data []byte

	// 1. NAL unit 1 (SPS: type 7)
	data = append(data, 0x00, 0x00, 0x00, 0x01)
	data = append(data, 0x07) // SPS
	data = append(data, 0x01, 0x02, 0x03)

	// 2. NAL unit 2 (IDR: type 5)
	data = append(data, 0x00, 0x00, 0x01)
	data = append(data, 0x05) // IDR Slice
	data = append(data, 0x11, 0x12)

	// Create temp file
	tmpFile, err := os.CreateTemp("", "annexb_test_*.h264")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write mock Annex B: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}

	demuxer := NewAnnexBDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	tr := tracks[0]
	if tr.CodecTag != "h264" {
		t.Errorf("Expected CodecTag 'h264', got '%s'", tr.CodecTag)
	}

	if len(tr.Samples) != 2 {
		t.Fatalf("Expected 2 samples, got %d", len(tr.Samples))
	}

	s1 := tr.Samples[0]
	if !s1.IsKeyframe {
		t.Errorf("Expected SPS NAL to be detected as keyframe")
	}

	s2 := tr.Samples[1]
	if !s2.IsKeyframe {
		t.Errorf("Expected IDR NAL to be detected as keyframe")
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	if pkt.PTS != 0 {
		t.Errorf("Expected PTS 0, got %d", pkt.PTS)
	}

	// Read next
	pkt, err = demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket 2 failed: %v", err)
	}
	if pkt.PTS != 3000 {
		t.Errorf("Expected PTS 3000, got %d", pkt.PTS)
	}
}
