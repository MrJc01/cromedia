package core

import (
	"sync"
	"sync/atomic"
)

// BufferPool manages reusable byte slices to minimize garbage collection overhead.
type BufferPool struct {
	pools    map[int]*sync.Pool
	sizes    []int
	getCount int64
	putCount int64
}

var globalPool *BufferPool

func init() {
	// Standard bucket sizes: 16KB, 64KB, 256KB, 1MB, 4MB
	globalPool = NewBufferPool([]int{16384, 65536, 262144, 1048576, 4194304})
}

// NewBufferPool creates a new pool manager with the given bucket sizes (must be sorted ascending).
func NewBufferPool(sizes []int) *BufferPool {
	pools := make(map[int]*sync.Pool)
	for _, size := range sizes {
		sz := size
		pools[sz] = &sync.Pool{
			New: func() interface{} {
				return make([]byte, sz)
			},
		}
	}
	return &BufferPool{pools: pools, sizes: sizes}
}

// Get retrieves a slice of at least the requested capacity.
func (p *BufferPool) Get(capacity int) []byte {
	atomic.AddInt64(&p.getCount, 1)
	for _, size := range p.sizes {
		if size >= capacity {
			buf := p.pools[size].Get().([]byte)
			return buf[:capacity]
		}
	}
	// Fallback for extremely large sizes
	return make([]byte, capacity)
}

// Put returns a slice back to the appropriate pool bucket.
func (p *BufferPool) Put(buf []byte) {
	atomic.AddInt64(&p.putCount, 1)
	capacity := cap(buf)
	// Find the matching bucket
	for i := len(p.sizes) - 1; i >= 0; i-- {
		size := p.sizes[i]
		if capacity == size {
			p.pools[size].Put(buf[:size]) // Reset slice boundaries to original capacity
			return
		}
	}
}

// GlobalGet retrieves a buffer from the global pool.
func GlobalGet(capacity int) []byte {
	return globalPool.Get(capacity)
}

// GlobalPut returns a buffer to the global pool.
func GlobalPut(buf []byte) {
	globalPool.Put(buf)
}

// GlobalStats returns the get and put counts of the global pool.
func GlobalStats() (int64, int64) {
	return atomic.LoadInt64(&globalPool.getCount), atomic.LoadInt64(&globalPool.putCount)
}

// GlobalResetStats resets the counters.
func GlobalResetStats() {
	atomic.StoreInt64(&globalPool.getCount, 0)
	atomic.StoreInt64(&globalPool.putCount, 0)
}
