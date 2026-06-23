package plugins

import (
	"fmt"
	"sync"
)

// CodecContextPool provides pooling for codec contexts to avoid expensive allocations.
type CodecContextPool struct {
	mu      sync.Mutex
	pools   map[string][]interface{}
	factory map[string]func() (interface{}, error)
}

var globalContextPool = NewCodecContextPool()

// NewCodecContextPool creates a new context pool.
func NewCodecContextPool() *CodecContextPool {
	return &CodecContextPool{
		pools:   make(map[string][]interface{}),
		factory: make(map[string]func() (interface{}, error)),
	}
}

// Register registers a creation factory function for a codec.
func (p *CodecContextPool) Register(codec string, fact func() (interface{}, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.factory[codec] = fact
}

// Get retrieves a codec context from the pool, or creates a new one using the factory.
func (p *CodecContextPool) Get(codec string) (interface{}, error) {
	p.mu.Lock()
	if list, exists := p.pools[codec]; exists && len(list) > 0 {
		ctx := list[len(list)-1]
		p.pools[codec] = list[:len(list)-1]
		p.mu.Unlock()
		return ctx, nil
	}
	fact, exists := p.factory[codec]
	p.mu.Unlock()
	if !exists {
		return nil, fmt.Errorf("no factory registered for codec context: %s", codec)
	}
	return fact()
}

// Put returns a codec context to the pool for reuse.
func (p *CodecContextPool) Put(codec string, ctx interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pools[codec] = append(p.pools[codec], ctx)
}

// GlobalGetContext retrieves a context from the global pool.
func GlobalGetContext(codec string) (interface{}, error) {
	return globalContextPool.Get(codec)
}

// GlobalPutContext returns a context to the global pool.
func GlobalPutContext(codec string, ctx interface{}) {
	globalContextPool.Put(codec, ctx)
}
