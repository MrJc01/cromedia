package audio

import (
	"math"

	"cromedia/core"
	"cromedia/core/filters"
)

func init() {
	filters.RegisterAudioFilter("equalizer", func(params map[string]interface{}) (core.AudioFilter, error) {
		f := &EqualizerFilter{LowGain: 1.0, MidGain: 1.0, HighGain: 1.0}
		if l, ok := params["low"].(float64); ok {
			f.LowGain = float32(l)
		}
		if m, ok := params["mid"].(float64); ok {
			f.MidGain = float32(m)
		}
		if h, ok := params["high"].(float64); ok {
			f.HighGain = float32(h)
		}
		return f, nil
	})

	filters.RegisterAudioFilter("tremolo", func(params map[string]interface{}) (core.AudioFilter, error) {
		f := &TremoloFilter{Frequency: 5.0, Depth: 0.5}
		if freq, ok := params["frequency"].(float64); ok {
			f.Frequency = freq
		}
		if d, ok := params["depth"].(float64); ok {
			f.Depth = d
		}
		return f, nil
	})

	filters.RegisterAudioFilter("chorus", func(params map[string]interface{}) (core.AudioFilter, error) {
		f := &ChorusFilter{DelayMs: 20.0, DepthMs: 2.0, Frequency: 1.0, Mix: 0.5}
		if del, ok := params["delay"].(float64); ok {
			f.DelayMs = del
		}
		if dep, ok := params["depth"].(float64); ok {
			f.DepthMs = dep
		}
		if freq, ok := params["frequency"].(float64); ok {
			f.Frequency = freq
		}
		if m, ok := params["mix"].(float64); ok {
			f.Mix = float32(m)
		}
		return f, nil
	})

	filters.RegisterAudioFilter("flanger", func(params map[string]interface{}) (core.AudioFilter, error) {
		f := &FlangerFilter{DelayMs: 3.0, DepthMs: 1.0, Frequency: 0.5, Feedback: 0.3, Mix: 0.5}
		if del, ok := params["delay"].(float64); ok {
			f.DelayMs = del
		}
		if dep, ok := params["depth"].(float64); ok {
			f.DepthMs = dep
		}
		if freq, ok := params["frequency"].(float64); ok {
			f.Frequency = freq
		}
		if fb, ok := params["feedback"].(float64); ok {
			f.Feedback = float32(fb)
		}
		if m, ok := params["mix"].(float64); ok {
			f.Mix = float32(m)
		}
		return f, nil
	})

	filters.RegisterAudioFilter("compand", func(params map[string]interface{}) (core.AudioFilter, error) {
		f := &CompandFilter{Threshold: 0.1, Gain: 2.0}
		if t, ok := params["threshold"].(float64); ok {
			f.Threshold = float32(t)
		}
		if g, ok := params["gain"].(float64); ok {
			f.Gain = float32(g)
		}
		return f, nil
	})

	filters.RegisterAudioFilter("earwax", func(params map[string]interface{}) (core.AudioFilter, error) {
		return &EarwaxFilter{}, nil
	})

	filters.RegisterAudioFilter("gate", func(params map[string]interface{}) (core.AudioFilter, error) {
		f := &GateFilter{Threshold: 0.01, Range: 0.0}
		if t, ok := params["threshold"].(float64); ok {
			f.Threshold = float32(t)
		}
		if r, ok := params["range"].(float64); ok {
			f.Range = float32(r)
		}
		return f, nil
	})

	filters.RegisterAudioFilter("pitch", func(params map[string]interface{}) (core.AudioFilter, error) {
		f := &PitchFilter{Ratio: 1.0}
		if r, ok := params["ratio"].(float64); ok {
			f.Ratio = r
		}
		return f, nil
	})
}

// EqualizerFilter adjusts low, mid, and high bands using low-pass and high-pass filters.
type EqualizerFilter struct {
	LowGain  float32
	MidGain  float32
	HighGain float32
}

func (f *EqualizerFilter) Process(frame *core.AudioFrame) (*core.AudioFrame, error) {
	lp := &core.LowPassFilter{CutoffHz: 250}
	hp := &core.HighPassFilter{CutoffHz: 4000}

	lowFrame, err := lp.Process(frame)
	if err != nil {
		return nil, err
	}

	highFrame, err := hp.Process(frame)
	if err != nil {
		return nil, err
	}

	out := make([]float32, len(frame.Data))
	for i := 0; i < len(frame.Data); i++ {
		lVal := lowFrame.Data[i]
		hVal := highFrame.Data[i]
		mVal := frame.Data[i] - lVal - hVal

		out[i] = lVal*f.LowGain + mVal*f.MidGain + hVal*f.HighGain
	}
	return &core.AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// TremoloFilter modulates amplitude over time.
type TremoloFilter struct {
	Frequency float64
	Depth     float64
	phase     float64
}

func (f *TremoloFilter) Process(frame *core.AudioFrame) (*core.AudioFrame, error) {
	out := make([]float32, len(frame.Data))
	dt := 1.0 / float64(frame.SampleRate)

	for i := 0; i < len(frame.Data); i += frame.Channels {
		lfo := 1.0 - f.Depth*(0.5*math.Sin(2.0*math.Pi*f.Frequency*f.phase)+0.5)
		for ch := 0; ch < frame.Channels; ch++ {
			out[i+ch] = frame.Data[i+ch] * float32(lfo)
		}
		f.phase += dt
	}
	return &core.AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// ChorusFilter mixes the source signal with a delayed modulated copy.
type ChorusFilter struct {
	DelayMs   float64
	DepthMs   float64
	Frequency float64
	Mix       float32
	phase     float64
	history   [][]float32
}

func (f *ChorusFilter) Process(frame *core.AudioFrame) (*core.AudioFrame, error) {
	numChannels := frame.Channels
	sampleRate := frame.SampleRate
	out := make([]float32, len(frame.Data))

	if len(f.history) < numChannels {
		f.history = make([][]float32, numChannels)
	}

	maxDelaySamples := int((f.DelayMs+f.DepthMs)*float64(sampleRate)/1000.0) + 10
	for ch := 0; ch < numChannels; ch++ {
		if len(f.history[ch]) < maxDelaySamples {
			f.history[ch] = make([]float32, maxDelaySamples)
		}
	}

	dt := 1.0 / float64(sampleRate)
	for i := 0; i < len(frame.Data); i += numChannels {
		mod := f.DepthMs * math.Sin(2.0*math.Pi*f.Frequency*f.phase)
		currDelayMs := f.DelayMs + mod
		delaySamples := currDelayMs * float64(sampleRate) / 1000.0

		for ch := 0; ch < numChannels; ch++ {
			idxF := float64(len(f.history[ch])) - delaySamples
			idx := int(math.Floor(idxF))
			frac := idxF - float64(idx)

			var delayedVal float32
			if idx >= 0 && idx < len(f.history[ch])-1 {
				delayedVal = f.history[ch][idx]*(1.0-float32(frac)) + f.history[ch][idx+1]*float32(frac)
			}

			out[i+ch] = frame.Data[i+ch]*(1.0-f.Mix) + delayedVal*f.Mix
			f.history[ch] = append(f.history[ch][1:], frame.Data[i+ch])
		}
		f.phase += dt
	}
	return &core.AudioFrame{Channels: numChannels, SampleRate: sampleRate, Data: out}, nil
}

// FlangerFilter applies modulated delay with feedback.
type FlangerFilter struct {
	DelayMs   float64
	DepthMs   float64
	Frequency float64
	Feedback  float32
	Mix       float32
	phase     float64
	history   [][]float32
}

func (f *FlangerFilter) Process(frame *core.AudioFrame) (*core.AudioFrame, error) {
	numChannels := frame.Channels
	sampleRate := frame.SampleRate
	out := make([]float32, len(frame.Data))

	if len(f.history) < numChannels {
		f.history = make([][]float32, numChannels)
	}

	maxDelaySamples := int((f.DelayMs+f.DepthMs)*float64(sampleRate)/1000.0) + 10
	for ch := 0; ch < numChannels; ch++ {
		if len(f.history[ch]) < maxDelaySamples {
			f.history[ch] = make([]float32, maxDelaySamples)
		}
	}

	dt := 1.0 / float64(sampleRate)
	for i := 0; i < len(frame.Data); i += numChannels {
		mod := f.DepthMs * math.Sin(2.0*math.Pi*f.Frequency*f.phase)
		currDelayMs := f.DelayMs + mod
		delaySamples := currDelayMs * float64(sampleRate) / 1000.0

		for ch := 0; ch < numChannels; ch++ {
			idxF := float64(len(f.history[ch])) - delaySamples
			idx := int(math.Floor(idxF))
			frac := idxF - float64(idx)

			var delayedVal float32
			if idx >= 0 && idx < len(f.history[ch])-1 {
				delayedVal = f.history[ch][idx]*(1.0-float32(frac)) + f.history[ch][idx+1]*float32(frac)
			}

			out[i+ch] = frame.Data[i+ch]*(1.0-f.Mix) + delayedVal*f.Mix
			valToPush := frame.Data[i+ch] + delayedVal*f.Feedback
			f.history[ch] = append(f.history[ch][1:], valToPush)
		}
		f.phase += dt
	}
	return &core.AudioFrame{Channels: numChannels, SampleRate: sampleRate, Data: out}, nil
}

// CompandFilter compresses or expands amplitude ranges.
type CompandFilter struct {
	Threshold float32
	Gain      float32
}

func (f *CompandFilter) Process(frame *core.AudioFrame) (*core.AudioFrame, error) {
	out := make([]float32, len(frame.Data))
	for i, v := range frame.Data {
		absVal := float32(math.Abs(float64(v)))
		if absVal > f.Threshold {
			sig := float32(1.0)
			if v < 0 {
				sig = -1.0
			}
			out[i] = sig * (f.Threshold + (absVal-f.Threshold)*f.Gain)
		} else {
			out[i] = v
		}
	}
	return &core.AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// EarwaxFilter simulates headphone speaker crossfeed.
type EarwaxFilter struct {
	historyR []float32
	historyL []float32
}

func (f *EarwaxFilter) Process(frame *core.AudioFrame) (*core.AudioFrame, error) {
	if frame.Channels != 2 {
		return frame, nil
	}

	out := make([]float32, len(frame.Data))
	delaySamples := int(0.00025 * float64(frame.SampleRate))
	if delaySamples <= 0 {
		delaySamples = 11
	}

	if len(f.historyL) < delaySamples {
		f.historyL = make([]float32, delaySamples)
		f.historyR = make([]float32, delaySamples)
	}

	alpha := float32(0.5)
	for i := 0; i < len(frame.Data); i += 2 {
		l := frame.Data[i]
		r := frame.Data[i+1]

		crossL := f.historyR[0] * alpha
		crossR := f.historyL[0] * alpha

		out[i] = l + crossL
		out[i+1] = r + crossR

		f.historyL = append(f.historyL[1:], l)
		f.historyR = append(f.historyR[1:], r)
	}
	return &core.AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// GateFilter silences signals below a noise threshold.
type GateFilter struct {
	Threshold float32
	Range     float32
}

func (f *GateFilter) Process(frame *core.AudioFrame) (*core.AudioFrame, error) {
	out := make([]float32, len(frame.Data))
	for i, v := range frame.Data {
		absVal := float32(math.Abs(float64(v)))
		if absVal < f.Threshold {
			out[i] = v * f.Range
		} else {
			out[i] = v
		}
	}
	return &core.AudioFrame{Channels: frame.Channels, SampleRate: frame.SampleRate, Data: out}, nil
}

// PitchFilter shifts pitch without speed adjustment.
type PitchFilter struct {
	Ratio float64
}

func (f *PitchFilter) Process(frame *core.AudioFrame) (*core.AudioFrame, error) {
	if f.Ratio == 1.0 || f.Ratio <= 0 {
		return frame, nil
	}

	numChannels := frame.Channels
	numSamples := len(frame.Data) / numChannels
	out := make([]float32, len(frame.Data))

	for i := 0; i < numSamples; i++ {
		srcIndexF := float64(i) * f.Ratio
		srcIdx := int(math.Floor(srcIndexF))
		frac := srcIndexF - float64(srcIdx)

		srcIdx = srcIdx % numSamples
		nextIdx := (srcIdx + 1) % numSamples

		for ch := 0; ch < numChannels; ch++ {
			v1 := frame.Data[srcIdx*numChannels+ch]
			v2 := frame.Data[nextIdx*numChannels+ch]
			out[i*numChannels+ch] = v1*(1.0-float32(frac)) + v2*float32(frac)
		}
	}
	return &core.AudioFrame{Channels: numChannels, SampleRate: frame.SampleRate, Data: out}, nil
}
