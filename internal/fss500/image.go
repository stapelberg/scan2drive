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

package fss500

import (
	"image"
	"image/color"
)

// Image represents one scanned DIN A4 sized page in 600 dpi
// (i.e. 4960x7016 pixels).
type Image struct {
	Pixels []byte
}

// Bounds returns the domain for which At can return non-zero color.
func (m *Image) Bounds() image.Rectangle {
	return image.Rect(0, 0, 4960, 7016)
}

// ColorModel returns the Image's color model.
func (m *Image) ColorModel() color.Model {
	return color.RGBAModel
}

// RGBAAt returns the color of the pixel at (x, y) as color.RGBA.
func (m *Image) RGBAAt(x, y int) color.RGBA {
	if x < 0 || x >= 4960 || y < 0 || y >= 7016 {
		return color.RGBA{}
	}

	const bytesPerLine = 3 * 4960
	const channels = 3
	i := bytesPerLine*y + channels*x
	return color.RGBA{
		R: uint8(m.Pixels[i+0]),
		G: uint8(m.Pixels[i+1]),
		B: uint8(m.Pixels[i+2]),
		A: uint8(0xff), // opaque
	}
}

// At returns the color of the pixel at (x, y).
func (m *Image) At(x, y int) color.Color {
	return m.RGBAAt(x, y)
}
