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
	"fmt"
	"image"
	"io"
	"time"

	"github.com/stapelberg/scan2drive/internal/pdf"
)

func writePDF(w io.Writer, compressed []*bytes.Buffer, bounds []image.Rectangle) error {
	var kids []pdf.Object
	var cnt int
	for idx, m := range compressed {
		if m == nil {
			continue
		}

		scanName := fmt.Sprintf("scan%d", cnt)
		kids = append(kids, &pdf.Page{
			Common: pdf.Common{ObjectName: fmt.Sprintf("page%d", cnt)},
			Resources: []pdf.Object{
				&pdf.Image{
					Common: pdf.Common{
						ObjectName: scanName,
						Stream:     m.Bytes(),
					},
					Bounds: bounds[idx],
				},
			},
			Parent: "pages",
			Contents: []pdf.Object{
				&pdf.Common{
					ObjectName: fmt.Sprintf("content%d", cnt),
					Stream:     []byte(fmt.Sprintf("q 595.28 0 0 841.89 0.00 0.00 cm /%s Do Q\n", scanName)),
				},
			},
		})
		cnt++
	}

	doc := &pdf.Catalog{
		Common: pdf.Common{ObjectName: "catalog"},
		Pages: &pdf.Pages{
			Common: pdf.Common{ObjectName: "pages"},
			Kids:   kids,
		},
	}
	info := &pdf.DocumentInfo{
		Common:       pdf.Common{ObjectName: "info"},
		CreationDate: time.Now(),
		Producer:     "https://github.com/stapelberg/scan2drive",
	}
	pdfEnc := pdf.NewEncoder(w)
	return pdfEnc.Encode(doc, info)
}
