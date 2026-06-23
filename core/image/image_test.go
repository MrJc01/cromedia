package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"testing"

	"cromedia/core"
)

func createSolidImage(w, h int, col color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, col)
		}
	}
	return img
}

func TestConverters(t *testing.T) {
	// 1. Create a mock solid image
	originalImg := createSolidImage(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	// 2. Convert to VideoFrame
	frame, err := ConvertToVideoFrame(originalImg)
	if err != nil {
		t.Fatalf("failed to convert image to VideoFrame: %v", err)
	}

	if frame.Width != 100 || frame.Height != 100 || frame.Format != "rgba" {
		t.Errorf("unexpected frame attributes: %+v", frame)
	}

	// Verify red pixel values at start
	if frame.Data[0] != 255 || frame.Data[1] != 0 || frame.Data[2] != 0 || frame.Data[3] != 255 {
		t.Errorf("unexpected pixel data in frame: %v", frame.Data[:4])
	}

	// 3. Convert back to Image
	imgBack, err := ConvertToImage(frame)
	if err != nil {
		t.Fatalf("failed to convert VideoFrame back to image: %v", err)
	}

	bounds := imgBack.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("unexpected bounds of converted image: %v", bounds)
	}

	r, g, b, a := imgBack.At(0, 0).RGBA()
	if r != 0xffff || g != 0 || b != 0 || a != 0xffff {
		t.Errorf("unexpected color of converted image: %d, %d, %d, %d", r, g, b, a)
	}
}

func TestFormatsRoundtrip(t *testing.T) {
	img := createSolidImage(32, 32, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	formats := []string{"jpeg", "png", "bmp", "tiff", "webp"}

	for _, format := range formats {
		var buf bytes.Buffer
		err := EncodeImage(&buf, img, format, 80)
		if err != nil {
			t.Errorf("failed to encode format %s: %v", format, err)
			continue
		}

		decoded, decFormat, err := DecodeImage(&buf)
		if err != nil {
			t.Errorf("failed to decode format %s: %v", format, err)
			continue
		}

		// JPEG is lossy, formats like webp/jpeg might rename format slightly
		if format == "jpeg" && decFormat != "jpeg" {
			t.Errorf("expected format jpeg, got %s", decFormat)
		}

		bounds := decoded.Bounds()
		if bounds.Dx() != 32 || bounds.Dy() != 32 {
			t.Errorf("expected bounds 32x32 for format %s, got %dx%d", format, bounds.Dx(), bounds.Dy())
		}
	}
}

func TestSequenceDemuxMux(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cromedia_sequence_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 3 dummy JPEG files in the sequence directory
	img := createSolidImage(64, 64, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	for i := 0; i < 3; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("frame_%04d.jpg", i))
		f, err := os.Create(filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := EncodeImage(f, img, "jpeg", 90); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}

	// 1. Test SequenceDemuxer
	pattern := filepath.Join(tmpDir, "frame_%04d.jpg")
	demuxer, err := NewSequenceDemuxer(pattern, 10.0)
	if err != nil {
		t.Fatalf("failed to create demuxer: %v", err)
	}
	defer demuxer.Close()

	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatalf("failed to probe demuxer: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	track := tracks[0]
	if track.Width != 64 || track.Height != 64 || track.CodecTag != "mjpeg" {
		t.Errorf("unexpected track info: %+v", track)
	}
	if len(track.Samples) != 3 {
		t.Errorf("expected 3 samples, got %d", len(track.Samples))
	}

	// Read packets and check
	for i := 0; i < 3; i++ {
		pkt, err := demuxer.ReadPacket()
		if err != nil {
			t.Fatalf("failed to read packet %d: %v", i, err)
		}
		if pkt.StreamIndex != 0 {
			t.Errorf("expected stream index 0, got %d", pkt.StreamIndex)
		}
		// Expect sample duration for 10fps timescale 90000 is 9000
		expectedPTS := int64(i) * 9000
		if pkt.PTS != expectedPTS {
			t.Errorf("expected PTS %d, got %d", expectedPTS, pkt.PTS)
		}
		if len(pkt.Data) == 0 {
			t.Error("expected packet data to be loaded, got empty slice")
		}
	}

	// Next read should be EOF
	_, err = demuxer.ReadPacket()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}

	// 2. Test SequenceMuxer
	outDir := filepath.Join(tmpDir, "out")
	outPattern := filepath.Join(outDir, "out_frame_%03d.png")
	muxer, err := NewSequenceMuxer(outPattern)
	if err != nil {
		t.Fatalf("failed to create muxer: %v", err)
	}
	defer muxer.Close()

	// Write frames directly
	frame := &core.VideoFrame{
		Width:  64,
		Height: 64,
		Format: "rgba",
		Data:   make([]byte, 64*64*4),
	}

	for i := 0; i < 2; i++ {
		err := muxer.WriteFrame(frame)
		if err != nil {
			t.Fatalf("failed to write frame %d: %v", i, err)
		}
	}

	// Check files are generated
	for i := 0; i < 2; i++ {
		filename := filepath.Join(outDir, fmt.Sprintf("out_frame_%03d.png", i))
		if _, err := os.Stat(filename); err != nil {
			t.Errorf("expected file %s to be created, but got: %v", filename, err)
		}
	}
}

// mock16KImage is a memory-efficient image with 16K bounds.
type mock16KImage struct{}
func (m mock16KImage) ColorModel() color.Model { return color.RGBAModel }
func (m mock16KImage) Bounds() image.Rectangle { return image.Rect(0, 0, 16384, 16384) }
func (m mock16KImage) At(x, y int) color.Color { return color.RGBA{} }

func Test16KImageLimit(t *testing.T) {
	img := mock16KImage{}
	frame, err := ConvertToVideoFrame(img)
	if err != nil {
		t.Fatalf("failed converting 16K mock image: %v", err)
	}
	if frame.Width != 16384 || frame.Height != 16384 {
		t.Errorf("unexpected frame resolution: %dx%d", frame.Width, frame.Height)
	}
}

func TestCorruptedImageFailSafe(t *testing.T) {
	corruptedBytes := []byte("not an image at all, just junk bytes")
	_, _, err := DecodeImage(bytes.NewReader(corruptedBytes))
	if err == nil {
		t.Error("expected error when decoding corrupted image bytes, got nil")
	}

	_, _, err = ParseJPEGMetadata(corruptedBytes)
	if err != nil {
		t.Errorf("expected JPEG metadata parser to return default orientation on corrupted data, got err: %v", err)
	}
}

func TestDescriptorLeak(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cromedia_descriptor_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy PNG file
	img := createSolidImage(10, 10, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	for i := 0; i < 5; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("file_%d.png", i))
		f, err := os.Create(filename)
		if err != nil {
			t.Fatal(err)
		}
		_ = EncodeImage(f, img, "png", 90)
		f.Close()
	}

	pattern := filepath.Join(tmpDir, "file_%d.png")
	demuxer, err := NewSequenceDemuxer(pattern, 25.0)
	if err != nil {
		t.Fatal(err)
	}
	defer demuxer.Close()

	_, err = demuxer.Probe()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		_, err := demuxer.ReadPacket()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnimatedGIFMuxDemux(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cromedia_gif_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Mux animated GIF
	gifPath := filepath.Join(tmpDir, "test.gif")
	muxer, err := NewSequenceMuxer(gifPath)
	if err != nil {
		t.Fatal(err)
	}

	frame := &core.VideoFrame{
		Width:  32,
		Height: 32,
		Format: "rgba",
		Data:   make([]byte, 32*32*4),
	}

	for i := 0; i < 3; i++ {
		err := muxer.WriteFrame(frame)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = muxer.WriteTrailer()
	if err != nil {
		t.Fatal(err)
	}

	// Verify file size is positive
	info, err := os.Stat(gifPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("failed to create valid GIF file: %v", err)
	}

	// 2. Demux animated GIF
	demuxer, err := NewSequenceDemuxer(gifPath, 10.0)
	if err != nil {
		t.Fatal(err)
	}
	defer demuxer.Close()

	tracks, err := demuxer.Probe()
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if track := tracks[0]; len(track.Samples) != 3 {
		t.Errorf("expected 3 samples, got %d", len(track.Samples))
	}

	// Read frames
	for i := 0; i < 3; i++ {
		pkt, err := demuxer.ReadPacket()
		if err != nil {
			t.Fatal(err)
		}
		if len(pkt.Data) == 0 {
			t.Error("expected non-empty frame payload")
		}
	}
}

func TestEXIFRotation(t *testing.T) {
	// Orientation 6: rotate 90 CW
	img := createSolidImage(10, 20, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	rotated := RotateImageByOrientation(img, 6)
	bounds := rotated.Bounds()

	if bounds.Dx() != 20 || bounds.Dy() != 10 {
		t.Errorf("expected swapped dimensions 20x10, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
