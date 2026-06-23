package pipeline

import (
	"image"
	"image/color"
	"os"
	"testing"

	"cromedia/core"
)

func TestRenderScenePipeline(t *testing.T) {
	// Create a mock image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}

	// Output file
	tmpFile, err := os.CreateTemp("", "render_out_*.mp4")
	if err != nil {
		t.Fatal(err)
	}
	outputPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(outputPath)

	// Setup channel
	frames := make(chan interface{}, 5)
	frames <- img
	frames <- img
	close(frames)

	// Run pipeline
	enc := &core.SimH264Encoder{}
	err = RenderScenePipeline(outputPath, 100, 100, 30, enc, frames, nil, nil)
	if err != nil {
		t.Fatalf("RenderScenePipeline failed: %v", err)
	}

	// Verify output file exists and is not empty
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Failed to stat output file: %v", err)
	}
	if info.Size() < 100 {
		t.Errorf("Output file too small: %d bytes", info.Size())
	}
}
