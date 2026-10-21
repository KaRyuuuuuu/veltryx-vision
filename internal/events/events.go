package events

import (
	"sync"
	"time"

	"github.com/veltryx/vision/pkg/models"
)

type Bus struct {
	mu      sync.Mutex
	next    uint64
	clients map[uint64]chan models.Event
}

func New() *Bus { return &Bus{clients: map[uint64]chan models.Event{}} }
func (b *Bus) Publish(kind, serial string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := models.Event{Type: kind, Serial: serial, Time: time.Now().UTC(), Data: data}
	for _, ch := range b.clients {
		select {
		case ch <- e:
		default:
		}
	}
}
func (b *Bus) Subscribe() (<-chan models.Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	id := b.next
	ch := make(chan models.Event, 32)
	b.clients[id] = ch
	var once sync.Once
	return ch, func() { once.Do(func() { b.mu.Lock(); defer b.mu.Unlock(); delete(b.clients, id); close(ch) }) }
}
