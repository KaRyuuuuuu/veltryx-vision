package frames

import "sync"

type Subscription struct {
	C      <-chan *Frame
	cancel func()
}

func (s Subscription) Close() { s.cancel() }

type Hub struct {
	mu          sync.Mutex
	next        uint64
	subscribers map[uint64]chan *Frame
	dropped     uint64
	latest      *Frame
}

func NewHub() *Hub { return &Hub{subscribers: make(map[uint64]chan *Frame)} }
func (h *Hub) Subscribe(capacity int) Subscription {
	if capacity < 1 {
		capacity = 1
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	id := h.next
	ch := make(chan *Frame, capacity)
	h.subscribers[id] = ch
	var once sync.Once
	return Subscription{ch, func() { once.Do(func() { h.mu.Lock(); defer h.mu.Unlock(); delete(h.subscribers, id); close(ch) }) }}
}

// Publish transfers ownership: callers must never mutate a published frame.
func (h *Hub) Publish(f *Frame) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.latest = f
	for _, ch := range h.subscribers {
		select {
		case ch <- f:
		default:
			select {
			case <-ch:
				h.dropped++
			default:
			}
			ch <- f
		}
	}
}
func (h *Hub) ClearLatest()    { h.mu.Lock(); defer h.mu.Unlock(); h.latest = nil }
func (h *Hub) Latest() *Frame  { h.mu.Lock(); defer h.mu.Unlock(); return h.latest }
func (h *Hub) Dropped() uint64 { h.mu.Lock(); defer h.mu.Unlock(); return h.dropped }
