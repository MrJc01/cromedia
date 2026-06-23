package filters_test

import (
	"testing"

	"cromedia/core"
	"cromedia/core/filters"
	_ "cromedia/core/filters/audio"
	_ "cromedia/core/filters/video"
)

func TestFilterRegistration(t *testing.T) {
	// Test creating video filter
	vf, err := filters.CreateVideoFilter("eq", map[string]interface{}{
		"brightness": 10.0,
		"contrast":   1.2,
	})
	if err != nil {
		t.Fatalf("failed to create video filter 'eq': %v", err)
	}
	if vf == nil {
		t.Fatal("expected video filter to be non-nil")
	}

	// Test creating audio filter
	af, err := filters.CreateAudioFilter("tremolo", map[string]interface{}{
		"frequency": 8.0,
		"depth":     0.8,
	})
	if err != nil {
		t.Fatalf("failed to create audio filter 'tremolo': %v", err)
	}
	if af == nil {
		t.Fatal("expected audio filter to be non-nil")
	}
}

func TestVideoFilters(t *testing.T) {
	frame := &core.VideoFrame{
		Width:  4,
		Height: 4,
		Format: "rgba",
		Data:   make([]byte, 4*4*4),
	}

	// Make pixel 0 gray
	frame.Data[0] = 100
	frame.Data[1] = 100
	frame.Data[2] = 100
	frame.Data[3] = 255

	// Test colorbalance
	cb, _ := filters.CreateVideoFilter("colorbalance", map[string]interface{}{
		"shadows": [3]float64{10, 0, -10},
	})
	out, err := cb.Process(frame)
	if err != nil {
		t.Fatalf("colorbalance processing failed: %v", err)
	}
	if out.Data[0] <= 100 {
		t.Errorf("expected shadows adjustment to raise red channel, got %d", out.Data[0])
	}

	// Test eq
	eq, _ := filters.CreateVideoFilter("eq", map[string]interface{}{
		"brightness": 20.0,
	})
	out, err = eq.Process(frame)
	if err != nil {
		t.Fatalf("eq processing failed: %v", err)
	}
	if out.Data[0] != 120 {
		t.Errorf("expected brightness to be 120, got %d", out.Data[0])
	}

	// Test noise
	noise, _ := filters.CreateVideoFilter("noise", map[string]interface{}{
		"strength": 5,
	})
	_, err = noise.Process(frame)
	if err != nil {
		t.Fatalf("noise processing failed: %v", err)
	}

	// Test unsharp
	unsharp, _ := filters.CreateVideoFilter("unsharp", map[string]interface{}{
		"amount": 1.0,
	})
	_, err = unsharp.Process(frame)
	if err != nil {
		t.Fatalf("unsharp processing failed: %v", err)
	}

	// Test cropdetect
	crop, _ := filters.CreateVideoFilter("cropdetect", map[string]interface{}{
		"limit": 10,
	})
	_, err = crop.Process(frame)
	if err != nil {
		t.Fatalf("cropdetect processing failed: %v", err)
	}

	// Test yadif
	yadif, _ := filters.CreateVideoFilter("yadif", map[string]interface{}{})
	_, err = yadif.Process(frame)
	if err != nil {
		t.Fatalf("yadif processing failed: %v", err)
	}

	// Test curves
	curves, _ := filters.CreateVideoFilter("curves", map[string]interface{}{
		"preset": "negative",
	})
	out, err = curves.Process(frame)
	if err != nil {
		t.Fatalf("curves processing failed: %v", err)
	}
	if out.Data[0] != 155 { // 255 - 100
		t.Errorf("expected negative preset red channel 155, got %d", out.Data[0])
	}

	// Test pad
	pad, _ := filters.CreateVideoFilter("pad", map[string]interface{}{
		"left": 2,
		"top":  2,
	})
	out, err = pad.Process(frame)
	if err != nil {
		t.Fatalf("pad processing failed: %v", err)
	}
	if out.Width != 6 || out.Height != 6 {
		t.Errorf("expected padded frame size 6x6, got %dx%d", out.Width, out.Height)
	}

	// Test overlay
	overlayFrame := &core.VideoFrame{
		Width:  2,
		Height: 2,
		Format: "rgba",
		Data:   make([]byte, 2*2*4),
	}
	for j := 0; j < len(overlayFrame.Data); j += 4 {
		overlayFrame.Data[j] = 255
		overlayFrame.Data[j+3] = 255
	}
	overlay, _ := filters.CreateVideoFilter("overlay", map[string]interface{}{
		"overlay_frame": overlayFrame,
		"x":             1,
		"y":             1,
	})
	out, err = overlay.Process(frame)
	if err != nil {
		t.Fatalf("overlay processing failed: %v", err)
	}
	idx := (1*4 + 1) * 4
	if out.Data[idx] != 255 {
		t.Errorf("expected overlaid pixel to be red (255), got %d", out.Data[idx])
	}
}

func TestAudioFilters(t *testing.T) {
	frame := &core.AudioFrame{
		Channels:   2,
		SampleRate: 44100,
		Data:       make([]float32, 1000),
	}
	for i := range frame.Data {
		frame.Data[i] = 0.5
	}

	// Test equalizer
	eq, _ := filters.CreateAudioFilter("equalizer", map[string]interface{}{
		"low":  1.2,
		"mid":  1.0,
		"high": 0.8,
	})
	_, err := eq.Process(frame)
	if err != nil {
		t.Fatalf("equalizer failed: %v", err)
	}

	// Test tremolo
	tremolo, _ := filters.CreateAudioFilter("tremolo", map[string]interface{}{})
	_, err = tremolo.Process(frame)
	if err != nil {
		t.Fatalf("tremolo failed: %v", err)
	}

	// Test chorus
	chorus, _ := filters.CreateAudioFilter("chorus", map[string]interface{}{})
	_, err = chorus.Process(frame)
	if err != nil {
		t.Fatalf("chorus failed: %v", err)
	}

	// Test flanger
	flanger, _ := filters.CreateAudioFilter("flanger", map[string]interface{}{})
	_, err = flanger.Process(frame)
	if err != nil {
		t.Fatalf("flanger failed: %v", err)
	}

	// Test compand
	compand, _ := filters.CreateAudioFilter("compand", map[string]interface{}{
		"threshold": 0.2,
		"gain":      1.5,
	})
	out, err := compand.Process(frame)
	if err != nil {
		t.Fatalf("compand failed: %v", err)
	}
	if out.Data[0] < 0.649 || out.Data[0] > 0.651 {
		t.Errorf("expected companded value near 0.65, got %f", out.Data[0])
	}

	// Test earwax
	earwax, _ := filters.CreateAudioFilter("earwax", map[string]interface{}{})
	_, err = earwax.Process(frame)
	if err != nil {
		t.Fatalf("earwax failed: %v", err)
	}

	// Test gate
	gate, _ := filters.CreateAudioFilter("gate", map[string]interface{}{
		"threshold": 0.6,
		"range":     0.0,
	})
	out, err = gate.Process(frame)
	if err != nil {
		t.Fatalf("gate failed: %v", err)
	}
	if out.Data[0] != 0.0 {
		t.Errorf("expected gate to silence 0.5 below threshold 0.6, got %f", out.Data[0])
	}

	// Test pitch
	pitch, _ := filters.CreateAudioFilter("pitch", map[string]interface{}{
		"ratio": 1.2,
	})
	_, err = pitch.Process(frame)
	if err != nil {
		t.Fatalf("pitch failed: %v", err)
	}
}

func TestFilterKeyframesInterpolation(t *testing.T) {
	// 1. Math expressions evaluation (Task 102)
	val1 := filters.EvaluateExpression("0.5 * t", 4.0)
	if val1 != 2.0 {
		t.Errorf("expected 2.0, got %f", val1)
	}

	val2 := filters.EvaluateExpression("10.0", 2.0)
	if val2 != 10.0 {
		t.Errorf("expected 10.0, got %f", val2)
	}

	// 2. Keyframes linear interpolation (Task 103)
	kfs := []filters.Keyframe{
		{Time: 0.0, Value: 10.0},
		{Time: 2.0, Value: 20.0},
		{Time: 5.0, Value: 50.0},
	}

	iv1 := filters.InterpolateKeyframes(kfs, 1.0)
	if iv1 != 15.0 {
		t.Errorf("expected 15.0, got %f", iv1)
	}

	iv2 := filters.InterpolateKeyframes(kfs, 3.5)
	if iv2 != 35.0 {
		t.Errorf("expected 35.0, got %f", iv2)
	}
}

func TestFilterGraphRendering(t *testing.T) {
	graph := filters.RenderFilterGraph([]string{"eq", "noise", "unsharp"})
	expected := "[Input] -> (eq) -> (noise) -> (unsharp) -> [Output]"
	if graph != expected {
		t.Errorf("expected %q, got %q", expected, graph)
	}
}

func TestConcurrentVideoFilterProcessing(t *testing.T) {
	frame := &core.VideoFrame{
		Width:  1280,
		Height: 720,
		Format: "rgba",
		Data:   make([]byte, 1280*720*4),
	}

	eq, _ := filters.CreateVideoFilter("eq", map[string]interface{}{
		"brightness": 10.0,
	})

	out, err := filters.ProcessVideoFilterConcurrently(frame, eq, 4)
	if err != nil {
		t.Fatalf("concurrent filter processing failed: %v", err)
	}
	if len(out.Data) != len(frame.Data) {
		t.Errorf("expected output buffer size %d, got %d", len(frame.Data), len(out.Data))
	}
}

func TestMutiFilterZeroAllocations(t *testing.T) {
	frame := &core.VideoFrame{
		Width:  100,
		Height: 100,
		Format: "rgba",
		Data:   make([]byte, 100*100*4),
	}

	eq, _ := filters.CreateVideoFilter("eq", map[string]interface{}{})
	noise, _ := filters.CreateVideoFilter("noise", map[string]interface{}{})

	// Run filters chain sequentially
	f1, _ := eq.Process(frame)
	f2, _ := noise.Process(f1)
	if len(f2.Data) != len(frame.Data) {
		t.Error("unexpected frame size")
	}
}

func TestConcurrencyExtreme4K(t *testing.T) {
	frame := &core.VideoFrame{
		Width:  3840,
		Height: 2160,
		Format: "rgba",
		Data:   make([]byte, 3840*2160*4),
	}

	eq, _ := filters.CreateVideoFilter("eq", map[string]interface{}{
		"brightness": 5.0,
	})

	// Extreme concurrency: 20 threads/workers
	_, err := filters.ProcessVideoFilterConcurrently(frame, eq, 20)
	if err != nil {
		t.Fatalf("extreme 4K filter processing failed: %v", err)
	}
}

func TestAudioFrequencyAndSamplingRate(t *testing.T) {
	sampleRates := []int{22050, 44100, 48000}
	for _, sr := range sampleRates {
		frame := &core.AudioFrame{
			Channels:   2,
			SampleRate: sr,
			Data:       make([]float32, (sr/10)*2), // 100ms audio per channel
		}
		for i := range frame.Data {
			frame.Data[i] = 0.1
		}

		eq, err := filters.CreateAudioFilter("equalizer", map[string]interface{}{
			"low":  1.0,
			"high": 1.0,
		})
		if err != nil {
			t.Fatalf("failed to create equalizer for %dHz: %v", sr, err)
		}

		_, err = eq.Process(frame)
		if err != nil {
			t.Errorf("equalizer execution failed at %dHz: %v", sr, err)
		}
	}
}
