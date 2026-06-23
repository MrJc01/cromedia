package pipeline

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"cromedia/core"
)

func TestReorderingAndBackpressure(t *testing.T) {
	// 10 samples: 3 GOPs (keyframes at 0, 4, 8)
	samples := []core.Sample{
		{ID: 1, IsKeyframe: true, Time: 0},
		{ID: 2, IsKeyframe: false, Time: 1000},
		{ID: 3, IsKeyframe: false, Time: 2000},
		{ID: 4, IsKeyframe: false, Time: 3000},
		{ID: 5, IsKeyframe: true, Time: 4000},
		{ID: 6, IsKeyframe: false, Time: 5000},
		{ID: 7, IsKeyframe: false, Time: 6000},
		{ID: 8, IsKeyframe: false, Time: 7000},
		{ID: 9, IsKeyframe: true, Time: 8000},
		{ID: 10, IsKeyframe: false, Time: 9000},
	}

	processor := func(gop *GOP) ([]byte, error) {
		// Mock dynamic processing delays to force out-of-order execution
		// GOP 0 takes 100ms, GOP 1 takes 10ms, GOP 2 takes 50ms
		var delay time.Duration
		switch gop.Index {
		case 0:
			delay = 100 * time.Millisecond
		case 1:
			delay = 10 * time.Millisecond
		case 2:
			delay = 50 * time.Millisecond
		default:
			delay = time.Duration(rand.Intn(20)) * time.Millisecond
		}
		time.Sleep(delay)
		return []byte{byte(gop.Index)}, nil
	}

	var receivedIndices []int
	consumer := func(res Result) error {
		receivedIndices = append(receivedIndices, res.GOPIndex)
		return nil
	}

	ctx := core.NewPipelineContext(nil)
	err := RunPipelinedOrdered(ctx, samples, 3, processor, consumer)
	if err != nil {
		t.Fatalf("Pipeline run failed: %v", err)
	}

	// Expect strictly [0, 1, 2] even though GOP 1 and GOP 2 finished first
	if len(receivedIndices) != 3 {
		t.Fatalf("Expected 3 GOP results, got %d", len(receivedIndices))
	}

	for i, idx := range receivedIndices {
		if idx != i {
			t.Errorf("Expected GOP index at position %d to be %d, got %d", i, i, idx)
		}
	}
}

func TestMemoryReclamationOnError(t *testing.T) {
	core.GlobalResetStats()

	// 10 samples: 3 GOPs
	samples := []core.Sample{
		{ID: 1, IsKeyframe: true, Time: 0},
		{ID: 2, IsKeyframe: false, Time: 1000},
		{ID: 3, IsKeyframe: true, Time: 2000},
		{ID: 4, IsKeyframe: false, Time: 3000},
		{ID: 5, IsKeyframe: true, Time: 4000},
		{ID: 6, IsKeyframe: false, Time: 5000},
	}

	processor := func(gop *GOP) ([]byte, error) {
		// Allocate buffer via BufferPool
		buf := core.GlobalGet(100)
		return buf, nil
	}

	// Consumer returns an error immediately on receiving the first GOP
	consumer := func(res Result) error {
		return errors.New("aborted by consumer")
	}

	ctx := core.NewPipelineContext(nil)
	err := RunPipelinedOrdered(ctx, samples, 3, processor, consumer)

	if err == nil {
		t.Fatal("Expected pipeline to fail, got nil")
	}

	// Give a small delay for any deferred cleanups to fully execute
	time.Sleep(50 * time.Millisecond)

	gets, puts := core.GlobalStats()
	if gets != puts {
		t.Errorf("Memory leak detected! Buffers retrieved: %d, returned: %d (delta: %d)", gets, puts, gets-puts)
	} else {
		t.Logf("Success! No leaks. Buffers retrieved: %d, returned: %d", gets, puts)
	}
}
