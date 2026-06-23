package demux

import (
	"io"
	"os"
	"testing"
)

func TestFLACDemuxer(t *testing.T) {
	var data []byte

	// 1. Signature "fLaC"
	data = append(data, 'f', 'L', 'a', 'C')

	// 2. STREAMINFO metadata block (34 bytes payload, header 4 bytes)
	data = append(data, 0x80) // Last block, type 0
	data = append(data, 0, 0, 34) // size 34

	streaminfo := make([]byte, 34)
	// Sample rate = 44100Hz -> 0x0AC44
	// 44100 >> 12 = 10 (0x0A)
	// (44100 >> 4) & 0xFF = 196 (0xC4)
	// (44100 & 0x0F) << 4 = 4 << 4 = 64 (0x40)
	streaminfo[10] = 0x0A
	streaminfo[11] = 0xC4
	streaminfo[12] = 0x40
	data = append(data, streaminfo...)

	// 3. Audio Frame (14 bytes)
	// Header: 0xFF, 0xF8, 0xC0, 0x00
	data = append(data, 0xFF, 0xF8, 0xC0, 0x00)
	dummyPayload := make([]byte, 10)
	data = append(data, dummyPayload...)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "flac_test_*.flac")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write mock FLAC: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek back: %v", err)
	}

	demuxer := NewFLACDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	tr := tracks[0]
	if tr.Timescale != 44100 {
		t.Errorf("Expected timescale 44100, got %d", tr.Timescale)
	}
	if tr.CodecTag != "flac" {
		t.Errorf("Expected CodecTag 'flac', got '%s'", tr.CodecTag)
	}

	if len(tr.Samples) != 1 {
		t.Fatalf("Expected 1 frame sample, got %d", len(tr.Samples))
	}

	s := tr.Samples[0]
	if s.Size != 14 {
		t.Errorf("Expected sample size 14, got %d", s.Size)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	if len(pkt.Data) != 14 {
		t.Errorf("Expected packet data size 14, got %d", len(pkt.Data))
	}
}
