package ocr

import (
	"context"
	"errors"

	"github.com/veltryx/vision/apps/screenshots"
)

var ErrUnavailable = errors.New("no OCR engine configured")

type Reading struct {
	Number     string  `json:"number"`
	Confidence float64 `json:"confidence"`
}
type Engine interface {
	Recognize(context.Context, screenshots.Capture) (Reading, error)
}
type Disabled struct{}

func (Disabled) Recognize(context.Context, screenshots.Capture) (Reading, error) {
	return Reading{}, ErrUnavailable
}
