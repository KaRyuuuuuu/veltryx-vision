package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/veltryx/vision/apps/capture"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, e any) {
	writeJSON(w, status, map[string]any{"error": fmt.Sprint(e)})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		fail(w, 400, e)
		return false
	}
	if e := d.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
		fail(w, 400, "expected one JSON document")
		return false
	}
	return true
}

// require is called with the lifecycle lock held.
func (s *Server) require(w http.ResponseWriter, r *http.Request) *capture.Session {
	v := s.cameras.Get(r.PathValue("serial"))
	if v == nil {
		fail(w, 404, "camera unavailable")
	}
	return v
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, 200, s.cameraList())
}
func (s *Server) discover(w http.ResponseWriter, r *http.Request) {
	if e := s.registry.Discover(r.Context()); e != nil {
		fail(w, 503, e)
		return
	}
	s.list(w, r)
}
func (s *Server) detail(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.require(w, r)
	if c != nil {
		writeJSON(w, 200, c.Info())
	}
}
