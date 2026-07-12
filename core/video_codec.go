package core

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"sync"
)

// PixelFormat represents the color format of a raw video frame
type PixelFormat string

const (
	PixelFormatYUV420P PixelFormat = "yuv420p"
	PixelFormatYUV422P PixelFormat = "yuv422p"
	PixelFormatYUV444P PixelFormat = "yuv444p"
	PixelFormatRGB      PixelFormat = "rgb24"
	PixelFormatRGBA     PixelFormat = "rgba"
)

// VideoFrame represents a raw uncompressed video frame
type VideoFrame struct {
	Width  int
	Height int
	Format PixelFormat
	Data   []byte // Interleaved or planar bytes depending on format
}

// VideoDecoder decodes encoded packets into raw VideoFrames
type VideoDecoder interface {
	Decode(pkt *Packet) (*VideoFrame, error)
	Close() error
}

// VideoEncoder encodes raw VideoFrames into packets
type VideoEncoder interface {
	Encode(frame *VideoFrame) (*Packet, error)
	Close() error
}

// MJPEGDecoder is a native Go decoder for MJPEG using image/jpeg
type MJPEGDecoder struct{}

func (d *MJPEGDecoder) Decode(pkt *Packet) (*VideoFrame, error) {
	img, err := jpeg.Decode(bytes.NewReader(pkt.Data))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	
	// Convert to RGBA
	rgba := image.NewRGBA(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}

	return &VideoFrame{
		Width:  w,
		Height: h,
		Format: PixelFormatRGBA,
		Data:   rgba.Pix,
	}, nil
}

func (d *MJPEGDecoder) Close() error { return nil }

// MJPEGEncoder is a native Go encoder for MJPEG using image/jpeg
type MJPEGEncoder struct {
	Quality int
}

func (e *MJPEGEncoder) Encode(frame *VideoFrame) (*Packet, error) {
	if frame.Format != PixelFormatRGBA {
		return nil, errors.New("mjpeg encoder only supports RGBA frames currently")
	}

	img := &image.RGBA{
		Pix:    frame.Data,
		Stride: frame.Width * 4,
		Rect:   image.Rect(0, 0, frame.Width, frame.Height),
	}

	buf := bytes.NewBuffer(nil)
	opt := &jpeg.Options{Quality: e.Quality}
	if opt.Quality == 0 {
		opt.Quality = 80
	}

	if err := jpeg.Encode(buf, img, opt); err != nil {
		return nil, err
	}

	return &Packet{
		ID:         NewPacketID(),
		Data:       buf.Bytes(),
		IsKeyframe: true,
	}, nil
}

func (e *MJPEGEncoder) Close() error { return nil }

// --- Simulated CGO Wrappers (libopenh264, x264, x265, libvpx, libdav1d, libaom, ProRes) ---

// DPBManager simulates a Decoded Picture Buffer for H.264/H.265 frame reference management.
type DPBManager struct {
	mu     sync.Mutex
	frames map[int]*VideoFrame
}

func NewDPBManager() *DPBManager {
	return &DPBManager{frames: make(map[int]*VideoFrame)}
}

func (dpb *DPBManager) AddFrame(poc int, frame *VideoFrame) {
	dpb.mu.Lock()
	defer dpb.mu.Unlock()
	if len(dpb.frames) >= 16 { // H.264 max DPB size is 16
		for k := range dpb.frames {
			delete(dpb.frames, k)
			break
		}
	}
	dpb.frames[poc] = frame
}

// ConvertYUV420ToRGBA converts YUV420p planar image to RGBA using parallel scanlines
func ConvertYUV420ToRGBA(yuv []byte, w, h int) []byte {
	rgba := make([]byte, w*h*4)
	ySize := w * h
	uvSize := ySize / 4
	uStart := ySize
	vStart := ySize + uvSize

	var wg sync.WaitGroup
	numWorkers := 8
	rowChunk := h / numWorkers
	if rowChunk == 0 {
		rowChunk = h
	}

	for worker := 0; worker < h; worker += rowChunk {
		wg.Add(1)
		go func(startY, endY int) {
			defer wg.Done()
			for y := startY; y < endY; y++ {
				for x := 0; x < w; x++ {
					yp := yuv[y*w+x]
					uvIdx := (y/2)*(w/2) + (x / 2)
					up := yuv[uStart+uvIdx]
					vp := yuv[vStart+uvIdx]

					// Convert YUV to RGB
					c := int(yp) - 16
					d := int(up) - 128
					e := int(vp) - 128

					r := (298*c + 409*e + 128) >> 8
					g := (298*c - 100*d - 208*e + 128) >> 8
					b := (298*c + 516*d + 128) >> 8

					if r < 0 { r = 0 } else if r > 255 { r = 255 }
					if g < 0 { g = 0 } else if g > 255 { g = 255 }
					if b < 0 { b = 0 } else if b > 255 { b = 255 }

					rgbaIdx := (y*w + x) * 4
					rgba[rgbaIdx] = byte(r)
					rgba[rgbaIdx+1] = byte(g)
					rgba[rgbaIdx+2] = byte(b)
					rgba[rgbaIdx+3] = 255
				}
			}
		}(worker, min(worker+rowChunk, h))
	}
	wg.Wait()
	return rgba
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ParseAnnexB Nal units parser (handles both Annex B and AVCC formats automatically)
func ParseAnnexBNalUnits(data []byte) [][]byte {
	if len(data) < 4 {
		return nil
	}
	// Check if it starts with Annex B start code (0x00000001 or 0x000001)
	isAnnexB := false
	if data[0] == 0 && data[1] == 0 {
		if data[2] == 1 || (data[2] == 0 && data[3] == 1) {
			isAnnexB = true
		}
	}
	if isAnnexB {
		var nals [][]byte
		i := 0
		start := -1
		for i < len(data)-4 {
			if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 1 {
				if start != -1 {
					nals = append(nals, data[start:i])
				}
				start = i + 4
				i += 4
				continue
			}
			if data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
				if start != -1 {
					nals = append(nals, data[start:i])
				}
				start = i + 3
				i += 3
				continue
			}
			i++
		}
		if start != -1 && start < len(data) {
			nals = append(nals, data[start:])
		}
		return nals
	}

	// Otherwise, parse as AVCC (4-byte length prefix)
	var nals [][]byte
	offset := 0
	for offset+4 <= len(data) {
		nalLen := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
		if nalLen < 0 || offset+4+nalLen > len(data) {
			break
		}
		nals = append(nals, data[offset+4:offset+4+nalLen])
		offset += 4 + nalLen
	}
	return nals
}
