package demux

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"

	"cromedia/core"
)

// FLVDemuxer handles demuxing of Flash Video (.flv) files.
type FLVDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewFLVDemuxer instantiates a new FLVDemuxer.
func NewFLVDemuxer(file *os.File) *FLVDemuxer {
	return &FLVDemuxer{file: file}
}

// Probe parses the FLV header and all tags to map tracks and samples.
func (d *FLVDemuxer) Probe() ([]core.Track, error) {
	info, err := d.file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// Read FLV Header (9 bytes)
	hdr := make([]byte, 9)
	if _, err := io.ReadFull(d.file, hdr); err != nil {
		return nil, fmt.Errorf("failed to read FLV header: %w", err)
	}

	if hdr[0] != 'F' || hdr[1] != 'L' || hdr[2] != 'V' || hdr[3] != 1 {
		return nil, fmt.Errorf("invalid FLV signature or version")
	}

	// Skip past header and read PreviousTagSize0 (4 bytes, usually 0)
	if _, err := d.file.Seek(int64(binary.BigEndian.Uint32(hdr[5:9])), io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek past FLV header: %w", err)
	}

	var prevTagSizeBuf [4]byte
	if _, err := io.ReadFull(d.file, prevTagSizeBuf[:]); err != nil {
		return nil, fmt.Errorf("failed to read PreviousTagSize0: %w", err)
	}

	var tracksMap = make(map[byte]*core.Track)
	var offset = int64(binary.BigEndian.Uint32(hdr[5:9])) + 4

	tagHeader := make([]byte, 11)
	for offset+15 <= fileSize { // 11 bytes tag header + at least 4 bytes PreviousTagSize
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		if _, err := io.ReadFull(d.file, tagHeader); err != nil {
			break
		}

		tagType := tagHeader[0]
		dataSize := uint32(tagHeader[1])<<16 | uint32(tagHeader[2])<<8 | uint32(tagHeader[3])
		ts := uint32(tagHeader[7])<<24 | uint32(tagHeader[4])<<16 | uint32(tagHeader[5])<<8 | uint32(tagHeader[6])

		payloadOffset := offset + 11

		// Read first byte of payload to detect codec if we haven't already
		var firstByte byte
		if dataSize > 0 {
			var b [1]byte
			if _, err := d.file.ReadAt(b[:], payloadOffset); err == nil {
				firstByte = b[0]
			}
		}

		if tagType == 8 || tagType == 9 { // 8 = Audio, 9 = Video
			t, exists := tracksMap[tagType]
			if !exists {
				var trackType core.TrackType
				var codecTag string
				if tagType == 9 {
					trackType = core.TrackTypeVideo
					codecID := firstByte & 0x0F
					switch codecID {
					case 7:
						codecTag = "avc1"
					case 12:
						codecTag = "hev1"
					default:
						codecTag = "flv1"
					}
				} else {
					trackType = core.TrackTypeAudio
					soundFormat := firstByte >> 4
					switch soundFormat {
					case 10:
						codecTag = "mp4a"
					case 2:
						codecTag = "mp3"
					default:
						codecTag = "pcm"
					}
				}

				t = &core.Track{
					ID:        int(tagType),
					Type:      trackType,
					Timescale: 1000,
					CodecTag:  codecTag,
				}
				d.tracks = append(d.tracks, *t)
				tracksMap[tagType] = &d.tracks[len(d.tracks)-1]
				t = tracksMap[tagType]
			}

			isKey := true
			if tagType == 9 {
				isKey = ((firstByte >> 4) == 1) // 1 = keyframe
			}

			sample := core.Sample{
				ID:         len(t.Samples) + 1,
				IsKeyframe: isKey,
				Offset:     payloadOffset,
				Size:       int64(dataSize),
				Time:       int64(ts),
				Duration:   0,
			}
			t.Samples = append(t.Samples, sample)
		}

		offset += 11 + int64(dataSize) + 4 // tag header + payload + PreviousTagSize
	}

	// Calculate durations
	for i := range d.tracks {
		t := &d.tracks[i]
		for j := 0; j < len(t.Samples); j++ {
			if j < len(t.Samples)-1 {
				t.Samples[j].Duration = t.Samples[j+1].Time - t.Samples[j].Time
			} else {
				t.Samples[j].Duration = 33 // Default duration ~30 fps
			}
		}
		if len(t.Samples) > 0 {
			t.Duration = uint64(t.Samples[len(t.Samples)-1].Time + t.Samples[len(t.Samples)-1].Duration)
		}
	}

	d.interleaved = d.buildInterleavedSamples()
	d.currentSample = 0

	return d.tracks, nil
}

// ReadPacket returns the next interleaved packet.
func (d *FLVDemuxer) ReadPacket() (*core.Packet, error) {
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
func (d *FLVDemuxer) Close() error {
	return d.file.Close()
}

func (d *FLVDemuxer) buildInterleavedSamples() []core.InterleavedSample {
	var all []core.InterleavedSample
	for ti, t := range d.tracks {
		ts := float64(t.Timescale)
		if ts == 0 {
			ts = 1000
		}
		for si, s := range t.Samples {
			all = append(all, core.InterleavedSample{
				TrackIndex:  ti,
				SampleIndex: si,
				TimeSeconds: float64(s.Time) / ts,
				Sample:      s,
			})
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].TimeSeconds != all[j].TimeSeconds {
			return all[i].TimeSeconds < all[j].TimeSeconds
		}
		return all[i].TrackIndex < all[j].TrackIndex
	})

	return all
}
