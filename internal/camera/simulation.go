package camera

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/veltryx/vision/internal/config"
	"github.com/veltryx/vision/internal/frames"
	"github.com/veltryx/vision/pkg/models"
)

type Simulation struct{ Count int }

func (s Simulation) Discover(ctx context.Context) ([]models.Camera, error) {
	var out []models.Camera
	for i := 0; i < s.Count; i++ {
		out = append(out, models.Camera{Serial: fmt.Sprintf("SIM-%03d", i+1), ID: fmt.Sprintf("sim:%d", i+1), Name: fmt.Sprintf("Simulation %d", i+1), Vendor: "Veltryx", Model: "Synthetic Gray8", Formats: []string{"Mono8"}, Settings: models.Settings{Width: 640, Height: 360, FPS: 25, Exposure: 10000}, Connected: true, Simulated: true})
	}
	return out, ctx.Err()
}
func (s Simulation) Open(_ context.Context, c models.Camera) (Device, error) {
	return &simDevice{info: c}, nil
}

type simDevice struct {
	mu   sync.Mutex
	info models.Camera
}

func (d *simDevice) Info() models.Camera { d.mu.Lock(); defer d.mu.Unlock(); return d.info }
func (d *simDevice) Configure(s models.Settings) error {
	if e := config.ValidateSettings(s); e != nil {
		return e
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.info.Settings = s
	return nil
}
func (d *simDevice) Close() error { return nil }
func (d *simDevice) Run(ctx context.Context, publish func(*frames.Frame)) error {
	var seq uint64
	for {
		s := d.Info().Settings
		timer := time.NewTimer(time.Duration(float64(time.Second) / s.FPS))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case now := <-timer.C:
			seq++
			p := make([]byte, s.Width*s.Height)
			x0 := int(seq*7)%(s.Width+100) - 100
			for y := 0; y < s.Height; y++ {
				for x := 0; x < s.Width; x++ {
					v := byte(24 + (y * 24 / s.Height))
					if x >= x0 && x < x0+90 && y > s.Height/3 && y < 2*s.Height/3 {
						v = 200
					}
					p[y*s.Width+x] = v
				}
			}
			publish(&frames.Frame{Serial: d.Info().Serial, Sequence: seq, Timestamp: now.UTC(), DeviceTimestamp: uint64(now.UnixNano()), Width: s.Width, Height: s.Height, Pixels: p})
		}
	}
}
