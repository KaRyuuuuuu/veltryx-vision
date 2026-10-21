//go:build !aravis || !cgo

package camera

import "errors"

func NewAravis() (Provider, error) {
	return nil, errors.New("Aravis disabled: build with -tags aravis and install aravis-0.8 development files, or explicitly use -simulate")
}
