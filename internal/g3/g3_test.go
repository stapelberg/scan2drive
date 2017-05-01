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

package g3

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"testing"
)

func TestWriteCode(t *testing.T) {
	for _, test := range []struct {
		num   int
		codes []code
		want  []byte
	}{
		{
			num: 1,
			codes: []code{
				terminatingCodesWhite[2], // 0111b
			},
			want: []byte{0x70}, // 01110000b (remainder padded)
		},

		{
			num: 2,
			codes: []code{
				terminatingCodesWhite[29], // 00000010b
			},
			want: []byte{0x2}, // 00000010b
		},

		{
			num: 3,
			codes: []code{
				terminatingCodesBlack[0], // 0000110111b
			},
			want: []byte{0xd, 0xc0}, // 00001101b, 11000000b (remainder padded)
		},

		{
			num: 4,
			codes: []code{
				terminatingCodesWhite[2], // 0111b
				terminatingCodesWhite[3], // 1000b
			},
			want: []byte{0x78}, // 01111000b
		},

		{
			num: 5,
			codes: []code{
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[7],  // 00011b
				terminatingCodesBlack[8],  // 000101b
			},
			want: []byte{
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x18, // 00011000b
				0xa0, // 10100000b (remainder padded)
			},
		},

		{
			num: 6,
			codes: []code{
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[7],  // 00011b
				terminatingCodesBlack[13], // 00000100b
			},
			want: []byte{
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x18, // 00011000b
				0x20, // 00100000b (remainder padded)
			},
		},

		{
			num: 7,
			codes: []code{
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[13], // 00000100b
				terminatingCodesBlack[7],  // 00011b
				terminatingCodesBlack[15], // 000011000b
			},
			want: []byte{
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x4,  // 00000100b
				0x18, // 00011000b
				0x60, // 01100000b (remainder padded)
			},
		},
	} {
		t.Run(fmt.Sprintf("%d", test.num), func(t *testing.T) {
			var buf bytes.Buffer
			e := NewEncoder(&buf)
			for _, code := range test.codes {
				e.writeCode(code)
			}
			if err := e.flushBits(); err != nil {
				t.Fatal(err)
			}
			if got, want := buf.Bytes(), test.want; !bytes.Equal(got, want) {
				t.Errorf("unexpected encoding result: got %x, want %x", got, want)
			}
		})
	}
}

func TestEncodeRun(t *testing.T) {
	type run struct {
		c color.Gray
		l int
	}
	for _, test := range []struct {
		num  int
		runs []run
		want []byte
	}{
		// test case 1 to 3 are taken from figure 9-7 in
		// http://www.fileformat.info/mirror/egff/ch09_05.htm, with
		// one fix: the page incorrectly encodes the second-to-last
		// code for test case 3 with 12 bits instead of 13 bits.
		{
			num: 1,
			runs: []run{
				{black, 20},
			},
			want: []byte{0xd, 0x0}, // [0000 1101 0000] 0000
		},

		{
			num: 2,
			runs: []run{
				{white, 100},
			},
			want: []byte{0xd8, 0xa8}, // [1101 1][000 1010 1]000
		},

		{
			num: 3,
			runs: []run{
				{black, 8800},
			},
			want: []byte{
				0x1,  // [0000 0001
				0xf0, // 1111] [0000
				0x1f, // 0001 1111]
				0x1,  // [0000 0001
				0xf0, // 1111] [0000
				0x3a, // 0011 1010]
				0x83, // [1000 0011
				0x50, // 0101] 0000
			},
		},

		{
			num: 4,
			runs: []run{
				{white, 64}, // edge case: 64 pixels is exactly on the edge
			},
			want: []byte{
				0xd9, // [1101 1][001
				0xa8, //  1010 1]000
			},
		},

		{
			num: 5,
			runs: []run{
				// requires one color-specific makeup code and a
				// terminating code
				{black, 1729},
			},
			want: []byte{
				0x03, // [0000 0011
				0x2a, //  0010 1][010]
			},
		},

		{
			num: 6,
			runs: []run{
				// requires one unspecific makeup code and a
				// terminating code
				{black, 1793},
			},
			want: []byte{
				0x01, // [0000 0001
				0x08, //  000][0 10]00
			},
		},

		{
			num: 7,
			runs: []run{
				// edge case: one makeup code, no color-specific
				// makeup codes, a terminating code of length 0
				{black, 2560},
			},
			want: []byte{
				0x01, // [0000 0001
				0xf0, //  1111][0000
				0xdc, //  1101 11]00
			},
		},
	} {
		t.Run(fmt.Sprintf("%d", test.num), func(t *testing.T) {
			var buf bytes.Buffer
			e := NewEncoder(&buf)
			for _, run := range test.runs {
				e.encodeRun(run.c, run.l)
			}
			if err := e.flushBits(); err != nil {
				t.Fatal(err)
			}
			if got, want := buf.Bytes(), test.want; !bytes.Equal(got, want) {
				t.Errorf("unexpected encoding result: got %x, want %x", got, want)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	bounds := image.Rect(0, 0, 65, 2)
	img := image.NewGray(bounds)

	// first line: 64 black, 1 white
	for x := 0; x < 64; x++ {
		img.SetGray(x, 0, black)
	}
	img.SetGray(64, 0, white)

	// second line: 65 white
	for x := 0; x < 65; x++ {
		img.SetGray(x, 1, white)
	}

	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(img); err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x00, // [0000 0000
		0x13, // 0001] [0011
		0x50, // 0101] [0000
		0x3c, // 0011 11][00
		0x37, // 0011 0111]
		0x1c, // [0001 11][00
		0x00, // 0000 0000
		0x76, // 01][11 011][0
		0x38, // 0011 1][000
		0x00, // 0000 0000
		0x80, // 1]000 0000
	}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("unexpected image encoding result: got %x, want %x", got, want)
	}
}

// BenchmarkEncodeRun-8   	20000000	        87.7 ns/op
func BenchmarkEncodeRun(b *testing.B) {
	var buf bytes.Buffer
	e := NewEncoder(&buf)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.encodeRun(black, 4965)
	}
}
