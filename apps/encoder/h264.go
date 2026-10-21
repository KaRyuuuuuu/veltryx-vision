//go:build !x264 || !cgo

package encoder

import "errors"

func New(c Config, w, h int) (Encoder, error) {
	return nil, errors.New("H.264 unavailable: rebuild with -tags x264 and install libx264 development files")
}
