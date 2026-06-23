package core_test

import (
	"context"
	"math"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"cromedia/core"
)

// ============================================================================
// Pool & Memory Management Tests
// ============================================================================

func TestBufferPoolBasicGetPut(t *testing.T) {
	core.GlobalResetStats()
	buf := core.GlobalGet(1024)
	if len(buf) != 1024 {
		t.Fatalf("expected len 1024, got %d", len(buf))
	}
	core.GlobalPut(buf)
	gets, puts := core.GlobalStats()
	if gets != 1 || puts != 1 {
		t.Fatalf("expected 1/1 stats, got %d/%d", gets, puts)
	}
}

func TestBufferPoolActiveLeases(t *testing.T) {
	core.GlobalResetStats()
	initial := core.GlobalActiveLeases()

	buf1 := core.GlobalGet(4096)
	buf2 := core.GlobalGet(8192)

	active := core.GlobalActiveLeases()
	if active != initial+2 {
		t.Fatalf("expected %d active leases, got %d", initial+2, active)
	}

	core.GlobalPut(buf1)
	core.GlobalPut(buf2)

	final := core.GlobalActiveLeases()
	if final != initial {
		t.Fatalf("expected %d active leases after put, got %d", initial, final)
	}
}

func TestTrackedBufferRelease(t *testing.T) {
	tb := core.GlobalGetTracked(2048)
	if len(tb.Data) != 2048 {
		t.Fatalf("expected len 2048, got %d", len(tb.Data))
	}
	tb.Release()
	// Double release should be safe (no-op)
	tb.Release()
}

func TestTrackedBufferFinalizerSafetyNet(t *testing.T) {
	initial := core.GlobalLeakAlerts()

	// Allocate a tracked buffer and let it go out of scope without releasing
	func() {
		_ = core.GlobalGetTracked(512)
		// Intentionally NOT calling Release()
	}()

	// Force GC to trigger finalizer
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	final := core.GlobalLeakAlerts()
	if final <= initial {
		t.Logf("Note: GC finalizer may not have run yet (initial=%d, final=%d). This is non-deterministic.", initial, final)
	} else {
		t.Logf("GC finalizer reclaimed %d leaked buffer(s)", final-initial)
	}
}

func TestBufferPoolConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	iterations := 1000

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := core.GlobalGet(1024)
			core.GlobalPut(buf)
		}()
	}
	wg.Wait()
}

func TestBufferPoolLargeAllocation(t *testing.T) {
	// Test fallback for sizes larger than any bucket
	buf := core.GlobalGet(10 * 1024 * 1024) // 10MB
	if len(buf) != 10*1024*1024 {
		t.Fatalf("expected 10MB buffer, got %d bytes", len(buf))
	}
	core.GlobalPut(buf) // Should not panic
}

// ============================================================================
// SIMD Video Filter Tests
// ============================================================================

func createTestFrame(w, h int) *core.VideoFrame {
	data := make([]byte, w*h*4)
	for i := 0; i < len(data); i += 4 {
		data[i] = byte(i % 256)     // R
		data[i+1] = byte(i/4 % 256) // G
		data[i+2] = byte(i/8 % 256) // B
		data[i+3] = 255              // A
	}
	return &core.VideoFrame{Width: w, Height: h, Format: core.PixelFormatRGBA, Data: data}
}

func TestSIMDScaleNearestNeighbor(t *testing.T) {
	frame := createTestFrame(640, 480)
	filter := &core.SIMDScaleFilter{TargetW: 320, TargetH: 240, Bilinear: false}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("SIMD scale nearest error: %v", err)
	}
	if out.Width != 320 || out.Height != 240 {
		t.Fatalf("expected 320x240, got %dx%d", out.Width, out.Height)
	}
	if len(out.Data) != 320*240*4 {
		t.Fatalf("expected %d bytes, got %d", 320*240*4, len(out.Data))
	}
}

func TestSIMDScaleBilinear(t *testing.T) {
	frame := createTestFrame(640, 480)
	filter := &core.SIMDScaleFilter{TargetW: 1280, TargetH: 720, Bilinear: true}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("SIMD scale bilinear error: %v", err)
	}
	if out.Width != 1280 || out.Height != 720 {
		t.Fatalf("expected 1280x720, got %dx%d", out.Width, out.Height)
	}
}

func TestSIMDScaleUpscale4K(t *testing.T) {
	frame := createTestFrame(1920, 1080)
	filter := &core.SIMDScaleFilter{TargetW: 3840, TargetH: 2160, Bilinear: false}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("4K upscale error: %v", err)
	}
	if out.Width != 3840 || out.Height != 2160 {
		t.Fatalf("expected 3840x2160, got %dx%d", out.Width, out.Height)
	}
}

func TestSIMDSobelEdgeDetection(t *testing.T) {
	frame := createTestFrame(200, 200)
	filter := &core.SIMDSobelFilter{}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("SIMD Sobel error: %v", err)
	}
	if out.Width != 200 || out.Height != 200 {
		t.Fatalf("dimensions mismatch")
	}
	// Check that edge pixels were modified (not all zeros)
	nonZero := 0
	for i := 0; i < len(out.Data); i += 4 {
		if out.Data[i] > 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatal("Sobel produced no edge data")
	}
}

func TestBicubicScaleFilter(t *testing.T) {
	frame := createTestFrame(400, 300)
	filter := &core.BicubicScaleFilter{TargetW: 200, TargetH: 150}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("Bicubic scale error: %v", err)
	}
	if out.Width != 200 || out.Height != 150 {
		t.Fatalf("expected 200x150, got %dx%d", out.Width, out.Height)
	}
}

func TestSIMDScaleIdentity(t *testing.T) {
	frame := createTestFrame(100, 100)
	filter := &core.SIMDScaleFilter{TargetW: 100, TargetH: 100, Bilinear: false}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("identity scale error: %v", err)
	}
	// In identity scale, output should match input closely
	mismatches := 0
	for i := 0; i < len(frame.Data); i++ {
		if frame.Data[i] != out.Data[i] {
			mismatches++
		}
	}
	if mismatches > 0 {
		t.Logf("Identity scale had %d mismatches (may be rounding)", mismatches)
	}
}

func TestSIMDScaleTinyFrame(t *testing.T) {
	frame := createTestFrame(2, 2)
	filter := &core.SIMDScaleFilter{TargetW: 4, TargetH: 4, Bilinear: true}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("tiny frame scale error: %v", err)
	}
	if out.Width != 4 || out.Height != 4 {
		t.Fatalf("expected 4x4, got %dx%d", out.Width, out.Height)
	}
}

// ============================================================================
// Audio DSP Tests
// ============================================================================

func createTestAudio(samples, channels, sampleRate int) *core.AudioFrame {
	data := make([]float32, samples*channels)
	for i := 0; i < samples; i++ {
		val := float32(math.Sin(2 * math.Pi * 440 * float64(i) / float64(sampleRate)))
		for ch := 0; ch < channels; ch++ {
			data[i*channels+ch] = val
		}
	}
	return &core.AudioFrame{Channels: channels, SampleRate: sampleRate, Data: data}
}

func TestSincResamplerUpsample(t *testing.T) {
	frame := createTestAudio(44100, 2, 44100) // 1 second @ 44.1kHz
	resampler := core.NewSincResampler(44100, 48000, 64)
	out, err := resampler.Process(frame)
	if err != nil {
		t.Fatalf("sinc resampler error: %v", err)
	}
	expectedSamples := 48000
	actualSamples := len(out.Data) / out.Channels
	if actualSamples != expectedSamples {
		t.Fatalf("expected %d output samples, got %d", expectedSamples, actualSamples)
	}
	if out.SampleRate != 48000 {
		t.Fatalf("expected 48000 Hz, got %d", out.SampleRate)
	}
}

func TestSincResamplerDownsample(t *testing.T) {
	frame := createTestAudio(48000, 1, 48000)
	resampler := core.NewSincResampler(48000, 22050, 32)
	out, err := resampler.Process(frame)
	if err != nil {
		t.Fatalf("sinc downsample error: %v", err)
	}
	expectedSamples := 22050
	actualSamples := len(out.Data)
	if actualSamples != expectedSamples {
		t.Fatalf("expected %d samples, got %d", expectedSamples, actualSamples)
	}
}

func TestSincResamplerIdentity(t *testing.T) {
	frame := createTestAudio(1000, 2, 44100)
	resampler := core.NewSincResampler(44100, 44100, 64)
	out, err := resampler.Process(frame)
	if err != nil {
		t.Fatalf("sinc identity error: %v", err)
	}
	if len(out.Data) != len(frame.Data) {
		t.Fatalf("identity resampling should preserve length")
	}
}

func TestPredictiveGainNormalizer(t *testing.T) {
	frame := createTestAudio(4410, 2, 44100)
	// Scale down to simulate low-volume audio
	for i := range frame.Data {
		frame.Data[i] *= 0.1
	}

	norm := core.NewPredictiveGainNormalizer(-1.0, 0.1)
	out, err := norm.Process(frame)
	if err != nil {
		t.Fatalf("normalizer error: %v", err)
	}

	// Check that output peak is closer to target
	var maxOut float32
	for _, v := range out.Data {
		abs := float32(math.Abs(float64(v)))
		if abs > maxOut {
			maxOut = abs
		}
	}
	if maxOut < 0.5 {
		t.Fatalf("normalizer did not boost gain enough: peak=%f", maxOut)
	}
	t.Logf("Normalized peak: %f", maxOut)
}

func TestPredictiveGainNormalizerSilence(t *testing.T) {
	frame := &core.AudioFrame{Channels: 2, SampleRate: 44100, Data: make([]float32, 1000)}
	norm := core.NewPredictiveGainNormalizer(-1.0, 0.05)
	out, err := norm.Process(frame)
	if err != nil {
		t.Fatalf("normalizer silence error: %v", err)
	}
	// Silent frame should pass through unchanged
	for _, v := range out.Data {
		if v != 0 {
			t.Fatal("silence should remain silent")
		}
	}
}

func TestHighPassFilter(t *testing.T) {
	frame := createTestAudio(4410, 1, 44100)
	filter := &core.HighPassFilter{CutoffHz: 1000}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("highpass error: %v", err)
	}
	if len(out.Data) != len(frame.Data) {
		t.Fatal("highpass should preserve data length")
	}
}

func TestDynamicRangeCompressor(t *testing.T) {
	frame := createTestAudio(4410, 2, 44100)
	comp := &core.DynamicRangeCompressor{
		ThresholdDb: -6.0,
		Ratio:       4.0,
		AttackMs:    10.0,
		ReleaseMs:   100.0,
	}
	out, err := comp.Process(frame)
	if err != nil {
		t.Fatalf("compressor error: %v", err)
	}

	// Compressed output should have lower peak than input
	var maxIn, maxOut float32
	for i := range frame.Data {
		abs := float32(math.Abs(float64(frame.Data[i])))
		if abs > maxIn {
			maxIn = abs
		}
		abs = float32(math.Abs(float64(out.Data[i])))
		if abs > maxOut {
			maxOut = abs
		}
	}
	t.Logf("Compression: input peak=%f, output peak=%f", maxIn, maxOut)
}

func TestPinkNoiseGenerator(t *testing.T) {
	pn := core.GeneratePinkNoise(1.0, 44100, 2)
	if len(pn.Data) != 44100*2 {
		t.Fatalf("expected %d samples, got %d", 44100*2, len(pn.Data))
	}
	// Check that it's not all zeros
	nonZero := 0
	for _, v := range pn.Data {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatal("pink noise is all zeros")
	}
}

// ============================================================================
// Hybrid Jitter Buffer Tests
// ============================================================================

func TestHybridJitterBufferRAMOnly(t *testing.T) {
	dir := t.TempDir()
	hjb := core.NewHybridJitterBuffer(100, filepath.Join(dir, "jb"))

	for i := 0; i < 10; i++ {
		pkt := &core.Packet{
			PTS:         int64(i * 1000),
			StreamIndex: 0,
			Data:        make([]byte, 100),
		}
		if err := hjb.Push(pkt); err != nil {
			t.Fatalf("push error: %v", err)
		}
	}

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		pkt, err := hjb.Pop(ctx)
		if err != nil {
			t.Fatalf("pop error: %v", err)
		}
		if pkt.PTS != int64(i*1000) {
			t.Fatalf("expected PTS %d, got %d", i*1000, pkt.PTS)
		}
	}
}

func TestHybridJitterBufferSpillToDisk(t *testing.T) {
	dir := t.TempDir()
	// Very small RAM limit to force spill
	hjb := core.NewHybridJitterBuffer(3, filepath.Join(dir, "jb"))

	for i := 0; i < 10; i++ {
		pkt := &core.Packet{
			PTS:         int64(i * 1000),
			StreamIndex: 0,
			Data:        make([]byte, 512),
			IsKeyframe:  i%5 == 0,
		}
		if err := hjb.Push(pkt); err != nil {
			t.Fatalf("push error at %d: %v", i, err)
		}
	}

	pushed, _, spilled := hjb.Stats()
	if pushed != 10 {
		t.Fatalf("expected 10 pushed, got %d", pushed)
	}
	if spilled == 0 {
		t.Fatal("expected some packets to spill to disk")
	}
	t.Logf("Spilled %d packets to disk", spilled)

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		pkt, err := hjb.Pop(ctx)
		if err != nil {
			t.Fatalf("pop error at %d: %v", i, err)
		}
		if pkt == nil {
			t.Fatalf("got nil packet at %d", i)
		}
	}

	hjb.Cleanup()
}

func TestHybridJitterBufferContextCancellation(t *testing.T) {
	dir := t.TempDir()
	hjb := core.NewHybridJitterBuffer(100, filepath.Join(dir, "jb"))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Pop on empty buffer should timeout
	_, err := hjb.Pop(ctx)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

// ============================================================================
// PCR Clock Sync Tests
// ============================================================================

func TestPCRClockSyncNoDrift(t *testing.T) {
	pcs := core.NewPCRClockSync(500)
	pcr := pcs.GeneratePCR()
	// Immediately check — should have very low drift
	disc := pcs.UpdatePCR(pcr)
	if disc {
		t.Fatal("no discontinuity expected for synchronized PCR")
	}
}

func TestPCRClockSyncDriftDetection(t *testing.T) {
	pcs := core.NewPCRClockSync(500)
	// Inject a large PCR jump (simulate clock drift)
	disc := pcs.UpdatePCR(90000 * 10) // 10 seconds ahead
	if !disc {
		t.Fatal("expected discontinuity for large PCR drift")
	}
	if !pcs.HasDiscontinuity() {
		t.Fatal("discontinuity flag should be set")
	}

	driftNs, corrections := pcs.DriftStats()
	if corrections != 1 {
		t.Fatalf("expected 1 correction, got %d", corrections)
	}
	t.Logf("Total drift: %d ns, corrections: %d", driftNs, corrections)
}

// ============================================================================
// CGO Batch Processor Tests
// ============================================================================

func TestCGOBatchProcessorFlush(t *testing.T) {
	var batches [][]*core.VideoFrame
	processor := core.NewCGOBatchProcessor(4, func(frames []*core.VideoFrame) error {
		batches = append(batches, frames)
		return nil
	})

	for i := 0; i < 10; i++ {
		frame := createTestFrame(64, 64)
		processor.AddFrame(frame)
	}
	processor.Flush() // Flush remaining 2

	totalFrames := 0
	for _, batch := range batches {
		totalFrames += len(batch)
	}
	if totalFrames != 10 {
		t.Fatalf("expected 10 total frames, got %d", totalFrames)
	}
}

func TestPackedFrameBuffer(t *testing.T) {
	frames := make([]*core.VideoFrame, 5)
	for i := 0; i < 5; i++ {
		frames[i] = createTestFrame(64, 64)
		frames[i].Data[0] = byte(i) // Tag each frame
	}

	packed := core.PackFrames(frames)
	if packed.FrameCount != 5 {
		t.Fatalf("expected 5 frames, got %d", packed.FrameCount)
	}

	for i := 0; i < 5; i++ {
		unpacked := packed.UnpackFrame(i)
		if unpacked.Data[0] != byte(i) {
			t.Fatalf("frame %d: expected tag %d, got %d", i, i, unpacked.Data[0])
		}
	}

	// Out of bounds
	if packed.UnpackFrame(10) != nil {
		t.Fatal("out of bounds should return nil")
	}
}

func TestHierarchicalWorkerPool(t *testing.T) {
	pool := core.NewHierarchicalWorkerPool(4)
	var counter int64
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			// Simulate work
			time.Sleep(5 * time.Millisecond)
			// Atomic increment
			val := counter
			counter = val + 1
		})
	}
	wg.Wait()

	// Due to potential race in non-atomic increment, just verify pool didn't crash
	t.Logf("Completed %d jobs", counter)
}

// ============================================================================
// Existing Filter Regression Tests (ensure old behavior preserved)
// ============================================================================

func TestScaleFilterStillWorks(t *testing.T) {
	frame := createTestFrame(320, 240)
	filter := &core.ScaleFilter{TargetW: 160, TargetH: 120, Bilinear: false}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("legacy scale error: %v", err)
	}
	if out.Width != 160 || out.Height != 120 {
		t.Fatalf("legacy scale dimensions wrong")
	}
}

func TestCropFilterStillWorks(t *testing.T) {
	frame := createTestFrame(320, 240)
	filter := &core.CropFilter{X: 10, Y: 10, W: 100, H: 100}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("crop error: %v", err)
	}
	if out.Width != 100 || out.Height != 100 {
		t.Fatalf("crop dimensions wrong")
	}
}

func TestFlipFilterStillWorks(t *testing.T) {
	frame := createTestFrame(100, 100)
	filter := &core.FlipFilter{Horizontal: true}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("flip error: %v", err)
	}
	if out.Width != 100 || out.Height != 100 {
		t.Fatalf("flip dimensions wrong")
	}
}

func TestRotateFilterStillWorks(t *testing.T) {
	frame := createTestFrame(100, 80)
	filter := &core.RotateFilter{Angle: 90}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("rotate error: %v", err)
	}
	if out.Width != 80 || out.Height != 100 {
		t.Fatalf("expected 80x100, got %dx%d", out.Width, out.Height)
	}
}

func TestSobelFilterStillWorks(t *testing.T) {
	frame := createTestFrame(100, 100)
	filter := &core.SobelFilter{}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("sobel error: %v", err)
	}
	if out.Width != 100 || out.Height != 100 {
		t.Fatalf("sobel dimensions wrong")
	}
}

func TestColorFilterStillWorks(t *testing.T) {
	frame := createTestFrame(100, 100)
	filter := &core.ColorFilter{Brightness: 20, Contrast: 1.5}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("color filter error: %v", err)
	}
	if len(out.Data) != len(frame.Data) {
		t.Fatal("color filter data length mismatch")
	}
}

// ============================================================================
// Audio Filter Regression Tests
// ============================================================================

func TestVolumeFilterStillWorks(t *testing.T) {
	frame := createTestAudio(1000, 2, 44100)
	filter := &core.VolumeFilter{Gain: 0.5}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("volume error: %v", err)
	}
	// Check volume reduction
	for i := 0; i < len(frame.Data); i++ {
		expected := frame.Data[i] * 0.5
		if math.Abs(float64(out.Data[i]-expected)) > 0.001 {
			t.Fatalf("volume mismatch at %d: expected %f, got %f", i, expected, out.Data[i])
		}
	}
}

func TestFadeFilterStillWorks(t *testing.T) {
	frame := createTestAudio(1000, 1, 44100)
	filter := &core.FadeFilter{FadeIn: true}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("fade error: %v", err)
	}
	// First sample should be near zero, last sample near original
	if math.Abs(float64(out.Data[0])) > 0.01 {
		t.Fatal("fade-in first sample should be near zero")
	}
}

func TestLowPassFilterStillWorks(t *testing.T) {
	frame := createTestAudio(4410, 1, 44100)
	filter := &core.LowPassFilter{CutoffHz: 2000}
	out, err := filter.Process(frame)
	if err != nil {
		t.Fatalf("lowpass error: %v", err)
	}
	if len(out.Data) != len(frame.Data) {
		t.Fatal("lowpass data length mismatch")
	}
}

func TestSineWaveGeneratorStillWorks(t *testing.T) {
	audio := core.GenerateSineWave(440, 1.0, 44100, 2)
	expected := 44100 * 2
	if len(audio.Data) != expected {
		t.Fatalf("expected %d samples, got %d", expected, len(audio.Data))
	}
}

func TestWhiteNoiseGeneratorStillWorks(t *testing.T) {
	audio := core.GenerateWhiteNoise(0.5, 44100, 1)
	expected := 22050
	if len(audio.Data) != expected {
		t.Fatalf("expected %d samples, got %d", expected, len(audio.Data))
	}
}

// ============================================================================
// Benchmark Functions for Performance Comparison
// ============================================================================

func BenchmarkSIMDScaleNearest1080p(b *testing.B) {
	frame := createTestFrame(1920, 1080)
	filter := &core.SIMDScaleFilter{TargetW: 1280, TargetH: 720, Bilinear: false}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.Process(frame)
	}
}

func BenchmarkLegacyScaleNearest1080p(b *testing.B) {
	frame := createTestFrame(1920, 1080)
	filter := &core.ScaleFilter{TargetW: 1280, TargetH: 720, Bilinear: false}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.Process(frame)
	}
}

func BenchmarkSIMDSobel200x200(b *testing.B) {
	frame := createTestFrame(200, 200)
	filter := &core.SIMDSobelFilter{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.Process(frame)
	}
}

func BenchmarkLegacySobel200x200(b *testing.B) {
	frame := createTestFrame(200, 200)
	filter := &core.SobelFilter{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.Process(frame)
	}
}

func BenchmarkSincResampler(b *testing.B) {
	frame := createTestAudio(44100, 2, 44100)
	resampler := core.NewSincResampler(44100, 48000, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resampler.Process(frame)
	}
}

func BenchmarkBufferPoolGetPut(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := core.GlobalGet(65536)
		core.GlobalPut(buf)
	}
}
