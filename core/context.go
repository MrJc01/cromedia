package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PipelineContext wraps Go's context and aggregates telemetry metrics and locks.
type PipelineContext struct {
	context.Context
	mu           sync.Mutex
	startTime    time.Time
	startUserCPU time.Duration
	startSysCPU  time.Duration
	latencies    map[string]time.Duration
	packetCount  map[string]int64
	errors       []error
}

// NewPipelineContext creates a new pipeline context.
func NewPipelineContext(parent context.Context) *PipelineContext {
	if parent == nil {
		parent = context.Background()
	}
	user, sys := GetCPUTimes()
	return &PipelineContext{
		Context:      parent,
		startTime:    time.Now(),
		startUserCPU: user,
		startSysCPU:  sys,
		latencies:    make(map[string]time.Duration),
		packetCount:  make(map[string]int64),
	}
}

// StartStage starts timing a stage. Returns a function to call when the stage finishes.
func (c *PipelineContext) StartStage(name string) func() {
	start := time.Now()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.latencies[name] += time.Since(start)
	}
}

// AddPacket increments the packet count for a given stage/operation.
func (c *PipelineContext) AddPacket(stage string, count int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.packetCount[stage] += count
}

// RecordError stores a non-fatal pipeline error.
func (c *PipelineContext) RecordError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errors = append(c.errors, err)
}

// GetTelemetry returns a snapshot of the current metrics, including CPU times.
func (c *PipelineContext) GetTelemetry() (time.Duration, time.Duration, time.Duration, map[string]time.Duration, map[string]int64, []error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	latenciesCopy := make(map[string]time.Duration)
	for k, v := range c.latencies {
		latenciesCopy[k] = v
	}

	packetCountCopy := make(map[string]int64)
	for k, v := range c.packetCount {
		packetCountCopy[k] = v
	}

	errorsCopy := make([]error, len(c.errors))
	copy(errorsCopy, c.errors)

	currentUser, currentSys := GetCPUTimes()
	userDiff := currentUser - c.startUserCPU
	sysDiff := currentSys - c.startSysCPU

	return time.Since(c.startTime), userDiff, sysDiff, latenciesCopy, packetCountCopy, errorsCopy
}

// PrintReport prints the performance report to stdout.
func (c *PipelineContext) PrintReport() {
	totalTime, userCPU, sysCPU, latencies, packets, errors := c.GetTelemetry()
	fmt.Printf("\n=== CroMedia Performance & Telemetry Report ===\n")
	fmt.Printf("Total Elapsed Time: %v\n", totalTime)
	fmt.Printf("CPU Usage: User: %v, System: %v\n", userCPU, sysCPU)

	fmt.Printf("\nStage Latencies:\n")
	for stage, dur := range latencies {
		pct := 0.0
		if totalTime > 0 {
			pct = float64(dur) / float64(totalTime) * 100.0
		}
		fmt.Printf("  - %-15s: %10v (%5.1f%%)\n", stage, dur, pct)
	}

	fmt.Printf("\nPacket Counts:\n")
	for stage, count := range packets {
		fmt.Printf("  - %-15s: %10d packets\n", stage, count)
	}

	if len(errors) > 0 {
		fmt.Printf("\nNon-Fatal Errors (%d):\n", len(errors))
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
	}
	fmt.Printf("===============================================\n")
}

// RecoverPanic catches any panics and records them as errors to prevent pipeline crash.
func (c *PipelineContext) RecoverPanic(onPanic func(err error)) {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("panic: %v", r)
		}
		c.RecordError(err)
		if onPanic != nil {
			onPanic(err)
		}
	}
}
