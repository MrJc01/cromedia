//go:build !nvidia
// +build !nvidia

package hardware

import (
	"cromedia/core/pipeline"
	"fmt"
)

// NewNVENCTranscoder returns a hardware-accelerated transcoder if available.
// This is the Stub version that runs when 'nvidia' build tag is NOT present.
func NewNVENCTranscoder() (pipeline.Transcoder, error) {
	return nil, fmt.Errorf("NVENC support not compiled. Use -tags nvidia to enable.")
}
