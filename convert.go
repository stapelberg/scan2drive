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
	"bytes"
	"image"
	"image/png"

	"github.com/stapelberg/scan2drive/internal/g3"
	"github.com/stapelberg/scan2drive/proto"
	"golang.org/x/net/context"
	"golang.org/x/net/trace"
)

func convertLogic(tr trace.Trace, length int, cb func(int) ([]byte, error)) (pdf []byte, thumb []byte, err error) {
	compressed := make([]*bytes.Buffer, length)
	bounds := make([]image.Rectangle, length)
	var first *image.Gray
	for idx := 0; idx < length; idx++ {
		var binarized *image.Gray
		{
			page, err := cb(idx)
			if err != nil {
				return nil, nil, err
			}
			img, _, err := image.Decode(bytes.NewReader(page))
			if err != nil {
				return nil, nil, err
			}
			tr.LazyPrintf("decoded %d bytes", len(page))

			var whitePct float64
			binarized, whitePct = binarize(img)
			blank := whitePct > 0.99
			tr.LazyPrintf("white percentage of page %d is %f, blank = %v", idx, whitePct, blank)
			if blank {
				continue
			}
		}

		binarized = rotate180(binarized)
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
		tr.LazyPrintf("compressed into %d bytes", buf.Len())
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

func (s *server) Convert(ctx context.Context, in *proto.ConvertRequest) (*proto.ConvertReply, error) {
	tr, _ := trace.FromContext(ctx)

	pdf, thumb, err := convertLogic(tr, len(in.ScannedPage), func(i int) ([]byte, error) {
		return in.ScannedPage[i], nil
	})
	if err != nil {
		return nil, err
	}

	return &proto.ConvertReply{PDF: pdf, Thumb: thumb}, nil
}
