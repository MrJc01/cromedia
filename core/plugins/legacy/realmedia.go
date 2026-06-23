//go:build legacy && legacy_rm

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

type RealMediaDemuxer struct {
	file       *os.File
	dataOffset int64
	dataSize   int64
	streams    []RMStream
	packetSize uint32
	pos        int64
}

type RMStream struct {
	ID    uint16
	Type  string // "audio" or "video"
	Codec string
}

func NewRealMediaDemuxer(file *os.File) (*RealMediaDemuxer, error) {
	d := &RealMediaDemuxer{file: file}
	if err := d.parseHeaders(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *RealMediaDemuxer) parseHeaders() error {
	var magic [4]byte
	if _, err := d.file.Read(magic[:]); err != nil {
		return err
	}
	if string(magic[:]) != ".RMF" {
		return errors.New("invalid RealMedia header magic")
	}

	// Skip size, version, and headers count (header is 14 bytes, magic was 4)
	_, _ = d.file.Seek(10, io.SeekCurrent)

	// Loop through chunks
	stat, err := d.file.Stat()
	if err != nil {
		return err
	}
	fileSize := stat.Size()

	for {
		pos, err := d.file.Seek(0, io.SeekCurrent)
		if err != nil || pos >= fileSize-8 {
			break
		}

		var chunkMagic [4]byte
		if _, err := d.file.Read(chunkMagic[:]); err != nil {
			break
		}
		var chunkSize uint32
		if err := binary.Read(d.file, binary.BigEndian, &chunkSize); err != nil {
			break
		}

		cType := string(chunkMagic[:])
		if cType == "PROP" {
			_, _ = d.file.Seek(int64(chunkSize)-8, io.SeekCurrent)
		} else if cType == "MDPR" {
			var version uint16
			_ = binary.Read(d.file, binary.BigEndian, &version)
			var streamNumber uint16
			_ = binary.Read(d.file, binary.BigEndian, &streamNumber)
			
			_, _ = d.file.Seek(28, io.SeekCurrent)
			
			var streamNameLen byte
			_ = binary.Read(d.file, binary.BigEndian, &streamNameLen)
			_, _ = d.file.Seek(int64(streamNameLen), io.SeekCurrent)
			
			var mimeTypeLen byte
			_ = binary.Read(d.file, binary.BigEndian, &mimeTypeLen)
			mimeBytes := make([]byte, mimeTypeLen)
			_, _ = d.file.Read(mimeBytes)
			mime := string(mimeBytes)

			s := RMStream{ID: streamNumber}
			if mime == "video/x-pn-realvideo" {
				s.Type = "video"
				s.Codec = "rv40"
			} else {
				s.Type = "audio"
				s.Codec = "cook"
			}
			d.streams = append(d.streams, s)

			rem := int64(chunkSize) - 8 - 2 - 2 - 28 - 1 - int64(streamNameLen) - 1 - int64(mimeTypeLen)
			if rem > 0 {
				_, _ = d.file.Seek(rem, io.SeekCurrent)
			}
		} else if cType == "DATA" {
			var version uint16
			_ = binary.Read(d.file, binary.BigEndian, &version)
			var numPackets uint32
			_ = binary.Read(d.file, binary.BigEndian, &numPackets)
			
			d.dataOffset, _ = d.file.Seek(0, io.SeekCurrent)
			d.dataSize = int64(chunkSize) - 14
			d.pos = d.dataOffset
			_, _ = d.file.Seek(d.dataSize, io.SeekCurrent)
		} else {
			_, _ = d.file.Seek(int64(chunkSize)-8, io.SeekCurrent)
		}
	}
	return nil
}

func (d *RealMediaDemuxer) Probe() ([]core.Track, error) {
	var tracks []core.Track
	for _, s := range d.streams {
		tType := core.TrackTypeVideo
		if s.Type == "audio" {
			tType = core.TrackTypeAudio
		}
		tracks = append(tracks, core.Track{
			ID:        int(s.ID),
			Type:      tType,
			Timescale: 1000,
			CodecTag:  s.Codec,
		})
	}
	return tracks, nil
}

func (d *RealMediaDemuxer) ReadPacket() (*core.Packet, error) {
	pos, err := d.file.Seek(d.pos, io.SeekStart)
	if err != nil || pos >= d.dataOffset+d.dataSize {
		return nil, io.EOF
	}

	var version uint16
	if err := binary.Read(d.file, binary.BigEndian, &version); err != nil {
		return nil, io.EOF
	}
	var length uint16
	_ = binary.Read(d.file, binary.BigEndian, &length)
	var streamNum uint16
	_ = binary.Read(d.file, binary.BigEndian, &streamNum)
	var timestamp uint32
	_ = binary.Read(d.file, binary.BigEndian, &timestamp)
	_, _ = d.file.Seek(2, io.SeekCurrent)

	payloadSize := int(length) - 12
	if payloadSize <= 0 {
		return nil, io.EOF
	}

	// Task 196: Discard invalid packets
	if payloadSize > 10*1024*1024 {
		_, _ = d.file.Seek(int64(payloadSize), io.SeekCurrent)
		d.pos += int64(length)
		return nil, errors.New("invalid RealMedia packet payload size")
	}

	data := make([]byte, payloadSize)
	n, err := d.file.Read(data)
	if err != nil || n < payloadSize {
		return nil, io.EOF
	}

	d.pos += int64(length)

	return &core.Packet{
		ID:          core.NewPacketID(),
		StreamIndex: int(streamNum),
		Data:        data,
		PTS:         int64(timestamp),
		DTS:         int64(timestamp),
		IsKeyframe:  true,
	}, nil
}

func (d *RealMediaDemuxer) Close() error {
	return d.file.Close()
}

type RealMediaDemuxerPlugin struct{}

func (p *RealMediaDemuxerPlugin) Name() string                     { return "rm" }
func (p *RealMediaDemuxerPlugin) Extensions() []string             { return []string{"rm", "rmvb"} }
func (p *RealMediaDemuxerPlugin) NewDemuxer(file string) (demux.Demuxer, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	return NewRealMediaDemuxer(f)
}

func init() {
	plugins.RegisterDemuxer(&RealMediaDemuxerPlugin{})
}
