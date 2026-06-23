package core

import (
	"math"
	"math/rand"
)

// AudioFilter defines the filter node interface for raw AudioFrames
type AudioFilter interface {
	Process(frame *AudioFrame) (*AudioFrame, error)
}

// VolumeFilter adjusts the gain/volume of audio samples
type VolumeFilter struct {
	Gain float32 // 1.0 = normal, 0.5 = half, 2.0 = double
}

func (f *VolumeFilter) Process(frame *AudioFrame) (*AudioFrame, error) {
	out := make([]float32, len(frame.Data))
	for i, v := range frame.Data {
		out[i] = v * f.Gain
	}
	return &AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// MuteFilter silences specified ranges or entire frame
type MuteFilter struct{}

func (f *MuteFilter) Process(frame *AudioFrame) (*AudioFrame, error) {
	return &AudioFrame{
		Channels:   frame.Channels,
		SampleRate: frame.SampleRate,
		Data:       make([]float32, len(frame.Data)),
	}, nil
}

// DelayFilter adds delay to audio samples
type DelayFilter struct {
	DelayMs    int
	SampleRate int
	buffer     []float32
}

func (f *DelayFilter) Process(frame *AudioFrame) (*AudioFrame, error) {
	delaySamples := (f.DelayMs * frame.SampleRate / 1000) * frame.Channels
	if len(f.buffer) < delaySamples {
		f.buffer = append(f.buffer, make([]float32, delaySamples-len(f.buffer))...)
	}

	out := make([]float32, len(frame.Data))
	for i := 0; i < len(frame.Data); i++ {
		out[i] = frame.Data[i] + f.buffer[i%delaySamples]
		f.buffer[i%delaySamples] = frame.Data[i]
	}
	return &AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// LowPassFilter simple IIR low-pass filter
type LowPassFilter struct {
	CutoffHz float64
	lastVal  []float32
}

func (f *LowPassFilter) Process(frame *AudioFrame) (*AudioFrame, error) {
	if len(f.lastVal) < frame.Channels {
		f.lastVal = make([]float32, frame.Channels)
	}

	dt := 1.0 / float64(frame.SampleRate)
	rc := 1.0 / (2 * math.Pi * f.CutoffHz)
	alpha := float32(dt / (rc + dt))

	out := make([]float32, len(frame.Data))
	for i := 0; i < len(frame.Data); i += frame.Channels {
		for ch := 0; ch < frame.Channels; ch++ {
			val := frame.Data[i+ch]
			f.lastVal[ch] = f.lastVal[ch] + alpha*(val-f.lastVal[ch])
			out[i+ch] = f.lastVal[ch]
		}
	}
	return &AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// FadeFilter applies Fade-In or Fade-Out to the frame
type FadeFilter struct {
	FadeIn  bool
	FadeOut bool
}

func (f *FadeFilter) Process(frame *AudioFrame) (*AudioFrame, error) {
	numSamples := len(frame.Data) / frame.Channels
	out := make([]float32, len(frame.Data))

	for i := 0; i < numSamples; i++ {
		var factor float32 = 1.0
		if f.FadeIn {
			factor = float32(i) / float32(numSamples)
		} else if f.FadeOut {
			factor = 1.0 - float32(i)/float32(numSamples)
		}

		for ch := 0; ch < frame.Channels; ch++ {
			out[i*frame.Channels+ch] = frame.Data[i*frame.Channels+ch] * factor
		}
	}
	return &AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// PeakNormalizer scales the entire frame based on peak amplitude
type PeakNormalizer struct {
	TargetDb float32 // target max peak e.g. -1.0dB
}

func (f *PeakNormalizer) Process(frame *AudioFrame) (*AudioFrame, error) {
	var maxVal float32 = 0.0
	for _, v := range frame.Data {
		abs := float32(math.Abs(float64(v)))
		if abs > maxVal {
			maxVal = abs
		}
	}

	if maxVal == 0 {
		return frame, nil
	}

	targetPeak := float32(math.Pow(10, float64(f.TargetDb)/20.0))
	gain := targetPeak / maxVal

	out := make([]float32, len(frame.Data))
	for i, v := range frame.Data {
		out[i] = v * gain
	}
	return &AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// SineGenerator generates a pure sine wave audio frame
func GenerateSineWave(freq float64, durationSec float64, sampleRate int, channels int) *AudioFrame {
	numSamples := int(durationSec * float64(sampleRate))
	data := make([]float32, numSamples*channels)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		val := float32(math.Sin(2 * math.Pi * freq * t))
		for ch := 0; ch < channels; ch++ {
			data[i*channels+ch] = val
		}
	}

	return &AudioFrame{
		Channels:   channels,
		SampleRate: sampleRate,
		Data:       data,
	}
}

// GenerateWhiteNoise generates random white noise audio frame
func GenerateWhiteNoise(durationSec float64, sampleRate int, channels int) *AudioFrame {
	numSamples := int(durationSec * float64(sampleRate))
	data := make([]float32, numSamples*channels)

	for i := 0; i < numSamples; i++ {
		val := float32(rand.Float64()*2.0 - 1.0)
		for ch := 0; ch < channels; ch++ {
			data[i*channels+ch] = val
		}
	}

	return &AudioFrame{
		Channels:   channels,
		SampleRate: sampleRate,
		Data:       data,
	}
}
