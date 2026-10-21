package detection

import (
	"testing"
	"time"

	"github.com/veltryx/vision/internal/frames"
	"github.com/veltryx/vision/pkg/models"
)

func zone() models.Zone {
	return models.Zone{ID: "test", Kind: "rectangle", Points: []models.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}, PixelDelta: 20, MinPixels: 2, ConfirmFrames: 2, QuietFrames: 2, CooldownMS: 1000, MaxChangedRatio: 0.8}
}
func frame(ms int, values ...byte) *frames.Frame {
	p := make([]byte, 16)
	copy(p, values)
	return &frames.Frame{Width: 4, Height: 4, Timestamp: time.UnixMilli(int64(ms)), Pixels: p}
}
func TestConfirmationRearmAndCooldown(t *testing.T) {
	d := New(zone())
	if d.Process(frame(1000), false).Triggered {
		t.Fatal("first frame")
	}
	if d.Process(frame(1100, 100, 100), false).Triggered {
		t.Fatal("early trigger")
	}
	if !d.Process(frame(1200), false).Triggered {
		t.Fatal("missing confirmed trigger")
	}
	if d.Process(frame(1300, 100, 100), false).Triggered {
		t.Fatal("not rearmed")
	}
	d.Process(frame(1400, 100, 100), false)
	d.Process(frame(1500, 100, 100), false)
	d.Process(frame(1600), false)
	if d.Process(frame(1700, 100, 100), false).Triggered {
		t.Fatal("cooldown")
	}
	if !d.Process(frame(2300), false).Triggered {
		t.Fatal("cooldown did not expire")
	}
}
func TestGlobalChangeAndNoiseFiltered(t *testing.T) {
	d := New(zone())
	d.Process(frame(1000), false)
	full := frame(1100)
	for i := range full.Pixels {
		full.Pixels[i] = 200
	}
	r := d.Process(full, true)
	if r.Triggered || r.Changed != 16 || len(r.Pixels) != 16 {
		t.Fatal(r)
	}
	d.Process(frame(1200), false)
	if d.Process(frame(1300, 10, 10), false).Triggered {
		t.Fatal("noise")
	}
	if d.Process(&frames.Frame{Width: 2, Height: 2, Pixels: make([]byte, 4)}, true).Triggered {
		t.Fatal("resize trigger")
	}
}

func TestDiagnosticStateExplainsMissingTrigger(t *testing.T) {
	d := New(zone())
	if r := d.Process(nil, false); r.Triggered {
		t.Fatal(r)
	}
	d.Process(frame(1000), false)
	r := d.Process(frame(1100, 100, 100), false)
	if r.State != "confirming" || r.Confirmed != 1 || r.Required != 2 || r.ROIPixels != 16 {
		t.Fatal(r)
	}
	r = d.Process(frame(1200), false)
	if r.State != "triggered" {
		t.Fatal(r)
	}
	r = d.Process(frame(1300, 100, 100), false)
	if r.State != "rearming" {
		t.Fatal(r)
	}
	f := frame(1400)
	for i := range f.Pixels {
		f.Pixels[i] = 200
	}
	if r = d.Process(f, false); r.State != "global_change" {
		t.Fatal(r)
	}
}
