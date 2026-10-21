package discovery

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/veltryx/vision/internal/camera"
	"github.com/veltryx/vision/pkg/models"
)

type Registry struct {
	mu       sync.RWMutex
	scan     sync.Mutex
	provider camera.Provider
	cameras  map[string]models.Camera
	changed  func([]models.Camera)
}

func New(p camera.Provider, changed func([]models.Camera)) *Registry {
	return &Registry{provider: p, cameras: map[string]models.Camera{}, changed: changed}
}
func (r *Registry) Discover(ctx context.Context) error {
	r.scan.Lock()
	defer r.scan.Unlock()
	list, e := r.provider.Discover(ctx)
	if e != nil {
		return e
	}
	r.mu.Lock()
	for id, c := range r.cameras {
		c.Connected = false
		r.cameras[id] = c
	}
	for _, c := range list {
		old := r.cameras[c.Serial]
		if c.Name == "" {
			c.Name = old.Name
		}
		if len(c.Formats) == 0 {
			c.Formats = old.Formats
		}
		if c.Settings.Width == 0 {
			c.Settings = old.Settings
		}
		if c.Capabilities == nil {
			c.Capabilities = old.Capabilities
		}
		c.Connected = true
		r.cameras[c.Serial] = c
	}
	r.mu.Unlock()
	if r.changed != nil {
		r.changed(r.List())
	}
	return nil
}
func (r *Registry) Run(ctx context.Context, interval time.Duration, report func(error)) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if e := r.Discover(ctx); e != nil && ctx.Err() == nil {
			report(e)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
func (r *Registry) List() []models.Camera {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]models.Camera, 0, len(r.cameras))
	for _, c := range r.cameras {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Serial < out[j].Serial })
	return out
}
func (r *Registry) UpdateInfo(c models.Camera) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.cameras[c.Serial]; ok {
		c.Connected = old.Connected
		r.cameras[c.Serial] = c
	}
}
