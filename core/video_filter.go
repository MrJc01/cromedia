package core

import (
	"math"
	"sync"
)

// VideoFilter represents a filter node in the video processing pipeline
type VideoFilter interface {
	Process(frame *VideoFrame) (*VideoFrame, error)
}

// ScaleFilter resizes raw frames using nearest-neighbor or bilinear filtering
type ScaleFilter struct {
	TargetW int
	TargetH int
	Bilinear bool
}

func (f *ScaleFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return nil, nil // Fallback/Skip for simplicity if not RGBA
	}

	outData := make([]byte, f.TargetW*f.TargetH*4)

	xRatio := float64(frame.Width) / float64(f.TargetW)
	yRatio := float64(frame.Height) / float64(f.TargetH)

	var wg sync.WaitGroup
	wg.Add(f.TargetH)

	for dy := 0; dy < f.TargetH; dy++ {
		go func(y int) {
			defer wg.Done()
			for x := 0; x < f.TargetW; x++ {
				var r, g, b, a byte
				if f.Bilinear {
					// Bilinear scaling
					srcX := float64(x) * xRatio
					srcY := float64(y) * yRatio
					xL := int(math.Floor(srcX))
					xH := xL + 1
					yL := int(math.Floor(srcY))
					yH := yL + 1

					if xH >= frame.Width { xH = frame.Width - 1 }
					if yH >= frame.Height { yH = frame.Height - 1 }

					dx := srcX - float64(xL)
					dyVal := srcY - float64(yL)

					idxLL := (yL*frame.Width + xL) * 4
					idxLH := (yL*frame.Width + xH) * 4
					idxHL := (yH*frame.Width + xL) * 4
					idxHH := (yH*frame.Width + xH) * 4

					for c := 0; c < 4; c++ {
						val := float64(frame.Data[idxLL+c])*(1-dx)*(1-dyVal) +
							float64(frame.Data[idxLH+c])*dx*(1-dyVal) +
							float64(frame.Data[idxHL+c])*(1-dx)*dyVal +
							float64(frame.Data[idxHH+c])*dx*dyVal
						if c == 0 { r = byte(val) }
						if c == 1 { g = byte(val) }
						if c == 2 { b = byte(val) }
						if c == 3 { a = byte(val) }
					}
				} else {
					// Nearest neighbor
					sx := int(float64(x) * xRatio)
					sy := int(float64(y) * yRatio)
					if sx >= frame.Width { sx = frame.Width - 1 }
					if sy >= frame.Height { sy = frame.Height - 1 }
					idx := (sy*frame.Width + sx) * 4
					r = frame.Data[idx]
					g = frame.Data[idx+1]
					b = frame.Data[idx+2]
					a = frame.Data[idx+3]
				}

				outIdx := (y*f.TargetW + x) * 4
				outData[outIdx] = r
				outData[outIdx+1] = g
				outData[outIdx+2] = b
				outData[outIdx+3] = a
			}
		}(dy)
	}
	wg.Wait()

	return &VideoFrame{
		Width:  f.TargetW,
		Height: f.TargetH,
		Format: PixelFormatRGBA,
		Data:   outData,
	}, nil
}

// CropFilter crops a sub-rectangle of the video
type CropFilter struct {
	X, Y, W, H int
}

func (f *CropFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return frame, nil
	}
	outData := make([]byte, f.W*f.H*4)
	for y := 0; y < f.H; y++ {
		srcY := f.Y + y
		if srcY >= frame.Height { continue }
		srcOffset := (srcY*frame.Width + f.X) * 4
		dstOffset := (y * f.W) * 4
		copy(outData[dstOffset:dstOffset+f.W*4], frame.Data[srcOffset:srcOffset+f.W*4])
	}
	return &VideoFrame{Width: f.W, Height: f.H, Format: PixelFormatRGBA, Data: outData}, nil
}

// FlipFilter flips the frame horizontally or vertically
type FlipFilter struct {
	Horizontal bool
	Vertical   bool
}

func (f *FlipFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return frame, nil
	}
	w, h := frame.Width, frame.Height
	outData := make([]byte, len(frame.Data))
	for y := 0; y < h; y++ {
		dy := y
		if f.Vertical {
			dy = h - 1 - y
		}
		for x := 0; x < w; x++ {
			dx := x
			if f.Horizontal {
				dx = w - 1 - x
			}
			srcOffset := (y*w + x) * 4
			dstOffset := (dy*w + dx) * 4
			copy(outData[dstOffset:dstOffset+4], frame.Data[srcOffset:srcOffset+4])
		}
	}
	return &VideoFrame{Width: w, Height: h, Format: PixelFormatRGBA, Data: outData}, nil
}

// RotateFilter rotates the image by 90, 180, or 270 degrees
type RotateFilter struct {
	Angle int // 90, 180, 270
}

func (f *RotateFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA || (f.Angle != 90 && f.Angle != 180 && f.Angle != 270) {
		return frame, nil
	}
	w, h := frame.Width, frame.Height
	var outW, outH int
	if f.Angle == 90 || f.Angle == 270 {
		outW, outH = h, w
	} else {
		outW, outH = w, h
	}
	outData := make([]byte, len(frame.Data))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch f.Angle {
			case 90:
				dx, dy = h-1-y, x
			case 180:
				dx, dy = w-1-x, h-1-y
			case 270:
				dx, dy = y, w-1-x
			}
			srcOffset := (y*w + x) * 4
			dstOffset := (dy*outW + dx) * 4
			copy(outData[dstOffset:dstOffset+4], frame.Data[srcOffset:srcOffset+4])
		}
	}
	return &VideoFrame{Width: outW, Height: outH, Format: PixelFormatRGBA, Data: outData}, nil
}

// ColorFilter adjusts brightness and contrast
type ColorFilter struct {
	Brightness float64 // -255 to 255
	Contrast   float64 // 0.0 to 3.0 (1.0 = normal)
}

func (f *ColorFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return frame, nil
	}
	outData := make([]byte, len(frame.Data))
	for i := 0; i < len(frame.Data); i += 4 {
		for c := 0; c < 3; c++ {
			val := float64(frame.Data[i+c])
			// Contrast
			val = (val-128.0)*f.Contrast + 128.0
			// Brightness
			val += f.Brightness
			if val < 0 { val = 0 } else if val > 255 { val = 255 }
			outData[i+c] = byte(val)
		}
		outData[i+3] = frame.Data[i+3] // alpha
	}
	return &VideoFrame{Width: frame.Width, Height: frame.Height, Format: PixelFormatRGBA, Data: outData}, nil
}

// OverlayFilter puts a watermark overlay image onto the frame with alpha blending
type OverlayFilter struct {
	Overlay *VideoFrame
	X, Y    int
}

func (f *OverlayFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA || f.Overlay.Format != PixelFormatRGBA {
		return frame, nil
	}
	outData := make([]byte, len(frame.Data))
	copy(outData, frame.Data)

	for y := 0; y < f.Overlay.Height; y++ {
		dy := f.Y + y
		if dy >= frame.Height || dy < 0 { continue }
		for x := 0; x < f.Overlay.Width; x++ {
			dx := f.X + x
			if dx >= frame.Width || dx < 0 { continue }

			overlayIdx := (y*f.Overlay.Width + x) * 4
			alpha := float64(f.Overlay.Data[overlayIdx+3]) / 255.0

			if alpha > 0 {
				dstIdx := (dy*frame.Width + dx) * 4
				for c := 0; c < 3; c++ {
					ovr := float64(f.Overlay.Data[overlayIdx+c])
					dst := float64(outData[dstIdx+c])
					outData[dstIdx+c] = byte(ovr*alpha + dst*(1.0-alpha))
				}
			}
		}
	}
	return &VideoFrame{Width: frame.Width, Height: frame.Height, Format: PixelFormatRGBA, Data: outData}, nil
}

// DrawTextFilter renders basic text using a built-in simple 8x8 bitmap font representation
type DrawTextFilter struct {
	Text string
	X, Y int
}

// Standard 8x8 bitmap font representation for ASCII characters (0-9, A-Z, space, etc.)
var font8x8 = map[rune][8]byte{
	' ': {0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	'A': {0x18, 0x24, 0x42, 0x42, 0x7E, 0x42, 0x42, 0x42},
	'B': {0x7C, 0x42, 0x42, 0x7C, 0x42, 0x42, 0x42, 0x7C},
	'C': {0x3C, 0x42, 0x40, 0x40, 0x40, 0x40, 0x42, 0x3C},
	'D': {0x78, 0x44, 0x42, 0x42, 0x42, 0x42, 0x44, 0x78},
	'E': {0x7E, 0x40, 0x40, 0x78, 0x40, 0x40, 0x40, 0x7E},
	'F': {0x7E, 0x40, 0x40, 0x78, 0x40, 0x40, 0x40, 0x40},
	'1': {0x10, 0x30, 0x10, 0x10, 0x10, 0x10, 0x10, 0x38},
	'2': {0x3C, 0x42, 0x02, 0x04, 0x08, 0x10, 0x20, 0x7E},
	'3': {0x3C, 0x02, 0x02, 0x1C, 0x02, 0x02, 0x02, 0x3C},
}

func (f *DrawTextFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return frame, nil
	}
	outData := make([]byte, len(frame.Data))
	copy(outData, frame.Data)

	currentX := f.X
	for _, char := range f.Text {
		bitmap, exists := font8x8[char]
		if !exists {
			bitmap = font8x8[' ']
		}
		for row := 0; row < 8; row++ {
			b := bitmap[row]
			dy := f.Y + row
			if dy >= frame.Height || dy < 0 { continue }
			for col := 0; col < 8; col++ {
				dx := currentX + col
				if dx >= frame.Width || dx < 0 { continue }
				
				// If bit is set, draw pixel (white)
				if (b & (0x80 >> col)) != 0 {
					idx := (dy*frame.Width + dx) * 4
					outData[idx] = 255
					outData[idx+1] = 255
					outData[idx+2] = 255
					outData[idx+3] = 255
				}
			}
		}
		currentX += 8 // advance font character width
	}

	return &VideoFrame{Width: frame.Width, Height: frame.Height, Format: PixelFormatRGBA, Data: outData}, nil
}

// SobelFilter applies edge detection
type SobelFilter struct{}

func (f *SobelFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return frame, nil
	}
	w, h := frame.Width, frame.Height
	outData := make([]byte, len(frame.Data))
	copy(outData, frame.Data)

	gx := [3][3]int{
		{-1, 0, 1},
		{-2, 0, 2},
		{-1, 0, 1},
	}
	gy := [3][3]int{
		{-1, -2, -1},
		{0, 0, 0},
		{1, 2, 1},
	}

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			var valX, valY int
			for ky := 0; ky < 3; ky++ {
				for kx := 0; kx < 3; kx++ {
					pxY := y + ky - 1
					pxX := x + kx - 1
					idx := (pxY*w + pxX) * 4
					// Grayscale value approximate
					gray := int(frame.Data[idx])*299/1000 + int(frame.Data[idx+1])*587/1000 + int(frame.Data[idx+2])*114/1000
					valX += gray * gx[ky][kx]
					valY += gray * gy[ky][kx]
				}
			}
			gVal := int(math.Sqrt(float64(valX*valX + valY*valY)))
			if gVal > 255 { gVal = 255 }

			outIdx := (y*w + x) * 4
			outData[outIdx] = byte(gVal)
			outData[outIdx+1] = byte(gVal)
			outData[outIdx+2] = byte(gVal)
		}
	}
	return &VideoFrame{Width: w, Height: h, Format: PixelFormatRGBA, Data: outData}, nil
}

// GenerateColorBars creates standard color bars frame
func GenerateColorBars(w, h int) *VideoFrame {
	data := make([]byte, w*h*4)
	colors := [][]byte{
		{255, 255, 255}, // White
		{255, 255, 0},   // Yellow
		{0, 255, 255},   // Cyan
		{0, 255, 0},     // Green
		{255, 0, 255},   // Magenta
		{255, 0, 0},     // Red
		{0, 0, 255},     // Blue
		{0, 0, 0},       // Black
	}

	barWidth := w / len(colors)
	if barWidth == 0 {
		barWidth = 1
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			colorIdx := x / barWidth
			if colorIdx >= len(colors) {
				colorIdx = len(colors) - 1
			}
			c := colors[colorIdx]
			idx := (y*w + x) * 4
			data[idx] = c[0]
			data[idx+1] = c[1]
			data[idx+2] = c[2]
			data[idx+3] = 255
		}
	}

	return &VideoFrame{Width: w, Height: h, Format: PixelFormatRGBA, Data: data}
}
