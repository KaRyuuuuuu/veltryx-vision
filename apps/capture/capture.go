package capture

import (
	"context"
	"sync"
	"time"

	"github.com/veltryx/vision/internal/camera"
	"github.com/veltryx/vision/internal/frames"
	"github.com/veltryx/vision/pkg/models"
)

type Stats struct {
	AcquisitionDropped uint64  `json:"acquisition_dropped"`
	Frames             uint64  `json:"frames"`
	Dropped            uint64  `json:"dropped"`
	FPS                float64 `json:"measured_fps"`
	LatencyMS          float64 `json:"latency_ms"`
	Running            bool    `json:"running"`
	Error              string  `json:"error,omitempty"`
}
type Session struct {
	op           sync.Mutex
	mu           sync.Mutex
	device       camera.Device
	Hub          *frames.Hub
	cancel       context.CancelFunc
	done         chan struct{}
	started      bool
	stats        Stats
	window       time.Time
	windowFrames uint64
	lastSequence uint64
}

func newSession(d camera.Device) *Session            { return &Session{device: d, Hub: frames.NewHub()} }
func (s *Session) Info() models.Camera               { return s.device.Info() }
func (s *Session) Configure(v models.Settings) error { return s.device.Configure(v) }
func (s *Session) Start(ctx context.Context) {
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	run, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.started = true
	s.stats.Running = true
	s.stats.Error = ""
	s.window = time.Now()
	s.windowFrames = 0
	s.lastSequence = 0
	go func() {
		err := s.device.Run(run, func(f *frames.Frame) {
			s.Hub.Publish(f)
			s.mu.Lock()
			if f.Sequence > s.lastSequence+1 && s.lastSequence > 0 {
				s.stats.AcquisitionDropped += f.Sequence - s.lastSequence - 1
			}
			s.lastSequence = f.Sequence
			s.stats.Frames++
			s.windowFrames++
			elapsed := time.Since(s.window)
			if elapsed >= time.Second {
				s.stats.FPS = float64(s.windowFrames) / elapsed.Seconds()
				s.window = time.Now()
				s.windowFrames = 0
			}
			s.stats.LatencyMS = float64(time.Since(f.Timestamp).Microseconds()) / 1000
			s.mu.Unlock()
		})
		s.mu.Lock()
		s.stats.Running = false
		s.stats.FPS = 0
		if err != nil && run.Err() == nil {
			s.stats.Error = err.Error()
		}
		s.mu.Unlock()
		close(s.done)
	}()
}
func (s *Session) Stop() {
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	cancel()
	<-done
	s.mu.Lock()
	s.started = false
	s.mu.Unlock()
}
func (s *Session) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.stats
	v.Dropped = s.Hub.Dropped()
	return v
}
func (s *Session) Close() error { s.Stop(); return s.device.Close() }
