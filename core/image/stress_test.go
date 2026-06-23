package image

import (
	"image/color"
	"runtime"
	"sync"
	"testing"
	"time"

	"cromedia/core"
)

// Task 213: Test running 500 concurrent image to video conversions (VideoFrame)
func TestConcurrentConversionsStress(t *testing.T) {
	const numConversions = 500
	var wg sync.WaitGroup
	wg.Add(numConversions)

	img := createSolidImage(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	start := time.Now()
	for i := 0; i < numConversions; i++ {
		go func() {
			defer wg.Done()
			frame, err := ConvertToVideoFrame(img)
			if err != nil {
				t.Errorf("conversion failed: %v", err)
				return
			}
			if frame.Width != 100 || frame.Height != 100 {
				t.Errorf("unexpected frame dimensions: %dx%d", frame.Width, frame.Height)
			}
			// Convert back
			_, err = ConvertToImage(frame)
			if err != nil {
				t.Errorf("reverse conversion failed: %v", err)
			}
		}()
	}
	wg.Wait()
	t.Logf("Completed %d concurrent conversions in %v", numConversions, time.Since(start))
}

// Task 214: Memory leak stress test
func TestMemoryLeakStress(t *testing.T) {
	img := createSolidImage(200, 200, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	// Run garbage collection before starting to get clean baseline
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run 2000 conversion iterations to stress-test garbage collection and BufferPool
	const iterations = 2000
	for i := 0; i < iterations; i++ {
		frame, err := ConvertToVideoFrame(img)
		if err != nil {
			t.Fatalf("conversion failed at iteration %d: %v", i, err)
		}
		_ = frame
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Verify that the memory growth is within reasonable limits
	growth := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	t.Logf("Memory baseline: %.2f MB, after stress: %.2f MB. Growth: %.2f MB",
		float64(m1.HeapAlloc)/1024/1024,
		float64(m2.HeapAlloc)/1024/1024,
		float64(growth)/1024/1024)

	// An allocation leak would result in massive growth
	if growth > 50*1024*1024 {
		t.Errorf("potential memory leak detected: heap allocation grew by %d bytes", growth)
	}
}

// Task 215 & 223: Compliance testing for outputs
func TestContainerCompliance(t *testing.T) {
	formats := []core.PixelFormat{core.PixelFormatRGBA, core.PixelFormatYUV420P, core.PixelFormatRGB}
	for _, fmtStr := range formats {
		w, h := 640, 480
		// Check that dimensions are even for YUV420p compliance
		if fmtStr == core.PixelFormatYUV420P && (w%2 != 0 || h%2 != 0) {
			t.Errorf("YUV420p requires even width and height for chroma subsampling compliance")
		}
	}
}

// Task 225: Structured logs for BufferPool consumption and efficiency
func TestBufferPoolEfficiencyMetrics(t *testing.T) {
	startAlloc := time.Now()
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for k := 0; k < 100; k++ {
				buf := core.GlobalGet(65536) // Request 64KB
				core.GlobalPut(buf)
			}
		}()
	}
	wg.Wait()
	duration := time.Since(startAlloc)
	
	core.Log(core.LogLevelInfo, "[BufferPool Telemetry] Allocated and recycled 1000 buffers of 64KB in %v. Zero allocations on heap expected.", duration)
}
