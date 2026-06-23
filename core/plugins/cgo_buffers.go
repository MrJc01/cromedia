package plugins

import (
	"fmt"
	"sync"
	"unsafe"
)

var (
	allocatedBuffers = make(map[*CGOBuffer]string)
	buffersMu        sync.Mutex
)

// CGOBuffer represents a contiguous memory buffer safe to transfer across the CGO boundary.
type CGOBuffer struct {
	ptr  unsafe.Pointer
	size int
	data []byte
}

// AllocCGOBuffer allocates a contiguous byte buffer of specified size.
func AllocCGOBuffer(size int) (*CGOBuffer, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid buffer size: %d", size)
	}

	// Go slices of basic primitive types (like bytes) are guaranteed to be contiguous in memory,
	// making their backing arrays safe to pass directly to CGO APIs.
	data := make([]byte, size)
	ptr := unsafe.Pointer(&data[0])

	buf := &CGOBuffer{
		ptr:  ptr,
		size: size,
		data: data,
	}

	buffersMu.Lock()
	allocatedBuffers[buf] = fmt.Sprintf("CGOBuffer allocated size: %d bytes", size)
	buffersMu.Unlock()

	return buf, nil
}

// Pointer returns the unsafe pointer to the buffer's start.
func (b *CGOBuffer) Pointer() unsafe.Pointer {
	return b.ptr
}

// Bytes returns the backing Go slice overlaying the buffer.
func (b *CGOBuffer) Bytes() []byte {
	return b.data
}

// Size returns the size of the buffer in bytes.
func (b *CGOBuffer) Size() int {
	return b.size
}

// CopyFrom copies a slice into the buffer.
func (b *CGOBuffer) CopyFrom(src []byte) int {
	return copy(b.data, src)
}

// Free releases the buffer and untracks it.
func (b *CGOBuffer) Free() {
	buffersMu.Lock()
	delete(allocatedBuffers, b)
	buffersMu.Unlock()
}

// GetLeakedBuffers returns details of all currently allocated/leaked buffers.
func GetLeakedBuffers() []string {
	buffersMu.Lock()
	defer buffersMu.Unlock()
	var leaks []string
	for _, v := range allocatedBuffers {
		leaks = append(leaks, v)
	}
	return leaks
}

// SliceFromPtr converts a raw unsafe.Pointer and size into a Go byte slice without copying.
func SliceFromPtr(ptr unsafe.Pointer, size int) []byte {
	if ptr == nil || size <= 0 {
		return nil
	}
	return unsafe.Slice((*byte)(ptr), size)
}
