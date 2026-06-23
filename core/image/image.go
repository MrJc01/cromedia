package image

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"cromedia/core"

	"github.com/deepteams/webp"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func init() {
	// Register formats that are not in Go's standard library.
	// Note: github.com/deepteams/webp already auto-registers itself.
	image.RegisterFormat("bmp", "BM", bmp.Decode, bmp.DecodeConfig)
	image.RegisterFormat("tiff", "II*\x00", tiff.Decode, tiff.DecodeConfig)
	image.RegisterFormat("tiff", "MM\x00*", tiff.Decode, tiff.DecodeConfig)
}

// ConvertToVideoFrame converts a standard image.Image to a core.VideoFrame in RGBA format.
func ConvertToVideoFrame(img image.Image) (*core.VideoFrame, error) {
	if img == nil {
		return nil, errors.New("image is nil")
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	rgba, ok := img.(*image.RGBA)
	if !ok {
		// Draw the image to a new RGBA canvas
		rgba = image.NewRGBA(bounds)
		draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	}

	// Task 57: Reutilizar buffers do BufferPool para evitar allocations
	data := core.GlobalGet(len(rgba.Pix))
	copy(data, rgba.Pix)

	return &core.VideoFrame{
		Width:  width,
		Height: height,
		Format: "rgba",
		Data:   data,
	}, nil
}

// ConvertToImage converts a core.VideoFrame (RGBA/RGB32) to a standard image.Image.
func ConvertToImage(frame *core.VideoFrame) (image.Image, error) {
	if frame == nil {
		return nil, errors.New("video frame is nil")
	}

	if frame.Format != "rgba" && frame.Format != "rgb32" {
		return nil, fmt.Errorf("unsupported format for image conversion: %s", frame.Format)
	}

	expectedSize := frame.Width * frame.Height * 4
	if len(frame.Data) < expectedSize {
		return nil, fmt.Errorf("buffer size %d too small for resolution %dx%d (rgba)", len(frame.Data), frame.Width, frame.Height)
	}

	// Create an image.RGBA view overlaying the frame data
	rgba := &image.RGBA{
		Pix:    frame.Data[:expectedSize],
		Stride: frame.Width * 4,
		Rect:   image.Rect(0, 0, frame.Width, frame.Height),
	}

	return rgba, nil
}

// DecodeImage sniffs magic bytes and decodes an image from a reader.
func DecodeImage(r io.Reader) (image.Image, string, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, "", err
	}
	return img, format, nil
}

// EncodeImage encodes an image.Image to a writer in the specified format.
func EncodeImage(w io.Writer, img image.Image, format string, quality int) error {
	format = strings.ToLower(format)
	switch format {
	case "jpeg", "jpg":
		if quality <= 0 {
			quality = 85
		}
		return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
	case "png":
		// Task 62: controlled fast PNG compression
		var encoder png.Encoder
		if quality == 1 {
			encoder.CompressionLevel = png.BestSpeed
		} else if quality == 2 {
			encoder.CompressionLevel = png.NoCompression
		} else {
			encoder.CompressionLevel = png.DefaultCompression
		}
		return encoder.Encode(w, img)
	case "bmp":
		return bmp.Encode(w, img)
	case "tiff", "tif":
		return tiff.Encode(w, img, nil)
	case "webp":
		if quality <= 0 {
			quality = 75
		}
		opts := &webp.EncoderOptions{
			Quality: float32(quality),
		}
		return webp.Encode(w, img, opts)
	default:
		return fmt.Errorf("unsupported encode format: %s", format)
	}
}

// ConvertRGBAToYUV420p converts a contiguous RGBA slice to planar YUV420p (Task 66).
func ConvertRGBAToYUV420p(w, h int, rgba []byte) []byte {
	ySize := w * h
	uvSize := (w / 2) * (h / 2)
	dst := core.GlobalGet(ySize + 2*uvSize)

	yOffset := 0
	uOffset := ySize
	vOffset := ySize + uvSize

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rgbaIdx := (y*w + x) * 4
			if rgbaIdx+3 >= len(rgba) {
				break
			}
			r := int(rgba[rgbaIdx])
			g := int(rgba[rgbaIdx+1])
			b := int(rgba[rgbaIdx+2])

			// Y
			yVal := (66*r + 129*g + 25*b + 128) >> 8 + 16
			dst[yOffset+y*w+x] = byte(yVal)

			// U & V (subsampled by 2x2)
			if y%2 == 0 && x%2 == 0 {
				uVal := (-38*r - 74*g + 112*b + 128) >> 8 + 128
				vVal := (112*r - 94*g - 18*b + 128) >> 8 + 128
				dst[uOffset+(y/2)*(w/2)+(x/2)] = byte(uVal)
				dst[vOffset+(y/2)*(w/2)+(x/2)] = byte(vVal)
			}
		}
	}
	return dst
}

// ConvertYUV420pToRGBA converts planar YUV420p to contiguous RGBA (Task 67).
func ConvertYUV420pToRGBA(w, h int, yuv []byte) []byte {
	dst := core.GlobalGet(w * h * 4)
	ySize := w * h
	uvSize := (w / 2) * (h / 2)

	yOffset := 0
	uOffset := ySize
	vOffset := ySize + uvSize

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if y*w+x >= len(yuv) {
				break
			}
			yVal := int(yuv[yOffset+y*w+x]) - 16

			uIdx := uOffset + (y/2)*(w/2) + (x/2)
			vIdx := vOffset + (y/2)*(w/2) + (x/2)
			if uIdx >= len(yuv) || vIdx >= len(yuv) {
				break
			}
			uVal := int(yuv[uIdx]) - 128
			vVal := int(yuv[vIdx]) - 128

			// YUV to RGB Integer conversion
			r := (298*yVal + 409*vVal + 128) >> 8
			g := (298*yVal - 100*uVal - 208*vVal + 128) >> 8
			b := (298*yVal + 516*uVal + 128) >> 8

			if r < 0 {
				r = 0
			} else if r > 255 {
				r = 255
			}
			if g < 0 {
				g = 0
			} else if g > 255 {
				g = 255
			}
			if b < 0 {
				b = 0
			} else if b > 255 {
				b = 255
			}

			rgbaIdx := (y*w + x) * 4
			dst[rgbaIdx] = byte(r)
			dst[rgbaIdx+1] = byte(g)
			dst[rgbaIdx+2] = byte(b)
			dst[rgbaIdx+3] = 255
		}
	}
	return dst
}

// ConvertTIFF16ToSDR converts a 16-bit image (typically image.RGBA64) to an 8-bit RGBA video frame (Task 78).
func ConvertTIFF16ToSDR(img image.Image) (*core.VideoFrame, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	dst := core.GlobalGet(w * h * 4)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.At(x+bounds.Min.X, y+bounds.Min.Y)
			r, g, b, a := c.RGBA() // returns 0-65535 values

			// Map 16-bit HDR range (0-65535) to 8-bit SDR range (0-255)
			r8 := byte(r >> 8)
			g8 := byte(g >> 8)
			b8 := byte(b >> 8)
			a8 := byte(a >> 8)

			rgbaIdx := (y*w + x) * 4
			dst[rgbaIdx] = r8
			dst[rgbaIdx+1] = g8
			dst[rgbaIdx+2] = b8
			dst[rgbaIdx+3] = a8
		}
	}

	return &core.VideoFrame{
		Width:  w,
		Height: h,
		Format: "rgba",
		Data:   dst,
	}, nil
}
