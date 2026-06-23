//go:build legacy && legacy_asf

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

type ASFDemuxerPlugin struct{}

func (p *ASFDemuxerPlugin) Name() string                     { return "asf" }
func (p *ASFDemuxerPlugin) Extensions() []string             { return []string{"asf", "wmv", "wma"} }
func (p *ASFDemuxerPlugin) NewDemuxer(file string) (demux.Demuxer, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	return NewASFDemuxer(f)
}

func init() {
	plugins.RegisterDemuxer(&ASFDemuxerPlugin{})
}


// GUID is a 16-byte ASF identifier.
type GUID [16]byte

var (
	ASFHeaderObjectGUID = GUID{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xEC, 0xE6}
	ASFDataObjectGUID   = GUID{0x36, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xEC, 0xE6}
	ASFFileProperties   = GUID{0xA1, 0xDC, 0xAB, 0x8C, 0x47, 0xA9, 0xCF, 0x11, 0x8E, 0xE4, 0x00, 0xC0, 0x0C, 0x20, 0x53, 0x65}
	ASFStreamProperties = GUID{0x91, 0x07, 0xDC, 0xB7, 0xB7, 0xA9, 0xCF, 0x11, 0x8E, 0xE6, 0x00, 0xC0, 0x0C, 0x20, 0x53, 0x65}
)

// ASFStream defines an internal ASF stream descriptor.
type ASFStream struct {
	ID        uint16
	Type      string // "audio" or "video"
	Codec     string
	TimeScale uint32
}

// ASFDemuxer parses Windows Media (ASF/WMV/WMA) container formats.
type ASFDemuxer struct {
	file       *os.File
	dataOffset int64
	dataSize   int64
	streams    []ASFStream
	packetSize uint32
	pos        int64
}

// NewASFDemuxer instantiates an ASFDemuxer.
func NewASFDemuxer(file *os.File) (*ASFDemuxer, error) {
	d := &ASFDemuxer{file: file}
	if err := d.parseHeader(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *ASFDemuxer) parseHeader() error {
	var g GUID
	if _, err := d.file.Read(g[:]); err != nil {
		return err
	}
	if g != ASFHeaderObjectGUID {
		return errors.New("not a valid ASF file: header GUID mismatch")
	}

	var size uint64
	var count uint32
	_ = binary.Read(d.file, binary.LittleEndian, &size)
	_ = binary.Read(d.file, binary.LittleEndian, &count)
	_, _ = d.file.Read(make([]byte, 2)) // Skip 2 bytes reserved

	// Parse sub-objects inside Header Object
	for i := uint32(0); i < count; i++ {
		var subG GUID
		var subSize uint64
		if _, err := d.file.Read(subG[:]); err != nil {
			break
		}
		_ = binary.Read(d.file, binary.LittleEndian, &subSize)

		if subG == ASFFileProperties {
			var filePropData [64]byte
			_, _ = d.file.Read(filePropData[:])
			d.packetSize = binary.LittleEndian.Uint32(filePropData[56:60])
		} else if subG == ASFStreamProperties {
			var streamType GUID
			_, _ = d.file.Read(streamType[:])
			var dummy GUID
			_, _ = d.file.Read(dummy[:])
			var timeOffset uint64
			_ = binary.Read(d.file, binary.LittleEndian, &timeOffset)
			var typeDataSize uint32
			_ = binary.Read(d.file, binary.LittleEndian, &typeDataSize)
			var errorDataSize uint32
			_ = binary.Read(d.file, binary.LittleEndian, &errorDataSize)
			var flags uint16
			_ = binary.Read(d.file, binary.LittleEndian, &flags)
			streamID := flags & 0x7F

			s := ASFStream{ID: streamID, TimeScale: 1000}
			// basic video check (video media type GUID starts with BC19EF81)
			if streamType[0] == 0x81 && streamType[1] == 0xEF {
				s.Type = "video"
				s.Codec = "wmv"
			} else {
				s.Type = "audio"
				s.Codec = "wma"
			}
			d.streams = append(d.streams, s)

			// skip remaining data of this block
			rem := int64(subSize) - 16 - 8 - 16 - 16 - 8 - 4 - 4 - 2
			if rem > 0 {
				_, _ = d.file.Seek(rem, io.SeekCurrent)
			}
		} else {
			// Skip unknown header sub-object
			if subSize > 24 {
				_, _ = d.file.Seek(int64(subSize)-24, io.SeekCurrent)
			}
		}
	}

	// Seek to data object
	var dataG GUID
	if _, err := d.file.Read(dataG[:]); err == nil && dataG == ASFDataObjectGUID {
		var dataSize uint64
		_ = binary.Read(d.file, binary.LittleEndian, &dataSize)
		pos, _ := d.file.Seek(0, io.SeekCurrent)
		d.dataOffset = pos + 16 // skip GUID and reserved bytes
		d.dataSize = int64(dataSize) - 26
		d.pos = d.dataOffset
	}

	return nil
}

// Probe maps parsed properties.
func (d *ASFDemuxer) Probe() ([]core.Track, error) {
	var tracks []core.Track
	for _, s := range d.streams {
		tType := core.TrackTypeVideo
		if s.Type == "audio" {
			tType = core.TrackTypeAudio
		}

		tracks = append(tracks, core.Track{
			ID:        int(s.ID),
			Type:      tType,
			Timescale: s.TimeScale,
			CodecTag:  s.Codec,
		})
	}
	return tracks, nil
}

// ReadPacket reads media packets from the data segment.
func (d *ASFDemuxer) ReadPacket() (*core.Packet, error) {
	if d.packetSize == 0 {
		return nil, io.EOF
	}

	pos, err := d.file.Seek(d.pos, io.SeekStart)
	if err != nil || pos >= d.dataOffset+d.dataSize {
		return nil, io.EOF
	}

	data := make([]byte, d.packetSize)
	n, err := d.file.Read(data)
	if err != nil || uint32(n) < d.packetSize {
		return nil, io.EOF
	}

	d.pos += int64(d.packetSize)

	// In ASF, packets are multiplexed. For basic decoding, we wrap the raw packet payload
	return &core.Packet{
		ID:          core.NewPacketID(),
		StreamIndex: 0,
		Data:        data,
		PTS:         d.pos / int64(d.packetSize),
		DTS:         d.pos / int64(d.packetSize),
		IsKeyframe:  true,
	}, nil
}

// Close closes the file descriptor.
func (d *ASFDemuxer) Close() error {
	return d.file.Close()
}
