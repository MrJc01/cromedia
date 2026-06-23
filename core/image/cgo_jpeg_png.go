package image

import (
	"errors"
	"image"
	"io"
)

// HasLibjpegTurbo returns true if compiled with libjpeg-turbo CGO bindings.
func HasLibjpegTurbo() bool {
	return false
}

// HasLibpng returns true if compiled with libpng CGO bindings.
func HasLibpng() bool {
	return false
}

// DecodeJPEGTurbo decodes JPEG using libjpeg-turbo. Fallback to stdlib.
func DecodeJPEGTurbo(r io.Reader) (image.Image, error) {
	return nil, errors.New("libjpeg-turbo CGO wrapper not active in this build")
}

// DecodePNGNative decodes PNG using libpng CGO wrapper. Fallback to stdlib.
func DecodePNGNative(r io.Reader) (image.Image, error) {
	return nil, errors.New("libpng CGO wrapper not active in this build")
}
