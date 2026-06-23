package video

import (
	"math"

	"cromedia/core"
	"cromedia/core/filters"
)

func init() {
	filters.RegisterVideoFilter("colorbalance", func(params map[string]interface{}) (core.VideoFilter, error) {
		f := &ColorBalanceFilter{}
		if s, ok := params["shadows"].([3]float64); ok {
			f.Shadows = s
		}
		if m, ok := params["midtones"].([3]float64); ok {
			f.Midtones = m
		}
		if h, ok := params["highlights"].([3]float64); ok {
			f.Highlights = h
		}
		return f, nil
	})

	filters.RegisterVideoFilter("eq", func(params map[string]interface{}) (core.VideoFilter, error) {
		f := &EqFilter{Contrast: 1.0, Saturation: 1.0, Gamma: 1.0}
		if b, ok := params["brightness"].(float64); ok {
			f.Brightness = b
		}
		if c, ok := params["contrast"].(float64); ok {
			f.Contrast = c
		}
		if s, ok := params["saturation"].(float64); ok {
			f.Saturation = s
		}
		if g, ok := params["gamma"].(float64); ok {
			f.Gamma = g
		}
		return f, nil
	})

	filters.RegisterVideoFilter("noise", func(params map[string]interface{}) (core.VideoFilter, error) {
		f := &NoiseFilter{}
		if s, ok := params["strength"].(int); ok {
			f.Strength = s
		}
		return f, nil
	})

	filters.RegisterVideoFilter("unsharp", func(params map[string]interface{}) (core.VideoFilter, error) {
		f := &UnsharpFilter{}
		if a, ok := params["amount"].(float64); ok {
			f.Amount = a
		}
		return f, nil
	})

	filters.RegisterVideoFilter("cropdetect", func(params map[string]interface{}) (core.VideoFilter, error) {
		f := &CropDetectFilter{Limit: 24, Round: 16}
		if l, ok := params["limit"].(int); ok {
			f.Limit = l
		}
		if r, ok := params["round"].(int); ok {
			f.Round = r
		}
		return f, nil
	})

	filters.RegisterVideoFilter("yadif", func(params map[string]interface{}) (core.VideoFilter, error) {
		f := &YadifFilter{}
		if m, ok := params["mode"].(int); ok {
			f.Mode = m
		}
		return f, nil
	})

	filters.RegisterVideoFilter("curves", func(params map[string]interface{}) (core.VideoFilter, error) {
		f := &CurvesFilter{}
		if p, ok := params["preset"].(string); ok {
			f.Preset = p
		}
		return f, nil
	})

	filters.RegisterVideoFilter("pad", func(params map[string]interface{}) (core.VideoFilter, error) {
		f := &PadFilter{Color: [4]byte{0, 0, 0, 255}}
		if l, ok := params["left"].(int); ok {
			f.Left = l
		}
		if r, ok := params["right"].(int); ok {
			f.Right = r
		}
		if t, ok := params["top"].(int); ok {
			f.Top = t
		}
		if b, ok := params["bottom"].(int); ok {
			f.Bottom = b
		}
		if c, ok := params["color"].([4]byte); ok {
			f.Color = c
		}
		return f, nil
	})

	filters.RegisterVideoFilter("overlay", func(params map[string]interface{}) (core.VideoFilter, error) {
		f := &OverlayFilter{Threshold: 30}
		if over, ok := params["overlay_frame"].(*core.VideoFrame); ok {
			f.OverlayFrame = over
		}
		if x, ok := params["x"].(int); ok {
			f.X = x
		}
		if y, ok := params["y"].(int); ok {
			f.Y = y
		}
		if ck, ok := params["chromakey"].(bool); ok {
			f.ChromaKey = ck
		}
		if cc, ok := params["chromacolor"].([3]byte); ok {
			f.ChromaColor = cc
		}
		if th, ok := params["threshold"].(int); ok {
			f.Threshold = th
		}
		return f, nil
	})
}

// ColorBalanceFilter adjusts tonal balance in shadows, midtones, and highlights.
type ColorBalanceFilter struct {
	Shadows    [3]float64
	Midtones   [3]float64
	Highlights [3]float64
}

func (f *ColorBalanceFilter) Process(frame *core.VideoFrame) (*core.VideoFrame, error) {
	if frame.Format != "rgba" {
		return frame, nil
	}
	out := make([]byte, len(frame.Data))
	for i := 0; i < len(frame.Data); i += 4 {
		r, g, b, a := float64(frame.Data[i]), float64(frame.Data[i+1]), float64(frame.Data[i+2]), frame.Data[i+3]
		y := (0.299*r + 0.587*g + 0.114*b) / 255.0
		ws := (1.0 - y) * (1.0 - y)
		wm := 4.0 * y * (1.0 - y)
		wh := y * y

		r2 := r + f.Shadows[0]*ws + f.Midtones[0]*wm + f.Highlights[0]*wh
		g2 := g + f.Shadows[1]*ws + f.Midtones[1]*wm + f.Highlights[1]*wh
		b2 := b + f.Shadows[2]*ws + f.Midtones[2]*wm + f.Highlights[2]*wh

		if r2 < 0 { r2 = 0 } else if r2 > 255 { r2 = 255 }
		if g2 < 0 { g2 = 0 } else if g2 > 255 { g2 = 255 }
		if b2 < 0 { b2 = 0 } else if b2 > 255 { b2 = 255 }

		out[i] = byte(r2)
		out[i+1] = byte(g2)
		out[i+2] = byte(b2)
		out[i+3] = a
	}
	return &core.VideoFrame{Width: frame.Width, Height: frame.Height, Format: frame.Format, Data: out}, nil
}

// EqFilter controls brightness, contrast, saturation, and gamma.
type EqFilter struct {
	Brightness float64
	Contrast   float64
	Saturation float64
	Gamma      float64
}

func (f *EqFilter) Process(frame *core.VideoFrame) (*core.VideoFrame, error) {
	if frame.Format != "rgba" {
		return frame, nil
	}
	out := make([]byte, len(frame.Data))
	for i := 0; i < len(frame.Data); i += 4 {
		r, g, b, a := float64(frame.Data[i]), float64(frame.Data[i+1]), float64(frame.Data[i+2]), frame.Data[i+3]

		r = (r-128)*f.Contrast + 128
		g = (g-128)*f.Contrast + 128
		b = (b-128)*f.Contrast + 128

		r += f.Brightness
		g += f.Brightness
		b += f.Brightness

		y := 0.299*r + 0.587*g + 0.114*b
		r = y + (r-y)*f.Saturation
		g = y + (g-y)*f.Saturation
		b = y + (b-y)*f.Saturation

		if f.Gamma != 1.0 && f.Gamma > 0 {
			r = 255.0 * math.Pow(r/255.0, 1.0/f.Gamma)
			g = 255.0 * math.Pow(g/255.0, 1.0/f.Gamma)
			b = 255.0 * math.Pow(b/255.0, 1.0/f.Gamma)
		}

		if r < 0 { r = 0 } else if r > 255 { r = 255 }
		if g < 0 { g = 0 } else if g > 255 { g = 255 }
		if b < 0 { b = 0 } else if b > 255 { b = 255 }

		out[i] = byte(r)
		out[i+1] = byte(g)
		out[i+2] = byte(b)
		out[i+3] = a
	}
	return &core.VideoFrame{Width: frame.Width, Height: frame.Height, Format: frame.Format, Data: out}, nil
}

// NoiseFilter injects granular grain noise.
type NoiseFilter struct {
	Strength int
}

func (f *NoiseFilter) Process(frame *core.VideoFrame) (*core.VideoFrame, error) {
	if frame.Format != "rgba" {
		return frame, nil
	}
	out := make([]byte, len(frame.Data))
	seed := int64(42)
	for i := 0; i < len(frame.Data); i += 4 {
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		var noiseVal int
		if f.Strength > 0 {
			noiseVal = int((seed % int64(f.Strength*2+1)) - int64(f.Strength))
		}

		r := int(frame.Data[i]) + noiseVal
		g := int(frame.Data[i+1]) + noiseVal
		b := int(frame.Data[i+2]) + noiseVal

		if r < 0 { r = 0 } else if r > 255 { r = 255 }
		if g < 0 { g = 0 } else if g > 255 { g = 255 }
		if b < 0 { b = 0 } else if b > 255 { b = 255 }

		out[i] = byte(r)
		out[i+1] = byte(g)
		out[i+2] = byte(b)
		out[i+3] = frame.Data[i+3]
	}
	return &core.VideoFrame{Width: frame.Width, Height: frame.Height, Format: frame.Format, Data: out}, nil
}

// UnsharpFilter applies spatial sharpening or box-like blurring.
type UnsharpFilter struct {
	Amount float64
}

func (f *UnsharpFilter) Process(frame *core.VideoFrame) (*core.VideoFrame, error) {
	if frame.Format != "rgba" {
		return frame, nil
	}
	w, h := frame.Width, frame.Height
	out := make([]byte, len(frame.Data))
	copy(out, frame.Data)

	cVal := 1.0 + 4.0*f.Amount
	sideVal := -f.Amount

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			idx := (y*w + x) * 4
			var sumR, sumG, sumB float64

			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					kIdx := ((y+ky)*w + (x+kx)) * 4
					weight := 0.0
					if ky == 0 && kx == 0 {
						weight = cVal
					} else if (ky == 0) || (kx == 0) {
						weight = sideVal
					}

					sumR += float64(frame.Data[kIdx]) * weight
					sumG += float64(frame.Data[kIdx+1]) * weight
					sumB += float64(frame.Data[kIdx+2]) * weight
				}
			}

			if sumR < 0 { sumR = 0 } else if sumR > 255 { sumR = 255 }
			if sumG < 0 { sumG = 0 } else if sumG > 255 { sumG = 255 }
			if sumB < 0 { sumB = 0 } else if sumB > 255 { sumB = 255 }

			out[idx] = byte(sumR)
			out[idx+1] = byte(sumG)
			out[idx+2] = byte(sumB)
		}
	}
	return &core.VideoFrame{Width: w, Height: h, Format: frame.Format, Data: out}, nil
}

// CropDetectFilter automatically detects active frame margins.
type CropDetectFilter struct {
	Limit     int
	Round     int
	DetectedW int
	DetectedH int
	DetectedX int
	DetectedY int
}

func (f *CropDetectFilter) Process(frame *core.VideoFrame) (*core.VideoFrame, error) {
	if frame.Format != "rgba" {
		return frame, nil
	}
	w, h := frame.Width, frame.Height
	limit := f.Limit

	minX, maxX := w, 0
	minY, maxY := h, 0

	for y := 0; y < h; y += 4 {
		for x := 0; x < w; x += 4 {
			idx := (y*w + x) * 4
			r, g, b := frame.Data[idx], frame.Data[idx+1], frame.Data[idx+2]
			if int(r) > limit || int(g) > limit || int(b) > limit {
				if x < minX { minX = x }
				if x > maxX { maxX = x }
				if y < minY { minY = y }
				if y > maxY { maxY = y }
			}
		}
	}

	if minX > maxX || minY > maxY {
		f.DetectedW, f.DetectedH, f.DetectedX, f.DetectedY = 0, 0, 0, 0
		return frame, nil
	}

	f.DetectedX = minX
	f.DetectedY = minY
	f.DetectedW = maxX - minX + 1
	f.DetectedH = maxY - minY + 1

	if f.Round > 1 {
		f.DetectedW = (f.DetectedW / f.Round) * f.Round
		f.DetectedH = (f.DetectedH / f.Round) * f.Round
	}
	return frame, nil
}

// YadifFilter deinterlaces frames spatially.
type YadifFilter struct {
	Mode int
}

func (f *YadifFilter) Process(frame *core.VideoFrame) (*core.VideoFrame, error) {
	if frame.Format != "rgba" {
		return frame, nil
	}
	w, h := frame.Width, frame.Height
	out := make([]byte, len(frame.Data))
	copy(out, frame.Data)

	for y := 1; y < h-1; y += 2 {
		for x := 0; x < w; x++ {
			idx := (y*w + x) * 4
			idxPrev := ((y-1)*w + x) * 4
			idxNext := ((y+1)*w + x) * 4

			out[idx] = byte((int(frame.Data[idxPrev]) + int(frame.Data[idxNext])) / 2)
			out[idx+1] = byte((int(frame.Data[idxPrev+1]) + int(frame.Data[idxNext+1])) / 2)
			out[idx+2] = byte((int(frame.Data[idxPrev+2]) + int(frame.Data[idxNext+2])) / 2)
		}
	}
	return &core.VideoFrame{Width: w, Height: h, Format: frame.Format, Data: out}, nil
}

// CurvesFilter applies color mapping curves based on presets.
type CurvesFilter struct {
	Preset string
}

func (f *CurvesFilter) Process(frame *core.VideoFrame) (*core.VideoFrame, error) {
	if frame.Format != "rgba" {
		return frame, nil
	}

	lutR := make([]byte, 256)
	lutG := make([]byte, 256)
	lutB := make([]byte, 256)

	for i := 0; i < 256; i++ {
		valR, valG, valB := i, i, i
		switch f.Preset {
		case "vintage":
			valR = int(float64(i)*0.85 + 20)
			valB = int(float64(i)*1.1 - 10)
		case "negative":
			valR = 255 - i
			valG = 255 - i
			valB = 255 - i
		}

		if valR < 0 { valR = 0 } else if valR > 255 { valR = 255 }
		if valG < 0 { valG = 0 } else if valG > 255 { valG = 255 }
		if valB < 0 { valB = 0 } else if valB > 255 { valB = 255 }

		lutR[i] = byte(valR)
		lutG[i] = byte(valG)
		lutB[i] = byte(valB)
	}

	out := make([]byte, len(frame.Data))
	for i := 0; i < len(frame.Data); i += 4 {
		out[i] = lutR[frame.Data[i]]
		out[i+1] = lutG[frame.Data[i+1]]
		out[i+2] = lutB[frame.Data[i+2]]
		out[i+3] = frame.Data[i+3]
	}
	return &core.VideoFrame{Width: frame.Width, Height: frame.Height, Format: frame.Format, Data: out}, nil
}

// PadFilter adds borders of a solid color around the video.
type PadFilter struct {
	Left   int
	Right  int
	Top    int
	Bottom int
	Color  [4]byte
}

func (f *PadFilter) Process(frame *core.VideoFrame) (*core.VideoFrame, error) {
	if frame.Format != "rgba" {
		return frame, nil
	}

	newW := frame.Width + f.Left + f.Right
	newH := frame.Height + f.Top + f.Bottom

	out := make([]byte, newW*newH*4)
	for i := 0; i < len(out); i += 4 {
		out[i] = f.Color[0]
		out[i+1] = f.Color[1]
		out[i+2] = f.Color[2]
		out[i+3] = f.Color[3]
	}

	for y := 0; y < frame.Height; y++ {
		srcIdx := y * frame.Width * 4
		dstIdx := ((y + f.Top) * newW + f.Left) * 4
		copy(out[dstIdx:dstIdx+frame.Width*4], frame.Data[srcIdx:srcIdx+frame.Width*4])
	}
	return &core.VideoFrame{Width: newW, Height: newH, Format: frame.Format, Data: out}, nil
}

// OverlayFilter superimposes another video stream/frame on top, with optional green chroma-key.
type OverlayFilter struct {
	OverlayFrame *core.VideoFrame
	X            int
	Y            int
	ChromaKey    bool
	ChromaColor  [3]byte
	Threshold    int
}

func (f *OverlayFilter) Process(frame *core.VideoFrame) (*core.VideoFrame, error) {
	if frame.Format != "rgba" || f.OverlayFrame == nil || f.OverlayFrame.Format != "rgba" {
		return frame, nil
	}

	w, h := frame.Width, frame.Height
	ow, oh := f.OverlayFrame.Width, f.OverlayFrame.Height

	out := make([]byte, len(frame.Data))
	copy(out, frame.Data)

	thresholdSq := f.Threshold * f.Threshold
	for oy := 0; oy < oh; oy++ {
		dy := oy + f.Y
		if dy < 0 || dy >= h {
			continue
		}

		for ox := 0; ox < ow; ox++ {
			dx := ox + f.X
			if dx < 0 || dx >= w {
				continue
			}

			oIdx := (oy*ow + ox) * 4
			dIdx := (dy*w + dx) * 4

			oR, oG, oB, oA := f.OverlayFrame.Data[oIdx], f.OverlayFrame.Data[oIdx+1], f.OverlayFrame.Data[oIdx+2], f.OverlayFrame.Data[oIdx+3]

			if f.ChromaKey {
				distSq := int(oR-f.ChromaColor[0])*int(oR-f.ChromaColor[0]) +
					int(oG-f.ChromaColor[1])*int(oG-f.ChromaColor[1]) +
					int(oB-f.ChromaColor[2])*int(oB-f.ChromaColor[2])

				if distSq < thresholdSq {
					continue
				}
			}

			alpha := float64(oA) / 255.0
			out[dIdx] = byte(float64(oR)*alpha + float64(out[dIdx])*(1.0-alpha))
			out[dIdx+1] = byte(float64(oG)*alpha + float64(out[dIdx+1])*(1.0-alpha))
			out[dIdx+2] = byte(float64(oB)*alpha + float64(out[dIdx+2])*(1.0-alpha))
		}
	}
	return &core.VideoFrame{Width: w, Height: h, Format: frame.Format, Data: out}, nil
}
