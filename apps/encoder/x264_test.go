//go:build x264 && cgo

package encoder

import (
	"testing"
	"time"

	"github.com/veltryx/vision/internal/frames"
)

func TestNativeH264AnnexB(t *testing.T) {
	e, err := New(Default(), 64, 48)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	f := &frames.Frame{Width: 64, Height: 48, Pixels: make([]byte, 64*48), Timestamp: time.Now()}
	sample, err := e.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(sample.Data) < 5 || sample.Data[0] != 0 || sample.Data[1] != 0 || sample.Data[2] != 0 || sample.Data[3] != 1 {
		t.Fatal("not Annex B H.264")
	}
	if sample.Duration != 40*time.Millisecond {
		t.Fatal(sample.Duration)
	}
	e.Close()
	if _, err = e.Encode(f); err == nil {
		t.Fatal("closed encoder accepted frame")
	}
}
