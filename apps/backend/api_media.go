package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/veltryx/vision/apps/screenshots"
	"github.com/veltryx/vision/internal/stream"
)

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.require(w, r)
	if c == nil {
		return
	}
	v := stream.Stats{}
	if rt := s.runtimes[r.PathValue("serial")]; rt.video != nil {
		v = rt.video.Stats()
	}
	writeJSON(w, 200, map[string]any{"capture": c.Stats(), "video": v, "motion": s.runtimes[r.PathValue("serial")].motion.Latest(), "screenshots": s.shots.Stats(r.PathValue("serial"))})
}
func (s *Server) frame(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	c := s.require(w, r)
	if c == nil {
		s.mu.Unlock()
		return
	}
	f := c.Hub.Latest()
	s.mu.Unlock()
	if !f.Valid() {
		fail(w, 409, "no frame available")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Frame-Timestamp", f.Timestamp.Format(time.RFC3339Nano))
	_ = screenshots.Write(w, f, "jpeg")
}
func (s *Server) screenshot(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Format string `json:"format"`
	}
	if !decode(w, r, &v) {
		return
	}
	if v.Format == "" {
		v.Format = "jpeg"
	}
	if v.Format != "jpeg" && v.Format != "png" {
		fail(w, 400, "format must be jpeg or png")
		return
	}
	s.mu.Lock()
	c := s.require(w, r)
	if c == nil {
		s.mu.Unlock()
		return
	}
	f := c.Hub.Latest()
	s.mu.Unlock()
	if !f.Valid() {
		fail(w, 409, "no frame available")
		return
	}
	shot, e := s.shots.Save(f, v.Format, "", "")
	if e != nil {
		fail(w, 500, e)
		return
	}
	s.bus.Publish("screenshot", f.Serial, shot)
	writeJSON(w, 201, shot)
}
func (s *Server) listScreenshots(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		var e error
		limit, e = strconv.Atoi(v)
		if e != nil || limit < 1 || limit > 1000 {
			fail(w, 400, "limit must be 1..1000")
			return
		}
	}
	writeJSON(w, 200, s.shots.List(r.URL.Query().Get("serial"), limit))
}
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	p, ok := s.shots.Path(r.PathValue("id"))
	if !ok {
		fail(w, 404, "capture not found")
		return
	}
	http.ServeFile(w, r, p)
}
func (s *Server) offer(w http.ResponseWriter, r *http.Request) {
	var v webrtc.SessionDescription
	if !decode(w, r, &v) {
		return
	}
	if v.Type != webrtc.SDPTypeOffer || len(v.SDP) > 256000 {
		fail(w, 400, "expected SDP offer")
		return
	}
	s.mu.Lock()
	c := s.require(w, r)
	if c == nil {
		s.mu.Unlock()
		return
	}
	if !c.Stats().Running {
		s.mu.Unlock()
		fail(w, 409, "acquisition stopped")
		return
	}
	rt := s.runtimes[r.PathValue("serial")]
	if rt.video != nil && rt.video.Stats().Error != "" {
		rt.video.Close()
		rt.video = nil
	}
	if rt.video == nil {
		ec := s.encoder
		ec.FPS = c.Info().Settings.FPS
		if c.Info().Settings.FPSUnlimited {
			ec.FPS = c.Stats().FPS
			if ec.FPS < 1 {
				ec.FPS = 25
			}
		}
		video, e := stream.New(s.ctx, c.Hub, ec)
		if e != nil {
			s.mu.Unlock()
			fail(w, 503, e)
			return
		}
		rt.video = video
	}
	video := rt.video
	s.mu.Unlock()
	answer, e := video.Answer(r.Context(), v)
	if e != nil {
		fail(w, 422, e)
		return
	}
	writeJSON(w, 200, answer)
}

func (s *Server) listPassages(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v < 1 || v > 200 {
			fail(w, 400, "limit must be 1..200")
			return
		}
		limit = v
	}
	writeJSON(w, 200, s.shots.Passages(r.URL.Query().Get("serial"), limit))
}
