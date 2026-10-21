package config

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/veltryx/vision/pkg/models"
)

func TestPersistentConcurrentUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for _, id := range []string{"a", "b", "c"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := models.Settings{Width: 640, Height: 360, FPS: 25, Exposure: 1000}
			if e := s.Update(id, func(c *Camera) { c.Settings = &v }); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	restored, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"a", "b", "c"} {
		if restored.Get(id).Settings.Width != 640 {
			t.Fatal(id)
		}
	}
	c := s.Get("a")
	c.Settings.Width = 12
	if s.Get("a").Settings.Width != 640 {
		t.Fatal("mutable configuration escaped")
	}
}
