package mux

import (
	"fmt"
	"os"
	"time"

	"cromedia/core"
)

// SRTMuxer writes SubRip (.srt) subtitle files.
type SRTMuxer struct {
	file      *os.File
	index     int
}

// NewSRTMuxer creates a new SRTMuxer.
func NewSRTMuxer(file *os.File) *SRTMuxer {
	return &SRTMuxer{file: file, index: 1}
}

func (m *SRTMuxer) WriteHeader(tracks []core.Track) error {
	return nil
}

func (m *SRTMuxer) WritePacket(pkt *core.Packet) error {
	startTime := time.Duration(pkt.PTS) * time.Millisecond
	endTime := time.Duration(pkt.PTS+pkt.Duration) * time.Millisecond

	// SRT time format: HH:MM:SS,mmm
	startFmt := formatSRTTime(startTime)
	endFmt := formatSRTTime(endTime)

	entry := fmt.Sprintf("%d\n%s --> %s\n%s\n\n", m.index, startFmt, endFmt, string(pkt.Data))
	m.index++

	_, err := m.file.WriteString(entry)
	return err
}

func (m *SRTMuxer) WriteTrailer() error {
	return nil
}

func (m *SRTMuxer) Close() error {
	return m.file.Close()
}

func formatSRTTime(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond

	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
