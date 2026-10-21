package encoder

import (
	"time"

	"github.com/veltryx/vision/internal/frames"
)

type Sample struct {
	Data      []byte
	Timestamp time.Time
	Duration  time.Duration
}
type Encoder interface {
	Encode(*frames.Frame) (Sample, error)
	Close() error
}
type Factory func(Config, int, int) (Encoder, error)
