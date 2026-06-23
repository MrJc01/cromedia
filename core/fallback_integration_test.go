package core

import (
	"os"
	"path/filepath"
	"testing"
)

// Task 222: End-to-end integration test simulating transparent fallback in production
func TestFFmpegFallbackIntegration(t *testing.T) {
	_, err := CheckFFmpegExecutable()
	if err != nil {
		t.Skip("ffmpeg executable not found in PATH, skipping integration test")
	}

	tmpDir, err := os.MkdirTemp("", "cromedia_fallback_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "dummy_input.wav")
	outputPath := filepath.Join(tmpDir, "dummy_output.wav")

	dummyWav := make([]byte, 44)
	copy(dummyWav[0:4], []byte("RIFF"))
	copy(dummyWav[8:12], []byte("WAVE"))
	copy(dummyWav[12:16], []byte("fmt "))
	binaryWriteLe(dummyWav[16:20], uint32(16))
	binaryWriteLe(dummyWav[20:22], uint16(1))
	binaryWriteLe(dummyWav[22:24], uint16(1))
	binaryWriteLe(dummyWav[24:28], uint32(8000))
	binaryWriteLe(dummyWav[28:32], uint32(16000))
	binaryWriteLe(dummyWav[32:34], uint16(2))
	binaryWriteLe(dummyWav[34:36], uint16(16))
	copy(dummyWav[36:40], []byte("data"))
	binaryWriteLe(dummyWav[40:44], uint32(0))

	if err := os.WriteFile(inputPath, dummyWav, 0644); err != nil {
		t.Fatal(err)
	}

	args := []string{"-y", "-i", inputPath, "-c:a", "copy", outputPath}
	err = RunFFmpegCompat(args)
	if err != nil {
		t.Fatalf("Fallback execution failed: %v", err)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("Expected fallback output file %s to be created, but it was not: %v", outputPath, err)
	}
}

func binaryWriteLe(b []byte, v interface{}) {
	switch val := v.(type) {
	case uint16:
		b[0] = byte(val)
		b[1] = byte(val >> 8)
	case uint32:
		b[0] = byte(val)
		b[1] = byte(val >> 8)
		b[2] = byte(val >> 16)
		b[3] = byte(val >> 24)
	}
}
