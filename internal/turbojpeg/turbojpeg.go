//go:build turbojpeg && arm64

package turbojpeg

import (
	"io"

	"github.com/stapelberg/turbojpeg/jpeg"
)

type Encoder = jpeg.Encoder

func NewEncoder(w io.Writer, quality, width, height int) (*Encoder, error) {
	return jpeg.NewRGBEncoder(w, &jpeg.EncoderOptions{
		Quality: quality,
	}, width, height)
}
