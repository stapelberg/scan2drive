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

// Package pdf implements a minimal PDF 1.7 writer, just functional
// enough to create a PDF file containing multiple CCITT fax-encoded
// DIN A4 sized pages.
//
// It follows the standard “PDF 32000-1:2008 PDF 1.7”:
// https://www.adobe.com/content/dam/Adobe/en/devnet/acrobat/pdfs/PDF32000_2008.pdf
package pdf

import (
	"fmt"
	"io"
	"strings"
	"time"
)

func dateString(t time.Time) string {
	return "D:" + t.Format("20060102150405-07'00'")
}

// ObjectID is a PDF object id.
type ObjectID int

// String implements fmt.Stringer.
func (o ObjectID) String() string {
	return fmt.Sprintf("%d 0 R", int(o))
}

// Object is implemented by all PDF objects.
type Object interface {
	// Objects returns all Objects which should be encoded into the
	// PDF file.
	Objects() []Object

	// Encode encodes the object into the PDF file w.
	Encode(w io.Writer, ids map[string]ObjectID) error

	// SetID updates the object id.
	SetID(id ObjectID)

	// Name returns the human-readable object name.
	Name() string

	fmt.Stringer
}

// Common represents a PDF object.
type Common struct {
	ObjectName string
	ID         ObjectID
	Stream     []byte
}

// String implements fmt.Stringer.
func (c *Common) String() string {
	return c.ID.String()
}

// SetID implements Object.
func (c *Common) SetID(id ObjectID) {
	c.ID = id
}

// Name implements Object.
func (c *Common) Name() string {
	return c.ObjectName
}

// Objects implements Object.
func (c *Common) Objects() []Object {
	return []Object{c}
}

// Encode implements Object.
func (c *Common) Encode(w io.Writer, ids map[string]ObjectID) error {
	_, err := fmt.Fprintf(w, `
%d 0 obj
<<
  /Length %d
>>
stream
%s
endstream
endobj`, c.ID, len(c.Stream), c.Stream)
	return err
}

// DocumentInfo represents a PDF document information object.
type DocumentInfo struct {
	Common

	CreationDate time.Time
	Producer     string
}

// Objects implements Object.
func (d *DocumentInfo) Objects() []Object {
	return []Object{d}
}

// Encode implements Object.
func (d *DocumentInfo) Encode(w io.Writer, ids map[string]ObjectID) error {
	_, err := fmt.Fprintf(w, `
%d 0 obj
<<
  /CreationDate (%s)
  /Producer (%s)
>>
endobj`, int(d.ID), dateString(d.CreationDate), d.Producer)
	return err
}

// Catalog represents a PDF catalog object.
type Catalog struct {
	Common
	Pages Object // Pages
}

// Objects implements Object.
func (r *Catalog) Objects() []Object {
	return append([]Object{r}, r.Pages.Objects()...)
}

// Encode implements Object.
func (r *Catalog) Encode(w io.Writer, ids map[string]ObjectID) error {
	_, err := fmt.Fprintf(w, `
%d 0 obj
<<
  /Type /Catalog
  /Pages %v
>>
endobj`, int(r.ID), r.Pages)
	return err
}

// Pages represents a PDF pages object
type Pages struct {
	Common
	Kids []Object // Page
}

// Objects implements Object.
func (p *Pages) Objects() []Object {
	result := []Object{p}
	for _, o := range p.Kids {
		result = append(result, o.Objects()...)
	}
	return result
}

// Encode implements Object.
func (p *Pages) Encode(w io.Writer, ids map[string]ObjectID) error {
	_, err := fmt.Fprintf(w, `
%d 0 obj
<<
  /Kids %v
  /Type /Pages
  /Count %d
>>
endobj`, int(p.ID), p.Kids, len(p.Kids))
	return err
}

// Page represents a PDF page object with size DIN A4
type Page struct {
	Common

	Resources []Object // Image
	Contents  []Object // Common (streams)

	// Parent contains the human-readable name of the parent object,
	// which will be translated into an object ID when encoding.
	Parent string
}

// Objects implements Object.
func (p *Page) Objects() []Object {
	result := []Object{p}
	for _, o := range p.Resources {
		result = append(result, o.Objects()...)
	}
	for _, o := range p.Contents {
		result = append(result, o.Objects()...)
	}
	return result
}

// Encode implements Object.
func (p *Page) Encode(w io.Writer, ids map[string]ObjectID) error {
	xObjects := make([]string, len(p.Resources))
	for idx, o := range p.Resources {
		xObjects[idx] = fmt.Sprintf("/%s %v", o.Name(), ids[o.Name()])
	}
	_, err := fmt.Fprintf(w, `
%d 0 obj
<<
  /Resources <<
    /XObject <<
%s
    >>
  >>
  /Contents %v
  /Parent %v
  /Type /Page
  /MediaBox [ 0 0 595.28 841.89 ]
>>
endobj`, int(p.ID), strings.Join(xObjects, "\n"), p.Contents, ids[p.Parent])
	return err
}

// Image represents a PDF image object containing a DIN A4-sized page
// scanned with 600dpi (i.e. into 4960x7016 pixels).
type Image struct {
	Common
}

// Objects implements Object.
func (i *Image) Objects() []Object { return []Object{i} }

// Encode implements Object.
func (i *Image) Encode(w io.Writer, ids map[string]ObjectID) error {
	_, err := fmt.Fprintf(w, `
%d 0 obj
<<
  /Subtype /Image
  /DecodeParms
  <<
    /K 0
    /EndOfBlock false
    /EndOfLine true
    /BlackIs1 false
    /Rows 7016
    /Columns 4960
  >>
  /Type /XObject
  /Width 4960
  /Filter /CCITTFaxDecode
  /Height 7016
  /Length %d
  /BitsPerComponent 1
  /ColorSpace /DeviceGray
>>
stream
%s
endstream
endobj`, int(i.Common.ID), len(i.Common.Stream), i.Common.Stream)
	return err
}

type countingWriter struct {
	cnt int
	w   io.Writer
}

func (cw *countingWriter) Write(p []byte) (n int, err error) {
	n, err = cw.w.Write(p)
	cw.cnt += n
	return n, err
}

// Encoder is a PDF writer.
type Encoder struct {
	w *countingWriter
}

// NewEncoder returns a ready-to-use Encoder writing to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w: &countingWriter{w: w},
	}
}

// writeXrefTable writes a cross-reference table to e.w. See also “PDF
// 32000-1:2008 PDF 1.7” section “7.5.4 Cross-Reference Table”
func (e *Encoder) writeXrefTable(Objects []Object, xrefOffsets []int) error {
	if _, err := fmt.Fprintf(e.w, "\nxref\n0 %d\n", len(Objects)+1); err != nil {
		return err
	}

	// index 0 can never point to a valid Object, so print an invalid entry:
	if _, err := fmt.Fprintf(e.w, "%010d %05d %s \n", 0, 65535, "f"); err != nil {
		return err
	}

	const generation = 0
	for _, offset := range xrefOffsets {
		if _, err := fmt.Fprintf(e.w, "%010d %05d %s \n", offset, generation, "n"); err != nil {
			return err
		}
	}
	return nil
}

// Encode writes the PDF file represented by the specified catalog.
func (e *Encoder) Encode(r *Catalog, info *DocumentInfo) error {
	// As per “PDF Explained: How a PDF File is Written”:
	// https://www.geekbooks.me/book/view/pdf-explained

	// (1.) Output the header.

	// Byte sequence 0xE2E3CFD3 as per the recommendation from
	// “Developing with PDF”. See “Chapter 1. PDF Syntax”:
	// https://www.safaribooksonline.com/library/view/developing-with-pdf/9781449327903/ch01.html#_header
	if _, err := e.w.Write(append([]byte("%PDF-1.0\n%"), 0xe2, 0xe3, 0xcf, 0xd3)); err != nil {
		return err
	}

	// Flatten the Object graph into a slice
	objects := append(r.Objects(), info.Objects()...)

	// (3.) Assign ids from 1 to n and store them in a lookup table
	// (some Objects need to resolve name references when encoding).
	ids := make(map[string]ObjectID, len(objects))
	for idx, obj := range objects {
		id := ObjectID(idx + 1)
		obj.SetID(id)
		ids[obj.Name()] = id
	}

	// (4.) Output the Objects one by one, starting with Object number
	// one, recording the byte offset of each for the cross-reference
	// table.
	xrefOffsets := make([]int, len(objects))
	for idx, obj := range objects {
		xrefOffsets[idx] = e.w.cnt + 1
		if err := obj.Encode(e.w, ids); err != nil {
			return err
		}
	}

	// (5.) Write the cross-reference table.
	xrefOffset := e.w.cnt

	if err := e.writeXrefTable(objects, xrefOffsets); err != nil {
		return err
	}

	// (6.) Write the trailer, trailer dictionary, and end-of-file marker.
	if _, err := fmt.Fprintf(e.w, `trailer
<<
  /Root %v
  /Size %d
  /Info %v
>>
startxref
%d
%%%%EOF
`, ids["catalog"], len(objects)+1, ids["info"], xrefOffset); err != nil {
		return err
	}

	return nil
}
