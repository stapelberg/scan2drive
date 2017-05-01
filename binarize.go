// Copyright 2016 Michael Stapelberg and contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"image"
	"image/color"
)

// binarize turns image into a black/white image.
func binarize(img image.Image) (*image.Gray, float64) {
	bounds := img.Bounds()
	out := image.NewGray(bounds)

	var white int
	// This loop arrangement is faster:
	// 49s in Y outer, then X inner
	// 63s in X outer, then Y inner
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			a := color.GrayModel.Convert(c).(color.Gray).Y
			if a > 127 {
				out.SetGray(x, y, color.Gray{0xff}) // white
				white++
			} else {
				out.SetGray(x, y, color.Gray{0x00}) // black
			}
		}
	}
	total := (bounds.Max.Y - bounds.Min.Y) * (bounds.Max.X - bounds.Min.X)
	return out, float64(white) / float64(total)
}

// binarizeFSS500 is a copy of binarize, directly using a 4960x7016
// pixel array in RGB format (as returned by the Fujitsu ScanSnap
// iX500).
//
// binarizeFSS500 runs in 3s on a Raspberry Pi 3 as opposed to 49s for
// binarize.
func binarizeFSS500(pixels []byte) (*image.Gray, float64) {
	bounds := image.Rect(0, 0, 4960, 7016)
	out := image.NewGray(bounds)

	var white int
	const channels = 3
	var r, g, b uint32
	var i, o int
	var a uint32
	for y := 0; y < 7016; y++ {
		for x := 0; x < 4960; x++ {
			i = channels*4960*y + channels*x

			r = uint32(pixels[i+0])
			r |= r << 8
			g = uint32(pixels[i+1])
			g |= g << 8
			b = uint32(pixels[i+2])
			b |= b << 8

			a = (19595*r + 38470*g + 7471*b + 1<<15) >> 24
			o = y*4960 + x
			if uint8(a) > 127 {
				out.Pix[o] = 0xff // white
				white++
			} else {
				out.Pix[o] = 0x00 // black
			}
		}
	}
	return out, float64(white) / float64(4960*7016)
}
