package discovery

import (
	"context"
	"errors"
	"github.com/veltryx/vision/internal/camera"
	"github.com/veltryx/vision/pkg/models"
	"sync"
	"testing"
)

type provider struct {
	mu   sync.Mutex
	list []models.Camera
	err  error
}

func (p *provider) Discover(context.Context) ([]models.Camera, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.list, p.err
}
func (p *provider) Open(context.Context, models.Camera) (camera.Device, error) {
	return nil, errors.New("not implemented in this test")
}
func TestDisconnectReconnectAndFailedScan(t *testing.T) {
	p := &provider{list: []models.Camera{{Serial: "a", ID: "id1"}}}
	r := New(p, nil)
	ctx := context.Background()
	if e := r.Discover(ctx); e != nil {
		t.Fatal(e)
	}
	r.UpdateInfo(models.Camera{Serial: "a", ID: "id1", Name: "Custom name"})
	p.list = nil
	if e := r.Discover(ctx); e != nil {
		t.Fatal(e)
	}
	list := r.List()
	if len(list) != 1 || list[0].Connected || list[0].Name != "Custom name" {
		t.Fatal(list)
	}
	p.list = []models.Camera{{Serial: "a", ID: "id2"}}
	if e := r.Discover(ctx); e != nil {
		t.Fatal(e)
	}
	list = r.List()
	if !list[0].Connected || list[0].ID != "id2" || list[0].Name != "Custom name" {
		t.Fatal(list)
	}
	p.err = errors.New("scan failed")
	if e := r.Discover(ctx); e == nil {
		t.Fatal("missing error")
	}
	if !r.List()[0].Connected {
		t.Fatal("failed scan changed connection state")
	}
}
