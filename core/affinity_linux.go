//go:build linux

package core

import (
	"runtime"
	"syscall"
	"unsafe"
)

// SetThreadAffinity pins the calling OS thread to the specified CPU IDs on Linux.
func SetThreadAffinity(cpuIDs []int) error {
	// Lock the goroutine to the current OS thread
	runtime.LockOSThread()

	if len(cpuIDs) == 0 {
		return nil
	}

	// Support up to 1024 CPUs using a 128-byte bitmask
	var mask [128]byte
	for _, cpu := range cpuIDs {
		if cpu >= 0 && cpu < 1024 {
			byteIdx := cpu / 8
			bitIdx := cpu % 8
			mask[byteIdx] |= 1 << bitIdx
		}
	}

	// Call sched_setaffinity(0, size, &mask)
	// PID 0 refers to the calling thread
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_SCHED_SETAFFINITY,
		0,
		uintptr(len(mask)),
		uintptr(unsafe.Pointer(&mask[0])),
	)

	if errno != 0 {
		return errno
	}
	return nil
}
