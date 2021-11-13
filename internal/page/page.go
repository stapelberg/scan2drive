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

// Package page implements scanned pages (as JPEG), which can either be already
// binarized or are binarized if not loaded to memory yet.
package page

import (
	"bytes"
	"image"
	"image/color"

	_ "image/jpeg"
)

type Any struct {
	jpegBytes []byte
	binarized *image.Gray
	whitePct  float64
}

func (p *Any) JPEGBytes() ([]byte, error) {
	return p.jpegBytes, nil
}

func (p *Any) Binarized() (*image.Gray, float64, error) {
	if p.binarized != nil {
		return p.binarized, p.whitePct, nil
	}

	img, _, err := image.Decode(bytes.NewReader(p.jpegBytes))
	if err != nil {
		return nil, 0, err
	}

	p.binarized, p.whitePct = binarize(img)
	return p.binarized, p.whitePct, nil
}

func JPEGPageFromBytes(b []byte) *Any {
	return &Any{jpegBytes: b}
}

func Binarized(jpegBytes []byte, binarized *image.Gray, whitePct float64) *Any {
	return &Any{
		jpegBytes: jpegBytes,
		binarized: binarized,
		whitePct:  whitePct,
	}
}

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
