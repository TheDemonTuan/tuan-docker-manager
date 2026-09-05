package events

import (
	"sync"

	"docker-panel/internal/models"
)

type Bus struct {
	subscribers map[chan models.DockerEvent]struct{}
	mu          sync.RWMutex
}

func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[chan models.DockerEvent]struct{}),
	}
}

func (b *Bus) Subscribe() chan models.DockerEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan models.DockerEvent, 50)
	b.subscribers[ch] = struct{}{}
	return ch
}

func (b *Bus) Unsubscribe(ch chan models.DockerEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
}

func (b *Bus) Publish(event models.DockerEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Non-blocking drop if consumer is slow
		}
	}
}
