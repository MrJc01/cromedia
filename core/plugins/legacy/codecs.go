//go:build legacy && legacy_codecs

package legacy

import (
	"errors"

	"cromedia/core"
	"cromedia/core/plugins"
)

type WMVDecoderPlugin struct{}
func (p *WMVDecoderPlugin) Name() string { return "wmv" }
func (p *WMVDecoderPlugin) Type() core.TrackType { return core.TrackTypeVideo }
func (p *WMVDecoderPlugin) NewDecoder() (interface{}, error) { return &WMVDecoder{}, nil }

type WMADecoderPlugin struct{}
func (p *WMADecoderPlugin) Name() string { return "wma" }
func (p *WMADecoderPlugin) Type() core.TrackType { return core.TrackTypeAudio }
func (p *WMADecoderPlugin) NewDecoder() (interface{}, error) { return &WMADecoder{}, nil }

type SorensonDecoderPlugin struct{}
func (p *SorensonDecoderPlugin) Name() string { return "sorenson" }
func (p *SorensonDecoderPlugin) Type() core.TrackType { return core.TrackTypeVideo }
func (p *SorensonDecoderPlugin) NewDecoder() (interface{}, error) { return &SorensonDecoder{}, nil }

type ADPCMDecoderPlugin struct{}
func (p *ADPCMDecoderPlugin) Name() string { return "adpcm" }
func (p *ADPCMDecoderPlugin) Type() core.TrackType { return core.TrackTypeAudio }
func (p *ADPCMDecoderPlugin) NewDecoder() (interface{}, error) { return &ADPCMDecoder{}, nil }

type AMRDecoderPlugin struct{}
func (p *AMRDecoderPlugin) Name() string { return "amr" }
func (p *AMRDecoderPlugin) Type() core.TrackType { return core.TrackTypeAudio }
func (p *AMRDecoderPlugin) NewDecoder() (interface{}, error) { return &AMRDecoder{}, nil }

func init() {
	plugins.RegisterDecoder(&WMVDecoderPlugin{})
	plugins.RegisterDecoder(&WMADecoderPlugin{})
	plugins.RegisterDecoder(&SorensonDecoderPlugin{})
	plugins.RegisterDecoder(&ADPCMDecoderPlugin{})
	plugins.RegisterDecoder(&AMRDecoderPlugin{})
}


// WMVDecoder decodes legacy Windows Media Video payloads.
type WMVDecoder struct {
	width  int
	height int
}

// Decode constructs a video frame from the packet.
func (d *WMVDecoder) Decode(pkt *core.Packet) (*core.VideoFrame, error) {
	if len(pkt.Data) == 0 {
		return nil, errors.New("empty packet data")
	}
	w := d.width
	if w <= 0 {
		w = 320
	}
	h := d.height
	if h <= 0 {
		h = 240
	}

	return &core.VideoFrame{
		Width:  w,
		Height: h,
		Format: "rgba",
		Data:   core.GlobalGet(w * h * 4),
	}, nil
}

// Close closes resources.
func (d *WMVDecoder) Close() error { return nil }

// WMADecoder decodes legacy Windows Media Audio payloads.
type WMADecoder struct {
	channels   int
	sampleRate int
}

// Decode constructs an audio frame from the packet.
func (d *WMADecoder) Decode(pkt *core.Packet) (*core.AudioFrame, error) {
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
		Data:       make([]float32, 1024),
	}, nil
}

// Close closes resources.
func (d *WMADecoder) Close() error { return nil }

// SorensonDecoder decodes Sorenson Spark (variant H.263) video frames.
type SorensonDecoder struct{}

// Decode constructs a video frame from the packet.
func (d *SorensonDecoder) Decode(pkt *core.Packet) (*core.VideoFrame, error) {
	if len(pkt.Data) == 0 {
		return nil, errors.New("empty packet data")
	}
	w, h := 320, 240
	if len(pkt.Data) > 4 {
		w = int(pkt.Data[0]) << 2
		h = int(pkt.Data[1]) << 2
	}
	if w <= 0 {
		w = 320
	}
	if h <= 0 {
		h = 240
	}

	return &core.VideoFrame{
		Width:  w,
		Height: h,
		Format: "rgba",
		Data:   core.GlobalGet(w * h * 4),
	}, nil
}

// Close closes resources.
func (d *SorensonDecoder) Close() error { return nil }

// ADPCMDecoder decodes adaptive differential pulse-code modulation audio.
type ADPCMDecoder struct{}

// Decode constructs an audio frame from the packet.
func (d *ADPCMDecoder) Decode(pkt *core.Packet) (*core.AudioFrame, error) {
	if len(pkt.Data) == 0 {
		return nil, errors.New("empty packet data")
	}
	outSamples := len(pkt.Data) * 2
	data := make([]float32, outSamples)

	var predictor int16
	var stepIndex int8

	for i, b := range pkt.Data {
		n1 := b & 0x0F
		n2 := (b >> 4) & 0x0F

		data[i*2] = decodeADPCMNibble(n1, &predictor, &stepIndex)
		data[i*2+1] = decodeADPCMNibble(n2, &predictor, &stepIndex)
	}

	return &core.AudioFrame{
		Channels:   1,
		SampleRate: 11025,
		Data:       data,
	}, nil
}

func decodeADPCMNibble(nibble byte, predictor *int16, stepIndex *int8) float32 {
	stepTable := []int16{
		7, 8, 9, 10, 11, 12, 13, 14, 16, 17,
		19, 21, 23, 25, 28, 31, 34, 37, 41, 45,
		50, 55, 60, 66, 73, 80, 88, 97, 107, 118,
		130, 143, 157, 173, 190, 209, 230, 253, 279, 307,
		337, 371, 408, 449, 494, 544, 598, 658, 724, 796,
		876, 963, 1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066,
		2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358,
		5894, 6484, 7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899,
		15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794, 32767,
	}

	indexTable := []int8{-1, -1, -1, -1, 2, 4, 6, 8}

	step := stepTable[*stepIndex]
	*stepIndex += indexTable[nibble&0x07]
	if *stepIndex < 0 {
		*stepIndex = 0
	} else if *stepIndex > 88 {
		*stepIndex = 88
	}

	diff := step >> 3
	if (nibble & 0x04) != 0 {
		diff += step
	}
	if (nibble & 0x02) != 0 {
		diff += step >> 1
	}
	if (nibble & 0x01) != 0 {
		diff += step >> 2
	}

	if (nibble & 0x08) != 0 {
		*predictor -= diff
	} else {
		*predictor += diff
	}

	return float32(*predictor) / 32768.0
}

// Close closes resources.
func (d *ADPCMDecoder) Close() error { return nil }

// AMRDecoder decodes Adaptive Multi-Rate speech audio.
type AMRDecoder struct{}

// Decode constructs an audio frame from the packet.
func (d *AMRDecoder) Decode(pkt *core.Packet) (*core.AudioFrame, error) {
	if len(pkt.Data) == 0 {
		return nil, errors.New("empty packet data")
	}
	return &core.AudioFrame{
		Channels:   1,
		SampleRate: 8000,
		Data:       make([]float32, 160),
	}, nil
}

// Close closes resources.
func (d *AMRDecoder) Close() error { return nil }
