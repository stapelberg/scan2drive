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
// limitations under the License./

package main

import (
	"bytes"
	"image"
	"image/png"
	"io/ioutil"
	"os"

	"github.com/stapelberg/scan2drive/internal/g3"
	"github.com/stapelberg/scan2drive/proto"
	"golang.org/x/net/context"
	"golang.org/x/net/trace"
	"golang.org/x/sync/errgroup"
)

func (s *server) Convert(ctx context.Context, in *proto.ConvertRequest) (*proto.ConvertReply, error) {
	tr, _ := trace.FromContext(ctx)

	tmpdir, err := ioutil.TempDir("", "scan2drive-convert")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpdir)

	var eg errgroup.Group
	compressed := make([]*bytes.Buffer, len(in.ScannedPage))
	binarized := make([]*image.Gray, len(in.ScannedPage))
	for idx, page := range in.ScannedPage {
		idx, page := idx, page // copy
		eg.Go(func() error {
			{
				img, _, err := image.Decode(bytes.NewReader(page))
				if err != nil {
					return err
				}
				tr.LazyPrintf("decoded %d bytes", len(page))

				var whitePct float64
				binarized[idx], whitePct = binarize(img)
				blank := whitePct > 0.99
				tr.LazyPrintf("white percentage of page %d is %f, blank = %v", idx, whitePct, blank)
				if blank {
					return nil
				}
			}

			binarized[idx] = rotate180(binarized[idx])

			// compress
			var buf bytes.Buffer
			if err := g3.NewEncoder(&buf).Encode(binarized[idx]); err != nil {
				return err
			}
			compressed[idx] = &buf
			tr.LazyPrintf("compressed into %d bytes", buf.Len())
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// create thumbnail: PNG-encode the first page
	var thumb []byte
	for _, img := range binarized {
		if img == nil {
			continue
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
		thumb = buf.Bytes()
		break
	}

	var buf bytes.Buffer
	if err := writePDF(&buf, compressed); err != nil {
		return nil, err
	}

	return &proto.ConvertReply{PDF: buf.Bytes(), Thumb: thumb}, nil
}
