package mux

import (
	"cromedia/core"
)

// Muxer defines the generic interface for writing multiplexed container files.
type Muxer interface {
	// WriteHeader writes the container format headers (e.g., ftyp, track headers).
	WriteHeader(tracks []core.Track) error
	// WritePacket writes a packet into the container.
	WritePacket(pkt *core.Packet) error
	// WriteTrailer writes concluding metadata structures (e.g., moov index/offsets, cues).
	WriteTrailer() error
	// Close flushes all pending data to disk and closes files.
	Close() error
}
