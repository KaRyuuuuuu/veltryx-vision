package frames

import (
	"sync"
	"testing"
)

func TestSlowConsumerReceivesLatest(t *testing.T) {
	h := NewHub()
	slow := h.Subscribe(2)
	defer slow.Close()
	fast := h.Subscribe(1)
	defer fast.Close()
	for i := uint64(1); i <= 8; i++ {
		h.Publish(&Frame{Sequence: i})
		if got := (<-fast.C).Sequence; got != i {
			t.Fatal(got)
		}
	}
	if got := (<-slow.C).Sequence; got != 7 {
		t.Fatal(got)
	}
	if got := (<-slow.C).Sequence; got != 8 {
		t.Fatal(got)
	}
	if h.Dropped() != 6 {
		t.Fatal(h.Dropped())
	}
	slow.Close()
	h.Publish(&Frame{})
	if _, ok := <-slow.C; ok {
		t.Fatal("subscription not closed")
	}
}
func TestConcurrentSubscribePublish(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				s := h.Subscribe(1)
				h.Publish(&Frame{})
				s.Close()
			}
		}()
	}
	wg.Wait()
}
func TestBufferOrderAndBound(t *testing.T) {
	b := NewBuffer(3)
	if len(b.Snapshot()) != 0 {
		t.Fatal("not empty")
	}
	for i := uint64(1); i <= 8; i++ {
		b.Push(&Frame{Sequence: i})
	}
	got := b.Snapshot()
	if len(got) != 3 {
		t.Fatal(len(got))
	}
	for i, f := range got {
		if f.Sequence != uint64(i+6) {
			t.Fatal(f.Sequence)
		}
	}
	b.Push(&Frame{Sequence: 9})
	if got[0].Sequence != 6 {
		t.Fatal("snapshot mutated")
	}
}
