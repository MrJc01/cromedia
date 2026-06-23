package core

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// NetworkStreamWriter defines the interface for streaming packets to network targets
type NetworkStreamWriter interface {
	WritePacket(pkt *Packet) error
	Close() error
}

// RTMPWriter simulates streaming to an RTMP/RTMPS endpoint
type RTMPWriter struct {
	URL string
}

func (w *RTMPWriter) WritePacket(pkt *Packet) error {
	// In a real implementation, this converts Packet data to RTMP message chunks
	// and writes to a TCP socket.
	return nil
}

func (w *RTMPWriter) Close() error { return nil }

// RTSPWriter simulates streaming to an RTSP endpoint
type RTSPWriter struct {
	URL string
}

func (w *RTSPWriter) WritePacket(pkt *Packet) error {
	// In a real implementation, this writes RTP packets over UDP or TCP interleaved
	return nil
}

func (w *RTSPWriter) Close() error { return nil }

// HLSSegmenter splits a continuous stream into HLS segments (.ts and .m3u8 playlist)
type HLSSegmenter struct {
	SegmentDurationSec float64
	outputDir          string
	currentSegNum      int
	currentSegDuration float64
}

func NewHLSSegmenter(dir string, durationSec float64) *HLSSegmenter {
	return &HLSSegmenter{
		SegmentDurationSec: durationSec,
		outputDir:          dir,
		currentSegNum:      0,
	}
}

func (s *HLSSegmenter) FeedPacket(pkt *Packet) error {
	// Simulates accumulating packets and writing segment files
	s.currentSegDuration += float64(pkt.Duration)
	if s.currentSegDuration >= s.SegmentDurationSec {
		s.currentSegNum++
		s.currentSegDuration = 0
	}
	return nil
}

// JitterBuffer buffers incoming network packets to smooth out network delivery jitter
type JitterBuffer struct {
	packets chan *Packet
}

func NewJitterBuffer(capacity int) *JitterBuffer {
	return &JitterBuffer{
		packets: make(chan *Packet, capacity),
	}
}

func (jb *JitterBuffer) Push(pkt *Packet) {
	select {
	case jb.packets <- pkt:
	default:
		// Drop packet or block depending on overflow policy
	}
}

func (jb *JitterBuffer) Pop(ctx context.Context) (*Packet, error) {
	select {
	case pkt := <-jb.packets:
		return pkt, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// NetworkRetryWithBackoff retries a network function with exponential backoff
func NetworkRetryWithBackoff(fn func() error, maxRetries int) error {
	backoff := 500 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		fmt.Printf("Network error: %v. Retrying in %v...\n", err, backoff)
		time.Sleep(backoff)
		backoff *= 2
	}
	return fmt.Errorf("network execution failed after %d retries", maxRetries)
}

// HTTPChunkedStreamer streams encoded packets over HTTP Chunked Transfer Encoding
type HTTPChunkedStreamer struct {
	w http.ResponseWriter
}

func NewHTTPChunkedStreamer(w http.ResponseWriter) *HTTPChunkedStreamer {
	return &HTTPChunkedStreamer{w: w}
}

func (s *HTTPChunkedStreamer) StreamPacket(pkt *Packet) error {
	flusher, ok := s.w.(http.Flusher)
	if !ok {
		return fmt.Errorf("writer does not support flushing")
	}
	if _, err := s.w.Write(pkt.Data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
