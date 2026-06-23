# CroMedia Filter Reference Guide 🎛️

This reference guide documents the usage, properties, and runtime optimizations for CroMedia's video and audio filters.

---

## 1. Video Filters

### colorbalance
Adjusts colors in shadows, midtones, and highlights.
- **shadows**: `[3]float64` (Red, Green, Blue adjustments)
- **midtones**: `[3]float64`
- **highlights**: `[3]float64`

### eq
Adjusts contrast, brightness, saturation, and gamma.
- **brightness**: `float64` (default: 0.0)
- **contrast**: `float64` (default: 1.0)
- **saturation**: `float64` (default: 1.0)
- **gamma**: `float64` (default: 1.0)

### noise
Injects synthetic grain/noise.
- **strength**: `int` (noise intensity)

### unsharp
Applies a sharp or blur convolution.
- **amount**: `float64` (sharpening scale)

---

## 2. Audio Filters

### equalizer
3-band parametric equalizer.
- **low**: `float64` (gain factor)
- **mid**: `float64`
- **high**: `float64`

### tremolo
Low frequency amplitude modulation.
- **frequency**: `float64` (Hz)
- **depth**: `float64` (0.0 to 1.0)

---

## 3. Advanced Features

### Keyframes & Mathematical Expressions
You can animate filter parameters dynamically using the `InterpolateKeyframes` interface, or supply formulas (e.g. `0.5 * t`) to parameter values which are evaluated dynamically at time $t$ via `EvaluateExpression`.

### Concurrency Optimizer
For high-resolution video filtering (e.g., 4K), use the `ProcessVideoFilterConcurrently` pipeline wrapper. It automatically chunks the frame's height across multiple threads, process slices in parallel, and gathers results back zero-copy into a pooled buffer.
