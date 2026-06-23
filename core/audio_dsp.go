package core

import (
	"math"
	"sync"
)

// SincResampler implements high-quality audio resampling using windowed sinc interpolation
// with pre-computed LUT coefficients for performance.
// Addresses expert criticism: "Sinc Resampling Otimizado" — uses pre-calculated coefficient
// tables to achieve precision without CPU degradation.
type SincResampler struct {
	InputRate  int
	OutputRate int
	Quality    int     // Number of sinc filter taps (higher = better quality, more CPU)
	lut        []float64 // Pre-computed sinc coefficients
	lutOnce    sync.Once
}

// NewSincResampler creates a resampler with the specified quality (recommended: 32-128 taps).
func NewSincResampler(inputRate, outputRate, quality int) *SincResampler {
	if quality <= 0 {
		quality = 64 // default
	}
	sr := &SincResampler{
		InputRate:  inputRate,
		OutputRate: outputRate,
		Quality:    quality,
	}
	sr.initLUT()
	return sr
}

// initLUT pre-computes the windowed sinc filter coefficients.
// Uses a Blackman-Harris window for excellent sidelobe suppression.
func (sr *SincResampler) initLUT() {
	sr.lutOnce.Do(func() {
		// LUT resolution: 256 sub-sample phases × quality taps
		const phases = 256
		taps := sr.Quality
		sr.lut = make([]float64, phases*taps)

		halfTaps := float64(taps) / 2.0
		for p := 0; p < phases; p++ {
			frac := float64(p) / float64(phases) // fractional phase [0, 1)
			for t := 0; t < taps; t++ {
				x := float64(t) - halfTaps + frac
				// Normalized sinc function
				var sincVal float64
				if math.Abs(x) < 1e-10 {
					sincVal = 1.0
				} else {
					sincVal = math.Sin(math.Pi*x) / (math.Pi * x)
				}
				// Blackman-Harris window
				n := float64(t) / float64(taps-1)
				window := 0.35875 - 0.48829*math.Cos(2*math.Pi*n) +
					0.14128*math.Cos(4*math.Pi*n) - 0.01168*math.Cos(6*math.Pi*n)
				sr.lut[p*taps+t] = sincVal * window
			}
		}
	})
}

// Process resamples an AudioFrame from InputRate to OutputRate using windowed sinc interpolation.
func (sr *SincResampler) Process(frame *AudioFrame) (*AudioFrame, error) {
	if sr.InputRate == sr.OutputRate {
		return frame, nil
	}

	channels := frame.Channels
	inSamples := len(frame.Data) / channels
	outSamples := int(float64(inSamples) * float64(sr.OutputRate) / float64(sr.InputRate))
	outData := make([]float32, outSamples*channels)

	ratio := float64(sr.InputRate) / float64(sr.OutputRate)
	taps := sr.Quality
	halfTaps := taps / 2
	const phases = 256

	for ch := 0; ch < channels; ch++ {
		for i := 0; i < outSamples; i++ {
			srcPos := float64(i) * ratio
			srcIdx := int(srcPos)
			frac := srcPos - float64(srcIdx)

			// Determine LUT phase index
			phase := int(frac * float64(phases))
			if phase >= phases {
				phase = phases - 1
			}

			var acc float64
			for t := 0; t < taps; t++ {
				sampleIdx := srcIdx - halfTaps + t
				if sampleIdx < 0 {
					sampleIdx = 0
				} else if sampleIdx >= inSamples {
					sampleIdx = inSamples - 1
				}
				acc += float64(frame.Data[sampleIdx*channels+ch]) * sr.lut[phase*taps+t]
			}

			// Clamp to [-1.0, 1.0]
			if acc > 1.0 {
				acc = 1.0
			} else if acc < -1.0 {
				acc = -1.0
			}
			outData[i*channels+ch] = float32(acc)
		}
	}

	return &AudioFrame{
		Channels:   channels,
		SampleRate: sr.OutputRate,
		Data:       outData,
	}, nil
}

// PredictiveGainNormalizer implements single-pass real-time audio normalization
// using a predictive gain estimator based on exponential moving average of peak levels.
// Addresses expert criticism: "Normalização em Tempo Real (Gain Estimators)" —
// eliminates the need for a double-pass by estimating gain from historical peaks.
type PredictiveGainNormalizer struct {
	TargetPeakDb float32 // Target peak in dB (e.g., -1.0)
	Alpha        float32 // EMA smoothing factor (0.01-0.1, lower = smoother)
	peakEMA      float32 // Running exponential moving average of observed peaks
	initialized  bool
}

// NewPredictiveGainNormalizer creates a normalizer with adaptive gain estimation.
func NewPredictiveGainNormalizer(targetDb float32, alpha float32) *PredictiveGainNormalizer {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.05
	}
	return &PredictiveGainNormalizer{
		TargetPeakDb: targetDb,
		Alpha:        alpha,
	}
}

// Process normalizes audio in a single pass using predictive gain estimation.
func (n *PredictiveGainNormalizer) Process(frame *AudioFrame) (*AudioFrame, error) {
	// Find current frame peak
	var currentPeak float32
	for _, v := range frame.Data {
		abs := float32(math.Abs(float64(v)))
		if abs > currentPeak {
			currentPeak = abs
		}
	}

	if currentPeak < 1e-10 {
		return frame, nil // Silent frame
	}

	// Update exponential moving average of peak levels
	if !n.initialized {
		n.peakEMA = currentPeak
		n.initialized = true
	} else {
		n.peakEMA = n.Alpha*currentPeak + (1-n.Alpha)*n.peakEMA
	}

	// Calculate gain from predicted peak
	targetPeak := float32(math.Pow(10, float64(n.TargetPeakDb)/20.0))
	gain := targetPeak / n.peakEMA

	// Apply gain with soft-knee limiter to prevent clipping
	out := make([]float32, len(frame.Data))
	for i, v := range frame.Data {
		val := v * gain
		// Soft-knee tanh limiter for clean saturation
		if math.Abs(float64(val)) > 0.95 {
			val = float32(math.Tanh(float64(val)))
		}
		out[i] = val
	}

	return &AudioFrame{
		Channels:   frame.Channels,
		SampleRate: frame.SampleRate,
		Data:       out,
	}, nil
}

// HighPassFilter is the complement to LowPassFilter for audio processing.
type HighPassFilter struct {
	CutoffHz float64
	lastVal  []float32
	lastIn   []float32
}

func (f *HighPassFilter) Process(frame *AudioFrame) (*AudioFrame, error) {
	if len(f.lastVal) < frame.Channels {
		f.lastVal = make([]float32, frame.Channels)
		f.lastIn = make([]float32, frame.Channels)
	}

	dt := 1.0 / float64(frame.SampleRate)
	rc := 1.0 / (2 * math.Pi * f.CutoffHz)
	alpha := float32(rc / (rc + dt))

	out := make([]float32, len(frame.Data))
	for i := 0; i < len(frame.Data); i += frame.Channels {
		for ch := 0; ch < frame.Channels; ch++ {
			val := frame.Data[i+ch]
			f.lastVal[ch] = alpha * (f.lastVal[ch] + val - f.lastIn[ch])
			f.lastIn[ch] = val
			out[i+ch] = f.lastVal[ch]
		}
	}
	return &AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// DynamicRangeCompressor implements a simple dynamic range compressor with attack/release.
type DynamicRangeCompressor struct {
	ThresholdDb float32
	Ratio       float32 // Compression ratio (e.g., 4.0 means 4:1)
	AttackMs    float32
	ReleaseMs   float32
	envelope    float32
}

func (c *DynamicRangeCompressor) Process(frame *AudioFrame) (*AudioFrame, error) {
	threshold := float32(math.Pow(10, float64(c.ThresholdDb)/20.0))
	attackCoeff := float32(math.Exp(-1.0 / (float64(c.AttackMs) * 0.001 * float64(frame.SampleRate))))
	releaseCoeff := float32(math.Exp(-1.0 / (float64(c.ReleaseMs) * 0.001 * float64(frame.SampleRate))))

	out := make([]float32, len(frame.Data))
	for i := 0; i < len(frame.Data); i += frame.Channels {
		// Calculate peak across channels for this sample
		var peak float32
		for ch := 0; ch < frame.Channels; ch++ {
			abs := float32(math.Abs(float64(frame.Data[i+ch])))
			if abs > peak {
				peak = abs
			}
		}

		// Envelope follower
		if peak > c.envelope {
			c.envelope = attackCoeff*c.envelope + (1-attackCoeff)*peak
		} else {
			c.envelope = releaseCoeff*c.envelope + (1-releaseCoeff)*peak
		}

		// Compute gain reduction
		var gain float32 = 1.0
		if c.envelope > threshold {
			dbOver := 20 * float32(math.Log10(float64(c.envelope/threshold)))
			dbReduced := dbOver * (1.0 - 1.0/c.Ratio)
			gain = float32(math.Pow(10, float64(-dbReduced)/20.0))
		}

		for ch := 0; ch < frame.Channels; ch++ {
			out[i+ch] = frame.Data[i+ch] * gain
		}
	}

	return &AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// GeneratePinkNoise creates pink noise (1/f spectrum) using the Voss-McCartney algorithm.
func GeneratePinkNoise(durationSec float64, sampleRate int, channels int) *AudioFrame {
	numSamples := int(durationSec * float64(sampleRate))
	data := make([]float32, numSamples*channels)

	// Voss-McCartney algorithm with 16 octaves
	const numOctaves = 16
	var octaves [numOctaves]float32
	var counter uint32

	for i := 0; i < numSamples; i++ {
		counter++
		// Determine which octave to update based on trailing zeros in counter
		for oct := 0; oct < numOctaves; oct++ {
			if counter&(1<<uint(oct)) != 0 {
				octaves[oct] = float32(math.Float64frombits(
					uint64(0x3FF)<<52|uint64(i*31+oct*997)&0xFFFFFFFFFFFFF,
				))*2.0 - 3.0
				break
			}
		}
		// Sum all octave contributions
		var sum float32
		for oct := 0; oct < numOctaves; oct++ {
			sum += octaves[oct]
		}
		sum /= float32(numOctaves)
		// Normalize to [-1, 1]
		if sum > 1.0 {
			sum = 1.0
		} else if sum < -1.0 {
			sum = -1.0
		}
		for ch := 0; ch < channels; ch++ {
			data[i*channels+ch] = sum
		}
	}

	return &AudioFrame{
		Channels:   channels,
		SampleRate: sampleRate,
		Data:       data,
	}
}
