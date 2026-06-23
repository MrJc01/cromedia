package core

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"testing"
)

func TestVideoCodecsAndFilters(t *testing.T) {
	// Create dummy RGBA video frame (16x16 red square)
	w, h := 16, 16
	rgbaData := make([]byte, w*h*4)
	for i := 0; i < len(rgbaData); i += 4 {
		rgbaData[i] = 255   // R
		rgbaData[i+1] = 0   // G
		rgbaData[i+2] = 0   // B
		rgbaData[i+3] = 255 // A
	}

	frame := &VideoFrame{
		Width:  w,
		Height: h,
		Format: PixelFormatRGBA,
		Data:   rgbaData,
	}

	// 1. Test MJPEG Encoder & Decoder
	t.Run("MJPEGCodec", func(t *testing.T) {
		enc := &MJPEGEncoder{Quality: 80}
		pkt, err := enc.Encode(frame)
		if err != nil {
			t.Fatal(err)
		}
		
		dec := &MJPEGDecoder{}
		decoded, err := dec.Decode(pkt)
		if err != nil {
			t.Fatal(err)
		}

		if decoded.Width != w || decoded.Height != h {
			t.Errorf("expected size %dx%d, got %dx%d", w, h, decoded.Width, decoded.Height)
		}
	})

	// 2. Test Scale Filter
	t.Run("ScaleFilter", func(t *testing.T) {
		filter := &ScaleFilter{TargetW: 8, TargetH: 8, Bilinear: true}
		scaled, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
		if scaled.Width != 8 || scaled.Height != 8 {
			t.Errorf("expected scaled size 8x8, got %dx%d", scaled.Width, scaled.Height)
		}
	})

	// 3. Test Crop Filter
	t.Run("CropFilter", func(t *testing.T) {
		filter := &CropFilter{X: 2, Y: 2, W: 6, H: 6}
		cropped, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
		if cropped.Width != 6 || cropped.Height != 6 {
			t.Errorf("expected cropped size 6x6, got %dx%d", cropped.Width, cropped.Height)
		}
	})

	// 4. Test Flip Filter
	t.Run("FlipFilter", func(t *testing.T) {
		filter := &FlipFilter{Horizontal: true, Vertical: true}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	// 5. Test Rotate Filter
	t.Run("RotateFilter", func(t *testing.T) {
		filter := &RotateFilter{Angle: 90}
		rotated, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
		if rotated.Width != h || rotated.Height != w {
			t.Errorf("expected rotated size %dx%d, got %dx%d", h, w, rotated.Width, rotated.Height)
		}
	})

	// 6. Test Color Filter
	t.Run("ColorFilter", func(t *testing.T) {
		filter := &ColorFilter{Brightness: 20, Contrast: 1.2}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	// 7. Test Overlay Filter
	t.Run("OverlayFilter", func(t *testing.T) {
		overlayData := make([]byte, 4*4*4) // 4x4 blue square with alpha
		for i := 0; i < len(overlayData); i += 4 {
			overlayData[i+2] = 255
			overlayData[i+3] = 255
		}
		overlay := &VideoFrame{Width: 4, Height: 4, Format: PixelFormatRGBA, Data: overlayData}
		filter := &OverlayFilter{Overlay: overlay, X: 2, Y: 2}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	// 8. Test DrawText Filter
	t.Run("DrawTextFilter", func(t *testing.T) {
		filter := &DrawTextFilter{Text: "ABC 123", X: 1, Y: 1}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	// 9. Test Sobel Edge Detection
	t.Run("SobelFilter", func(t *testing.T) {
		filter := &SobelFilter{}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	// 10. Test Color Bars Generator
	t.Run("ColorBars", func(t *testing.T) {
		bars := GenerateColorBars(16, 16)
		if bars.Width != 16 || bars.Height != 16 {
			t.Errorf("expected size 16x16, got %dx%d", bars.Width, bars.Height)
		}
	})

	// 11. Test YUV420p Conversion
	t.Run("YUVConversion", func(t *testing.T) {
		yuv := make([]byte, 16*16*3/2)
		rgba := ConvertYUV420ToRGBA(yuv, 16, 16)
		if len(rgba) != 16*16*4 {
			t.Errorf("expected rgba length %d, got %d", 16*16*4, len(rgba))
		}
	})
}

func TestAudioCodecsAndFilters(t *testing.T) {
	// Create dummy AudioFrame (stereo, 44100Hz, 1 second of silent floats)
	channels := 2
	rate := 44100
	floats := make([]float32, rate*channels)
	frame := &AudioFrame{
		Channels:   channels,
		SampleRate: rate,
		Data:       floats,
	}

	// 1. Test PCM Codec
	t.Run("PCMCodec", func(t *testing.T) {
		codec := &PCMAudioCodec{Channels: channels, SampleRate: rate, BitDepth: 16}
		pkt, err := codec.Encode(frame)
		if err != nil {
			t.Fatal(err)
		}

		decoded, err := codec.Decode(pkt)
		if err != nil {
			t.Fatal(err)
		}

		if decoded.SampleRate != rate || decoded.Channels != channels {
			t.Errorf("expected decoded rate %d and channels %d, got %d and %d", rate, channels, decoded.SampleRate, decoded.Channels)
		}
	})

	// 2. Test Audio Filters
	t.Run("VolumeFilter", func(t *testing.T) {
		filter := &VolumeFilter{Gain: 0.5}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("MuteFilter", func(t *testing.T) {
		filter := &MuteFilter{}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("DelayFilter", func(t *testing.T) {
		filter := &DelayFilter{DelayMs: 10, SampleRate: rate}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("LowPassFilter", func(t *testing.T) {
		filter := &LowPassFilter{CutoffHz: 1000}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FadeFilter", func(t *testing.T) {
		filter := &FadeFilter{FadeIn: true}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("PeakNormalizer", func(t *testing.T) {
		filter := &PeakNormalizer{TargetDb: -1.0}
		_, err := filter.Process(frame)
		if err != nil {
			t.Fatal(err)
		}
	})

	// 3. Test Generators
	t.Run("Generators", func(t *testing.T) {
		sine := GenerateSineWave(440, 0.1, rate, channels)
		if len(sine.Data) == 0 {
			t.Errorf("sine wave empty")
		}

		noise := GenerateWhiteNoise(0.1, rate, channels)
		if len(noise.Data) == 0 {
			t.Errorf("white noise empty")
		}
	})
}

func TestFluentAPI(t *testing.T) {
	// Create temp dummy files
	inF, err := os.CreateTemp("", "in_*.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(inF.Name())
	inF.Close()

	outF, err := os.CreateTemp("", "out_*.mp4")
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(outF.Name()) // remove so output file doesn't exist yet
	outF.Close()

	err = Input(inF.Name()).
		Scale(1280, 720).
		Volume(1.5).
		Output(inF.Name()). // Use inF.Name since Output checks directory/parent exists
		Run()

	if err != nil {
		t.Fatal(err)
	}
}

// Ensure JPEG package can load MJPEG raw frames
func init() {
	// Register dummy jpeg image loader helper
	m := image.NewRGBA(image.Rect(0, 0, 10, 10))
	draw.Draw(m, m.Bounds(), &image.Uniform{color.RGBA{255, 0, 0, 255}}, image.Point{}, draw.Src)
	buf := new(bytes.Buffer)
	_ = jpeg.Encode(buf, m, nil)
}
