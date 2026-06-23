package demux

import (
	"encoding/binary"
	"io"
	"os"
	"testing"
)

func TestWebPDemuxer(t *testing.T) {
	var data []byte

	// 1. RIFF/WEBP Header
	data = append(data, 'R', 'I', 'F', 'F')
	data = append(data, 0, 0, 0, 0) // Placeholder
	data = append(data, 'W', 'E', 'B', 'P')

	// 2. ANMF Chunk (Animation Frame)
	data = append(data, 'A', 'N', 'M', 'F')
	
	var chunkSize [4]byte
	binary.LittleEndian.PutUint32(chunkSize[:], 20) // size of ANMF payload (16 meta + 4 dummy VP8)
	data = append(data, chunkSize[:]...)

	// ANMF Meta: 16 bytes
	data = append(data, 0, 0, 0) // X
	data = append(data, 0, 0, 0) // Y
	data = append(data, 0, 0, 0) // Width - 1
	data = append(data, 0, 0, 0) // Height - 1
	data = append(data, 0x64, 0x00, 0x00) // Duration = 100 ms (little endian)
	data = append(data, 0) // Flags

	// Dummy frame payload: 4 bytes
	data = append(data, 0x01, 0x02, 0x03, 0x04)

	// Update RIFF size
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))

	// Create temp file
	tmpFile, err := os.CreateTemp("", "webp_test_*.webp")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write mock WebP: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}

	demuxer := NewWebPDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	tr := tracks[0]
	if tr.CodecTag != "webp" {
		t.Errorf("Expected CodecTag 'webp', got '%s'", tr.CodecTag)
	}

	if len(tr.Samples) != 1 {
		t.Fatalf("Expected 1 sample, got %d", len(tr.Samples))
	}

	s := tr.Samples[0]
	if s.Time != 0 {
		t.Errorf("Expected sample time 0, got %d", s.Time)
	}
	if s.Duration != 100 {
		t.Errorf("Expected sample duration 100, got %d", s.Duration)
	}
	if s.Size != 4 {
		t.Errorf("Expected sample payload size 4, got %d", s.Size)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}
	if len(pkt.Data) != 4 {
		t.Errorf("Expected packet data size 4, got %d", len(pkt.Data))
	}
}
