package camera

import (
	"context"

	"github.com/veltryx/vision/internal/frames"
	"github.com/veltryx/vision/pkg/models"
)

type Device interface {
	Info() models.Camera
	Configure(models.Settings) error
	Run(context.Context, func(*frames.Frame)) error
	Close() error
}
type Provider interface {
	Discover(context.Context) ([]models.Camera, error)
	Open(context.Context, models.Camera) (Device, error)
}
