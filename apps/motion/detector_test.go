package motion

import (
	"context"
	"github.com/veltryx/vision/apps/screenshots"
	"github.com/veltryx/vision/internal/events"
	"github.com/veltryx/vision/internal/frames"
	"github.com/veltryx/vision/pkg/models"
	"testing"
	"time"
)

func TestPassageIncludesOriginalPrePostFrames(t *testing.T) {
	m, e := screenshots.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	bus := events.New()
	ch, unsubscribe := bus.Subscribe()
	defer unsubscribe()
	z := models.Zone{ID: "line", Kind: "rectangle", Points: []models.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}, PixelDelta: 20, MinPixels: 2, ConfirmFrames: 1, QuietFrames: 2, CooldownMS: 1000, MaxChangedRatio: 0.8, PreFrames: 1, PostFrames: 1}
	worker := New([]models.Zone{z}, m, bus)
	worker.Debug(true)
	hub := frames.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); worker.Run(ctx, hub) }()
	defer func() { cancel(); <-done }()
	publish := func(seq uint64, active bool) {
		pixels := make([]byte, 16)
		if active {
			pixels[0] = 100
			pixels[1] = 100
		}
		hub.Publish(&frames.Frame{Serial: "source", Sequence: seq, Timestamp: time.Now(), DeviceTimestamp: seq * 100, Width: 4, Height: 4, Pixels: pixels})
	}
	deadline := time.After(3 * time.Second)
	// First frame primes the detector; wait for its diagnostic event to ensure subscription.
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	ready := false
	for !ready {
		select {
		case <-ticker.C:
			publish(1, false)
		case e := <-ch:
			ready = e.Type == "motion"
		case <-deadline:
			t.Fatal("worker did not subscribe")
		}
	}
	publish(2, true)
	for {
		select {
		case e := <-ch:
			if e.Type == "passage" {
				publish(3, true)
				goto waitForImages
			}
		case <-deadline:
			t.Fatal("missing passage")
		}
	}
waitForImages:
	for {
		list := m.List("source", 10)
		if len(list) >= 3 {
			if len(list) != 3 {
				t.Fatal("unexpected capture count", len(list))
			}
			id := list[0].Passage
			seen := map[uint64]bool{}
			for _, s := range list {
				if s.Passage != id || s.DeviceTimestamp != s.Sequence*100 {
					t.Fatal(s)
				}
				seen[s.Sequence] = true
			}
			groups := m.Passages("source", 10)
			if len(groups) != 1 || groups[0].Representative.Sequence != 2 || groups[0].Representative.Role != "trigger" {
				t.Fatal("wrong representative frame", groups)
			}
			if !seen[1] || !seen[2] || !seen[3] {
				t.Fatal(seen)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatal("missing pre/post images", list)
		case <-time.After(5 * time.Millisecond):
		}
	}
}
