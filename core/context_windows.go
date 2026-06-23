//go:build windows

package core

import (
	"time"
)

func GetCPUTimes() (time.Duration, time.Duration) {
	return 0, 0
}
