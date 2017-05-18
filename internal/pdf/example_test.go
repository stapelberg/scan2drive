package pdf_test

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/stapelberg/scan2drive/internal/pdf"
)

func Example() {
	doc := &pdf.Catalog{
		Common: pdf.Common{ObjectName: "catalog"},
		Pages: &pdf.Pages{
			Common: pdf.Common{ObjectName: "pages"},
			Kids: []pdf.Object{
				&pdf.Page{
					Common: pdf.Common{ObjectName: "page0"},
					Resources: []pdf.Object{
						&pdf.Image{
							Common: pdf.Common{
								ObjectName: "scan0",
								Stream:     []byte("ß"),
							},
						},
					},
					Parent: "pages",
					Contents: []pdf.Object{
						&pdf.Common{
							ObjectName: "content0",
							Stream:     []byte(fmt.Sprintf("q 595.28 0 0 841.89 0.00 0.00 cm /%s Do Q\n", "scan0")),
						},
					},
				},
			},
		},
	}
	info := &pdf.DocumentInfo{
		Common:       pdf.Common{ObjectName: "info"},
		CreationDate: time.Unix(1493650928, 0).UTC(),
		Producer:     "https://github.com/stapelberg/scan2drive",
	}
	var buf bytes.Buffer
	if err := pdf.NewEncoder(&buf).Encode(doc, info); err != nil {
		log.Fatal(err)
	}
	// Convert the binary data at the beginning of a PDF to a
	// printable sequence of bytes for the Go example test
	// infrastructure.
	printable := bytes.Replace(buf.Bytes(), []byte{0xe2, 0xe3, 0xcf, 0xd3}, []byte("<0xE2E3CFD3>"), 1)
	// Remove trailing whitespace from xref table for more convenient
	// editing of example_test.go.
	printable = bytes.Replace(printable, []byte("00000 n "), []byte("00000 n"), -1)
	printable = bytes.Replace(printable, []byte("65535 f "), []byte("65535 f"), -1)
	os.Stdout.Write(printable)

	// Output:
	// %PDF-1.0
	// %<0xE2E3CFD3>
	// 1 0 obj
	// <<
	//   /Type /Catalog
	//   /Pages 2 0 R
	// >>
	// endobj
	// 2 0 obj
	// <<
	//   /Kids [3 0 R]
	//   /Type /Pages
	//   /Count 1
	// >>
	// endobj
	// 3 0 obj
	// <<
	//   /Resources <<
	//     /XObject <<
	// /scan0 4 0 R
	//     >>
	//   >>
	//   /Contents [5 0 R]
	//   /Parent 2 0 R
	//   /Type /Page
	//   /MediaBox [ 0 0 595.28 841.89 ]
	// >>
	// endobj
	// 4 0 obj
	// <<
	//   /Subtype /Image
	//   /DecodeParms
	//   <<
	//     /K 0
	//     /EndOfBlock false
	//     /EndOfLine true
	//     /BlackIs1 false
	//     /Rows 7016
	//     /Columns 4960
	//   >>
	//   /Type /XObject
	//   /Width 4960
	//   /Filter /CCITTFaxDecode
	//   /Height 7016
	//   /Length 2
	//   /BitsPerComponent 1
	//   /ColorSpace /DeviceGray
	// >>
	// stream
	// ß
	// endstream
	// endobj
	// 5 0 obj
	// <<
	//   /Length 45
	// >>
	// stream
	// q 595.28 0 0 841.89 0.00 0.00 cm /scan0 Do Q
	//
	// endstream
	// endobj
	// 6 0 obj
	// <<
	//   /CreationDate (D:20170501150208+00'00')
	//   /Producer (https://github.com/stapelberg/scan2drive)
	// >>
	// endobj
	// xref
	// 0 7
	// 0000000000 65535 f
	// 0000000015 00000 n
	// 0000000068 00000 n
	// 0000000131 00000 n
	// 0000000293 00000 n
	// 0000000613 00000 n
	// 0000000710 00000 n
	// trailer
	// <<
	//   /Root 1 0 R
	//   /Size 7
	//   /Info 6 0 R
	// >>
	// startxref
	// 827
	// %%EOF
}
