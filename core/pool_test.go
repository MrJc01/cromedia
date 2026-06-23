package core

import "testing"

func TestBufferPool(t *testing.T) {
	pool := NewBufferPool([]int{1024, 4096, 16384})

	// Test exact bucket matches
	buf := pool.Get(512)
	if len(buf) != 512 || cap(buf) != 1024 {
		t.Errorf("Expected cap 1024 for size 512, got len %d cap %d", len(buf), cap(buf))
	}
	pool.Put(buf)

	buf = pool.Get(2048)
	if len(buf) != 2048 || cap(buf) != 4096 {
		t.Errorf("Expected cap 4096 for size 2048, got len %d cap %d", len(buf), cap(buf))
	}
	pool.Put(buf)

	// Test fallback for sizes larger than maximum bucket
	buf = pool.Get(32768)
	if len(buf) != 32768 || cap(buf) != 32768 {
		t.Errorf("Expected cap 32768 for size 32768, got len %d cap %d", len(buf), cap(buf))
	}
	// Putting fallback buffer should not crash or map to pool
	pool.Put(buf)
}

func TestGlobalPool(t *testing.T) {
	buf := GlobalGet(100)
	if len(buf) != 100 {
		t.Errorf("Expected len 100, got %d", len(buf))
	}
	GlobalPut(buf)
}
