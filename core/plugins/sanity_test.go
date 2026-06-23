package plugins

import (
	"bytes"
	"errors"
	"testing"

	"cromedia/core"
)

func TestCGOBuffer(t *testing.T) {
	buf, err := AllocCGOBuffer(1024)
	if err != nil {
		t.Fatalf("failed to allocate CGOBuffer: %v", err)
	}

	if buf.Size() != 1024 {
		t.Errorf("expected size 1024, got %d", buf.Size())
	}

	if buf.Pointer() == nil {
		t.Error("expected non-nil unsafe pointer")
	}

	if len(buf.Bytes()) != 1024 {
		t.Errorf("expected slice length 1024, got %d", len(buf.Bytes()))
	}

	// Test copy
	src := []byte("hello world")
	n := buf.CopyFrom(src)
	if n != len(src) {
		t.Errorf("expected copied %d bytes, got %d", len(src), n)
	}
	if string(buf.Bytes()[:len(src)]) != "hello world" {
		t.Errorf("expected buffer contents to be 'hello world', got %s", string(buf.Bytes()[:len(src)]))
	}

	// Test invalid allocation
	_, err = AllocCGOBuffer(-1)
	if err == nil {
		t.Error("expected error for negative buffer size, got nil")
	}
}

func TestStubGenerator(t *testing.T) {
	var buf bytes.Buffer
	err := GenerateCPluginHeader(&buf)
	if err != nil {
		t.Fatalf("failed to generate plugin header: %v", err)
	}

	content := buf.String()
	expectedSubstrings := []string{
		"typedef struct",
		"PluginMetadata",
		"PluginPacket",
		"PluginVideoFrame",
		"PluginAudioFrame",
		"InitPlugin",
		"GetPluginMetadata",
	}

	for _, sub := range expectedSubstrings {
		if !bytes.Contains(buf.Bytes(), []byte(sub)) {
			t.Errorf("expected header to contain %q, but it didn't. Content:\n%s", sub, content)
		}
	}
}

func TestFrameSanityValidation(t *testing.T) {
	// 1. Valid video frame
	vFrame := &core.VideoFrame{
		Width:  640,
		Height: 480,
		Format: "yuv420p",
		Data:   make([]byte, (640*480*3)/2),
	}
	if err := ValidateVideoFrame(vFrame); err != nil {
		t.Errorf("expected valid video frame to pass, got %v", err)
	}

	// 2. Video frame invalid dimensions
	vFrameBadDim := &core.VideoFrame{
		Width:  -1,
		Height: 480,
		Format: "yuv420p",
	}
	if err := ValidateVideoFrame(vFrameBadDim); !errors.Is(err, ErrInvalidFrameDimensions) {
		t.Errorf("expected ErrInvalidFrameDimensions, got %v", err)
	}

	// 3. Video frame invalid buffer size
	vFrameBadBuf := &core.VideoFrame{
		Width:  640,
		Height: 480,
		Format: "rgba",
		Data:   make([]byte, 100), // Should be 640*480*4
	}
	if err := ValidateVideoFrame(vFrameBadBuf); !errors.Is(err, ErrInvalidFrameBufferSize) {
		t.Errorf("expected ErrInvalidFrameBufferSize, got %v", err)
	}

	// 4. Valid audio frame
	aFrame := &core.AudioFrame{
		Channels:   2,
		SampleRate: 44100,
		Data:       make([]float32, 1024),
	}
	if err := ValidateAudioFrame(aFrame); err != nil {
		t.Errorf("expected valid audio frame to pass, got %v", err)
	}

	// 5. Audio frame invalid channels
	aFrameBadChan := &core.AudioFrame{
		Channels:   0,
		SampleRate: 44100,
	}
	if err := ValidateAudioFrame(aFrameBadChan); !errors.Is(err, ErrInvalidAudioChannels) {
		t.Errorf("expected ErrInvalidAudioChannels, got %v", err)
	}

	// 6. Audio frame invalid sample rate
	aFrameBadRate := &core.AudioFrame{
		Channels:   2,
		SampleRate: 100,
	}
	if err := ValidateAudioFrame(aFrameBadRate); !errors.Is(err, ErrInvalidAudioSampleRate) {
		t.Errorf("expected ErrInvalidAudioSampleRate, got %v", err)
	}
}
