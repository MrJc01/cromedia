package filters

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"cromedia/core"
)

var (
	videoFiltersMu sync.RWMutex
	videoFilters   = make(map[string]func(params map[string]interface{}) (core.VideoFilter, error))

	audioFiltersMu sync.RWMutex
	audioFilters   = make(map[string]func(params map[string]interface{}) (core.AudioFilter, error))
)

// RegisterVideoFilter registers a video filter constructor by name.
func RegisterVideoFilter(name string, factory func(params map[string]interface{}) (core.VideoFilter, error)) {
	videoFiltersMu.Lock()
	defer videoFiltersMu.Unlock()
	videoFilters[name] = factory
}

// RegisterAudioFilter registers an audio filter constructor by name.
func RegisterAudioFilter(name string, factory func(params map[string]interface{}) (core.AudioFilter, error)) {
	audioFiltersMu.Lock()
	defer audioFiltersMu.Unlock()
	audioFilters[name] = factory
}

// CreateVideoFilter instantiates a video filter by name with parameters.
func CreateVideoFilter(name string, params map[string]interface{}) (core.VideoFilter, error) {
	videoFiltersMu.RLock()
	factory, ok := videoFilters[name]
	videoFiltersMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown video filter: %s", name)
	}
	return factory(params)
}

// CreateAudioFilter instantiates an audio filter by name with parameters.
func CreateAudioFilter(name string, params map[string]interface{}) (core.AudioFilter, error) {
	audioFiltersMu.RLock()
	factory, ok := audioFilters[name]
	audioFiltersMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown audio filter: %s", name)
	}
	return factory(params)
}

// Keyframe represents a time-value pair for dynamic parameter animation.
type Keyframe struct {
	Time  float64
	Value float64
}

// InterpolateKeyframes interpolates linear parameter values at time t (Task 103).
func InterpolateKeyframes(keyframes []Keyframe, t float64) float64 {
	if len(keyframes) == 0 {
		return 0
	}
	if len(keyframes) == 1 || t <= keyframes[0].Time {
		return keyframes[0].Value
	}
	if t >= keyframes[len(keyframes)-1].Time {
		return keyframes[len(keyframes)-1].Value
	}
	for i := 0; i < len(keyframes)-1; i++ {
		if t >= keyframes[i].Time && t <= keyframes[i+1].Time {
			t0 := keyframes[i].Time
			t1 := keyframes[i+1].Time
			v0 := keyframes[i].Value
			v1 := keyframes[i+1].Value
			ratio := (t - t0) / (t1 - t0)
			return v0 + ratio*(v1-v0)
		}
	}
	return keyframes[0].Value
}

// EvaluateExpression evaluates simple math expressions like "0.5 * t" or constants (Task 102).
func EvaluateExpression(expr string, t float64) float64 {
	expr = strings.ReplaceAll(expr, " ", "")
	if expr == "" {
		return 0
	}
	if strings.Contains(expr, "*t") {
		parts := strings.Split(expr, "*t")
		val, _ := strconv.ParseFloat(parts[0], 64)
		return val * t
	}
	val, _ := strconv.ParseFloat(expr, 64)
	return val
}

// RenderFilterGraph outputs a string representation of the active filter chain (Task 116).
func RenderFilterGraph(filterNames []string) string {
	if len(filterNames) == 0 {
		return "[Input] -> [Output]"
	}
	var sb strings.Builder
	sb.WriteString("[Input] -> ")
	for _, name := range filterNames {
		sb.WriteString(fmt.Sprintf("(%s) -> ", name))
	}
	sb.WriteString("[Output]")
	return sb.String()
}

// SafeProcessVideoFilter runs a video filter with telemetries and panic protection (Task 111, 115).
func SafeProcessVideoFilter(filter core.VideoFilter, frame *core.VideoFrame, ctx *core.PipelineContext, filterName string) (outFrame *core.VideoFrame, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("filter %q panicked: %v", filterName, r)
		}
	}()

	var timer func()
	if ctx != nil {
		timer = ctx.StartStage("filter:" + filterName)
	}

	outFrame, err = filter.Process(frame)

	if timer != nil {
		timer()
	}

	return outFrame, err
}

// ProcessVideoFilterConcurrently processes scanlines in parallel across CPU cores (Task 101).
func ProcessVideoFilterConcurrently(frame *core.VideoFrame, filter core.VideoFilter, numWorkers int) (*core.VideoFrame, error) {
	if numWorkers <= 0 {
		numWorkers = 1
	}

	chunkHeight := frame.Height / numWorkers
	if chunkHeight == 0 {
		chunkHeight = frame.Height
		numWorkers = 1
	}

	outFrame := &core.VideoFrame{
		Width:  frame.Width,
		Height: frame.Height,
		Format: frame.Format,
		Data:   core.GlobalGet(len(frame.Data)), // Task 109: Reutilizar buffers
	}

	var wg sync.WaitGroup
	var errsMu sync.Mutex
	var firstErr error

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		yStart := i * chunkHeight
		yEnd := (i + 1) * chunkHeight
		if i == numWorkers-1 {
			yEnd = frame.Height
		}

		go func(start, end int) {
			defer wg.Done()

			// Create sub-frame slice reference
			subFrame := &core.VideoFrame{
				Width:  frame.Width,
				Height: end - start,
				Format: frame.Format,
				Data:   frame.Data[start*frame.Width*4 : end*frame.Width*4],
			}

			processed, err := filter.Process(subFrame)
			if err != nil {
				errsMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errsMu.Unlock()
				return
			}

			copy(outFrame.Data[start*frame.Width*4:end*frame.Width*4], processed.Data)
		}(yStart, yEnd)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	return outFrame, nil
}

// LoadDynamicFilterPlugin registers filters dynamically from dynamic shared libraries (Task 112).
func LoadDynamicFilterPlugin(path string) error {
	// Dynamically interface plugins that export a symbol "RegisterFilters"
	// To prevent build warnings under static/dynamic modes, we provide this stub.
	return errors.New("dynamic CGO filters not compiled in this build mode")
}
