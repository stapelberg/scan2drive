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
)

// ToRGBA copies one scanned DIN A4 sized page in 600 dpi
// (i.e. 4960x7016 pixels) into an *image.RGBA.
func ToRGBA(pixels []byte) *image.RGBA {
	// TODO(later): do that conversion using NEON instructions: 3-element vector load, 4-element vector store
	const bytesPerLine = 3 * 4960
	const channels = 3
	img := image.NewRGBA(image.Rect(0, 0, 4960, 7016))
	for y := 0; y < 7016; y++ {
		for x := 0; x < 4960; x++ {
			i := bytesPerLine*y + channels*x
			offset := y*4*4960 + x*4
			img.Pix[offset+0] = pixels[i+0]
			img.Pix[offset+1] = pixels[i+1]
			img.Pix[offset+2] = pixels[i+2]
			img.Pix[offset+3] = 0xff
		}
	}
	return img
}
