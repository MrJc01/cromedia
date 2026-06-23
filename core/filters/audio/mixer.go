package audio

import (
	"math"

	"cromedia/core"
)

// MixAudioFrames mixes two audio frames together with independent volumes and a limiter.
func MixAudioFrames(f1, f2 *core.AudioFrame, vol1, vol2 float32, useSoftLimiter bool) *core.AudioFrame {
	if f1 == nil && f2 == nil {
		return nil
	}
	if f1 == nil {
		out := make([]float32, len(f2.Data))
		for i, v := range f2.Data {
			out[i] = v * vol2
			if out[i] > 1.0 {
				out[i] = 1.0
			} else if out[i] < -1.0 {
				out[i] = -1.0
			}
		}
		return &core.AudioFrame{Channels: f2.Channels, SampleRate: f2.SampleRate, Data: out}
	}
	if f2 == nil {
		out := make([]float32, len(f1.Data))
		for i, v := range f1.Data {
			out[i] = v * vol1
			if out[i] > 1.0 {
				out[i] = 1.0
			} else if out[i] < -1.0 {
				out[i] = -1.0
			}
		}
		return &core.AudioFrame{Channels: f1.Channels, SampleRate: f1.SampleRate, Data: out}
	}

	// 1. Resample f2 if sample rates differ
	targetRate := f1.SampleRate
	f2Resampled := f2
	if f2.SampleRate != targetRate {
		f2Resampled = core.ResampleLinear(f2, targetRate)
	}

	// 2. Channel matching (mono to stereo or stereo to mono)
	targetChannels := f1.Channels
	f2Matched := f2Resampled
	if f2Resampled.Channels != targetChannels {
		if targetChannels == 2 && f2Resampled.Channels == 1 {
			// Mono to Stereo: duplicate each mono sample to both channels
			newData := make([]float32, len(f2Resampled.Data)*2)
			for i := 0; i < len(f2Resampled.Data); i++ {
				newData[i*2] = f2Resampled.Data[i]
				newData[i*2+1] = f2Resampled.Data[i]
			}
			f2Matched = &core.AudioFrame{Channels: 2, SampleRate: targetRate, Data: newData}
		} else if targetChannels == 1 && f2Resampled.Channels == 2 {
			// Stereo to Mono: average left and right channels
			newData := make([]float32, len(f2Resampled.Data)/2)
			for i := 0; i < len(newData); i++ {
				newData[i] = (f2Resampled.Data[i*2] + f2Resampled.Data[i*2+1]) * 0.5
			}
			f2Matched = &core.AudioFrame{Channels: 1, SampleRate: targetRate, Data: newData}
		}
	}

	// 3. Perform the sample-by-sample mix
	maxLen := len(f1.Data)
	if len(f2Matched.Data) > maxLen {
		maxLen = len(f2Matched.Data)
	}

	outData := make([]float32, maxLen)
	for i := 0; i < maxLen; i++ {
		var v1, v2 float32
		if i < len(f1.Data) {
			v1 = f1.Data[i] * vol1
		}
		if i < len(f2Matched.Data) {
			v2 = f2Matched.Data[i] * vol2
		}

		sum := v1 + v2
		if useSoftLimiter {
			// C1-continuous asymptotic soft limiter above 0.75 threshold
			absSum := float32(math.Abs(float64(sum)))
			if absSum > 0.75 {
				sig := float32(1.0)
				if sum < 0 {
					sig = -1.0
				}
				x := absSum - 0.75
				sum = sig * (0.75 + 0.25*(x/(0.25+x)))
			}
		}

		// Hard clipping protection
		if sum > 1.0 {
			sum = 1.0
		} else if sum < -1.0 {
			sum = -1.0
		}

		outData[i] = sum
	}

	return &core.AudioFrame{
		Channels:   targetChannels,
		SampleRate: targetRate,
		Data:       outData,
	}
}
