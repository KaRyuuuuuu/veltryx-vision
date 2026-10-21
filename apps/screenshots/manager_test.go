package screenshots

import (
	"image/jpeg"
	"os"
	"testing"
	"time"

	"github.com/veltryx/vision/internal/frames"
)

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	m, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	f := &frames.Frame{Serial: "test/serial", Timestamp: time.Now().UTC(), Width: 4, Height: 4, Pixels: make([]byte, 16), DeviceTimestamp: 42}
	c, e := m.Save(f, "jpeg", "passage", "roi")
	if e != nil {
		t.Fatal(e)
	}
	p, ok := m.Path(c.ID)
	if !ok {
		t.Fatal("missing image")
	}
	file, e := os.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	img, e := jpeg.Decode(file)
	if e != nil || img.Bounds().Dx() != 4 {
		t.Fatal(e)
	}
	restored, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	list := restored.List("test/serial", 10)
	if len(list) != 1 || list[0].DeviceTimestamp != 42 || list[0].Passage != "passage" {
		t.Fatal(list)
	}
	if _, e = m.Save(f, "gif", "", ""); e == nil {
		t.Fatal("unsupported format")
	}
}

func TestStatisticsSurviveRestartAndGroupPassages(t *testing.T) {
	dir := t.TempDir()
	m, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	f := &frames.Frame{Serial: "a", Timestamp: time.Now(), Width: 2, Height: 2, Pixels: make([]byte, 4)}
	for _, id := range []string{"one", "one", "two"} {
		if _, e = m.Save(f, "png", id, "roi"); e != nil {
			t.Fatal(e)
		}
	}
	f.Serial = "b"
	if _, e = m.Save(f, "png", "one", "roi"); e != nil {
		t.Fatal(e)
	}
	restored, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	s := restored.Stats("a")
	if s.Images != 3 || s.Passages != 2 || s.Last == nil {
		t.Fatal(s)
	}
	if restored.Stats("b").Images != 1 {
		t.Fatal("camera counts mixed")
	}
	if restored.Stats("missing").Images != 0 {
		t.Fatal("missing camera count")
	}
}
