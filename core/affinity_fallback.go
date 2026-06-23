//go:build !linux

package core

// SetThreadAffinity is a no-op fallback on non-Linux operating systems.
func SetThreadAffinity(cpuIDs []int) error {
	return nil
}
