package mux

import (
	"fmt"
	"os"
	"time"

	"cromedia/core"
)

// WebVTTMuxer writes WebVTT subtitle files.
type WebVTTMuxer struct {
	file *os.File
}

// NewWebVTTMuxer creates a new WebVTTMuxer.
func NewWebVTTMuxer(file *os.File) *WebVTTMuxer {
	return &WebVTTMuxer{file: file}
}

func (m *WebVTTMuxer) WriteHeader(tracks []core.Track) error {
	_, err := m.file.WriteString("WEBVTT\n\n")
	return err
}

func (m *WebVTTMuxer) WritePacket(pkt *core.Packet) error {
	startTime := time.Duration(pkt.PTS) * time.Millisecond
	endTime := time.Duration(pkt.PTS+pkt.Duration) * time.Millisecond

	// WebVTT time format: HH:MM:SS.mmm
	startFmt := formatVTTTime(startTime)
	endFmt := formatVTTTime(endTime)

	entry := fmt.Sprintf("%s --> %s\n%s\n\n", startFmt, endFmt, string(pkt.Data))
	_, err := m.file.WriteString(entry)
	return err
}

func (m *WebVTTMuxer) WriteTrailer() error {
	return nil
}

func (m *WebVTTMuxer) Close() error {
	return m.file.Close()
}

func formatVTTTime(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond

	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
