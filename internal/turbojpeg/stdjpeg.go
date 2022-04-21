//go:build !(turbojpeg && arm64)

package turbojpeg

import (
	"image"
	"image/color"
	"image/jpeg"
	"io"
)

type Encoder struct {
	w         io.Writer
	quality   int
	width     int
	img       *image.RGBA
	strideRGB int
	yoffset   int
}

func NewEncoder(w io.Writer, quality, width, height int) (*Encoder, error) {
	return &Encoder{
		w:         w,
		quality:   quality,
		width:     width,
		img:       image.NewRGBA(image.Rect(0, 0, width, height)),
		strideRGB: 3 * width,
	}, nil
}

func (e *Encoder) EncodePixels(pix []byte, lines int) {
	for y := 0; y < lines; y++ {
		for x := 0; x < e.width; x++ {
			e.img.SetRGBA(x, e.yoffset+y, color.RGBA{
				R: pix[(y*e.strideRGB)+(x*3)+0],
				G: pix[(y*e.strideRGB)+(x*3)+1],
				B: pix[(y*e.strideRGB)+(x*3)+2],
				A: 0xFF,
			})
		}
	}
	e.yoffset += lines
}

func (e *Encoder) Flush() error {
	return jpeg.Encode(e.w, e.img, &jpeg.Options{
		Quality: e.quality,
	})
}
