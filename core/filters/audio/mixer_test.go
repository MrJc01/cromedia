package audio

import (
	"math"
	"testing"

	"cromedia/core"
)

func TestMixAudioFrames(t *testing.T) {
	// 1. Basic Mix (Stereo, same samplerate)
	f1 := &core.AudioFrame{
		Channels:   2,
		SampleRate: 44100,
		Data:       []float32{0.5, -0.2},
	}
	f2 := &core.AudioFrame{
		Channels:   2,
		SampleRate: 44100,
		Data:       []float32{0.3, 0.4},
	}

	mixed := MixAudioFrames(f1, f2, 1.0, 1.0, false)
	if mixed.Channels != 2 || mixed.SampleRate != 44100 {
		t.Errorf("Expected 2 channels, 44100Hz, got %d channels, %dHz", mixed.Channels, mixed.SampleRate)
	}
	if math.Abs(float64(mixed.Data[0]-0.8)) > 1e-6 || math.Abs(float64(mixed.Data[1]-0.2)) > 1e-6 {
		t.Errorf("Expected mix data [0.8, 0.2], got %v", mixed.Data)
	}

	// 2. Volume independent control
	mixedVol := MixAudioFrames(f1, f2, 0.5, 2.0, false) // 0.5 * 0.5 + 2.0 * 0.3 = 0.25 + 0.6 = 0.85
	if math.Abs(float64(mixedVol.Data[0]-0.85)) > 1e-6 {
		t.Errorf("Expected volume-mixed sample 0.85, got %f", mixedVol.Data[0])
	}

	// 3. Clipping and Soft Limiter check
	fHigh1 := &core.AudioFrame{Channels: 1, SampleRate: 8000, Data: []float32{0.9}}
	fHigh2 := &core.AudioFrame{Channels: 1, SampleRate: 8000, Data: []float32{0.8}}

	// Without soft limiter (hard clipping) -> 0.9 + 0.8 = 1.7 -> clipped to 1.0
	mixedHard := MixAudioFrames(fHigh1, fHigh2, 1.0, 1.0, false)
	if mixedHard.Data[0] != 1.0 {
		t.Errorf("Expected hard clipping to clamp at 1.0, got %f", mixedHard.Data[0])
	}

	// With soft limiter -> 1.7 is compressed to < 1.0
	mixedSoft := MixAudioFrames(fHigh1, fHigh2, 1.0, 1.0, true)
	if mixedSoft.Data[0] >= 1.0 || mixedSoft.Data[0] <= 0.75 {
		t.Errorf("Expected soft limiter to compress 1.7 between 0.75 and 1.0, got %f", mixedSoft.Data[0])
	}

	// 4. Channel Matching (Mono to Stereo)
	fMono := &core.AudioFrame{Channels: 1, SampleRate: 8000, Data: []float32{0.5}}
	fStereo := &core.AudioFrame{Channels: 2, SampleRate: 8000, Data: []float32{0.2, 0.3}}

	mixedMonoStereo := MixAudioFrames(fStereo, fMono, 1.0, 1.0, false)
	if mixedMonoStereo.Channels != 2 {
		t.Errorf("Expected mixed output to be stereo (2 channels), got %d", mixedMonoStereo.Channels)
	}
	if math.Abs(float64(mixedMonoStereo.Data[0]-0.7)) > 1e-6 || math.Abs(float64(mixedMonoStereo.Data[1]-0.8)) > 1e-6 {
		t.Errorf("Expected [0.7, 0.8], got %v", mixedMonoStereo.Data)
	}

	// 5. Channel Matching (Stereo to Mono)
	mixedStereoMono := MixAudioFrames(fMono, fStereo, 1.0, 1.0, false)
	if mixedStereoMono.Channels != 1 {
		t.Errorf("Expected mixed output to be mono (1 channel), got %d", mixedStereoMono.Channels)
	}
	if math.Abs(float64(mixedStereoMono.Data[0]-0.75)) > 1e-6 {
		t.Errorf("Expected 0.75, got %v", mixedStereoMono.Data)
	}
}
