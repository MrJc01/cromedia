//go:build legacy && legacy_avi

package legacy

import (
	"encoding/binary"
	"errors"
	"io"
	"os"

	"cromedia/core"
	"cromedia/core/demux"
	"cromedia/core/plugins"
)

type AVIDemuxerPlugin struct{}

func (p *AVIDemuxerPlugin) Name() string                     { return "avi" }
func (p *AVIDemuxerPlugin) Extensions() []string             { return []string{"avi"} }
func (p *AVIDemuxerPlugin) NewDemuxer(file string) (demux.Demuxer, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	return NewAVIDemuxer(f)
}

func init() {
	plugins.RegisterDemuxer(&AVIDemuxerPlugin{})
}


// AVIDemuxer parses AVI (Audio Video Interleaved) RIFF containers.
type AVIDemuxer struct {
	file     *os.File
	moviPos  int64
	moviSize int64
	streams  []AVIStream
	index    []AVIIndexEntry
	currIdx  int
}

// AVIStream contains metadata for a single video or audio stream in AVI.
type AVIStream struct {
	ID    int
	Type  string // "vids" or "auds"
	Codec string // FourCC
	Scale uint32
	Rate  uint32
	Width uint32
	Height uint32
}

// AVIIndexEntry represents a single frame index in the 'idx1' list.
type AVIIndexEntry struct {
	ChunkID string
	Flags   uint32
	Offset  int64 // Absolute file offset
	Size    uint32
}

// NewAVIDemuxer creates a demuxer from an open AVI file descriptor.
func NewAVIDemuxer(file *os.File) (*AVIDemuxer, error) {
	d := &AVIDemuxer{file: file}
	if err := d.parseHeaders(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *AVIDemuxer) parseHeaders() error {
	var fourcc [4]byte
	var size uint32

	// Read RIFF header
	if _, err := d.file.Read(fourcc[:]); err != nil {
		return err
	}
	if string(fourcc[:]) != "RIFF" {
		return errors.New("not a valid RIFF file")
	}

	if err := binary.Read(d.file, binary.LittleEndian, &size); err != nil {
		return err
	}

	if _, err := d.file.Read(fourcc[:]); err != nil {
		return err
	}
	if string(fourcc[:]) != "AVI " {
		return errors.New("not a valid AVI file")
	}

	// Seek and parse LIST chunks
	stat, err := d.file.Stat()
	if err != nil {
		return err
	}
	fileSize := stat.Size()

	var currentStream *AVIStream

	for {
		pos, err := d.file.Seek(0, io.SeekCurrent)
		if err != nil || pos >= fileSize-8 {
			break
		}

		if _, err := d.file.Read(fourcc[:]); err != nil {
			break
		}
		if err := binary.Read(d.file, binary.LittleEndian, &size); err != nil {
			break
		}

		tag := string(fourcc[:])
		if tag == "LIST" {
			var listType [4]byte
			if _, err := d.file.Read(listType[:]); err != nil {
				break
			}
			lType := string(listType[:])
			if lType == "movi" {
				d.moviPos = pos + 12
				d.moviSize = int64(size) - 4
				// Skip LIST movi data for now
				_, _ = d.file.Seek(int64(size)-4, io.SeekCurrent)
			} else if lType == "strl" {
				stream := AVIStream{ID: len(d.streams)}
				d.streams = append(d.streams, stream)
				currentStream = &d.streams[len(d.streams)-1]
			}
		} else if tag == "strh" && currentStream != nil {
			var strhData [48]byte
			if _, err := d.file.Read(strhData[:]); err == nil {
				currentStream.Type = string(strhData[0:4])
				currentStream.Codec = string(strhData[8:12])
				currentStream.Scale = binary.LittleEndian.Uint32(strhData[20:24])
				currentStream.Rate = binary.LittleEndian.Uint32(strhData[24:28])
			}
		} else if tag == "strf" && currentStream != nil {
			if currentStream.Type == "vids" {
				var strfData [40]byte
				if _, err := d.file.Read(strfData[:]); err == nil {
					currentStream.Width = binary.LittleEndian.Uint32(strfData[4:8])
					currentStream.Height = binary.LittleEndian.Uint32(strfData[8:12])
				}
			} else {
				_, _ = d.file.Seek(int64(size), io.SeekCurrent)
			}
		} else if tag == "idx1" {
			entryCount := int(size) / 16
			for i := 0; i < entryCount; i++ {
				var entry [16]byte
				if _, err := d.file.Read(entry[:]); err != nil {
					break
				}
				offset := int64(binary.LittleEndian.Uint32(entry[8:12]))
				// Offset inside idx1 can be absolute or relative to moviPos.
				// We adjust it based on absolute verification.
				absOffset := offset
				if offset < d.moviPos {
					absOffset = d.moviPos + offset
				}
				d.index = append(d.index, AVIIndexEntry{
					ChunkID: string(entry[0:4]),
					Flags:   binary.LittleEndian.Uint32(entry[4:8]),
					Offset:  absOffset,
					Size:    binary.LittleEndian.Uint32(entry[12:16]),
				})
			}
		} else {
			// Skip unhandled chunk
			_, _ = d.file.Seek(int64(size), io.SeekCurrent)
		}
	}
	return nil
}

// Probe returns tracks and stream configuration.
func (d *AVIDemuxer) Probe() ([]core.Track, error) {
	var tracks []core.Track
	for _, stream := range d.streams {
		tType := core.TrackTypeVideo
		codecTag := stream.Codec
		if stream.Type == "auds" {
			tType = core.TrackTypeAudio
			codecTag = "pcm"
		}

		timescale := stream.Rate
		if timescale == 0 {
			timescale = 25
		}

		var samples []core.Sample
		sampleIdx := 0
		var pts int64

		for _, idxEntry := range d.index {
			// ChunkIDs look like "00db", "01wb"
			streamNum := int(idxEntry.ChunkID[0]-'0')*10 + int(idxEntry.ChunkID[1]-'0')
			if streamNum == stream.ID {
				isKey := (idxEntry.Flags & 0x10) != 0 // AVIIF_KEYFRAME
				samples = append(samples, core.Sample{
					ID:         sampleIdx + 1,
					IsKeyframe: isKey,
					Offset:     idxEntry.Offset + 8, // offset after chunk id + size
					Size:       int64(idxEntry.Size),
					Time:       pts,
					Duration:   int64(stream.Scale),
				})
				pts += int64(stream.Scale)
				sampleIdx++
			}
		}

		track := core.Track{
			ID:        stream.ID,
			Type:      tType,
			Timescale: timescale,
			Width:     stream.Width,
			Height:    stream.Height,
			CodecTag:  codecTag,
			Samples:   samples,
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}

// ReadPacket reads frames using index offsets.
func (d *AVIDemuxer) ReadPacket() (*core.Packet, error) {
	if d.currIdx >= len(d.index) {
		return nil, io.EOF
	}

	entry := d.index[d.currIdx]
	d.currIdx++

	// Read frame payload
	_, err := d.file.Seek(entry.Offset+8, io.SeekStart)
	if err != nil {
		return nil, err
	}

	data := make([]byte, entry.Size)
	if _, err := d.file.Read(data); err != nil {
		return nil, err
	}

	streamNum := int(entry.ChunkID[0]-'0')*10 + int(entry.ChunkID[1]-'0')
	isKey := (entry.Flags & 0x10) != 0

	return &core.Packet{
		ID:          core.NewPacketID(),
		StreamIndex: streamNum,
		Data:        data,
		PTS:         int64(d.currIdx), // simple frame count for stream PTS
		DTS:         int64(d.currIdx),
		IsKeyframe:  isKey,
	}, nil
}

// Seek seeks to the nearest keyframe at or before the given PTS (based on stream 0).
func (d *AVIDemuxer) Seek(pts int64) error {
	targetIdx := -1
	var stream0Count int64
	for i, entry := range d.index {
		streamNum := int(entry.ChunkID[0]-'0')*10 + int(entry.ChunkID[1]-'0')
		if streamNum == 0 {
			if stream0Count <= pts {
				targetIdx = i
			}
			stream0Count++
		}
	}
	if targetIdx == -1 {
		d.currIdx = 0
		return nil
	}
	for i := targetIdx; i >= 0; i-- {
		entry := d.index[i]
		isKey := (entry.Flags & 0x10) != 0
		if isKey {
			d.currIdx = i
			return nil
		}
	}
	d.currIdx = targetIdx
	return nil
}

// Close closes the file descriptor.
func (d *AVIDemuxer) Close() error {
	return d.file.Close()
}
