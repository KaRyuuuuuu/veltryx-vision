package screenshots

import (
	"errors"
	"github.com/veltryx/vision/internal/frames"
)

// Rotate turns original Gray8 pixels clockwise without changing the source frame.
func Rotate(f *frames.Frame, degrees int) (*frames.Frame, error) {
	if !f.Valid() {
		return nil, errors.New("invalid image")
	}
	if degrees == 0 {
		return f, nil
	}
	if degrees != 90 && degrees != 180 && degrees != 270 {
		return nil, errors.New("invalid rotation")
	}
	out := *f
	if degrees != 180 {
		out.Width, out.Height = f.Height, f.Width
	}
	out.Pixels = make([]byte, len(f.Pixels))
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			var dx, dy int
			switch degrees {
			case 90:
				dx, dy = f.Height-1-y, x
			case 180:
				dx, dy = f.Width-1-x, f.Height-1-y
			case 270:
				dx, dy = y, f.Width-1-x
			}
			out.Pixels[dy*out.Width+dx] = f.Pixels[y*f.Width+x]
		}
	}
	return &out, nil
}
