package demux

import (
	"cromedia/core"
)

// Demuxer defines the generic interface for container parsers (e.g. MP4, WebM, MPEG-TS).
type Demuxer interface {
	// Probe parses headers and initializes track metadata without loading sample data.
	Probe() ([]core.Track, error)
	// ReadPacket reads the next available packet in presentation/interleaved order.
	// Returns io.EOF when there are no more packets.
	ReadPacket() (*core.Packet, error)
	// Close releases system resources associated with the demuxer.
	Close() error
}
