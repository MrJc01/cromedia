package demux

import (
	"io"
	"os"
	"testing"
)

func TestAACDemuxer(t *testing.T) {
	var data []byte

	// Construct one valid 25-byte ADTS frame
	// Header: 7 bytes
	data = append(data, 0xFF) // Syncword 1
	data = append(data, 0xF1) // Syncword 2 (4 bits) + MPEG-4 (0) + Layer (00) + Protection Absent (1)
	data = append(data, 0x50) // Profile LC (1) + Sampling Freq 44100Hz (4)
	data = append(data, 0x80) // Channels (2) + Frame length bit 12, 11 (0)
	data = append(data, 0x03) // Frame length bits 10-3 (3)
	data = append(data, 0x2F) // Frame length bits 2-0 (1) + buffer fullness
	data = append(data, 0xFC) // Buffer fullness + AAC frames (0 = 1 frame)

	// Append 18 bytes of dummy AAC payload to make total frame size 25 bytes
	dummyPayload := make([]byte, 18)
	data = append(data, dummyPayload...)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "aac_test_*.aac")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write mock AAC: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek back: %v", err)
	}

	demuxer := NewAACDemuxer(tmpFile)
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
	if tr.CodecTag != "mp4a" {
		t.Errorf("Expected CodecTag 'mp4a', got '%s'", tr.CodecTag)
	}

	if len(tr.Samples) != 1 {
		t.Fatalf("Expected 1 frame sample, got %d", len(tr.Samples))
	}

	s := tr.Samples[0]
	if s.Size != 25 {
		t.Errorf("Expected sample size 25, got %d", s.Size)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	if len(pkt.Data) != 25 {
		t.Errorf("Expected packet data size 25, got %d", len(pkt.Data))
	}
}
