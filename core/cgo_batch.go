package core

import (
	"sync"
)

// CGOBatchProcessor groups video frames into batches before sending to C-based
// encoders (x264, x265, etc.) to reduce the per-frame CGO context switching overhead.
// Addresses expert criticism: "Processamento em Batch (CGO Buffer Packing)" —
// groups frames into chunks to minimize C-Go boundary transitions.
type CGOBatchProcessor struct {
	mu         sync.Mutex
	batchSize  int
	buffer     []*VideoFrame
	onFlush    func(frames []*VideoFrame) error // Callback when batch is ready
	flushed    int64
	totalInput int64
}

// NewCGOBatchProcessor creates a batch processor with the specified batch size.
// onFlush is called with a complete batch of frames ready for C encoding.
func NewCGOBatchProcessor(batchSize int, onFlush func(frames []*VideoFrame) error) *CGOBatchProcessor {
	if batchSize <= 0 {
		batchSize = 8 // Default: send 8 frames per CGO call
	}
	return &CGOBatchProcessor{
		batchSize: batchSize,
		buffer:    make([]*VideoFrame, 0, batchSize),
		onFlush:   onFlush,
	}
}

// AddFrame adds a frame to the batch buffer. Flushes automatically when full.
func (bp *CGOBatchProcessor) AddFrame(frame *VideoFrame) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.totalInput++
	bp.buffer = append(bp.buffer, frame)

	if len(bp.buffer) >= bp.batchSize {
		return bp.flush()
	}
	return nil
}

// Flush forces all buffered frames to be processed, even if batch is not full.
func (bp *CGOBatchProcessor) Flush() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.flush()
}

// flush sends the current buffer to the onFlush callback. Must be called with lock held.
func (bp *CGOBatchProcessor) flush() error {
	if len(bp.buffer) == 0 {
		return nil
	}

	// Create a copy of the batch to send
	batch := make([]*VideoFrame, len(bp.buffer))
	copy(batch, bp.buffer)
	bp.buffer = bp.buffer[:0]
	bp.flushed++

	return bp.onFlush(batch)
}

// Stats returns the total input frames and flush count.
func (bp *CGOBatchProcessor) Stats() (totalInput, flushCount int64) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.totalInput, bp.flushed
}

// PackedFrameBuffer packs multiple RGBA video frames into a single contiguous
// byte buffer for efficient transfer across the CGO boundary.
// Layout: [Frame0 RGBA data][Frame1 RGBA data]...[FrameN RGBA data]
type PackedFrameBuffer struct {
	Data       []byte
	FrameCount int
	FrameSize  int // Size of each frame in bytes (W*H*4)
	Width      int
	Height     int
}

// PackFrames packs multiple VideoFrames into a single contiguous buffer.
func PackFrames(frames []*VideoFrame) *PackedFrameBuffer {
	if len(frames) == 0 {
		return nil
	}

	w := frames[0].Width
	h := frames[0].Height
	frameSize := w * h * 4
	totalSize := frameSize * len(frames)

	packed := &PackedFrameBuffer{
		Data:       make([]byte, totalSize),
		FrameCount: len(frames),
		FrameSize:  frameSize,
		Width:      w,
		Height:     h,
	}

	for i, frame := range frames {
		copy(packed.Data[i*frameSize:(i+1)*frameSize], frame.Data)
	}

	return packed
}

// UnpackFrame extracts a single frame from the packed buffer by index.
func (pb *PackedFrameBuffer) UnpackFrame(index int) *VideoFrame {
	if index < 0 || index >= pb.FrameCount {
		return nil
	}

	start := index * pb.FrameSize
	end := start + pb.FrameSize
	data := make([]byte, pb.FrameSize)
	copy(data, pb.Data[start:end])

	return &VideoFrame{
		Width:  pb.Width,
		Height: pb.Height,
		Format: PixelFormatRGBA,
		Data:   data,
	}
}

// HierarchicalWorkerPool implements a global shared worker pool across multiple pipelines
// to prevent goroutine explosion under high concurrency.
// Addresses expert criticism: "Worker Pools Hierárquicos" — shared pool stabilizes
// the Go scheduler when many pipelines run simultaneously.
type HierarchicalWorkerPool struct {
	mu          sync.Mutex
	maxWorkers  int
	activeJobs  int64
	sem         chan struct{} // Global concurrency semaphore
}

// NewHierarchicalWorkerPool creates a global pool with maximum concurrent workers.
func NewHierarchicalWorkerPool(maxWorkers int) *HierarchicalWorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = 16
	}
	return &HierarchicalWorkerPool{
		maxWorkers: maxWorkers,
		sem:        make(chan struct{}, maxWorkers),
	}
}

// Submit submits a job to the global pool. Blocks if the pool is at capacity.
func (hwp *HierarchicalWorkerPool) Submit(job func()) {
	hwp.sem <- struct{}{} // Acquire slot
	go func() {
		defer func() {
			<-hwp.sem // Release slot
		}()
		job()
	}()
}

// ActiveJobs returns the current number of running jobs.
func (hwp *HierarchicalWorkerPool) ActiveJobs() int {
	return len(hwp.sem)
}

// Capacity returns the maximum number of concurrent jobs.
func (hwp *HierarchicalWorkerPool) Capacity() int {
	return hwp.maxWorkers
}

// GlobalWorkerPool is the shared hierarchical worker pool for all pipelines.
var GlobalWorkerPool = NewHierarchicalWorkerPool(32)
