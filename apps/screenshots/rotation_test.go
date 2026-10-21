package screenshots

import (
	"bytes"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/veltryx/vision/internal/frames"
)

func TestRotationBeforeSaving(t *testing.T) {
	stamp := time.Unix(1, 123456789)
	f := &frames.Frame{Serial: "a", Width: 3, Height: 2, Pixels: []byte{1, 2, 3, 4, 5, 6}, Timestamp: stamp, DeviceTimestamp: 42, Sequence: 7}
	for _, tt := range []struct {
		rotation, w, h int
		pixels         []byte
	}{{0, 3, 2, []byte{1, 2, 3, 4, 5, 6}}, {90, 2, 3, []byte{4, 1, 5, 2, 6, 3}}, {180, 3, 2, []byte{6, 5, 4, 3, 2, 1}}, {270, 2, 3, []byte{3, 6, 2, 5, 1, 4}}} {
		out, e := Rotate(f, tt.rotation)
		if e != nil {
			t.Fatal(e)
		}
		if out.Width != tt.w || out.Height != tt.h || !bytes.Equal(out.Pixels, tt.pixels) {
			t.Fatal(tt.rotation, out)
		}
		m, e := New(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		m.SetRotationSource(func(string) int { return tt.rotation })
		c, e := m.Save(f, "png", "passage", "roi")
		if e != nil {
			t.Fatal(e)
		}
		path, _ := m.Path(c.ID)
		file, e := os.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		img, e := png.Decode(file)
		file.Close()
		if e != nil {
			t.Fatal(e)
		}
		if img.Bounds().Dx() != tt.w || img.Bounds().Dy() != tt.h || c.Rotation != tt.rotation || !c.Timestamp.Equal(stamp) || c.DeviceTimestamp != 42 || c.Sequence != 7 {
			t.Fatal(c)
		}
		for y := 0; y < tt.h; y++ {
			for x := 0; x < tt.w; x++ {
				red, _, _, _ := img.At(x, y).RGBA()
				if byte(red>>8) != tt.pixels[y*tt.w+x] {
					t.Fatal("incorrect saved pixels")
				}
			}
		}
	}
	if !bytes.Equal(f.Pixels, []byte{1, 2, 3, 4, 5, 6}) || f.Width != 3 || f.Height != 2 {
		t.Fatal("source image modified")
	}
	if _, e := Rotate(f, 45); e == nil {
		t.Fatal("invalid angle accepted")
	}
}
