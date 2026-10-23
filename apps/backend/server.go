package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/veltryx/vision/apps/capture"
	"github.com/veltryx/vision/apps/encoder"
	"github.com/veltryx/vision/apps/motion"
	"github.com/veltryx/vision/apps/screenshots"
	"github.com/veltryx/vision/internal/camera"
	"github.com/veltryx/vision/internal/config"
	"github.com/veltryx/vision/internal/discovery"
	"github.com/veltryx/vision/internal/events"
	"github.com/veltryx/vision/internal/stream"
	"github.com/veltryx/vision/pkg/models"
)

type runtime struct {
	motion *motion.Worker
	cancel context.CancelFunc
	done   chan struct{}
	video  *stream.Stream
}
type Server struct {
	mu            sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	registry      *discovery.Registry
	cameras       *capture.Manager
	config        *config.Store
	shots         *screenshots.Manager
	bus           *events.Bus
	runtimes      map[string]*runtime
	stopped       map[string]bool
	encoder       encoder.Config
	discoveryDone chan struct{}
}

func NewServer(parent context.Context, p camera.Provider, path, dir string, ec encoder.Config) (*Server, error) {
	cfg, e := config.Open(path)
	if e != nil {
		return nil, e
	}
	shots, e := screenshots.New(dir)
	if e != nil {
		return nil, e
	}
	shots.SetRotationSource(cfg.Rotation)
	ctx, cancel := context.WithCancel(parent)
	s := &Server{ctx: ctx, cancel: cancel, config: cfg, shots: shots, cameras: capture.New(p), bus: events.New(), runtimes: map[string]*runtime{}, stopped: map[string]bool{}, encoder: ec, discoveryDone: make(chan struct{})}
	s.registry = discovery.New(p, s.reconcile)
	go func() {
		defer close(s.discoveryDone)
		s.registry.Run(ctx, 3*time.Second, func(e error) { slog.Warn("discovery", "error", e) })
	}()
	return s, nil
}
func (s *Server) reconcile(list []models.Camera) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	for _, c := range list {
		existing := s.cameras.Get(c.Serial)
		if !c.Connected || (existing != nil && existing.Stats().Error != "") {
			s.remove(c.Serial)
			if !c.Connected {
				continue
			}
		}
		if s.cameras.Get(c.Serial) != nil {
			continue
		}
		session, e := s.cameras.Open(s.ctx, c)
		if e != nil {
			slog.Warn("camera open", "serial", c.Serial, "error", e)
			continue
		}
		cfg := s.config.Get(c.Serial)
		if cfg.Settings != nil {
			if e = session.Configure(*cfg.Settings); e != nil {
				slog.Warn("restore camera settings", "serial", c.Serial, "error", e)
			}
		}
		s.registry.UpdateInfo(session.Info())
		worker := motion.New(cfg.Zones, s.shots, s.bus)
		ctx, cancel := context.WithCancel(s.ctx)
		r := &runtime{motion: worker, cancel: cancel, done: make(chan struct{})}
		s.runtimes[c.Serial] = r
		go func() { defer close(r.done); worker.Run(ctx, session.Hub) }()
		if !s.stopped[c.Serial] {
			session.Start(s.ctx)
		}
	}
	s.bus.Publish("cameras", "", s.cameraList())
}
func (s *Server) remove(serial string) {
	if r := s.runtimes[serial]; r != nil {
		r.cancel()
		<-r.done
		if r.video != nil {
			r.video.Close()
		}
		delete(s.runtimes, serial)
	}
	s.cameras.Remove(serial)
}
func (s *Server) cameraList() []models.Camera {
	list := s.registry.List()
	for i, c := range list {
		if session := s.cameras.Get(c.Serial); session != nil {
			list[i] = session.Info()
			list[i].Connected = c.Connected && session.Stats().Error == ""
		} else {
			list[i].Connected = false
		}
	}
	return list
}
func (s *Server) Close() {
	s.cancel()
	<-s.discoveryDone
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.runtimes {
		s.remove(id)
	}
	s.cameras.Close()
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/v1/cameras", s.list)
	mux.HandleFunc("POST /api/v1/discovery", s.discover)
	mux.HandleFunc("GET /api/v1/cameras/{serial}", s.detail)
	mux.HandleFunc("PUT /api/v1/cameras/{serial}/settings", s.settings)
	mux.HandleFunc("GET /api/v1/cameras/{serial}/orientation", s.getOrientation)
	mux.HandleFunc("PUT /api/v1/cameras/{serial}/orientation", s.putOrientation)
	mux.HandleFunc("GET /api/v1/cameras/{serial}/roi", s.getROI)
	mux.HandleFunc("PUT /api/v1/cameras/{serial}/roi", s.putROI)
	mux.HandleFunc("POST /api/v1/cameras/{serial}/motion-test", s.debug)
	mux.HandleFunc("POST /api/v1/cameras/{serial}/stream/start", s.start)
	mux.HandleFunc("POST /api/v1/cameras/{serial}/stream/stop", s.stop)
	mux.HandleFunc("POST /api/v1/cameras/{serial}/webrtc", s.offer)
	mux.HandleFunc("GET /api/v1/cameras/{serial}/stats", s.stats)
	mux.HandleFunc("GET /api/v1/cameras/{serial}/frame", s.frame)
	mux.HandleFunc("POST /api/v1/cameras/{serial}/screenshots", s.screenshot)
	mux.HandleFunc("GET /api/v1/passages", s.listPassages)
	mux.HandleFunc("GET /api/v1/screenshots", s.listScreenshots)
	mux.HandleFunc("GET /api/v1/screenshots/{id}/image", s.image)
	mux.HandleFunc("GET /api/v1/events", s.events)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}
