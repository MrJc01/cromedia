package demux

import (
	"io"

	"cromedia/core"
)

// ChannelDemuxer implements the Demuxer interface, streaming packets from a Go channel.
type ChannelDemuxer struct {
	tracks []core.Track
	ch     <-chan *core.Packet
}

// NewChannelDemuxer instantiates a new ChannelDemuxer.
func NewChannelDemuxer(tracks []core.Track, ch <-chan *core.Packet) *ChannelDemuxer {
	return &ChannelDemuxer{
		tracks: tracks,
		ch:     ch,
	}
}

// Probe returns pre-configured tracks.
func (d *ChannelDemuxer) Probe() ([]core.Track, error) {
	return d.tracks, nil
}

// ReadPacket reads a packet from the channel, returning io.EOF when closed.
func (d *ChannelDemuxer) ReadPacket() (*core.Packet, error) {
	pkt, ok := <-d.ch
	if !ok {
		return nil, io.EOF
	}
	return pkt, nil
}

// Close is a no-op for channel streaming.
func (d *ChannelDemuxer) Close() error {
	return nil
}
