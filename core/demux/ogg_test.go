package demux

import (
	"encoding/binary"
	"io"
	"os"
	"testing"
)

func TestOggDemuxer(t *testing.T) {
	var data []byte

	// 1. Ogg Page Header
	data = append(data, 'O', 'g', 'g', 'S') // Capture pattern
	data = append(data, 0)                   // Version
	data = append(data, 0)                   // Header type

	var granulePos [8]byte
	binary.LittleEndian.PutUint64(granulePos[:], 960)
	data = append(data, granulePos[:]...) // Granule position

	var serial [4]byte
	binary.LittleEndian.PutUint32(serial[:], 12345)
	data = append(data, serial[:]...) // Serial number

	var seq [4]byte
	binary.LittleEndian.PutUint32(seq[:], 1)
	data = append(data, seq[:]...) // Sequence number

	data = append(data, 0, 0, 0, 0) // Checksum
	data = append(data, 1)          // Segment count
	data = append(data, 8)          // Segment size (OpusHead length)

	// Segment payload
	data = append(data, []byte("OpusHead")...)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "ogg_test_*.ogg")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write mock Ogg: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek back: %v", err)
	}

	demuxer := NewOggDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	tr := tracks[0]
	if tr.ID != 12345 {
		t.Errorf("Expected track ID 12345, got %d", tr.ID)
	}
	if tr.CodecTag != "opus" {
		t.Errorf("Expected codec tag 'opus', got '%s'", tr.CodecTag)
	}

	if len(tr.Samples) != 1 {
		t.Fatalf("Expected 1 sample, got %d", len(tr.Samples))
	}

	s := tr.Samples[0]
	if s.Time != 960 {
		t.Errorf("Expected sample time 960, got %d", s.Time)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	if string(pkt.Data) != "OpusHead" {
		t.Errorf("Expected packet data 'OpusHead', got '%s'", string(pkt.Data))
	}
}
