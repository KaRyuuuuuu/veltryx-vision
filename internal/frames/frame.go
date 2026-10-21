// Package frames distributes immutable, owned Gray8 camera frames.
package frames

import "time"

type Frame struct {
	Serial          string
	Sequence        uint64
	Timestamp       time.Time
	DeviceTimestamp uint64
	Width, Height   int
	Pixels          []byte
}

func (f *Frame) Valid() bool {
	return f != nil && f.Width > 0 && f.Height > 0 && len(f.Pixels) == f.Width*f.Height
}
