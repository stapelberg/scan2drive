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

// Package legacyconvert contains the not-yet-refactored scan2drive conversion
// logic.
package legacyconvert

import (
	"bytes"
	"image"
	"image/png"

	"github.com/stapelberg/scan2drive/internal/g3"
	"github.com/stapelberg/scan2drive/internal/page"
	"golang.org/x/net/trace"
)

func ConvertLogic(tr trace.Trace, pages []*page.Any) (pdf []byte, thumb []byte, err error) {
	compressed := make([]*bytes.Buffer, len(pages))
	bounds := make([]image.Rectangle, len(pages))
	var first *image.Gray
	for idx, page := range pages {
		var binarized *image.Gray
		{
			bin, whitePct, err := page.Binarized()
			if err != nil {
				return nil, nil, err
			}
			binarized = bin
			blank := whitePct > 0.99
			tr.LazyPrintf("white percentage of page %d is %f, blank = %v", idx, whitePct, blank)
			if blank {
				continue
			}
		}

		if first == nil {
			first = binarized
		}

		// compress
		var buf bytes.Buffer
		if err := g3.NewEncoder(&buf).Encode(binarized); err != nil {
			return nil, nil, err
		}
		compressed[idx] = &buf
		bounds[idx] = binarized.Bounds()
		tr.LazyPrintf("g3-compressed into %d bytes", buf.Len())
	}

	// create thumbnail: PNG-encode the first page
	if first != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, first); err != nil {
			return nil, nil, err
		}
		thumb = buf.Bytes()
	}

	var buf bytes.Buffer
	if err := writePDF(&buf, compressed, bounds); err != nil {
		return nil, nil, err
	}

	return buf.Bytes(), thumb, nil
}
