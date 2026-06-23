package demux

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"cromedia/core"
)

// WebVTTDemuxer handles demuxing of WebVTT (.vtt) subtitle files.
type WebVTTDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewWebVTTDemuxer instantiates a new WebVTTDemuxer.
func NewWebVTTDemuxer(file *os.File) *WebVTTDemuxer {
	return &WebVTTDemuxer{file: file}
}

// Probe parses the WebVTT file cues to build samples.
func (d *WebVTTDemuxer) Probe() ([]core.Track, error) {
	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	track := core.Track{
		ID:        1,
		Type:      core.TrackTypeMeta,
		Timescale: 1000,
		CodecTag:  "vtt",
	}

	scanner := bufio.NewScanner(d.file)
	var state = 0 // 0: Expecting header/index, 1: Expecting times, 2: Expecting text
	var currentStart int64
	var currentEnd int64
	var textOffset int64
	var textSize int64

	var filePos int64 = 0
	var firstLine = true

	parseVTTTime := func(tStr string) int64 {
		// Formats: "00:11.000 --> 00:13.000" or "00:02:17.440 --> 00:02:20.375"
		parts := strings.Split(tStr, "-->")
		if len(parts) < 2 {
			return 0
		}
		startStr := strings.TrimSpace(parts[0])
		endStr := strings.TrimSpace(parts[1])

		parseSingle := func(s string) int64 {
			// strip off style/settings if present after timestamp (e.g. "00:11.000 line:0")
			s = strings.Fields(s)[0]
			tParts := strings.Split(s, ":")
			if len(tParts) == 2 {
				// MM:SS.mmm
				m, _ := strconv.ParseFloat(tParts[0], 64)
				sec, _ := strconv.ParseFloat(tParts[1], 64)
				return int64((m*60 + sec) * 1000)
			} else if len(tParts) == 3 {
				// HH:MM:SS.mmm
				h, _ := strconv.ParseFloat(tParts[0], 64)
				m, _ := strconv.ParseFloat(tParts[1], 64)
				sec, _ := strconv.ParseFloat(tParts[2], 64)
				return int64((h*3600 + m*60 + sec) * 1000)
			}
			return 0
		}

		currentStart = parseSingle(startStr)
		currentEnd = parseSingle(endStr)
		return currentStart
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineLen := int64(len(line)) + 1 // +1 for newline

		trimmed := strings.TrimSpace(line)
		if firstLine {
			firstLine = false
			if !strings.HasPrefix(trimmed, "WEBVTT") {
				return nil, fmt.Errorf("invalid WebVTT signature")
			}
			filePos += lineLen
			continue
		}

		switch state {
		case 0: // Expecting index or times
			if trimmed != "" {
				if strings.Contains(trimmed, "-->") {
					parseVTTTime(trimmed)
					textOffset = filePos + lineLen
					textSize = 0
					state = 2
				} else {
					state = 1
				}
			}
		case 1: // Expecting times
			if strings.Contains(trimmed, "-->") {
				parseVTTTime(trimmed)
				textOffset = filePos + lineLen
				textSize = 0
				state = 2
			}
		case 2: // Expecting text lines
			if trimmed == "" {
				if textSize > 0 {
					sample := core.Sample{
						ID:         len(track.Samples) + 1,
						IsKeyframe: true,
						Offset:     textOffset,
						Size:       textSize,
						Time:       currentStart,
						Duration:   currentEnd - currentStart,
					}
					track.Samples = append(track.Samples, sample)
				}
				state = 0
			} else {
				textSize += lineLen
			}
		}
		filePos += lineLen
	}

	if state == 2 && textSize > 0 {
		sample := core.Sample{
			ID:         len(track.Samples) + 1,
			IsKeyframe: true,
			Offset:     textOffset,
			Size:       textSize,
			Time:       currentStart,
			Duration:   currentEnd - currentStart,
		}
		track.Samples = append(track.Samples, sample)
	}

	if len(track.Samples) == 0 {
		return nil, fmt.Errorf("no cues found in WebVTT file")
	}

	track.Duration = uint64(track.Samples[len(track.Samples)-1].Time + track.Samples[len(track.Samples)-1].Duration)
	d.tracks = []core.Track{track}

	d.interleaved = make([]core.InterleavedSample, len(track.Samples))
	for i, s := range track.Samples {
		d.interleaved[i] = core.InterleavedSample{
			TrackIndex:  0,
			SampleIndex: i,
			TimeSeconds: float64(s.Time) / 1000.0,
			Sample:      s,
		}
	}
	d.currentSample = 0

	return d.tracks, nil
}

// ReadPacket returns the next interleaved packet.
func (d *WebVTTDemuxer) ReadPacket() (*core.Packet, error) {
	if d.currentSample >= len(d.interleaved) {
		return nil, io.EOF
	}

	is := d.interleaved[d.currentSample]
	d.currentSample++

	if _, err := d.file.Seek(is.Sample.Offset, io.SeekStart); err != nil {
		return nil, err
	}

	buf := core.GlobalGet(int(is.Sample.Size))
	if _, err := io.ReadFull(d.file, buf); err != nil {
		core.GlobalPut(buf)
		return nil, err
	}

	pkt := &core.Packet{
		ID:          core.NewPacketID(),
		StreamIndex: is.TrackIndex,
		Data:        buf,
		PTS:         is.Sample.Time,
		DTS:         is.Sample.Time,
		Duration:    is.Sample.Duration,
		IsKeyframe:  is.Sample.IsKeyframe,
	}

	return pkt, nil
}

// Close closes the underlying media file.
func (d *WebVTTDemuxer) Close() error {
	return d.file.Close()
}
