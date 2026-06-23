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

// SRTDemuxer handles demuxing of SubRip (.srt) subtitle files.
type SRTDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewSRTDemuxer instantiates a new SRTDemuxer.
func NewSRTDemuxer(file *os.File) *SRTDemuxer {
	return &SRTDemuxer{file: file}
}

// Probe parses the SRT file line-by-line to build samples.
func (d *SRTDemuxer) Probe() ([]core.Track, error) {
	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	track := core.Track{
		ID:        1,
		Type:      core.TrackTypeMeta,
		Timescale: 1000,
		CodecTag:  "srt",
	}

	scanner := bufio.NewScanner(d.file)
	var state = 0 // 0: Expecting index, 1: Expecting times, 2: Expecting text
	var currentStart int64
	var currentEnd int64
	var textOffset int64
	var textSize int64

	var filePos int64 = 0

	parseTime := func(tStr string) int64 {
		// Formats: "00:02:17,440" or similar
		parts := strings.Split(tStr, "-->")
		if len(parts) < 2 {
			return 0
		}
		// Clean spaces
		startStr := strings.TrimSpace(parts[0])
		endStr := strings.TrimSpace(parts[1])

		parseSingle := func(s string) int64 {
			s = strings.ReplaceAll(s, ",", ".")
			tParts := strings.Split(s, ":")
			if len(tParts) < 3 {
				return 0
			}
			h, _ := strconv.ParseFloat(tParts[0], 64)
			m, _ := strconv.ParseFloat(tParts[1], 64)
			sec, _ := strconv.ParseFloat(tParts[2], 64)
			return int64((h*3600 + m*60 + sec) * 1000)
		}

		currentStart = parseSingle(startStr)
		currentEnd = parseSingle(endStr)
		return currentStart
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineLen := int64(len(line)) + 1 // +1 for newline character

		trimmed := strings.TrimSpace(line)
		switch state {
		case 0: // Expecting cue index
			if trimmed != "" {
				state = 1
			}
		case 1: // Expecting times "00:00:00,000 --> 00:00:00,000"
			if strings.Contains(trimmed, "-->") {
				parseTime(trimmed)
				textOffset = filePos + lineLen
				textSize = 0
				state = 2
			}
		case 2: // Expecting subtitle lines
			if trimmed == "" {
				// Blank line ends cue block
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

	// Catch the last subtitle cue if file doesn't end with blank line
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
		return nil, fmt.Errorf("no subtitles found in SRT file")
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
func (d *SRTDemuxer) ReadPacket() (*core.Packet, error) {
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
func (d *SRTDemuxer) Close() error {
	return d.file.Close()
}
