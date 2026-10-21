package detection

import (
	"time"

	"github.com/veltryx/vision/internal/frames"
	"github.com/veltryx/vision/internal/roi"
	"github.com/veltryx/vision/pkg/models"
)

type Result struct {
	Name      string         `json:"name"`
	State     string         `json:"state"`
	ROIPixels int            `json:"roi_pixels"`
	Confirmed int            `json:"confirmed"`
	Required  int            `json:"required"`
	Timestamp time.Time      `json:"timestamp"`
	Zone      string         `json:"zone"`
	Changed   int            `json:"changed"`
	Threshold int            `json:"threshold"`
	Triggered bool           `json:"triggered"`
	Pixels    []models.Point `json:"pixels,omitempty"`
}
type Detector struct {
	Zone               models.Zone
	previous           *frames.Frame
	mask               []roi.Span
	maskPixels         int
	consecutive, quiet int
	armed              bool
	last               time.Time
}

func New(z models.Zone) *Detector { return &Detector{Zone: z, armed: true} }
func (d *Detector) Process(f *frames.Frame, debug bool) Result {
	r := Result{Zone: d.Zone.ID, Threshold: d.Zone.MinPixels, Name: d.Zone.Name, State: "initializing", Required: d.Zone.ConfirmFrames}
	if !f.Valid() {
		return r
	}
	r.Timestamp = f.Timestamp
	if d.previous == nil || d.previous.Width != f.Width || d.previous.Height != f.Height {
		d.mask, d.maskPixels = roi.Raster(d.Zone, f.Width, f.Height)
		d.previous = f
		d.consecutive = 0
		return r
	}
	if f.Sequence > 0 && d.previous.Sequence > 0 && f.Sequence != d.previous.Sequence+1 {
		d.consecutive = 0
	}
	for _, span := range d.mask {
		for i := span.Start; i < span.End; i++ {
			delta := int(f.Pixels[i]) - int(d.previous.Pixels[i])
			if delta < 0 {
				delta = -delta
			}
			if delta >= d.Zone.PixelDelta {
				r.Changed++
				if debug && len(r.Pixels) < 2048 {
					r.Pixels = append(r.Pixels, models.Point{X: (float64(i%f.Width) + 0.5) / float64(f.Width), Y: (float64(i/f.Width) + 0.5) / float64(f.Height)})
				}
			}
		}
	}
	d.previous = f
	active := r.Changed >= d.Zone.MinPixels && float64(r.Changed) <= float64(d.maskPixels)*d.Zone.MaxChangedRatio
	if active {
		d.consecutive++
		d.quiet = 0
	} else {
		d.consecutive = 0
		d.quiet++
		if d.quiet >= d.Zone.QuietFrames {
			d.armed = true
		}
	}
	if d.armed && d.consecutive >= d.Zone.ConfirmFrames && (d.last.IsZero() || f.Timestamp.Sub(d.last) >= time.Duration(d.Zone.CooldownMS)*time.Millisecond) {
		r.Triggered = true
		d.armed = false
		d.last = f.Timestamp
		d.consecutive = 0
	}
	r.ROIPixels = d.maskPixels
	r.Confirmed = d.consecutive
	switch {
	case r.Triggered:
		r.State = "triggered"
	case float64(r.Changed) > float64(d.maskPixels)*d.Zone.MaxChangedRatio:
		r.State = "global_change"
	case !d.armed:
		r.State = "rearming"
	case !d.last.IsZero() && f.Timestamp.Sub(d.last) < time.Duration(d.Zone.CooldownMS)*time.Millisecond:
		r.State = "cooldown"
	case active:
		r.State = "confirming"
	default:
		r.State = "idle"
	}
	return r
}
