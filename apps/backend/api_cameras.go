package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/veltryx/vision/internal/config"
	"github.com/veltryx/vision/pkg/models"
)

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	var v models.Settings
	if !decode(w, r, &v) {
		return
	}
	if e := config.ValidateSettings(v); e != nil {
		fail(w, 400, e)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.require(w, r)
	if c == nil {
		return
	}
	id := r.PathValue("serial")
	running := c.Stats().Running
	c.Stop()
	rt := s.runtimes[id]
	if rt.video != nil {
		rt.video.Close()
		rt.video = nil
	}
	before := c.Info().Settings
	rollback := func(status int, cause error) {
		if e := c.Configure(before); e != nil {
			s.remove(id)
			fail(w, 500, errors.Join(cause, fmt.Errorf("rollback failed; camera will reconnect: %w", e)))
			return
		}
		if running {
			c.Start(s.ctx)
		}
		fail(w, status, cause)
	}
	if e := c.Configure(v); e != nil {
		rollback(422, e)
		return
	}
	c.Hub.ClearLatest()
	actual := c.Info().Settings
	if e := s.config.Update(id, func(cfg *config.Camera) { cfg.Settings = &actual }); e != nil {
		rollback(500, e)
		return
	}
	rt.motion.Configure(s.config.Get(id).Zones)
	if running {
		c.Start(s.ctx)
	}
	s.bus.Publish("settings", id, actual)
	writeJSON(w, 200, actual)
}
func (s *Server) getROI(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.require(w, r) == nil {
		return
	}
	z := s.config.Get(r.PathValue("serial")).Zones
	if z == nil {
		z = []models.Zone{}
	}
	writeJSON(w, 200, z)
}
func (s *Server) putROI(w http.ResponseWriter, r *http.Request) {
	var zones []models.Zone
	if !decode(w, r, &zones) {
		return
	}
	if e := config.ValidateZones(zones); e != nil {
		fail(w, 400, e)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.require(w, r) == nil {
		return
	}
	id := r.PathValue("serial")
	if e := s.config.Update(id, func(c *config.Camera) { c.Zones = zones }); e != nil {
		fail(w, 500, e)
		return
	}
	s.runtimes[id].motion.Configure(zones)
	writeJSON(w, 200, zones)
}
func (s *Server) debug(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Enabled bool `json:"enabled"`
	}
	if !decode(w, r, &v) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.require(w, r) == nil {
		return
	}
	s.runtimes[r.PathValue("serial")].motion.Debug(v.Enabled)
	writeJSON(w, 200, v)
}
func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.require(w, r)
	if c == nil {
		return
	}
	s.stopped[r.PathValue("serial")] = false
	c.Start(s.ctx)
	writeJSON(w, 200, c.Stats())
}
func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.require(w, r)
	if c == nil {
		return
	}
	id := r.PathValue("serial")
	s.stopped[id] = true
	c.Stop()
	if rt := s.runtimes[id]; rt.video != nil {
		rt.video.Close()
		rt.video = nil
	}
	writeJSON(w, 200, c.Stats())
}

func (s *Server) getOrientation(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.require(w, r) == nil {
		return
	}
	cfg := s.config.Get(r.PathValue("serial"))
	writeJSON(w, 200, map[string]any{"rotation": s.config.Rotation(r.PathValue("serial")), "configured": cfg.Rotation != nil})
}
func (s *Server) putOrientation(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Rotation *int `json:"rotation"`
	}
	if !decode(w, r, &v) {
		return
	}
	if v.Rotation == nil || !config.ValidRotation(*v.Rotation) {
		fail(w, 400, "rotation must be 0, 90, 180 or 270")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.require(w, r) == nil {
		return
	}
	id := r.PathValue("serial")
	if e := s.config.Update(id, func(c *config.Camera) { c.Rotation = v.Rotation }); e != nil {
		fail(w, 500, e)
		return
	}
	data := map[string]any{"rotation": *v.Rotation, "configured": true}
	s.bus.Publish("orientation", id, data)
	writeJSON(w, 200, data)
}
