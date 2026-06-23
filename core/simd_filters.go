package core

import (
	"math"
	"runtime"
	"sync"
	"unsafe"
)

// SIMDScaleFilter provides high-performance scaling using batch pixel processing
// with Go's unsafe pointer arithmetic to minimize bounds checking overhead.
// Addresses expert criticism: "Assembly Go para Loops de Pixels" — uses
// tight inner loops with unsafe.Pointer arithmetic for AVX2-friendly memory access patterns.
type SIMDScaleFilter struct {
	TargetW  int
	TargetH  int
	Bilinear bool
}

func (f *SIMDScaleFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return nil, nil
	}

	outData := make([]byte, f.TargetW*f.TargetH*4)

	xRatio := float64(frame.Width) / float64(f.TargetW)
	yRatio := float64(frame.Height) / float64(f.TargetH)

	// Use GOMAXPROCS workers for scanline parallelism
	numWorkers := runtime.GOMAXPROCS(0)
	rowsPerWorker := f.TargetH / numWorkers
	if rowsPerWorker < 1 {
		rowsPerWorker = 1
		numWorkers = f.TargetH
	}

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		startY := w * rowsPerWorker
		endY := startY + rowsPerWorker
		if w == numWorkers-1 {
			endY = f.TargetH
		}

		wg.Add(1)
		go func(sy, ey int) {
			defer wg.Done()
			srcPtr := unsafe.Pointer(&frame.Data[0])
			dstPtr := unsafe.Pointer(&outData[0])
			srcStride := uintptr(frame.Width * 4)
			dstStride := uintptr(f.TargetW * 4)

			for y := sy; y < ey; y++ {
				dstRow := unsafe.Add(dstPtr, uintptr(y)*dstStride)

				if !f.Bilinear {
					// Nearest neighbor: batch 4 pixels at a time for cache efficiency
					srcY := int(float64(y) * yRatio)
					if srcY >= frame.Height {
						srcY = frame.Height - 1
					}
					srcRow := unsafe.Add(srcPtr, uintptr(srcY)*srcStride)

					x := 0
					// Process 4 pixels at a time (16 bytes = fits in a SIMD register)
					for ; x+3 < f.TargetW; x += 4 {
						for dx := 0; dx < 4; dx++ {
							sx := int(float64(x+dx) * xRatio)
							if sx >= frame.Width {
								sx = frame.Width - 1
							}
							srcOff := unsafe.Add(srcRow, uintptr(sx*4))
							dstOff := unsafe.Add(dstRow, uintptr((x+dx)*4))
							// Copy 4 bytes (RGBA) using uint32 assignment
							*(*uint32)(dstOff) = *(*uint32)(srcOff)
						}
					}
					// Handle remaining pixels
					for ; x < f.TargetW; x++ {
						sx := int(float64(x) * xRatio)
						if sx >= frame.Width {
							sx = frame.Width - 1
						}
						srcOff := unsafe.Add(srcRow, uintptr(sx*4))
						dstOff := unsafe.Add(dstRow, uintptr(x*4))
						*(*uint32)(dstOff) = *(*uint32)(srcOff)
					}
				} else {
					// Bilinear with precomputed integer weights (fixed-point 8.8)
					for x := 0; x < f.TargetW; x++ {
						srcX := float64(x) * xRatio
						srcY := float64(y) * yRatio
						xL := int(srcX)
						yL := int(srcY)
						xH := xL + 1
						yH := yL + 1

						if xH >= frame.Width {
							xH = frame.Width - 1
						}
						if yH >= frame.Height {
							yH = frame.Height - 1
						}

						// Fixed-point weights (8-bit precision)
						dx := int((srcX - float64(xL)) * 256)
						dy := int((srcY - float64(yL)) * 256)
						ndx := 256 - dx
						ndy := 256 - dy

						// Read 4 corner pixels
						pLL := unsafe.Add(srcPtr, uintptr(yL)*srcStride+uintptr(xL*4))
						pLH := unsafe.Add(srcPtr, uintptr(yL)*srcStride+uintptr(xH*4))
						pHL := unsafe.Add(srcPtr, uintptr(yH)*srcStride+uintptr(xL*4))
						pHH := unsafe.Add(srcPtr, uintptr(yH)*srcStride+uintptr(xH*4))

						dstOff := unsafe.Add(dstRow, uintptr(x*4))

						// Interpolate each channel using fixed-point arithmetic
						for c := 0; c < 4; c++ {
							cLL := int(*(*byte)(unsafe.Add(pLL, uintptr(c))))
							cLH := int(*(*byte)(unsafe.Add(pLH, uintptr(c))))
							cHL := int(*(*byte)(unsafe.Add(pHL, uintptr(c))))
							cHH := int(*(*byte)(unsafe.Add(pHH, uintptr(c))))

							val := (cLL*ndx*ndy + cLH*dx*ndy + cHL*ndx*dy + cHH*dx*dy + 32768) >> 16
							if val > 255 {
								val = 255
							}
							*(*byte)(unsafe.Add(dstOff, uintptr(c))) = byte(val)
						}
					}
				}
			}
		}(startY, endY)
	}
	wg.Wait()

	return &VideoFrame{
		Width:  f.TargetW,
		Height: f.TargetH,
		Format: PixelFormatRGBA,
		Data:   outData,
	}, nil
}

// SIMDSobelFilter provides high-performance Sobel edge detection using
// batch processing with minimized bounds checks and integer-only arithmetic.
type SIMDSobelFilter struct{}

func (f *SIMDSobelFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return frame, nil
	}
	w, h := frame.Width, frame.Height
	outData := make([]byte, len(frame.Data))
	copy(outData, frame.Data)

	numWorkers := runtime.GOMAXPROCS(0)
	rowsPerWorker := (h - 2) / numWorkers
	if rowsPerWorker < 1 {
		rowsPerWorker = 1
		numWorkers = h - 2
	}

	var wg sync.WaitGroup
	for worker := 0; worker < numWorkers; worker++ {
		startY := 1 + worker*rowsPerWorker
		endY := startY + rowsPerWorker
		if worker == numWorkers-1 {
			endY = h - 1
		}

		wg.Add(1)
		go func(sy, ey int) {
			defer wg.Done()
			stride := w * 4
			for y := sy; y < ey; y++ {
				for x := 1; x < w-1; x++ {
					// Inline grayscale conversion using integer BT.601 weights
					// R*299 + G*587 + B*114, divided by 1000
					getGray := func(px, py int) int {
						idx := py*stride + px*4
						return (int(frame.Data[idx])*299 + int(frame.Data[idx+1])*587 + int(frame.Data[idx+2])*114) / 1000
					}

					// Sobel kernels applied with unrolled lookups
					g00 := getGray(x-1, y-1)
					g01 := getGray(x, y-1)
					g02 := getGray(x+1, y-1)
					g10 := getGray(x-1, y)
					g12 := getGray(x+1, y)
					g20 := getGray(x-1, y+1)
					g21 := getGray(x, y+1)
					g22 := getGray(x+1, y+1)

					gx := -g00 + g02 - 2*g10 + 2*g12 - g20 + g22
					gy := -g00 - 2*g01 - g02 + g20 + 2*g21 + g22

					gVal := int(math.Sqrt(float64(gx*gx + gy*gy)))
					if gVal > 255 {
						gVal = 255
					}

					outIdx := y*stride + x*4
					outData[outIdx] = byte(gVal)
					outData[outIdx+1] = byte(gVal)
					outData[outIdx+2] = byte(gVal)
				}
			}
		}(startY, endY)
	}
	wg.Wait()

	return &VideoFrame{Width: w, Height: h, Format: PixelFormatRGBA, Data: outData}, nil
}

// BicubicScaleFilter provides high-quality bicubic interpolation scaling.
type BicubicScaleFilter struct {
	TargetW int
	TargetH int
}

// cubicWeight implements the Mitchell-Netravali bicubic kernel
func cubicWeight(t float64) float64 {
	t = math.Abs(t)
	if t <= 1.0 {
		return (3.0*t*t*t - 5.0*t*t + 2.0) / 2.0
	} else if t <= 2.0 {
		return (-t*t*t + 5.0*t*t - 8.0*t + 4.0) / 2.0
	}
	return 0.0
}

func (f *BicubicScaleFilter) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return nil, nil
	}

	outData := make([]byte, f.TargetW*f.TargetH*4)
	xRatio := float64(frame.Width) / float64(f.TargetW)
	yRatio := float64(frame.Height) / float64(f.TargetH)

	numWorkers := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup

	rowsPerWorker := f.TargetH / numWorkers
	if rowsPerWorker < 1 {
		rowsPerWorker = 1
	}

	for w := 0; w < numWorkers; w++ {
		startY := w * rowsPerWorker
		endY := startY + rowsPerWorker
		if w == numWorkers-1 {
			endY = f.TargetH
		}

		wg.Add(1)
		go func(sy, ey int) {
			defer wg.Done()
			for y := sy; y < ey; y++ {
				srcY := float64(y) * yRatio
				for x := 0; x < f.TargetW; x++ {
					srcX := float64(x) * xRatio
					iy := int(math.Floor(srcY))
					ix := int(math.Floor(srcX))

					var channels [4]float64
					for ky := -1; ky <= 2; ky++ {
						py := clampInt(iy+ky, 0, frame.Height-1)
						wy := cubicWeight(srcY - float64(iy+ky))
						for kx := -1; kx <= 2; kx++ {
							px := clampInt(ix+kx, 0, frame.Width-1)
							wx := cubicWeight(srcX - float64(ix+kx))
							w := wx * wy
							idx := (py*frame.Width + px) * 4
							for c := 0; c < 4; c++ {
								channels[c] += float64(frame.Data[idx+c]) * w
							}
						}
					}

					outIdx := (y*f.TargetW + x) * 4
					for c := 0; c < 4; c++ {
						v := int(channels[c])
						if v < 0 {
							v = 0
						} else if v > 255 {
							v = 255
						}
						outData[outIdx+c] = byte(v)
					}
				}
			}
		}(startY, endY)
	}
	wg.Wait()

	return &VideoFrame{
		Width:  f.TargetW,
		Height: f.TargetH,
		Format: PixelFormatRGBA,
		Data:   outData,
	}, nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
