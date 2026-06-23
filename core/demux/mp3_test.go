package demux

import (
	"io"
	"os"
	"testing"
)

func TestMP3Demuxer(t *testing.T) {
	var data []byte

	// 1. Prepend ID3v2 Header (10 bytes)
	data = append(data, 'I', 'D', '3', 3, 0, 0) // ID3v2.3
	data = append(data, 0, 0, 0, 0)             // Size = 0 synchsafe bytes (0 bytes payload)

	// 2. Prepend valid MPEG audio frame (417 bytes)
	// Header: 0xFF, 0xFB, 0x90, 0x00
	// 0xFFFB -> Syncword + MPEG V1, Layer III, no CRC
	// 0x90 -> 128kbps (bitrate code 9), 44100Hz (samplerate code 0), padding = 0
	// 0x00 -> Stereo mode
	frameHeader := []byte{0xFF, 0xFB, 0x90, 0x00}
	data = append(data, frameHeader...)

	// Dummy frame payload to fill out 417 bytes
	dummyPayload := make([]byte, 417-4)
	for i := range dummyPayload {
		dummyPayload[i] = 0xAA
	}
	data = append(data, dummyPayload...)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "mp3_test_*.mp3")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write mock MP3: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek back: %v", err)
	}

	demuxer := NewMP3Demuxer(tmpFile)
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
	if tr.CodecTag != "mp3" {
		t.Errorf("Expected CodecTag 'mp3', got '%s'", tr.CodecTag)
	}

	if len(tr.Samples) != 1 {
		t.Fatalf("Expected 1 frame sample, got %d", len(tr.Samples))
	}

	s := tr.Samples[0]
	if s.Size != 417 {
		t.Errorf("Expected sample size 417, got %d", s.Size)
	}

	pkt, err := demuxer.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}

	if len(pkt.Data) != 417 {
		t.Errorf("Expected packet data size 417, got %d", len(pkt.Data))
	}
}
