package mux

import (
	"cromedia/core"
)

// ChannelMuxer implements the Muxer interface, writing packets to a Go channel.
type ChannelMuxer struct {
	ch chan<- *core.Packet
}

// NewChannelMuxer instantiates a new ChannelMuxer.
func NewChannelMuxer(ch chan<- *core.Packet) *ChannelMuxer {
	return &ChannelMuxer{ch: ch}
}

// WriteHeader is a no-op for channel streaming.
func (m *ChannelMuxer) WriteHeader(tracks []core.Track) error {
	return nil
}

// WritePacket pushes the packet to the Go channel.
func (m *ChannelMuxer) WritePacket(pkt *core.Packet) error {
	m.ch <- pkt
	return nil
}

// WriteTrailer closes the channel to signal EOF.
func (m *ChannelMuxer) WriteTrailer() error {
	close(m.ch)
	return nil
}

// Close is a no-op for channel streaming.
func (m *ChannelMuxer) Close() error {
	return nil
}
