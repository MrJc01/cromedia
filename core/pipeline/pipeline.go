package pipeline

import (
	"context"
	"fmt"
	"sync"

	"cromedia/core"
)

// GOP (Group of Pictures) represents a slice of samples starting with a Keyframe
type GOP struct {
	Index   int // Sequential index starting at 0
	ID      int // Start sample index (original ID)
	Samples []core.Sample
}

// Segmenter splits a list of samples into GOPs
type Segmenter struct {
	samples []core.Sample
	current int
	index   int
}

func NewSegmenter(samples []core.Sample) *Segmenter {
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
		Index:   s.index,
		ID:      start,
		Samples: s.samples[start:end],
	}
	s.index++
	s.current = end
	return gop
}

// Result holds the processed data for a GOP
type Result struct {
	GOPIndex int
	GOPID    int
	Data     []byte
	Err      error
}

// WorkerPool manages parallel processing
type WorkerPool struct {
	Workers   int
	Jobs      chan *GOP
	Results   chan Result
	wg        sync.WaitGroup
	closeOnce sync.Once
}

func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		Workers: workers,
		Jobs:    make(chan *GOP, workers*2),
		Results: make(chan Result, workers*2),
	}
}

// Start launches the workers
func (wp *WorkerPool) Start(ctx context.Context, processor func(*GOP) ([]byte, error)) {
	for i := 0; i < wp.Workers; i++ {
		wp.wg.Add(1)
		go func(workerID int) {
			defer wp.wg.Done()
			for {
				select {
				case gop, ok := <-wp.Jobs:
					if !ok {
						return
					}
					data, err := processor(gop)
					select {
					case wp.Results <- Result{
						GOPIndex: gop.Index,
						GOPID:    gop.ID,
						Data:     data,
						Err:      err,
					}:
					case <-ctx.Done():
						if data != nil {
							core.GlobalPut(data)
						}
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}
}

// CloseResults closes the results channel in a thread-safe manner
func (wp *WorkerPool) CloseResults() {
	wp.closeOnce.Do(func() {
		close(wp.Results)
	})
}

// Wait closes the results channel after all workers are done
func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
	wp.CloseResults()
}

// RunPipelinedOrdered executes the pipeline and delivers results in strict chronological order with backpressure
func RunPipelinedOrdered(ctx *core.PipelineContext, samples []core.Sample, workers int, processor func(*GOP) ([]byte, error), consumer func(Result) error) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	segmenter := NewSegmenter(samples)
	pool := NewWorkerPool(workers)

	// Semaphore for backpressure: maximum active GOPs in pipeline = workers * 2
	semLimit := workers * 2
	if semLimit < 2 {
		semLimit = 2
	}
	sem := make(chan struct{}, semLimit)

	// Safe resource reclamation in case of error/exit
	reorderMap := make(map[int]Result)
	defer func() {
		cancel()
		pool.wg.Wait()
		pool.CloseResults()

		// Drain Results and release buffers
		for res := range pool.Results {
			if res.Data != nil {
				core.GlobalPut(res.Data)
			}
		}

		// Drain reorderMap and release buffers
		for _, res := range reorderMap {
			if res.Data != nil {
				core.GlobalPut(res.Data)
			}
		}
	}()

	pool.Start(cancelCtx, processor)

	// Producer
	go func() {
		for {
			gop := segmenter.NextGOP()
			if gop == nil {
				close(pool.Jobs)
				break
			}
			select {
			case sem <- struct{}{}: // Acquire slot
				select {
				case pool.Jobs <- gop:
				case <-cancelCtx.Done():
					return
				}
			case <-cancelCtx.Done():
				return
			}
		}
	}()

	go pool.Wait()

	// Reordering buffer
	nextGOPIndex := 0

	for res := range pool.Results {
		if res.Err != nil {
			return res.Err
		}

		reorderMap[res.GOPIndex] = res

		// Deliver all sequential results that are ready
		for {
			nextRes, exists := reorderMap[nextGOPIndex]
			if !exists {
				break
			}

			// Deliver to consumer
			if err := consumer(nextRes); err != nil {
				return err
			}

			// Clean up map and advance pointer
			delete(reorderMap, nextGOPIndex)
			nextGOPIndex++
			select {
			case <-sem: // Release slot
			default:
			}
		}
	}

	fmt.Printf("Pipeline finished. Processed %d GOPs sequentially.\n", nextGOPIndex)
	return nil
}

// RunPipelined is the backward-compatible version of the pipeline runner
func RunPipelined(samples []core.Sample, workers int, processor func(*GOP) ([]byte, error)) error {
	ctx := core.NewPipelineContext(nil)
	return RunPipelinedOrdered(ctx, samples, workers, processor, func(res Result) error {
		return nil
	})
}
