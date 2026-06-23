//go:build !libavfilter

package bridge

import (
	"errors"

	"cromedia/core"
)

// NewAVFilterBridge returns a stub filter because libavfilter CGO bridge is not compiled in.
func NewAVFilterBridge(filterSpec string) (core.VideoFilter, error) {
	return nil, errors.New("libavfilter bridge is not compiled. Rebuild with -tags 'libavfilter'")
}
