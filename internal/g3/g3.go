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

// Package g3 implements data encoding using the CCITT (renamed to
// ITU-T in 1993) fax standard in its Group 3, One-Dimensional (G31D)
// variant.
//
// It follows the standard ITU-T T.4 (07/2003), section 4.1:
// https://www.itu.int/rec/T-REC-T.4-200307-I/en
package g3

import (
	"image"
	"image/color"
	"io"
)

var white = color.Gray{0xff}
var black = color.Gray{0x00}

// Encoder is a Group 3 One-Dimensional fax encoder.
type Encoder struct {
	w       io.Writer
	numbits int
	current uint64
}

// NewEncoder returns a ready-to-use Encoder, writing to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w: w,
	}
}

// writeBits writes n (< 64) bits to e.w in units of whole bytes. Bits
// might remain in the internal buffer until flushBits() is called.
func (e *Encoder) writeBits(bits uint64, n int) error {
	if e.numbits+n > 64 {
		// n bits will not fit into e.current: add as many bits as we
		// have available, then flush them out to make more space.
		avail := 64 - e.numbits
		first := bits >> uint(n-avail)
		e.current = (e.current << uint(avail)) | first
		e.numbits = 64
		if err := e.flushBits(); err != nil {
			return err
		}

		bits ^= (first << uint(n-avail))
		n -= avail
	}
	e.current = (e.current << uint(n)) | bits
	e.numbits += n
	return nil
}

// flushBits writes the remaining bits to e.w, padding the last byte
// with zero bits if necessary.
func (e *Encoder) flushBits() error {
	if e.numbits == 0 {
		return nil
	}

	bits := e.current
	bytes := e.numbits / 8
	// Fill the remainder of the byte with zero bits.
	if rest := e.numbits % 8; rest != 0 {
		bits <<= uint(8 - rest)
		bytes++
	}
	var b [8]byte
	for i := 0; i < bytes; i++ {
		b[i] = byte(bits >> uint((bytes-i-1)*8))
	}
	e.numbits = 0
	_, err := e.w.Write(b[:bytes])
	return err
}

func (e *Encoder) writeCode(c code) error {
	return e.writeBits(uint64(c.Value), c.Length)
}

// encodeRun encodes a pixel run into zero or more makeup codes and
// precisely one terminating code. Codes are color-specific (except
// for makeup codes for lengths ≥ 28×64 = 1792).
func (e *Encoder) encodeRun(c color.Gray, l int) error {
	sixtyFours := l / 64
	remainder := l % 64

	terminating := terminatingCodesWhite
	makeup := makeupCodesWhite
	if c == black {
		terminating = terminatingCodesBlack
		makeup = makeupCodesBlack
	}

	for sixtyFours > 40 {
		if err := e.writeCode(makeup[40]); err != nil {
			return err
		}
		sixtyFours -= 40
	}
	if sixtyFours > 0 {
		if err := e.writeCode(makeup[sixtyFours]); err != nil {
			return err
		}
	}
	return e.writeCode(terminating[remainder])
}

// Encode compresses the specified image using Group 3 One-Dimensional
// fax encoding.
//
// If the resulting bit stream does not end on a byte boundary, zero
// bits are added as padding.
func (e *Encoder) Encode(m *image.Gray) error {
	// From T.4 4.1.2 EOL: In addition, this signal will occur prior
	// to the first data line of a page.
	if err := e.writeCode(endOfLine); err != nil {
		return err
	}

	bounds := m.Bounds()
	var runLen int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		run := white // lines always start with a white run
		runLen = 0
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if gray := m.GrayAt(x, y); gray != run {
				// run completed, flush
				if err := e.encodeRun(run, runLen); err != nil {
					return err
				}
				run = gray
				runLen = 0
			}
			runLen++
		}

		// run must contain at least 1 pixel, so flush
		if err := e.encodeRun(run, runLen); err != nil {
			return err
		}

		if err := e.writeCode(endOfLine); err != nil {
			return err
		}
	}
	return e.flushBits()
}
