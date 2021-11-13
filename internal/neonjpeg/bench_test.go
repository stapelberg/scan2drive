//go:build arm64
// +build arm64

package neonjpeg

import (
	"bytes"
	"image"
	"os"
	"testing"

	"image/draw"
	"image/jpeg"
	_ "image/jpeg"
)

// BenchmarkEncode-4   	       1	2462227395 ns/op # 2.46s on rpi3b
// BenchmarkEncode-4   	       1	1597318018 ns/op # 1.59s on rpi4b

// new results on scan2drive (rpi4b) as of 2021-11-13:
// name      time/op
// Encode-4  650ms ± 0%

func BenchmarkEncode(b *testing.B) {
	// TODO: scan an okay-to-publish testdata file
	// TODO: get rid of hard-coding the reverse sshfs path:
	f, err := os.Open("home/michael/go/src/github.com/stapelberg/scan2drive/internal/neonjpeg/testdata/page3.jpg")
	if err != nil {
		b.Fatal(err)
	}
	// cfg, format, err := image.DecodeConfig(f)
	img, err := jpeg.Decode(f)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// t.Logf("cfg: %+v", cfg)
	// t.Logf("format: %+v", format)
	//f.Seek(0, io.SeekStart)

	//img, _, err := image.Decode(f)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("img = %T", img)

	bounds := img.Bounds()
	src := img
	m := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(m, m.Bounds(), src, bounds.Min, draw.Src)

	// t.Logf("b.Min.Y = %d, b.Max.Y = %d", bounds.Min.Y, bounds.Max.Y)
	// t.Logf("b.Min.X = %d, b.Max.X = %d", bounds.Min.X, bounds.Max.X)

	bmbuf := make([]byte, 4960*7016*3)

	// 4960 rows in a file
	// 7016 columns (pixels) in a row
	for y := bounds.Min.Y; /* 0 */ y < bounds.Max.Y; /* 7016 */ y++ {
		for x := bounds.Min.X; /* 0 */ x < bounds.Max.X; /* 4960 */ x++ {
			r := m.Pix[(y-m.Rect.Min.Y)*m.Stride+(x-m.Rect.Min.X)*4+0]
			g := m.Pix[(y-m.Rect.Min.Y)*m.Stride+(x-m.Rect.Min.X)*4+1]
			b := m.Pix[(y-m.Rect.Min.Y)*m.Stride+(x-m.Rect.Min.X)*4+2]
			bmbuf[(4960*y)+(x*3)+0] = r
			bmbuf[(4960*y)+(x*3)+1] = g
			bmbuf[(4960*y)+(x*3)+2] = b
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		enc, err := Encode(&buf, image.Point{4960, 7016}, &Options{
			Quality: 75, // like scanimage(1)
		})
		if err != nil {
			b.Fatal(err)
		}

		mbuf := bmbuf
		// Grab a chunk of 16 full rows (as required by neonjpeg), store
		// the remaining bytes for the next iteration.
		for len(mbuf)/3/4960 > 16 {
			height := len(mbuf) / 3 / 4960

			chunk := mbuf[:16*3*4960]
			cl := len(chunk)

			height = len(chunk) / 3 / 4960
			//fmt.Fprintf(os.Stderr, "len(chunk) = %v, height = %d\n", cl, height)
			if padding := height % 16; padding != 0 {
				chunk = append(chunk, make([]byte, padding*3*4960)...)
			}

			enc.EncodePixels(chunk, height)
			mbuf = mbuf[cl:]
		}
		if err := enc.Flush(); err != nil {
			b.Fatal(err)
		}
	}
}
