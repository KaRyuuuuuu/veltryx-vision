package screenshots

import (
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/veltryx/vision/internal/frames"
)

// CaptureJPEGQuality favors compact storage while retaining full image dimensions.
const CaptureJPEGQuality = 60

// Write is used by the live preview and ROI editor.
func Write(w io.Writer, f *frames.Frame, format string) error {
	return write(w, f, format, 92)
}

// WriteCapture compresses persisted screenshots independently of video/preview.
func WriteCapture(w io.Writer, f *frames.Frame, format string) error {
	return write(w, f, format, CaptureJPEGQuality)
}

func write(w io.Writer, f *frames.Frame, format string, quality int) error {
	img := &image.Gray{Pix: f.Pixels, Stride: f.Width, Rect: image.Rect(0, 0, f.Width, f.Height)}
	if format == "png" {
		return png.Encode(w, img)
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
}
