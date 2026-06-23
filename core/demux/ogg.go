package demux

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"sort"

	"cromedia/core"
)

// OggDemuxer handles demuxing of Ogg (.ogg) files.
type OggDemuxer struct {
	file          *os.File
	tracks        []core.Track
	interleaved   []core.InterleavedSample
	currentSample int
}

// NewOggDemuxer instantiates a new OggDemuxer.
func NewOggDemuxer(file *os.File) *OggDemuxer {
	return &OggDemuxer{file: file}
}

type oggTrackState struct {
	track       *core.Track
	packetCount int64
	lastGranule int64
}

// Probe parses the Ogg pages and reassembles packets/samples.
func (d *OggDemuxer) Probe() ([]core.Track, error) {
	info, err := d.file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	tracksMap := make(map[uint32]*oggTrackState)
	var offset int64 = 0

	pageHdr := make([]byte, 27)
	for offset+27 <= fileSize {
		if _, err := d.file.Seek(offset, io.SeekStart); err != nil {
			break
		}
		if _, err := io.ReadFull(d.file, pageHdr); err != nil {
			break
		}

		if pageHdr[0] != 'O' || pageHdr[1] != 'g' || pageHdr[2] != 'g' || pageHdr[3] != 'S' {
			// Search for sync word if not aligned
			offset++
			continue
		}

		version := pageHdr[4]
		if version != 0 {
			offset += 4
			continue
		}

		granulePos := int64(binary.LittleEndian.Uint64(pageHdr[6:14]))
		serial := binary.LittleEndian.Uint32(pageHdr[14:18])
		nSegments := int(pageHdr[26])

		segmentTable := make([]byte, nSegments)
		if _, err := io.ReadFull(d.file, segmentTable); err != nil {
			break
		}

		var payloadSize int64 = 0
		for _, lenByte := range segmentTable {
			payloadSize += int64(lenByte)
		}

		state, exists := tracksMap[serial]
		if !exists {
			// Read the first page first packet to identify codec
			var firstPacket []byte
			if payloadSize > 0 {
				firstPacket = make([]byte, segmentTable[0])
				_, _ = d.file.ReadAt(firstPacket, offset+27+int64(nSegments))
			}

			var trackType core.TrackType = core.TrackTypeAudio
			var codecTag = "opus"
			if bytes.HasPrefix(firstPacket, []byte("OpusHead")) {
				codecTag = "opus"
			} else if bytes.HasPrefix(firstPacket, []byte("\x01vorbis")) {
				codecTag = "vorbis"
			} else if bytes.HasPrefix(firstPacket, []byte("\x80theora")) {
				codecTag = "theora"
				trackType = core.TrackTypeVideo
			} else if bytes.HasPrefix(firstPacket, []byte("fLaC")) || bytes.HasPrefix(firstPacket, []byte("\x7fFLAC")) {
				codecTag = "flac"
			}

			d.tracks = append(d.tracks, core.Track{
				ID:        int(serial),
				Type:      trackType,
				Timescale: 48000, // Default to 48kHz for Ogg
				CodecTag:  codecTag,
			})
			tracksMap[serial] = &oggTrackState{
				track:       &d.tracks[len(d.tracks)-1],
				packetCount: 0,
				lastGranule: 0,
			}
			state = tracksMap[serial]
		}

		// Reassemble packets
		var packetOffset = offset + 27 + int64(nSegments)
		var curPacketSize int64 = 0
		var packetStartOffset = packetOffset

		for _, lenByte := range segmentTable {
			curPacketSize += int64(lenByte)
			if lenByte < 255 {
				// Packet completed
				sampleTime := granulePos
				if sampleTime < 0 || sampleTime == -1 {
					// Fallback to linear calculation
					sampleTime = state.packetCount * 960 // Assumed frame size for 48kHz (20ms)
				}

				sample := core.Sample{
					ID:         len(state.track.Samples) + 1,
					IsKeyframe: true, // Audio packets are random access points
					Offset:     packetStartOffset,
					Size:       curPacketSize,
					Time:       sampleTime,
					Duration:   0,
				}
				state.track.Samples = append(state.track.Samples, sample)
				state.packetCount++

				packetOffset += curPacketSize
				packetStartOffset = packetOffset
				curPacketSize = 0
			} else {
				packetOffset += int64(lenByte)
			}
		}

		offset += 27 + int64(nSegments) + payloadSize
	}

	// Calculate durations
	for i := range d.tracks {
		t := &d.tracks[i]
		for j := 0; j < len(t.Samples); j++ {
			if j < len(t.Samples)-1 {
				t.Samples[j].Duration = t.Samples[j+1].Time - t.Samples[j].Time
			} else {
				t.Samples[j].Duration = 960 // Default duration (20ms at 48kHz)
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
func (d *OggDemuxer) ReadPacket() (*core.Packet, error) {
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
func (d *OggDemuxer) Close() error {
	return d.file.Close()
}

func (d *OggDemuxer) buildInterleavedSamples() []core.InterleavedSample {
	var all []core.InterleavedSample
	for ti, t := range d.tracks {
		ts := float64(t.Timescale)
		if ts == 0 {
			ts = 48000
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
