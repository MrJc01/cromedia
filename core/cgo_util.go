package core

import (
	"errors"
	"sync"
)

// CGORegistry manages Go objects mapped to unique integer handles.
// This allows Go applications using CGO to pass safe integer references to C memory,
// avoiding violations of the Go garbage collection pointer rules (which restrict passing
// Go pointers referencing other Go pointers to C).
type CGORegistry struct {
	mu      sync.RWMutex
	nextID  uintptr
	objects map[uintptr]interface{}
}

// NewCGORegistry instantiates a new CGORegistry.
func NewCGORegistry() *CGORegistry {
	return &CGORegistry{
		nextID:  1,
		objects: make(map[uintptr]interface{}),
	}
}

// GlobalCGO is a shared registry instance for CGO pointer mapping.
var GlobalCGO = NewCGORegistry()

// Register saves an object into the registry and returns its unique uintptr handle.
func (r *CGORegistry) Register(val interface{}) uintptr {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := r.nextID
	r.nextID++
	r.objects[id] = val
	return id
}

// Lookup retrieves the object mapped to the given handle.
func (r *CGORegistry) Lookup(id uintptr) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	val, exists := r.objects[id]
	if !exists {
		return nil, errors.New("cgo registry: handle not found or expired")
	}
	return val, nil
}

// Unregister deletes the handle from the registry, freeing the Go object for garbage collection.
func (r *CGORegistry) Unregister(id uintptr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.objects, id)
}
