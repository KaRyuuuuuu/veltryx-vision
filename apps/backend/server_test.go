package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/veltryx/vision/apps/encoder"
	"github.com/veltryx/vision/apps/screenshots"
	"github.com/veltryx/vision/internal/camera"
	"github.com/veltryx/vision/pkg/models"
)

func TestSimulatedAPI(t *testing.T) {
	dir := t.TempDir()
	s, e := NewServer(context.Background(), camera.Simulation{Count: 2}, filepath.Join(dir, "config.json"), filepath.Join(dir, "shots"), encoder.Default())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	handler := s.Handler()
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
		return w
	}
	w := request("POST", "/api/v1/discovery", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var list []models.Camera
	if e = json.Unmarshal(w.Body.Bytes(), &list); e != nil || len(list) != 2 {
		t.Fatal(e, list)
	}
	deadline := time.Now().Add(3 * time.Second)
	for s.cameras.Get("SIM-001").Hub.Latest() == nil {
		if time.Now().After(deadline) {
			t.Fatal("no frames")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if w = request("PUT", "/api/v1/cameras/SIM-001/settings", `{"width":0,"height":2,"fps":0}`); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w = request("PUT", "/api/v1/cameras/SIM-001/roi", `[{"id":"bad"}]`); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w = request("PUT", "/api/v1/cameras/SIM-001/orientation", `{"rotation":45}`); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w = request("PUT", "/api/v1/cameras/SIM-001/orientation", `{}`); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w = request("PUT", "/api/v1/cameras/SIM-001/orientation", `{"rotation":90}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if s.config.Rotation("SIM-001") != 90 {
		t.Fatal("orientation not persisted")
	}
	if w = request("POST", "/api/v1/cameras/SIM-001/screenshots", `{"format":"png"}`); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var shot screenshots.Capture
	_ = json.Unmarshal(w.Body.Bytes(), &shot)
	if shot.Rotation != 90 || shot.Width != 360 || shot.Height != 640 {
		t.Fatal("saved screenshot not rotated", shot)
	}
	if w = request("GET", "/api/v1/screenshots/"+shot.ID+"/image", ""); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal(w.Code)
	}
	if w = request("POST", "/api/v1/cameras/SIM-001/stream/stop", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if s.cameras.Get("SIM-001").Stats().Running {
		t.Fatal("did not stop")
	}
	if !s.cameras.Get("SIM-002").Stats().Running {
		t.Fatal("stopped unrelated camera")
	}
	if w = request("POST", "/api/v1/cameras/SIM-001/stream/start", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = request("GET", "/api/v1/cameras/missing", ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
