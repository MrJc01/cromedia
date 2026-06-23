package demux

import (
	"fmt"
	"io"
	"os"

	"cromedia/core"
)

// MP3Demuxer handles demuxing of MP3 files.
type MP3Demuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewMP3Demuxer instantiates a new MP3Demuxer.
func NewMP3Demuxer(file *os.File) *MP3Demuxer {
	return &MP3Demuxer{file: file}
}

// Probe skips ID3v2 tags and maps MP3 frames to samples.
func (d *MP3Demuxer) Probe() ([]core.Track, error) {
	info, err := d.file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// 1. Check for ID3v2 Tag and skip it
	var id3Header [10]byte
	if _, err := io.ReadFull(d.file, id3Header[:]); err == nil {
		if string(id3Header[0:3]) == "ID3" {
			// Size is synchsafe 4 bytes at [6:10]
			size := int64(id3Header[6])<<21 | int64(id3Header[7])<<14 | int64(id3Header[8])<<7 | int64(id3Header[9])
			if _, err := d.file.Seek(10+size, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to seek past ID3 tag: %w", err)
			}
		} else {
			// No ID3v2 tag, reset seek
			_, _ = d.file.Seek(0, io.SeekStart)
		}
	}

	offset, err := d.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	var sampleRate uint32
	var trackCreated bool
	var track core.Track
	var sampleTime int64 = 0

	headerBuf := make([]byte, 4)
	for offset+4 <= fileSize {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		if _, err := io.ReadFull(d.file, headerBuf); err != nil {
			break
		}

		// Frame Sync: 11 bits set: 0xFFE0 mask
		if headerBuf[0] != 0xFF || (headerBuf[1]&0xE0) != 0xE0 {
			// Not a sync point, scan byte by byte
			offset++
			continue
		}

		// Extract header properties
		versionID := (headerBuf[1] & 0x18) >> 3  // 00: 2.5, 10: v2, 11: v1
		layerID := (headerBuf[1] & 0x06) >> 1    // 01: Layer III, 10: Layer II, 11: Layer I
		bitrateIdx := (headerBuf[2] & 0xF0) >> 4
		samplerateIdx := (headerBuf[2] & 0x0C) >> 2
		padding := int64((headerBuf[2] & 0x02) >> 1)

		if bitrateIdx == 0x0F || bitrateIdx == 0x00 || samplerateIdx == 0x03 || layerID == 0 {
			// Invalid headers
			offset++
			continue
		}

		// Resolve Sample Rate
		var sr int
		switch versionID {
		case 3: // V1
			rates := []int{44100, 48000, 32000}
			sr = rates[samplerateIdx]
		case 2: // V2
			rates := []int{22050, 24000, 16000}
			sr = rates[samplerateIdx]
		case 0: // V2.5
			rates := []int{11025, 12000, 8000}
			sr = rates[samplerateIdx]
		default:
			offset++
			continue
		}

		// Resolve Bitrate (kbps)
		var br int
		if versionID == 3 { // V1
			switch layerID {
			case 3: // Layer I
				rates := []int{0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448, 0}
				br = rates[bitrateIdx]
			case 2: // Layer II
				rates := []int{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0}
				br = rates[bitrateIdx]
			case 1: // Layer III
				rates := []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
				br = rates[bitrateIdx]
			}
		} else { // V2 & V2.5
			switch layerID {
			case 3: // Layer I
				rates := []int{0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256, 0}
				br = rates[bitrateIdx]
			default: // Layer II & III
				rates := []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
				br = rates[bitrateIdx]
			}
		}

		if br == 0 {
			offset++
			continue
		}

		// Calculate frame size and samples per frame
		var frameSamples int64
		var frameSize int64
		if layerID == 3 { // Layer I
			frameSamples = 384
			frameSize = (12 * int64(br) * 1000 / int64(sr) + padding) * 4
		} else if layerID == 2 { // Layer II
			frameSamples = 1152
			frameSize = 144 * int64(br) * 1000 / int64(sr) + padding
		} else { // Layer III
			if versionID == 3 { // V1
				frameSamples = 1152
				frameSize = 144 * int64(br) * 1000 / int64(sr) + padding
			} else { // V2 / V2.5
				frameSamples = 576
				frameSize = 72 * int64(br) * 1000 / int64(sr) + padding
			}
		}

		if frameSize <= 0 {
			offset++
			continue
		}

		if !trackCreated {
			sampleRate = uint32(sr)
			track = core.Track{
				ID:        1,
				Type:      core.TrackTypeAudio,
				Timescale: sampleRate,
				CodecTag:  "mp3",
				Volume:    256,
			}
			trackCreated = true
		}

		sample := core.Sample{
			ID:         len(track.Samples) + 1,
			IsKeyframe: true,
			Offset:     offset,
			Size:       frameSize,
			Time:       sampleTime,
			Duration:   frameSamples,
		}
		track.Samples = append(track.Samples, sample)

		sampleTime += frameSamples
		offset += frameSize
	}

	if !trackCreated {
		return nil, fmt.Errorf("no valid MP3/MPEG audio frames found")
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
func (d *MP3Demuxer) ReadPacket() (*core.Packet, error) {
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
func (d *MP3Demuxer) Close() error {
	return d.file.Close()
}
