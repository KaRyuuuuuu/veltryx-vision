package roi

import (
	"math"
	"testing"

	"github.com/veltryx/vision/pkg/models"
)

func validZone() models.Zone {
	return models.Zone{ID: "a", Kind: "rectangle", Points: []models.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}, PixelDelta: 20, MinPixels: 1, ConfirmFrames: 2, QuietFrames: 2, MaxChangedRatio: 0.8}
}
func TestNormalizedMask(t *testing.T) {
	z := validZone()
	if e := Validate(z); e != nil {
		t.Fatal(e)
	}
	if !Contains(z.Points, 0.5, 0.5) || Contains(z.Points, 1.5, 0.5) {
		t.Fatal("containment")
	}
	if len(Mask(z, 8, 4)) != 32 {
		t.Fatal("mask size")
	}
	z.Points = []models.Point{{X: 0, Y: 0}, {X: 0.5, Y: 0}, {X: 0.5, Y: 1}, {X: 0, Y: 1}}
	if len(Mask(z, 8, 4)) != 16 {
		t.Fatal("normalized rectangle")
	}
}
func TestInvalidZones(t *testing.T) {
	for _, change := range []func(*models.Zone){func(z *models.Zone) { z.Points[0].X = math.NaN() }, func(z *models.Zone) { z.Points[0].X = 1.1 }, func(z *models.Zone) {
		z.Points = []models.Point{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 1, Y: 0}, {X: 0, Y: 1}}
	}, func(z *models.Zone) { z.ConfirmFrames = 0 }, func(z *models.Zone) { z.PreFrames = 61 }} {
		z := validZone()
		change(&z)
		if Validate(z) == nil {
			t.Fatal("accepted invalid ROI")
		}
	}
}
func TestConcavePolygon(t *testing.T) {
	z := validZone()
	z.Kind = "polygon"
	z.Points = []models.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0.5, Y: 0.5}, {X: 0, Y: 1}}
	if e := Validate(z); e != nil {
		t.Fatal(e)
	}
	if Contains(z.Points, 0.5, 0.9) {
		t.Fatal("concavity filled")
	}
}

func TestRasterMatchesPixelCenters(t *testing.T) {
	z := validZone()
	z.Kind = "polygon"
	z.Points = []models.Point{{X: 0.05, Y: 0.1}, {X: 0.95, Y: 0.2}, {X: 0.7, Y: 0.9}, {X: 0.5, Y: 0.4}, {X: 0.1, Y: 0.9}}
	for _, size := range [][2]int{{16, 9}, {127, 83}, {4, 4}} {
		w, h := size[0], size[1]
		mask := map[int]bool{}
		for _, i := range Mask(z, w, h) {
			mask[i] = true
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				want := Contains(z.Points, (float64(x)+0.5)/float64(w), (float64(y)+0.5)/float64(h))
				if mask[y*w+x] != want {
					t.Fatalf("raster mismatch at %d,%d", x, y)
				}
			}
		}
	}
}
