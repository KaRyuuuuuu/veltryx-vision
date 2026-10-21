package screenshots

import (
	"bytes"
	"image/jpeg"
	"testing"
	"time"

	"github.com/veltryx/vision/internal/frames"
)

func TestCompactCaptureRetainsDimensionsAndTimestamp(t *testing.T) {
	f := &frames.Frame{Width: 128, Height: 96, Timestamp: time.Unix(1, 123456789), Pixels: make([]byte, 128*96)}
	// Fine texture exercises JPEG compression rather than an almost empty image.
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			f.Pixels[y*f.Width+x] = byte(x*37 + y*71 + x*y)
		}
	}
	original := append([]byte(nil), f.Pixels...)
	stamp := f.Timestamp
	var preview, compact bytes.Buffer
	if e := Write(&preview, f, "jpeg"); e != nil {
		t.Fatal(e)
	}
	if e := WriteCapture(&compact, f, "jpeg"); e != nil {
		t.Fatal(e)
	}
	if compact.Len() >= preview.Len()*3/4 {
		t.Fatalf("insufficient size reduction: %d vs %d", compact.Len(), preview.Len())
	}
	img, e := jpeg.Decode(bytes.NewReader(compact.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	if img.Bounds().Dx() != f.Width || img.Bounds().Dy() != f.Height {
		t.Fatal("dimensions changed")
	}
	if !bytes.Equal(f.Pixels, original) || !f.Timestamp.Equal(stamp) {
		t.Fatal("original frame modified")
	}
	t.Logf("JPEG %d -> %d bytes", preview.Len(), compact.Len())
}
