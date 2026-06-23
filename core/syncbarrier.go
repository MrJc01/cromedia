package core

import (
	"context"
)

// SyncBarrier coordinates packets from multiple input channels,
// multiplexing and outputting them in strict chronological order.
type SyncBarrier struct {
	inputs []<-chan *Packet
}

// NewSyncBarrier creates a new SyncBarrier for the given list of packet input channels.
func NewSyncBarrier(inputs []<-chan *Packet) *SyncBarrier {
	return &SyncBarrier{
		inputs: inputs,
	}
}

// Run reads from all input channels, orders packets chronologically using the provided getPTSNano callback,
// and pushes them to the out channel. It runs until all input channels are closed or the context is cancelled.
func (sb *SyncBarrier) Run(ctx context.Context, out chan<- *Packet, getPTSNano func(*Packet) int64) error {
	defer close(out)

	n := len(sb.inputs)
	active := make([]bool, n)
	for i := 0; i < n; i++ {
		active[i] = true
	}

	// Buffer to store the current head packet of each input channel
	heads := make([]*Packet, n)

	for {
		// Fill empty heads for active channels
		for i := 0; i < n; i++ {
			if !active[i] || heads[i] != nil {
				continue
			}

			select {
			case pkt, ok := <-sb.inputs[i]:
				if !ok {
					active[i] = false
				} else {
					heads[i] = pkt
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Check if all inputs are finished
		allDone := true
		for i := 0; i < n; i++ {
			if active[i] || heads[i] != nil {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}

		// Find the head packet with the minimum PTS
		minIdx := -1
		var minPTS int64

		for i := 0; i < n; i++ {
			if heads[i] == nil {
				continue
			}
			pts := getPTSNano(heads[i])
			if minIdx == -1 || pts < minPTS {
				minIdx = i
				minPTS = pts
			}
		}

		if minIdx != -1 {
			// Emit the oldest packet
			select {
			case out <- heads[minIdx]:
				heads[minIdx] = nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}
