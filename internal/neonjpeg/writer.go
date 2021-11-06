// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build arm64
// +build arm64

package neonjpeg

import (
	"bufio"
	"image"
	"image/color"
	"io"
	"unsafe"
)

// min returns the minimum of two integers.
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// bitCount counts the number of bits needed to hold an integer.
var bitCount = [256]byte{
	0, 1, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4,
	5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
}

type quantIndex int

const (
	quantIndexLuminance quantIndex = iota
	quantIndexChrominance
	nQuantIndex
)

// unscaledQuant are the unscaled quantization tables in zig-zag order. Each
// encoder copies and scales the tables according to its quality parameter.
// The values are derived from section K.1 after converting from natural to
// zig-zag order.
var unscaledQuant = [nQuantIndex][blockSize]byte{
	// Luminance.
	{
		16, 11, 12, 14, 12, 10, 16, 14,
		13, 14, 18, 17, 16, 19, 24, 40,
		26, 24, 22, 22, 24, 49, 35, 37,
		29, 40, 58, 51, 61, 60, 57, 51,
		56, 55, 64, 72, 92, 78, 64, 68,
		87, 69, 55, 56, 80, 109, 81, 87,
		95, 98, 103, 104, 103, 62, 77, 113,
		121, 112, 100, 120, 92, 101, 103, 99,
	},
	// Chrominance.
	{
		17, 18, 18, 24, 21, 24, 47, 26,
		26, 47, 99, 66, 56, 66, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
	},
}

type huffIndex int

const (
	huffIndexLuminanceDC huffIndex = iota
	huffIndexLuminanceAC
	huffIndexChrominanceDC
	huffIndexChrominanceAC
	nHuffIndex
)

// huffmanSpec specifies a Huffman encoding.
type huffmanSpec struct {
	// count[i] is the number of codes of length i bits.
	count [16]byte
	// value[i] is the decoded value of the i'th codeword.
	value []byte
}

// theHuffmanSpec is the Huffman encoding specifications.
// This encoder uses the same Huffman encoding for all images.
var theHuffmanSpec = [nHuffIndex]huffmanSpec{
	// Luminance DC.
	{
		[16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0},
		[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	// Luminance AC.
	{
		[16]byte{0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 125},
		[]byte{
			0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
			0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
			0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
			0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
			0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
			0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
			0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
			0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
			0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
			0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
			0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
			0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
			0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
			0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
			0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
			0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
			0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
			0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
			0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
			0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
			0xf9, 0xfa,
		},
	},
	// Chrominance DC.
	{
		[16]byte{0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0},
		[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	// Chrominance AC.
	{
		[16]byte{0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 119},
		[]byte{
			0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
			0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
			0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
			0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
			0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
			0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
			0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
			0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
			0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
			0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
			0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
			0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
			0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
			0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
			0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
			0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
			0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
			0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
			0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
			0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
			0xf9, 0xfa,
		},
	},
}

// huffmanLUT is a compiled look-up table representation of a huffmanSpec.
// Each value maps to a uint32 of which the 8 most significant bits hold the
// codeword size in bits and the 24 least significant bits hold the codeword.
// The maximum codeword size is 16 bits.
type huffmanLUT []uint32

func (h *huffmanLUT) init(s huffmanSpec) {
	maxValue := 0
	for _, v := range s.value {
		if int(v) > maxValue {
			maxValue = int(v)
		}
	}
	*h = make([]uint32, maxValue+1)
	code, k := uint32(0), 0
	for i := 0; i < len(s.count); i++ {
		nBits := uint32(i+1) << 24
		for j := uint8(0); j < s.count[i]; j++ {
			(*h)[s.value[k]] = nBits | code
			code++
			k++
		}
		code <<= 1
	}
}

// theHuffmanLUT are compiled representations of theHuffmanSpec.
var theHuffmanLUT [4]huffmanLUT

func init() {
	for i, s := range theHuffmanSpec {
		theHuffmanLUT[i].init(s)
	}
}

// writer is a buffered writer.
type writer interface {
	Flush() error
	io.Writer
	io.ByteWriter
}

// encoder encodes an image to the JPEG format.
type Encoder struct {
	// w is the writer to write to. err is the first error encountered during
	// writing. All attempted writes after the first error become no-ops.
	w   writer
	err error
	// buf is a scratch buffer.
	buf [16]byte
	// bits and nBits are accumulated bits to write to w.
	bits, nBits uint32
	// quant is the scaled quantization tables, in zig-zag order.
	quant [nQuantIndex][blockSize]byte

	state *state
}

func (e *Encoder) flush() {
	if e.err != nil {
		return
	}
	e.err = e.w.Flush()
}

func (e *Encoder) write(p []byte) {
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(p)
}

func (e *Encoder) writeByte(b byte) {
	if e.err != nil {
		return
	}
	e.err = e.w.WriteByte(b)
}

// emit emits the least significant nBits bits of bits to the bit-stream.
// The precondition is bits < 1<<nBits && nBits <= 16.
func (e *Encoder) emit(bits, nBits uint32) {
	nBits += e.nBits
	bits <<= 32 - nBits
	bits |= e.bits
	for nBits >= 8 {
		b := uint8(bits >> 24)
		e.writeByte(b)
		if b == 0xff {
			e.writeByte(0x00)
		}
		bits <<= 8
		nBits -= 8
	}
	e.bits, e.nBits = bits, nBits
}

// emitHuff emits the given value with the given Huffman encoder.
func (e *Encoder) emitHuff(h huffIndex, value int32) {
	x := theHuffmanLUT[h][value]
	e.emit(x&(1<<24-1), x>>24)
}

// emitHuffRLE emits a run of runLength copies of value encoded with the given
// Huffman encoder.
func (e *Encoder) emitHuffRLE(h huffIndex, runLength, value int32) {
	a, b := value, value
	if a < 0 {
		a, b = -value, value-1
	}
	var nBits uint32
	if a < 0x100 {
		nBits = uint32(bitCount[a])
	} else {
		nBits = 8 + uint32(bitCount[a>>8])
	}
	e.emitHuff(h, runLength<<4|int32(nBits))
	if nBits > 0 {
		e.emit(uint32(b)&(1<<nBits-1), nBits)
	}
}

// writeMarkerHeader writes the header for a marker with the given length.
func (e *Encoder) writeMarkerHeader(marker uint8, markerlen int) {
	e.buf[0] = 0xff
	e.buf[1] = marker
	e.buf[2] = uint8(markerlen >> 8)
	e.buf[3] = uint8(markerlen & 0xff)
	e.write(e.buf[:4])
}

// writeDQT writes the Define Quantization Table marker.
func (e *Encoder) writeDQT() {
	const markerlen = 2 + int(nQuantIndex)*(1+blockSize)
	e.writeMarkerHeader(dqtMarker, markerlen)
	for i := range e.quant {
		e.writeByte(uint8(i))
		e.write(e.quant[i][:])
	}
}

// writeSOF0 writes the Start Of Frame (Baseline) marker.
func (e *Encoder) writeSOF0(size image.Point, nComponent int) {
	markerlen := 8 + 3*nComponent
	e.writeMarkerHeader(sof0Marker, markerlen)
	e.buf[0] = 8 // 8-bit color.
	e.buf[1] = uint8(size.Y >> 8)
	e.buf[2] = uint8(size.Y & 0xff)
	e.buf[3] = uint8(size.X >> 8)
	e.buf[4] = uint8(size.X & 0xff)
	e.buf[5] = uint8(nComponent)
	if nComponent == 1 {
		e.buf[6] = 1
		// No subsampling for grayscale image.
		e.buf[7] = 0x11
		e.buf[8] = 0x00
	} else {
		for i := 0; i < nComponent; i++ {
			e.buf[3*i+6] = uint8(i + 1)
			// We use 4:2:0 chroma subsampling.
			e.buf[3*i+7] = "\x22\x11\x11"[i]
			e.buf[3*i+8] = "\x00\x01\x01"[i]
		}
	}
	e.write(e.buf[:3*(nComponent-1)+9])
}

// writeDHT writes the Define Huffman Table marker.
func (e *Encoder) writeDHT(nComponent int) {
	markerlen := 2
	specs := theHuffmanSpec[:]
	if nComponent == 1 {
		// Drop the Chrominance tables.
		specs = specs[:2]
	}
	for _, s := range specs {
		markerlen += 1 + 16 + len(s.value)
	}
	e.writeMarkerHeader(dhtMarker, markerlen)
	for i, s := range specs {
		e.writeByte("\x00\x10\x01\x11"[i])
		e.write(s.count[:])
		e.write(s.value)
	}
}

// div returns a/b rounded to the nearest integer, instead of rounded to zero.
func div(a, b int32) int32 {
	if a >= 0 {
		return (a + (b >> 1)) / b
	}
	return -((-a + (b >> 1)) / b)
}

// writeBlock writes a block of pixel data using the given quantization table,
// returning the post-quantized DC value of the DCT-transformed block. b is in
// natural (not zig-zag) order.
func (e *Encoder) writeBlock(b *block, q quantIndex, prevDC int32) int32 {
	fdct(b)
	// Emit the DC delta.
	dc := div(b[0], 8*int32(e.quant[q][0]))
	e.emitHuffRLE(huffIndex(2*q+0), 0, dc-prevDC)
	// Emit the AC components.
	h, runLength := huffIndex(2*q+1), int32(0)
	for zig := 1; zig < blockSize; zig++ {
		ac := div(b[unzig[zig]], 8*int32(e.quant[q][zig]))
		if ac == 0 {
			runLength++
		} else {
			for runLength > 15 {
				e.emitHuff(h, 0xf0)
				runLength -= 16
			}
			e.emitHuffRLE(h, runLength, ac)
			runLength = 0
		}
	}
	if runLength > 0 {
		e.emitHuff(h, 0x00)
	}
	return dc
}

// toYCbCr converts the 8x8 region of m whose top-left corner is p to its
// YCbCr values.
func toYCbCr(m image.Image, p image.Point, yBlock, cbBlock, crBlock *block) {
	b := m.Bounds()
	xmax := b.Max.X - 1
	ymax := b.Max.Y - 1
	for j := 0; j < 8; j++ {
		for i := 0; i < 8; i++ {
			r, g, b, _ := m.At(min(p.X+i, xmax), min(p.Y+j, ymax)).RGBA()
			yy, cb, cr := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			yBlock[8*j+i] = int32(yy)
			cbBlock[8*j+i] = int32(cb)
			crBlock[8*j+i] = int32(cr)
		}
	}
}

// grayToY stores the 8x8 region of m whose top-left corner is p in yBlock.
func grayToY(m *image.Gray, p image.Point, yBlock *block) {
	b := m.Bounds()
	xmax := b.Max.X - 1
	ymax := b.Max.Y - 1
	pix := m.Pix
	for j := 0; j < 8; j++ {
		for i := 0; i < 8; i++ {
			idx := m.PixOffset(min(p.X+i, xmax), min(p.Y+j, ymax))
			yBlock[8*j+i] = int32(pix[idx])
		}
	}
}

// rgbaToYCbCr is a specialized version of toYCbCr for image.RGBA images.
func rgbaToYCbCr(m *image.RGBA, p image.Point, yBlock, cbBlock, crBlock *block) {
	b := m.Bounds()
	xmax := b.Max.X - 1
	ymax := b.Max.Y - 1
	for j := 0; j < 8; j++ {
		sj := p.Y + j
		if sj > ymax {
			sj = ymax
		}
		offset := (sj-b.Min.Y)*m.Stride - b.Min.X*4
		for i := 0; i < 8; i++ {
			sx := p.X + i
			if sx > xmax {
				sx = xmax
			}
			pix := m.Pix[offset+sx*4:]
			yy, cb, cr := color.RGBToYCbCr(pix[0], pix[1], pix[2])
			yBlock[8*j+i] = int32(yy)
			cbBlock[8*j+i] = int32(cb)
			crBlock[8*j+i] = int32(cr)
		}
	}
}

// scale scales the 16x16 region represented by the 4 src blocks to the 8x8
// dst block.
func scale(dst *block, src *[4]block) {
	for i := 0; i < 4; i++ {
		dstOff := (i&2)<<4 | (i&1)<<2
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				j := 16*y + 2*x
				sum := src[i][j] + src[i][j+1] + src[i][j+8] + src[i][j+9]
				dst[8*y+x+dstOff] = (sum + 2) >> 2
			}
		}
	}
}

// sosHeaderY is the SOS marker "\xff\xda" followed by 8 bytes:
//	- the marker length "\x00\x08",
//	- the number of components "\x01",
//	- component 1 uses DC table 0 and AC table 0 "\x01\x00",
//	- the bytes "\x00\x3f\x00". Section B.2.3 of the spec says that for
//	  sequential DCTs, those bytes (8-bit Ss, 8-bit Se, 4-bit Ah, 4-bit Al)
//	  should be 0x00, 0x3f, 0x00<<4 | 0x00.
var sosHeaderY = []byte{
	0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00,
}

// sosHeaderYCbCr is the SOS marker "\xff\xda" followed by 12 bytes:
//	- the marker length "\x00\x0c",
//	- the number of components "\x03",
//	- component 1 uses DC table 0 and AC table 0 "\x01\x00",
//	- component 2 uses DC table 1 and AC table 1 "\x02\x11",
//	- component 3 uses DC table 1 and AC table 1 "\x03\x11",
//	- the bytes "\x00\x3f\x00". Section B.2.3 of the spec says that for
//	  sequential DCTs, those bytes (8-bit Ss, 8-bit Se, 4-bit Ah, 4-bit Al)
//	  should be 0x00, 0x3f, 0x00<<4 | 0x00.
var sosHeaderYCbCr = []byte{
	0xff, 0xda, 0x00, 0x0c, 0x03, 0x01, 0x00, 0x02,
	0x11, 0x03, 0x11, 0x00, 0x3f, 0x00,
}

var divisors = [2][64 * 4]int16{
	[64 * 4]int16{
		-32768, -21845, -13107, -32768,
		-21845, -13107, -25206, -31711,
		-21845, -21845, -28087, -13107,
		-25206, -29378, -30583, -28087,
		-28087, -28087, -32768, -21845,
		-13107, -29378, -5617, -28087,
		-28087, -7282, -17873, -30583,
		-25206, -17873, -13107, -31711,
		-7282, -17873, -10348, -28087,
		-3855, -27406, -25206, -11763,
		-21845, -7282, -28087, -32768,
		-14386, -25206, -28744, -19946,
		-23593, -32768, -11763, -17873,
		-25206, -31156, -30583, -24415,
		-7282, -19946, -21845, -22737,
		-28087, -23593, -25206, -23593,
		32, 24, 20, 32,
		48, 80, 104, 125,
		24, 24, 29, 40,
		52, 116, 120, 113,
		29, 29, 32, 48,
		80, 116, 140, 113,
		29, 37, 44, 60,
		104, 176, 160, 125,
		37, 44, 77, 113,
		136, 221, 208, 157,
		48, 73, 113, 128,
		165, 208, 229, 185,
		101, 128, 157, 176,
		208, 244, 240, 204,
		145, 185, 192, 197,
		225, 201, 208, 201,
		2048, 2048, 2048, 2048,
		1024, 512, 512, 512,
		2048, 2048, 2048, 1024,
		1024, 512, 512, 512,
		2048, 2048, 2048, 1024,
		512, 512, 256, 512,
		2048, 1024, 1024, 1024,
		512, 256, 256, 512,
		1024, 1024, 512, 512,
		256, 256, 256, 256,
		1024, 512, 512, 512,
		256, 256, 256, 256,
		512, 512, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		5, 5, 5, 5,
		6, 7, 7, 7,
		5, 5, 5, 6,
		6, 7, 7, 7,
		5, 5, 5, 6,
		7, 7, 8, 7,
		5, 6, 6, 6,
		7, 8, 8, 7,
		6, 6, 7, 7,
		8, 8, 8, 8,
		6, 7, 7, 7,
		8, 8, 8, 8,
		7, 7, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
	},
	[64 * 4]int16{
		-7282, -7282, -21845, -21845,
		-23593, -23593, -23593, -23593,
		-7282, -17873, -25206, -1986,
		-23593, -23593, -23593, -23593,
		-21845, -25206, -28087, -23593,
		-23593, -23593, -23593, -23593,
		-21845, -1986, -23593, -23593,
		-23593, -23593, -23593, -23593,
		-23593, -23593, -23593, -23593,
		-23593, -23593, -23593, -23593,
		-23593, -23593, -23593, -23593,
		-23593, -23593, -23593, -23593,
		-23593, -23593, -23593, -23593,
		-23593, -23593, -23593, -23593,
		-23593, -23593, -23593, -23593,
		-23593, -23593, -23593, -23593,
		37, 37, 48, 96,
		201, 201, 201, 201,
		37, 44, 52, 133,
		201, 201, 201, 201,
		48, 52, 113, 201,
		201, 201, 201, 201,
		96, 133, 201, 201,
		201, 201, 201, 201,
		201, 201, 201, 201,
		201, 201, 201, 201,
		201, 201, 201, 201,
		201, 201, 201, 201,
		201, 201, 201, 201,
		201, 201, 201, 201,
		201, 201, 201, 201,
		201, 201, 201, 201,
		1024, 1024, 1024, 512,
		256, 256, 256, 256,
		1024, 1024, 1024, 256,
		256, 256, 256, 256,
		1024, 1024, 512, 256,
		256, 256, 256, 256,
		512, 256, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		256, 256, 256, 256,
		6, 6, 6, 7,
		8, 8, 8, 8,
		6, 6, 6, 8,
		8, 8, 8, 8,
		6, 6, 7, 8,
		8, 8, 8, 8,
		7, 8, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
		8, 8, 8, 8,
	},
}

const yIndex = 0
const cbIndex = 1
const crIndex = 2

func rgbyccconvertasm(output_width uint32, input_buf **uint8, output_buf ***uint8, output_row uint32, num_rows uint32)

func downsampleasm(image_width uint32, max_v_samp_factor int32, v_samp_factor uint32, width_blocks uint32, input_data **uint8, output_data **uint8)

func convsampasm(sample_data **uint8, start_col uint32, workspace *int16)
func fdctasm(workspace *int16)
func quantizeasm(coef_block *int16, divisors *int16, workspace *int16)

type state struct {
	next_output_byte *byte
	free_in_buffer   int64
	savable_state    struct {
		put_buffer  int64
		put_bits    int32
		last_dc_val [3]int32
	}
}
type tbl struct {
	ehufco [256]uint32 // code
	ehufsi [256]byte   // length of code
}

func huffasm(state *state, buffer *byte, block *int16, last_dc_val int32, dctbl, actbl *tbl) *byte

var workspace [64]int16

func forward_DCT(quant_tbl_no int32, sample_data **uint8, coef_blocks *int16, start_row uint32, start_col uint32, num_blocks uint32) {
	convsampasm(sample_data, start_col, &workspace[0])
	fdctasm(&workspace[0])
	quantizeasm(coef_blocks, &divisors[quant_tbl_no][0], &workspace[0])
}

var ljtbls [4]tbl

func (h *tbl) init(s huffmanSpec) {
	maxValue := 0
	for _, v := range s.value {
		if int(v) > maxValue {
			maxValue = int(v)
		}
	}
	code, k := uint32(0), 0
	for i := 0; i < len(s.count); i++ {
		//nBits := uint32(i+1) << 24
		for j := uint8(0); j < s.count[i]; j++ {
			h.ehufco[s.value[k]] = code
			h.ehufsi[s.value[k]] = byte(i + 1)
			code++
			k++
		}
		code <<= 1
	}
}

func init() {
	for i, s := range theHuffmanSpec {
		ljtbls[i].init(s)
	}
}

var MCU_membership = [...]int{
	0, // Y
	0, // Y
	0, // Y
	0, // Y
	1, // Cb
	2, // Cr
}

// RGB is an in-memory image storing R, G, B pixel triples as used by the NEON
// assembler code.
type RGB struct {
	Pix  []uint8
	Rect image.Rectangle
}

func (r *RGB) Bounds() image.Rectangle {
	return r.Rect
}

// writeSOS writes the StartOfScan marker.
func (e *Encoder) EncodePixels(pix []uint8, height int) {
	var (
		color_buf  [3][2 * 4960]uint8
		output_buf [3][16 * 4960]uint8
	)
	buffer := make([]byte, blockSize*4) // TODO: how long?
	var MCU_buffer [10][blockSize]int16
	for y := 0; y < height; y += 16 {
		yOut := 0
		for yOff := 0; yOff < 16; yOff += 2 {
			ptrs := [...]*uint8{
				&pix[(y+yOff+0)*4960*3+0],
				&pix[(y+yOff+1)*4960*3+0],
			}
			btrsY := [...]*uint8{
				&color_buf[0][4960*0],
				&color_buf[0][4960*1],
			}
			btrsCb := [...]*uint8{
				&color_buf[1][4960*0],
				&color_buf[1][4960*1],
			}
			btrsCr := [...]*uint8{
				&color_buf[2][4960*0],
				&color_buf[2][4960*1],
			}

			bptrs := [...]**uint8{
				&btrsY[0],
				&btrsCb[0],
				&btrsCr[0],
			}
			rgbyccconvertasm(4960, // output_width
				&ptrs[0],  // input_buf = input_buf + *in_row_ctr
				&bptrs[0], // output_buf = color_buf
				0,         // output_row
				2)         // num_rows

			// do down-sampling for the entire 16 rows, 2 rows at a time

			// just copy the Y rows:
			copy(output_buf[0][4960*(2*yOut):], color_buf[0][4960*0:4960*0+4960])
			copy(output_buf[0][4960*(2*yOut+1):], color_buf[0][4960*1:4960*1+4960])

			outputCbPtrs := [...]*uint8{
				&output_buf[1][2480*(yOut)],
				&output_buf[1][2480*(yOut+1)],
			}
			downsampleasm(4960, // image_width
				2, /* max_v_samp_factor */
				1, /* v_samp_factor */
				310,
				&btrsCb[0],
				&outputCbPtrs[0]) // output_buf[1][4960*yOut]

			outputCrPtrs := [...]*uint8{
				&output_buf[2][2480*(yOut)],
				&output_buf[2][2480*(yOut+1)],
			}
			downsampleasm(4960, // image_width
				2, /* max_v_samp_factor */
				1, /* v_samp_factor */
				310,
				&btrsCr[0],
				&outputCrPtrs[0]) // output_buf[1][4960*yOut]

			yOut++
		}

		for MCU_col_num := uint32(0); MCU_col_num <= 309; MCU_col_num++ {
			var blkn int32
			var ypos, xpos uint32

			// Component 0: Y
			xpos = MCU_col_num * 16
			for yindex := 0; yindex < 2; yindex++ {
				outputBufPtrs := [...]*uint8{
					&output_buf[0][4960*(ypos+0)],
					&output_buf[0][4960*(ypos+1)],
					&output_buf[0][4960*(ypos+2)],
					&output_buf[0][4960*(ypos+3)],
					&output_buf[0][4960*(ypos+4)],
					&output_buf[0][4960*(ypos+5)],
					&output_buf[0][4960*(ypos+6)],
					&output_buf[0][4960*(ypos+7)],
				}
				for blockOff := uint32(0); blockOff < 2; blockOff++ {
					forward_DCT(
						0, // quant_tbl_no
						&outputBufPtrs[0],
						&MCU_buffer[blkn][0],
						0,               /*ypos*/ // start_row
						xpos+8*blockOff, // start_col
						1)               // num_blocks: MCU_width

					blkn++
				}
				ypos += 8 // DCTSIZE
			}

			// Component 1: Cb
			xpos = MCU_col_num * 8
			ypos = 0
			outputBufPtrs := [...]*uint8{
				&output_buf[1][2480*(ypos+0)],
				&output_buf[1][2480*(ypos+1)],
				&output_buf[1][2480*(ypos+2)],
				&output_buf[1][2480*(ypos+3)],
				&output_buf[1][2480*(ypos+4)],
				&output_buf[1][2480*(ypos+5)],
				&output_buf[1][2480*(ypos+6)],
				&output_buf[1][2480*(ypos+7)],
			}
			forward_DCT(
				1, // quant_tbl_no
				&outputBufPtrs[0],
				&MCU_buffer[blkn][0],
				0,    // start_row
				xpos, // start_col
				1)    // num_blocks: MCU_width
			blkn++

			// Component 2: Cr
			xpos = MCU_col_num * 8
			outputBufPtrs = [...]*uint8{
				&output_buf[2][2480*(ypos+0)],
				&output_buf[2][2480*(ypos+1)],
				&output_buf[2][2480*(ypos+2)],
				&output_buf[2][2480*(ypos+3)],
				&output_buf[2][2480*(ypos+4)],
				&output_buf[2][2480*(ypos+5)],
				&output_buf[2][2480*(ypos+6)],
				&output_buf[2][2480*(ypos+7)],
			}
			forward_DCT(
				1, // quant_tbl_no
				&outputBufPtrs[0],
				&MCU_buffer[blkn][0],
				0,    // start_row
				xpos, // start_col
				1)    // num_blocks: MCU_width
			blkn++

			for blkn := 0; blkn < 6; /* blocks_in_mcu */ blkn++ {
				ci := MCU_membership[blkn]
				dcTblNo := huffIndexLuminanceDC
				acTblNo := huffIndexLuminanceAC
				if ci != 0 {
					dcTblNo = huffIndexChrominanceDC
					acTblNo = huffIndexChrominanceAC
				}

				// TODO: load_buffer

				res := huffasm(
					e.state,                               // state
					&buffer[0],                            // buffer
					&MCU_buffer[blkn][0],                  // block
					e.state.savable_state.last_dc_val[ci], // last_dc_val
					&(ljtbls[dcTblNo]),                    // dctbl
					&(ljtbls[acTblNo]))                    // actbl
				e.state.savable_state.last_dc_val[ci] = int32(MCU_buffer[blkn][0])
				n := (uintptr(unsafe.Pointer(res)) - uintptr(unsafe.Pointer(&buffer[0])))
				e.w.Write(buffer[:n])
				// TODO: store_buffer
			}

		}
	}
}

// DefaultQuality is the default quality encoding parameter.
const DefaultQuality = 75

// Options are the encoding parameters.
// Quality ranges from 1 to 100 inclusive, higher is better.
type Options struct {
	Quality int
}

// Encode writes the Image m to w in JPEG 4:2:0 baseline format with the given
// options. Default parameters are used if a nil *Options is passed.
func Encode(w io.Writer, size image.Point, o *Options) (*Encoder, error) {
	// b := m.Bounds()
	// if b.Dx() >= 1<<16 || b.Dy() >= 1<<16 {
	// 	return nil, errors.New("neonjpeg: image is too large to encode")
	// }
	// width := b.Max.X - b.Min.X
	// if (len(m.Pix)/width/3)%16 != 0 {
	// 	return nil, errors.New("neonjpeg: m.Pix height must be a multiple of 16")
	// }
	var e Encoder
	if ww, ok := w.(writer); ok {
		e.w = ww
	} else {
		e.w = bufio.NewWriter(w)
	}
	// Clip quality to [1, 100].
	quality := DefaultQuality
	if o != nil {
		quality = o.Quality
		if quality < 1 {
			quality = 1
		} else if quality > 100 {
			quality = 100
		}
	}
	// Convert from a quality rating to a scaling factor.
	var scale int
	if quality < 50 {
		scale = 5000 / quality
	} else {
		scale = 200 - quality*2
	}
	// Initialize the quantization tables.
	for i := range e.quant {
		for j := range e.quant[i] {
			x := int(unscaledQuant[i][j])
			x = (x*scale + 50) / 100
			if x < 1 {
				x = 1
			} else if x > 255 {
				x = 255
			}
			e.quant[i][j] = uint8(x)
		}
	}
	// Compute number of components based on input image type.
	nComponent := 3
	// Write the Start Of Image marker.
	e.buf[0] = 0xff
	e.buf[1] = 0xd8
	e.write(e.buf[:2])
	// Write the quantization tables.
	e.writeDQT()
	// Write the image dimensions.
	e.writeSOF0(size, nComponent)
	// Write the Huffman tables.
	e.writeDHT(nComponent)
	// Write the image data.
	e.write(sosHeaderYCbCr)

	output_byte := make([]byte, blockSize*4)
	s := state{
		next_output_byte: &output_byte[0],
		free_in_buffer:   blockSize * 4, // TODO
	}
	e.state = &s

	//e.writeSOS(m)
	return &e, e.err
}

func (e *Encoder) Flush() error {
	// Pad the last byte with 1's.
	e.emit(0x7f, 7)
	// Write the End Of Image marker.
	e.buf[0] = 0xff
	e.buf[1] = 0xd9
	e.write(e.buf[:2])
	e.flush()
	return e.err
}
