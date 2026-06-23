//go:build legacy && legacy_mp2

package legacy

import (
	"errors"
	"io"
	"os"

	"cromedia/core"
	"cromedia/core/demux"
	"cromedia/core/plugins"
)

type MP2Demuxer struct {
	file       *os.File
	sampleRate int
	channels   int
	bitrate    int
	pos        int64
	fileSize   int64
}

func NewMP2Demuxer(file *os.File) (*MP2Demuxer, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	d := &MP2Demuxer{file: file, fileSize: stat.Size()}
	if err := d.parseHeader(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *MP2Demuxer) parseHeader() error {
	buf := make([]byte, 4)
	for {
		pos, err := d.file.Seek(d.pos, io.SeekStart)
		if err != nil || pos >= d.fileSize-4 {
			return errors.New("no MP2 sync word found")
		}

		if _, err := d.file.Read(buf); err != nil {
			return err
		}

		if buf[0] == 0xFF && (buf[1]&0xE0) == 0xE0 && (buf[1]&0x06) == 0x04 {
			bitrateIndex := (buf[2] >> 4) & 0x0F
			sampleRateIndex := (buf[2] >> 2) & 0x03
			
			bitrates := []int{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0}
			sampleRates := []int{44100, 48000, 32000, 0}

			d.bitrate = bitrates[bitrateIndex] * 1000
			d.sampleRate = sampleRates[sampleRateIndex]
			if (buf[3] & 0xC0) == 0xC0 {
				d.channels = 1
			} else {
				d.channels = 2
			}
			break
		}
		d.pos++
	}
	return nil
}

func (d *MP2Demuxer) Probe() ([]core.Track, error) {
	return []core.Track{
		{
			ID:        0,
			Type:      core.TrackTypeAudio,
			Timescale: uint32(d.sampleRate),
			CodecTag:  "mp2",
		},
	}, nil
}

func (d *MP2Demuxer) ReadPacket() (*core.Packet, error) {
	pos, err := d.file.Seek(d.pos, io.SeekStart)
	if err != nil || pos >= d.fileSize-4 {
		return nil, io.EOF
	}

	buf := make([]byte, 4)
	if _, err := d.file.Read(buf); err != nil {
		return nil, io.EOF
	}

	if !(buf[0] == 0xFF && (buf[1]&0xE0) == 0xE0 && (buf[1]&0x06) == 0x04) {
		d.pos++
		return d.ReadPacket()
	}

	bitrateIndex := (buf[2] >> 4) & 0x0F
	sampleRateIndex := (buf[2] >> 2) & 0x03
	padding := (buf[2] >> 1) & 0x01

	bitrates := []int{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0}
	sampleRates := []int{44100, 48000, 32000, 0}

	br := bitrates[bitrateIndex] * 1000
	sr := sampleRates[sampleRateIndex]
	if br == 0 || sr == 0 {
		return nil, errors.New("invalid bitrate or sample rate in frame")
	}

	frameSize := 144*br/sr + int(padding)
	
	_, _ = d.file.Seek(d.pos, io.SeekStart)
	data := make([]byte, frameSize)
	n, err := d.file.Read(data)
	if err != nil || n < frameSize {
		return nil, io.EOF
	}

	d.pos += int64(frameSize)

	return &core.Packet{
		ID:          core.NewPacketID(),
		StreamIndex: 0,
		Data:        data,
		PTS:         d.pos,
		DTS:         d.pos,
		IsKeyframe:  true,
	}, nil
}

func (d *MP2Demuxer) Close() error {
	return d.file.Close()
}

type MP2DemuxerPlugin struct{}
func (p *MP2DemuxerPlugin) Name() string                     { return "mp2" }
func (p *MP2DemuxerPlugin) Extensions() []string             { return []string{"mp2"} }
func (p *MP2DemuxerPlugin) NewDemuxer(file string) (demux.Demuxer, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	return NewMP2Demuxer(f)
}

type MP2Decoder struct {
	channels   int
	sampleRate int
}

func (d *MP2Decoder) Decode(pkt *core.Packet) (*core.AudioFrame, error) {
	if len(pkt.Data) == 0 {
		return nil, errors.New("empty packet data")
	}
	ch := d.channels
	if ch <= 0 {
		ch = 2
	}
	sr := d.sampleRate
	if sr <= 0 {
		sr = 44100
	}
	return &core.AudioFrame{
		Channels:   ch,
		SampleRate: sr,
		Data:       make([]float32, 1152),
	}, nil
}

func (d *MP2Decoder) Close() error { return nil }

type MP2DecoderPlugin struct{}
func (p *MP2DecoderPlugin) Name() string                 { return "mp2" }
func (p *MP2DecoderPlugin) Type() core.TrackType         { return core.TrackTypeAudio }
func (p *MP2DecoderPlugin) NewDecoder() (interface{}, error) {
	return &MP2Decoder{}, nil
}

func init() {
	plugins.RegisterDemuxer(&MP2DemuxerPlugin{})
	plugins.RegisterDecoder(&MP2DecoderPlugin{})
}
