package motion

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/veltryx/vision/apps/screenshots"
	"github.com/veltryx/vision/internal/detection"
	"github.com/veltryx/vision/internal/events"
	"github.com/veltryx/vision/internal/frames"
	"github.com/veltryx/vision/pkg/models"
)

type pending struct {
	id, zone  string
	remaining int
}
type Worker struct {
	mu       sync.Mutex
	zones    []models.Zone
	latest   []detection.Result
	revision uint64
	debug    bool
	shots    *screenshots.Manager
	bus      *events.Bus
}

func New(z []models.Zone, s *screenshots.Manager, b *events.Bus) *Worker {
	return &Worker{zones: z, shots: s, bus: b}
}
func (w *Worker) Configure(z []models.Zone) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.latest = nil
	w.zones = z
	w.revision++
}
func (w *Worker) Latest() []detection.Result {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]detection.Result, len(w.latest))
	copy(out, w.latest)
	return out
}
func (w *Worker) Debug(v bool) { w.mu.Lock(); defer w.mu.Unlock(); w.debug = v }
func (w *Worker) Run(ctx context.Context, hub *frames.Hub) {
	sub := hub.Subscribe(4)
	defer sub.Close()
	ring := frames.NewBuffer(1)
	frameBytes := 0
	var detectors []*detection.Detector
	var rev uint64 = ^uint64(0)
	var jobs []pending
	lastDebug := time.Time{}
	queue := screenshots.NewQueue(w.shots, func(c screenshots.Capture, e error) {
		if e != nil {
			slog.Error("screenshot failed", "error", e)
			w.bus.Publish("error", c.Serial, e.Error())
		} else {
			w.bus.Publish("screenshot", c.Serial, c)
		}
	})
	defer queue.Close()
	save := func(f *frames.Frame, id, zone string) { queue.Enqueue(f, id, zone) }

	for {
		select {
		case <-ctx.Done():
			return
		case f := <-sub.C:
			if len(f.Pixels) != frameBytes {
				frameBytes = len(f.Pixels)
				capacity := min(60, (64*1024*1024)/max(1, frameBytes))
				ring = frames.NewBuffer(capacity)
			}
			w.mu.Lock()
			if rev != w.revision {
				detectors = nil
				for _, z := range w.zones {
					detectors = append(detectors, detection.New(z))
				}
				rev = w.revision
			}
			debug := w.debug
			w.mu.Unlock()
			remaining := jobs[:0]
			for _, j := range jobs {
				save(f, j.id, j.zone)
				j.remaining--
				if j.remaining > 0 {
					remaining = append(remaining, j)
				}
			}
			jobs = remaining
			emit := debug && time.Since(lastDebug) >= 100*time.Millisecond
			results := make([]detection.Result, 0, len(detectors))
			for _, d := range detectors {
				r := d.Process(f, emit)
				summary := r
				summary.Pixels = nil
				results = append(results, summary)
				if emit || r.Triggered {
					w.bus.Publish("motion", f.Serial, r)
				}
				if r.Triggered {
					if len(jobs) >= 64 {
						w.bus.Publish("error", f.Serial, "passage queue full")
						continue
					}
					id := screenshots.ID()
					past := ring.Snapshot()
					start := len(past) - d.Zone.PreFrames
					if start < 0 {
						start = 0
					}
					for _, p := range past[start:] {
						save(p, id, d.Zone.ID)
					}
					save(f, id, d.Zone.ID)
					if d.Zone.PostFrames > 0 {
						jobs = append(jobs, pending{id, d.Zone.ID, d.Zone.PostFrames})
					}
					w.bus.Publish("passage", f.Serial, map[string]string{"id": id, "zone": d.Zone.ID})
				}
			}
			w.mu.Lock()
			if rev == w.revision {
				w.latest = results
			}
			w.mu.Unlock()
			if emit {
				lastDebug = time.Now()
			}
			if len(detectors) > 0 {
				ring.Push(f)
			}
		}
	}
}
