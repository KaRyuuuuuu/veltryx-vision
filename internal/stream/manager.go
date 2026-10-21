package stream

import (
	"context"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/veltryx/vision/apps/encoder"
	"github.com/veltryx/vision/internal/frames"
)

type Stats struct {
	Bitrate float64 `json:"bitrate_kbps"`
	Frames  uint64  `json:"encoded_frames"`
	Peers   int     `json:"peers"`
	Error   string  `json:"error,omitempty"`
}
type Stream struct {
	closed        bool
	closeOnce     sync.Once
	mu            sync.Mutex
	track         *webrtc.TrackLocalStaticSample
	peers         map[*webrtc.PeerConnection]struct{}
	stats         Stats
	cancel        context.CancelFunc
	done          chan struct{}
	width, height int
	config        encoder.Config
}

func New(ctx context.Context, hub *frames.Hub, c encoder.Config) (*Stream, error) {
	f := hub.Latest()
	if !f.Valid() {
		return nil, errNoFrame
	}
	c.FPS = clampFPS(c.FPS)
	enc, e := encoder.New(c, f.Width, f.Height)
	if e != nil {
		return nil, e
	}
	track, e := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"}, "video", "veltryx")
	if e != nil {
		enc.Close()
		return nil, e
	}
	run, cancel := context.WithCancel(ctx)
	s := &Stream{track: track, peers: map[*webrtc.PeerConnection]struct{}{}, cancel: cancel, done: make(chan struct{}), width: f.Width, height: f.Height, config: c}
	go s.run(run, hub, enc)
	return s, nil
}
func clampFPS(f float64) float64 {
	if f < 1 {
		return 25
	}
	return f
}
func (s *Stream) run(ctx context.Context, hub *frames.Hub, enc encoder.Encoder) {
	defer close(s.done)
	defer enc.Close()
	sub := hub.Subscribe(2)
	defer sub.Close()
	start := time.Now()
	bytes := 0
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-sub.C:
			sample, e := enc.Encode(f)
			if e != nil {
				s.mu.Lock()
				s.stats.Error = e.Error()
				s.mu.Unlock()
				return
			}
			if len(sample.Data) == 0 {
				continue
			}
			if !last.IsZero() {
				sample.Duration = f.Timestamp.Sub(last)
				if sample.Duration <= 0 || sample.Duration > time.Second {
					sample.Duration = time.Second / time.Duration(clampFPS(s.config.FPS))
				}
			}
			last = f.Timestamp
			e = s.track.WriteSample(media.Sample{Data: sample.Data, Duration: sample.Duration})
			s.mu.Lock()
			s.stats.Frames++
			bytes += len(sample.Data)
			if time.Since(start) >= time.Second {
				s.stats.Bitrate = float64(bytes*8) / time.Since(start).Seconds() / 1000
				bytes = 0
				start = time.Now()
			}
			if e != nil {
				s.stats.Error = e.Error()
			}
			s.mu.Unlock()
		}
	}
}
func (s *Stream) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.stats
	v.Peers = len(s.peers)
	return v
}
func (s *Stream) Close() { s.closeOnce.Do(s.close) }
func (s *Stream) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	<-s.done
	s.mu.Lock()
	peers := make([]*webrtc.PeerConnection, 0, len(s.peers))
	for p := range s.peers {
		peers = append(peers, p)
	}
	s.mu.Unlock()
	for _, p := range peers {
		_ = p.Close()
	}
}
