package core

import (
	"sync"
)

// Event represents a system event with a topic and arbitrary data.
type Event struct {
	Topic string
	Data  interface{}
}

// EventListener represents a callback to receive events.
type EventListener func(event Event)

// EventBus is a concurrent publisher-subscriber manager.
type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]EventListener
}

var globalBus *EventBus
var once sync.Once

// GetEventBus retrieves the singleton instance of the global event bus.
func GetEventBus() *EventBus {
	once.Do(func() {
		globalBus = &EventBus{
			listeners: make(map[string][]EventListener),
		}
	})
	return globalBus
}

// Subscribe registers a listener for a specific topic.
func (eb *EventBus) Subscribe(topic string, listener EventListener) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.listeners[topic] = append(eb.listeners[topic], listener)
}

// Publish broadcasts an event to all subscribers of the topic.
func (eb *EventBus) Publish(topic string, data interface{}) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	listeners, exists := eb.listeners[topic]
	if !exists {
		return
	}

	event := Event{Topic: topic, Data: data}
	for _, listener := range listeners {
		// Run callback in a separate goroutine to avoid blocking the publisher
		go listener(event)
	}
}
