//go:build !windows

package core

import (
	"syscall"
	"time"
)

func GetCPUTimes() (time.Duration, time.Duration) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err == nil {
		user := time.Duration(usage.Utime.Sec)*time.Second + time.Duration(usage.Utime.Usec)*time.Microsecond
		sys := time.Duration(usage.Stime.Sec)*time.Second + time.Duration(usage.Stime.Usec)*time.Microsecond
		return user, sys
	}
	return 0, 0
}
