package demux

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestSRTDemuxer(t *testing.T) {
	srtContent := `1
00:00:01,000 --> 00:00:02,500
Hello World!

2
00:00:03,000 --> 00:00:04,100
Testing subtitles.
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "srt_test_*.srt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(srtContent); err != nil {
		t.Fatalf("Failed to write mock SRT: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}

	demuxer := NewSRTDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	tr := tracks[0]
	if tr.CodecTag != "srt" {
		t.Errorf("Expected CodecTag 'srt', got '%s'", tr.CodecTag)
	}

	if len(tr.Samples) != 2 {
		t.Fatalf("Expected 2 samples, got %d", len(tr.Samples))
	}

	s1 := tr.Samples[0]
	if s1.Time != 1000 {
		t.Errorf("Expected first sample time 1000, got %d", s1.Time)
	}
	if s1.Duration != 1500 {
		t.Errorf("Expected first sample duration 1500, got %d", s1.Duration)
	}

	s2 := tr.Samples[1]
	if s2.Time != 3000 {
		t.Errorf("Expected second sample time 3000, got %d", s2.Time)
	}
	if s2.Duration != 1100 {
		t.Errorf("Expected second sample duration 1100, got %d", s2.Duration)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	text := string(pkt.Data)
	if !strings.Contains(text, "Hello World!") {
		t.Errorf("Expected subtitle 'Hello World!', got '%s'", text)
	}
}
