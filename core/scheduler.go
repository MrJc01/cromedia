package core

import (
	"fmt"
	"sync"
)

// GOP (Group of Pictures) represents a slice of samples starting with a Keyframe
type GOP struct {
	ID      int
	Samples []Sample
}

// Segmenter splits a list of samples into GOPs
type Segmenter struct {
	samples []Sample
	current int
}

func NewSegmenter(samples []Sample) *Segmenter {
	return &Segmenter{samples: samples}
}

// NextGOP returns the next GOP or nil if done
func (s *Segmenter) NextGOP() *GOP {
	if s.current >= len(s.samples) {
		return nil
	}

	start := s.current
	// Find next keyframe or end of samples
	end := start + 1
	for end < len(s.samples) {
		if s.samples[end].IsKeyframe {
			break
		}
		end++
	}

	gop := &GOP{
		ID:      start, // Use start index as ID for now (or sequential 0, 1, 2...)
		Samples: s.samples[start:end],
	}
	s.current = end
	return gop
}

// Result holds the processed data for a GOP
type Result struct {
	GOPID int
	Data  []byte // Processed bytes (mocked for now)
	Err   error
}

// WorkerPool manages parallel processing
type WorkerPool struct {
	Workers int
	Jobs    chan *GOP
	Results chan Result
	wg      sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		Workers: workers,
		Jobs:    make(chan *GOP, workers*2),
		Results: make(chan Result, workers*2), // Buffered to avoid blocking workers
	}
}

// Start launches the workers
func (wp *WorkerPool) Start(processor func(*GOP) ([]byte, error)) {
	for i := 0; i < wp.Workers; i++ {
		wp.wg.Add(1)
		go func(workerID int) {
			defer wp.wg.Done()
			for gop := range wp.Jobs {
				// Simulate processing
				data, err := processor(gop)
				wp.Results <- Result{
					GOPID: gop.ID,
					Data:  data,
					Err:   err,
				}
			}
		}(i)
	}
}

// Wait closes the results channel after all workers are done
// Should be called in a separate goroutine if consuming results in main thread!
func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
	close(wp.Results)
}

// RunPipelined executes the pipeline: Segmenter -> Workers -> Ordered Consumer
func RunPipelined(samples []Sample, workers int, processor func(*GOP) ([]byte, error)) error {
	segmenter := NewSegmenter(samples)
	pool := NewWorkerPool(workers)

	// 1. Start Workers
	pool.Start(processor)

	// 2. Producer (Segmenter)
	go func() {
		for {
			gop := segmenter.NextGOP()
			if gop == nil {
				close(pool.Jobs)
				break
			}
			pool.Jobs <- gop
		}
	}()

	// 3. Close Results when Workers finish
	go pool.Wait()

	// 4. Consumer (Ordered)
	// We need to re-order results because workers finish out of order.
	// Since we don't have a sophisticated re-ordering buffer yet,
	// for this MVP, we just collect results and print/verify.
	// Real implementation needs a PriorityQueue or Buffer to write sequentially.

	// For now, let's just count and verify
	count := 0
	for res := range pool.Results {
		if res.Err != nil {
			return res.Err
		}
		// fmt.Printf("Processed GOP %d (Size: %d bytes)\n", res.GOPID, len(res.Data))
		count++
	}
	fmt.Printf("Pipeline finished. Processed %d GOPs.\n", count)
	return nil
}
