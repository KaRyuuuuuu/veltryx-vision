package roi

import (
	"errors"
	"math"

	"github.com/veltryx/vision/pkg/models"
)

func Validate(z models.Zone) error {
	if z.ID == "" || len(z.ID) > 80 || len(z.Name) > 160 {
		return errors.New("invalid ROI identity")
	}
	if z.Kind != "rectangle" && z.Kind != "polygon" {
		return errors.New("kind must be rectangle or polygon")
	}
	if len(z.Points) < 3 || len(z.Points) > 64 || (z.Kind == "rectangle" && len(z.Points) != 4) {
		return errors.New("invalid vertex count")
	}
	area := 0.0
	for i, p := range z.Points {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) || p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
			return errors.New("coordinates must be finite and normalized")
		}
		q := z.Points[(i+1)%len(z.Points)]
		if p == q {
			return errors.New("duplicate adjacent ROI vertices")
		}
		area += p.X*q.Y - q.X*p.Y
	}
	if z.Kind == "rectangle" {
		p := z.Points
		if !(p[0].Y == p[1].Y && p[1].X == p[2].X && p[2].Y == p[3].Y && p[3].X == p[0].X) {
			return errors.New("rectangle edges must align with image axes")
		}
	}
	if math.Abs(area) < 1e-8 {
		return errors.New("ROI has zero area")
	}
	for i := range z.Points {
		a, b := z.Points[i], z.Points[(i+1)%len(z.Points)]
		for j := i + 1; j < len(z.Points); j++ {
			if j == i+1 || (i == 0 && j == len(z.Points)-1) {
				continue
			}
			c, d := z.Points[j], z.Points[(j+1)%len(z.Points)]
			if intersects(a, b, c, d) {
				return errors.New("ROI edges intersect")
			}
		}
	}
	if z.PixelDelta < 1 || z.PixelDelta > 255 || z.MinPixels < 1 || z.ConfirmFrames < 1 || z.ConfirmFrames > 120 || z.QuietFrames < 1 || z.QuietFrames > 120 || z.CooldownMS < 0 || z.CooldownMS > 3600000 || z.PreFrames < 0 || z.PreFrames > 60 || z.PostFrames < 0 || z.PostFrames > 60 || z.MaxChangedRatio <= 0 || z.MaxChangedRatio > 1 {
		return errors.New("invalid motion parameters")
	}
	return nil
}
func cross(a, b, c models.Point) float64 { return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X) }
func intersects(a, b, c, d models.Point) bool {
	if math.Max(a.X, b.X) < math.Min(c.X, d.X) || math.Max(c.X, d.X) < math.Min(a.X, b.X) || math.Max(a.Y, b.Y) < math.Min(c.Y, d.Y) || math.Max(c.Y, d.Y) < math.Min(a.Y, b.Y) {
		return false
	}
	return cross(a, b, c)*cross(a, b, d) <= 0 && cross(c, d, a)*cross(c, d, b) <= 0
}
