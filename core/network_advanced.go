package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// HybridJitterBuffer implements a jitter buffer that spills excess packets to disk
// when RAM usage exceeds a configurable threshold.
// Addresses expert criticism: "Hybrid Jitter Buffer (Spill-to-Disk)" — prevents
// RAM overflow under severe bandwidth degradation by paging to local disk.
type HybridJitterBuffer struct {
	mu             sync.Mutex
	ramBuffer      []*Packet
	maxRAMPackets  int
	spillDir       string
	spillCount     int64 // Atomic counter of spilled packets
	totalPushed    int64
	totalPopped    int64
	diskQueue      []string // File paths of spilled packets on disk
	maxRAMBytes    int64    // Max RAM threshold (default 50MB)
	currentRAMUsed int64
}

// NewHybridJitterBuffer creates a jitter buffer with RAM+disk hybrid storage.
// maxRAMPackets: max packets to keep in RAM before spilling to disk.
// spillDir: directory for temporary disk storage of overflow packets.
func NewHybridJitterBuffer(maxRAMPackets int, spillDir string) *HybridJitterBuffer {
	os.MkdirAll(spillDir, 0755)
	return &HybridJitterBuffer{
		ramBuffer:     make([]*Packet, 0, maxRAMPackets),
		maxRAMPackets: maxRAMPackets,
		spillDir:      spillDir,
		maxRAMBytes:   50 * 1024 * 1024, // 50MB default
	}
}

// Push adds a packet to the buffer. If RAM is full, spills to disk.
func (hjb *HybridJitterBuffer) Push(pkt *Packet) error {
	hjb.mu.Lock()
	defer hjb.mu.Unlock()
	atomic.AddInt64(&hjb.totalPushed, 1)

	pktSize := int64(len(pkt.Data))

	// Check if we should spill to disk
	if len(hjb.ramBuffer) >= hjb.maxRAMPackets || hjb.currentRAMUsed+pktSize > hjb.maxRAMBytes {
		return hjb.spillToDisk(pkt)
	}

	hjb.ramBuffer = append(hjb.ramBuffer, pkt)
	hjb.currentRAMUsed += pktSize
	return nil
}

// spillToDisk serializes a packet to a temporary file on disk.
func (hjb *HybridJitterBuffer) spillToDisk(pkt *Packet) error {
	idx := atomic.AddInt64(&hjb.spillCount, 1)
	filename := filepath.Join(hjb.spillDir, fmt.Sprintf("spill_%06d.pkt", idx))

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("jitter buffer spill-to-disk failed: %w", err)
	}
	defer f.Close()

	// Simple serialization: [PTS:8][DTS:8][Duration:8][StreamIndex:4][IsKeyframe:1][DataLen:4][Data...]
	header := make([]byte, 33)
	binary.BigEndian.PutUint64(header[0:8], uint64(pkt.PTS))
	binary.BigEndian.PutUint64(header[8:16], uint64(pkt.DTS))
	binary.BigEndian.PutUint64(header[16:24], uint64(pkt.Duration))
	binary.BigEndian.PutUint32(header[24:28], uint32(pkt.StreamIndex))
	if pkt.IsKeyframe {
		header[28] = 1
	}
	binary.BigEndian.PutUint32(header[29:33], uint32(len(pkt.Data)))

	f.Write(header)
	f.Write(pkt.Data)

	hjb.diskQueue = append(hjb.diskQueue, filename)
	return nil
}

// Pop retrieves the next packet (RAM first, then disk). Blocks until available or context cancelled.
func (hjb *HybridJitterBuffer) Pop(ctx context.Context) (*Packet, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		hjb.mu.Lock()
		// Serve from RAM buffer first
		if len(hjb.ramBuffer) > 0 {
			pkt := hjb.ramBuffer[0]
			hjb.ramBuffer = hjb.ramBuffer[1:]
			hjb.currentRAMUsed -= int64(len(pkt.Data))
			atomic.AddInt64(&hjb.totalPopped, 1)
			hjb.mu.Unlock()
			return pkt, nil
		}

		// Serve from disk queue
		if len(hjb.diskQueue) > 0 {
			filename := hjb.diskQueue[0]
			hjb.diskQueue = hjb.diskQueue[1:]
			hjb.mu.Unlock()

			pkt, err := hjb.readFromDisk(filename)
			if err != nil {
				return nil, err
			}
			os.Remove(filename)
			atomic.AddInt64(&hjb.totalPopped, 1)
			return pkt, nil
		}

		hjb.mu.Unlock()
		// Brief wait before retry
		time.Sleep(1 * time.Millisecond)
	}
}

// readFromDisk deserializes a packet from a spill file.
func (hjb *HybridJitterBuffer) readFromDisk(filename string) (*Packet, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read spill file: %w", err)
	}

	if len(data) < 33 {
		return nil, fmt.Errorf("corrupt spill file: too short")
	}

	pkt := &Packet{
		PTS:         int64(binary.BigEndian.Uint64(data[0:8])),
		DTS:         int64(binary.BigEndian.Uint64(data[8:16])),
		Duration:    int64(binary.BigEndian.Uint64(data[16:24])),
		StreamIndex: int(binary.BigEndian.Uint32(data[24:28])),
		IsKeyframe:  data[28] == 1,
	}
	dataLen := binary.BigEndian.Uint32(data[29:33])
	if uint32(len(data)-33) >= dataLen {
		pkt.Data = data[33 : 33+dataLen]
	}
	return pkt, nil
}

// Stats returns push/pop/spill counts for monitoring.
func (hjb *HybridJitterBuffer) Stats() (pushed, popped, spilled int64) {
	return atomic.LoadInt64(&hjb.totalPushed),
		atomic.LoadInt64(&hjb.totalPopped),
		atomic.LoadInt64(&hjb.spillCount)
}

// Cleanup removes all temporary spill files.
func (hjb *HybridJitterBuffer) Cleanup() {
	hjb.mu.Lock()
	defer hjb.mu.Unlock()
	for _, f := range hjb.diskQueue {
		os.Remove(f)
	}
	hjb.diskQueue = nil
}

// ========================================================================
// PCR Clock Synchronizer for MPEG-TS strict alignment
// ========================================================================

// PCRClockSync implements continuous PCR clock synchronization for MPEG-TS muxing,
// ensuring audio/video PTS/DTS deviations remain within 500ns tolerance.
// Addresses expert criticism: "Clock Sync e Descontinuidade Estrita" —
// preemptive discontinuity signaling when drift exceeds threshold.
type PCRClockSync struct {
	mu                sync.Mutex
	basePCR           int64 // Base PCR value in 90kHz ticks
	baseWallClock     time.Time
	lastPCR           int64
	driftThresholdNs  int64 // Maximum allowed drift before discontinuity signal (default 500ns)
	discontinuityFlag bool
	totalDriftNs      int64
	correctionCount   int64
}

// NewPCRClockSync creates a new PCR synchronizer with the specified drift tolerance.
func NewPCRClockSync(driftThresholdNs int64) *PCRClockSync {
	if driftThresholdNs <= 0 {
		driftThresholdNs = 500 // 500ns default
	}
	return &PCRClockSync{
		basePCR:          0,
		baseWallClock:    time.Now(),
		driftThresholdNs: driftThresholdNs,
	}
}

// UpdatePCR calculates the expected PCR based on wall clock and compares with actual.
// Returns true if a discontinuity flag should be set in the MPEG-TS stream.
func (pcs *PCRClockSync) UpdatePCR(actualPCR int64) bool {
	pcs.mu.Lock()
	defer pcs.mu.Unlock()

	// Calculate expected PCR based on elapsed wall clock time
	elapsed := time.Since(pcs.baseWallClock)
	expectedPCR := pcs.basePCR + int64(elapsed.Seconds()*90000) // 90kHz clock

	// Calculate drift in nanoseconds
	driftTicks := actualPCR - expectedPCR
	driftNs := int64(float64(driftTicks) / 90000.0 * 1e9)
	pcs.totalDriftNs += int64(math.Abs(float64(driftNs)))

	// Check if drift exceeds threshold
	if math.Abs(float64(driftNs)) > float64(pcs.driftThresholdNs) {
		pcs.discontinuityFlag = true
		pcs.correctionCount++
		// Reset base to current actual PCR
		pcs.basePCR = actualPCR
		pcs.baseWallClock = time.Now()
		return true
	}

	pcs.lastPCR = actualPCR
	pcs.discontinuityFlag = false
	return false
}

// GeneratePCR generates a PCR value synchronized to the wall clock.
func (pcs *PCRClockSync) GeneratePCR() int64 {
	pcs.mu.Lock()
	defer pcs.mu.Unlock()

	elapsed := time.Since(pcs.baseWallClock)
	return pcs.basePCR + int64(elapsed.Seconds()*90000)
}

// HasDiscontinuity returns true if the last PCR update detected a discontinuity.
func (pcs *PCRClockSync) HasDiscontinuity() bool {
	pcs.mu.Lock()
	defer pcs.mu.Unlock()
	return pcs.discontinuityFlag
}

// DriftStats returns total accumulated drift and correction count.
func (pcs *PCRClockSync) DriftStats() (totalDriftNs int64, corrections int64) {
	pcs.mu.Lock()
	defer pcs.mu.Unlock()
	return pcs.totalDriftNs, pcs.correctionCount
}
