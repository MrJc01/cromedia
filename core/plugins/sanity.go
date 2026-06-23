package plugins

import (
	"errors"
	"fmt"

	"cromedia/core"
)

var (
	// ErrInvalidFrameDimensions is returned when dimensions are zero, negative, or unreasonably large.
	ErrInvalidFrameDimensions = errors.New("invalid frame dimensions")

	// ErrInvalidFrameFormat is returned when the frame pixel format is missing or invalid.
	ErrInvalidFrameFormat = errors.New("invalid frame format")

	// ErrInvalidFrameBufferSize is returned when the buffer size is too small for the resolution/format.
	ErrInvalidFrameBufferSize = errors.New("invalid frame buffer size")

	// ErrInvalidAudioChannels is returned when audio channels count is out of standard limits.
	ErrInvalidAudioChannels = errors.New("invalid audio channels count")

	// ErrInvalidAudioSampleRate is returned when audio sample rate is out of standard limits.
	ErrInvalidAudioSampleRate = errors.New("invalid audio sample rate")
)

// ValidateVideoFrame verifies that a VideoFrame's data size matches its configuration.
func ValidateVideoFrame(f *core.VideoFrame) error {
	if f == nil {
		return errors.New("video frame is nil")
	}
	if f.Width <= 0 || f.Height <= 0 || f.Width > 16384 || f.Height > 16384 {
		return fmt.Errorf("%w: width=%d, height=%d", ErrInvalidFrameDimensions, f.Width, f.Height)
	}

	if f.Format == "" {
		return fmt.Errorf("%w: format cannot be empty", ErrInvalidFrameFormat)
	}

	// Calculate expected buffer size based on format
	var expectedSize int
	switch f.Format {
	case "rgba", "rgb32":
		expectedSize = f.Width * f.Height * 4
	case "rgb", "rgb24":
		expectedSize = f.Width * f.Height * 3
	case "yuv420p":
		// Y + U + V (1 + 1/4 + 1/4 = 1.5 bytes per pixel)
		expectedSize = (f.Width * f.Height * 3) / 2
	default:
		// For other formats, just ensure the buffer is non-empty
		if len(f.Data) == 0 {
			return fmt.Errorf("%w: buffer is empty for format %s", ErrInvalidFrameBufferSize, f.Format)
		}
		return nil
	}

	if len(f.Data) < expectedSize {
		return fmt.Errorf("%w: expected at least %d bytes, got %d for format %s", ErrInvalidFrameBufferSize, expectedSize, len(f.Data), f.Format)
	}

	return nil
}

// ValidateAudioFrame verifies that an AudioFrame has plausible attributes and buffers.
func ValidateAudioFrame(f *core.AudioFrame) error {
	if f == nil {
		return errors.New("audio frame is nil")
	}
	if f.Channels <= 0 || f.Channels > 64 {
		return fmt.Errorf("%w: %d", ErrInvalidAudioChannels, f.Channels)
	}
	if f.SampleRate < 8000 || f.SampleRate > 192000 {
		return fmt.Errorf("%w: %d Hz", ErrInvalidAudioSampleRate, f.SampleRate)
	}
	if len(f.Data) == 0 {
		return fmt.Errorf("%w: empty audio buffer", ErrInvalidFrameBufferSize)
	}
	if len(f.Data)%f.Channels != 0 {
		return fmt.Errorf("%w: buffer length %d is not a multiple of channel count %d", ErrInvalidFrameBufferSize, len(f.Data), f.Channels)
	}
	return nil
}
