package roi

import (
	"github.com/veltryx/vision/pkg/models"
	"math"
	"sort"
)

func Contains(points []models.Point, x, y float64) bool {
	inside := false
	j := len(points) - 1
	for i, p := range points {
		q := points[j]
		if (p.Y > y) != (q.Y > y) && x < (q.X-p.X)*(y-p.Y)/(q.Y-p.Y)+p.X {
			inside = !inside
		}
		j = i
	}
	return inside
}

// Span is a contiguous half-open run of pixel indices in a single image row.
// Row spans avoid retaining one machine-sized index per pixel for every ROI.
type Span struct{ Start, End int }

func Raster(z models.Zone, w, h int) ([]Span, int) {
	if w <= 0 || h <= 0 {
		return nil, 0
	}
	spans := make([]Span, 0, h)
	intersections := make([]float64, 0, len(z.Points))
	count := 0
	for row := 0; row < h; row++ {
		y := (float64(row) + 0.5) / float64(h)
		intersections = intersections[:0]
		for i, p := range z.Points {
			q := z.Points[(i+1)%len(z.Points)]
			if (p.Y > y) != (q.Y > y) {
				intersections = append(intersections, (q.X-p.X)*(y-p.Y)/(q.Y-p.Y)+p.X)
			}
		}
		sort.Float64s(intersections)
		for i := 0; i+1 < len(intersections); i += 2 {
			left := max(0, min(w, int(math.Ceil(intersections[i]*float64(w)-0.5))))
			right := max(0, min(w, int(math.Ceil(intersections[i+1]*float64(w)-0.5))))
			if right > left {
				spans = append(spans, Span{Start: row*w + left, End: row*w + right})
				count += right - left
			}
		}
	}
	return spans, count
}
func Mask(z models.Zone, w, h int) []int {
	spans, count := Raster(z, w, h)
	out := make([]int, 0, count)
	for _, span := range spans {
		for i := span.Start; i < span.End; i++ {
			out = append(out, i)
		}
	}
	return out
}
