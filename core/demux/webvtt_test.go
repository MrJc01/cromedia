package demux

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestWebVTTDemuxer(t *testing.T) {
	vttContent := `WEBVTT

00:00.500 --> 00:02.000
VTT subtitle cue.

00:02:10.000 --> 00:02:15.000
Another cue.
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "webvtt_test_*.vtt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(vttContent); err != nil {
		t.Fatalf("Failed to write mock WebVTT: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}

	demuxer := NewWebVTTDemuxer(tmpFile)
	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	tr := tracks[0]
	if tr.CodecTag != "vtt" {
		t.Errorf("Expected CodecTag 'vtt', got '%s'", tr.CodecTag)
	}

	if len(tr.Samples) != 2 {
		t.Fatalf("Expected 2 samples, got %d", len(tr.Samples))
	}

	s1 := tr.Samples[0]
	if s1.Time != 500 {
		t.Errorf("Expected first sample time 500, got %d", s1.Time)
	}
	if s1.Duration != 1500 {
		t.Errorf("Expected first sample duration 1500, got %d", s1.Duration)
	}

	s2 := tr.Samples[1]
	if s2.Time != 130000 { // 2m 10s = 130s = 130000ms
		t.Errorf("Expected second sample time 130000, got %d", s2.Time)
	}
	if s2.Duration != 5000 {
		t.Errorf("Expected second sample duration 5000, got %d", s2.Duration)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	text := string(pkt.Data)
	if !strings.Contains(text, "VTT subtitle cue.") {
		t.Errorf("Expected subtitle 'VTT subtitle cue.', got '%s'", text)
	}
}
