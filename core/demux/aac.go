package demux

import (
	"fmt"
	"io"
	"os"

	"cromedia/core"
)

// AACDemuxer handles demuxing of raw ADTS AAC (.aac) files.
type AACDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewAACDemuxer instantiates a new AACDemuxer.
func NewAACDemuxer(file *os.File) *AACDemuxer {
	return &AACDemuxer{file: file}
}

var aacSampleRates = []uint32{
	96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350,
}

// Probe parses the ADTS headers to map AAC frames to samples.
func (d *AACDemuxer) Probe() ([]core.Track, error) {
	info, err := d.file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var sampleRate uint32
	var trackCreated bool
	var track core.Track
	var sampleTime int64 = 0

	var offset int64 = 0
	headerBuf := make([]byte, 7)

	for offset+7 <= fileSize {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		if _, err := io.ReadFull(d.file, headerBuf); err != nil {
			break
		}

		// Syncword: 12 bits set: 0xFFF
		if headerBuf[0] != 0xFF || (headerBuf[1]&0xF0) != 0xF0 {
			// Not aligned, scan byte by byte
			offset++
			continue
		}

		// Sampling frequency index: bits [18:21] -> headerBuf[2] bits [5:2] (i.e. (b[2] & 0x3C) >> 2)
		sfIdx := (headerBuf[2] & 0x3C) >> 2
		if int(sfIdx) >= len(aacSampleRates) {
			offset++
			continue
		}

		// Frame length: 13 bits starting at bit 30
		// headerBuf[3] bits [1:0] (2 bits)
		// headerBuf[4] (8 bits)
		// headerBuf[5] bits [7:5] (3 bits)
		frameLen := (uint32(headerBuf[3]&0x03) << 11) | (uint32(headerBuf[4]) << 3) | (uint32(headerBuf[5]&0xE0) >> 5)
		if frameLen < 7 || offset+int64(frameLen) > fileSize {
			offset++
			continue
		}

		if !trackCreated {
			sampleRate = aacSampleRates[sfIdx]
			track = core.Track{
				ID:        1,
				Type:      core.TrackTypeAudio,
				Timescale: sampleRate,
				CodecTag:  "mp4a",
				Volume:    256,
			}
			trackCreated = true
		}

		sample := core.Sample{
			ID:         len(track.Samples) + 1,
			IsKeyframe: true,
			Offset:     offset,
			Size:       int64(frameLen),
			Time:       sampleTime,
			Duration:   1024, // AAC standard frame duration is 1024 samples
		}
		track.Samples = append(track.Samples, sample)

		sampleTime += 1024
		offset += int64(frameLen)
	}

	if !trackCreated {
		return nil, fmt.Errorf("no valid AAC ADTS frames found")
	}

	track.Duration = uint64(sampleTime)
	d.tracks = []core.Track{track}

	d.interleaved = make([]core.InterleavedSample, len(track.Samples))
	for i, s := range track.Samples {
		d.interleaved[i] = core.InterleavedSample{
			TrackIndex:  0,
			SampleIndex: i,
			TimeSeconds: float64(s.Time) / float64(sampleRate),
			Sample:      s,
		}
	}
	d.currentSample = 0

	return d.tracks, nil
}

// ReadPacket returns the next interleaved packet.
func (d *AACDemuxer) ReadPacket() (*core.Packet, error) {
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
func (d *AACDemuxer) Close() error {
	return d.file.Close()
}
