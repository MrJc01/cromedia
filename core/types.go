package core

import "sync/atomic"

var globalPacketID int64

// Packet represents a single chunk of encoded audio, video, or metadata payload.
type Packet struct {
	ID          int64  // Unique packet identifier for traceability
	StreamIndex int    // 0-based index of the track/stream
	Data        []byte // Raw payload bytes
	PTS         int64  // Presentation Timestamp (in track timescale units)
	DTS         int64  // Decoding Timestamp (in track timescale units)
	Duration    int64  // Duration of the packet (in track timescale units)
	IsKeyframe  bool   // Identifies if this packet is a random access point (I-Frame)
}

// NewPacketID generates a globally unique packet sequence ID.
func NewPacketID() int64 {
	return atomic.AddInt64(&globalPacketID, 1)
}
