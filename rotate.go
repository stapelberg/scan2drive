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
)

// rotate180 returns the specified image rotated by 180 degrees.
func rotate180(img *image.Gray) *image.Gray {
	bounds := img.Bounds()
	out := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.GrayAt(x, y)
			out.SetGray(bounds.Max.X-x-1, bounds.Max.Y-y-1, c)
		}
	}
	return out
}
