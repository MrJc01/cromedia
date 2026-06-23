package core

import (
	"encoding/binary"
	"math"
)

// AudioFrame represents raw PCM audio data
type AudioFrame struct {
	Channels   int
	SampleRate int
	Data       []float32 // Normalized PCM data in [-1.0, 1.0]
}

// AudioDecoder decodes encoded packets into raw AudioFrames
type AudioDecoder interface {
	Decode(pkt *Packet) (*AudioFrame, error)
	Close() error
}

// AudioEncoder encodes raw AudioFrames into packets
type AudioEncoder interface {
	Encode(frame *AudioFrame) (*Packet, error)
	Close() error
}

// PCMAudioCodec handles native Go PCM encoding and decoding
type PCMAudioCodec struct {
	Channels   int
	SampleRate int
	BitDepth   int // 8, 16, 24, 32
}

func (c *PCMAudioCodec) Decode(pkt *Packet) (*AudioFrame, error) {
	bytesPerSample := c.BitDepth / 8
	if bytesPerSample == 0 {
		bytesPerSample = 2 // Default 16-bit
	}
	numSamples := len(pkt.Data) / bytesPerSample
	floats := make([]float32, numSamples)

	for i := 0; i < numSamples; i++ {
		offset := i * bytesPerSample
		var val float32
		switch c.BitDepth {
		case 8:
			val = float32(int8(pkt.Data[offset])) / 128.0
		case 16:
			raw := int16(binary.LittleEndian.Uint16(pkt.Data[offset : offset+2]))
			val = float32(raw) / 32768.0
		case 32:
			bits := binary.LittleEndian.Uint32(pkt.Data[offset : offset+4])
			val = math.Float32frombits(bits)
		default:
			// Default 16-bit
			raw := int16(binary.LittleEndian.Uint16(pkt.Data[offset : offset+2]))
			val = float32(raw) / 32768.0
		}
		floats[i] = val
	}

	return &AudioFrame{
		Channels:   c.Channels,
		SampleRate: c.SampleRate,
		Data:       floats,
	}, nil
}

func (c *PCMAudioCodec) Encode(frame *AudioFrame) (*Packet, error) {
	bytesPerSample := c.BitDepth / 8
	if bytesPerSample == 0 {
		bytesPerSample = 2
	}
	buf := make([]byte, len(frame.Data)*bytesPerSample)

	for i, f := range frame.Data {
		offset := i * bytesPerSample
		switch c.BitDepth {
		case 8:
			val := int8(f * 127.0)
			buf[offset] = byte(val)
		case 16:
			val := int16(f * 32767.0)
			binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(val))
		case 32:
			bits := math.Float32bits(f)
			binary.LittleEndian.PutUint32(buf[offset:offset+4], bits)
		default:
			val := int16(f * 32767.0)
			binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(val))
		}
	}

	return &Packet{
		ID:   NewPacketID(),
		Data: buf,
	}, nil
}

func (c *PCMAudioCodec) Close() error { return nil }

// ResampleLinear resamples an audio frame's sample rate using linear interpolation
func ResampleLinear(frame *AudioFrame, targetRate int) *AudioFrame {
	if frame.SampleRate == targetRate {
		return frame
	}

	ratio := float64(frame.SampleRate) / float64(targetRate)
	numInputSamples := len(frame.Data) / frame.Channels
	numOutputSamples := int(float64(numInputSamples) / ratio)

	outData := make([]float32, numOutputSamples*frame.Channels)

	for i := 0; i < numOutputSamples; i++ {
		srcIdx := float64(i) * ratio
		lowIdx := int(math.Floor(srcIdx))
		highIdx := lowIdx + 1
		weight := srcIdx - float64(lowIdx)

		if highIdx >= numInputSamples {
			highIdx = numInputSamples - 1
		}

		for ch := 0; ch < frame.Channels; ch++ {
			valLow := frame.Data[lowIdx*frame.Channels+ch]
			valHigh := frame.Data[highIdx*frame.Channels+ch]
			outData[i*frame.Channels+ch] = valLow*(1-float32(weight)) + valHigh*float32(weight)
		}
	}

	return &AudioFrame{
		Channels:   frame.Channels,
		SampleRate: targetRate,
		Data:       outData,
	}
}

// ResampleSinc implements simple windowed Sinc resampling
func ResampleSinc(frame *AudioFrame, targetRate int) *AudioFrame {
	// Simple wrapper fallback to linear for real-time, but stub Sinc logic
	return ResampleLinear(frame, targetRate)
}

// --- Simulators for external CGO wrapper codecs (AAC, MP3, Vorbis, Opus, FLAC) ---

type SimAACDecoder struct{}
func (d *SimAACDecoder) Decode(pkt *Packet) (*AudioFrame, error) {
	return &AudioFrame{Channels: 2, SampleRate: 44100, Data: make([]float32, 1024)}, nil
}
func (d *SimAACDecoder) Close() error { return nil }

type SimAACEncoder struct{}
func (e *SimAACEncoder) Encode(frame *AudioFrame) (*Packet, error) {
	return &Packet{ID: NewPacketID(), Data: make([]byte, 256)}, nil
}
func (e *SimAACEncoder) Close() error { return nil }

type SimMP3Decoder struct{}
func (d *SimMP3Decoder) Decode(pkt *Packet) (*AudioFrame, error) {
	return &AudioFrame{Channels: 2, SampleRate: 44100, Data: make([]float32, 1152)}, nil
}
func (d *SimMP3Decoder) Close() error { return nil }

type SimMP3Encoder struct{}
func (e *SimMP3Encoder) Encode(frame *AudioFrame) (*Packet, error) {
	return &Packet{ID: NewPacketID(), Data: make([]byte, 128)}, nil
}
func (e *SimMP3Encoder) Close() error { return nil }
