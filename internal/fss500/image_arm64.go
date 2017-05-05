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

// defined in image_arm64.s
func rgb2rgba(res, pixels []byte)

// ToRGBA copies one scanned DIN A4 sized page in 600 dpi
// (i.e. 4960x7016 pixels) into an *image.RGBA.
func ToRGBA(pixels []byte) *image.RGBA {
	res := make([]byte, (len(pixels)/3)*4)
	rgb2rgba(res, pixels)
	return &image.RGBA{
		Pix:    res,
		Stride: 4 * 4960,
		Rect:   image.Rect(0, 0, 4960, 7016),
	}
}
