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

// Package fss500 is a driver for the Fujitsu ScanSnap iX500 document
// scanner, implemented from scratch based on USB traffic
// captures. Terminology has been chosen to be consistent with the
// SANE fujitsu driver’s terminology where appropriate.
//
// See also https://www.staff.uni-mainz.de/tacke/scsi/SCSI2-15.html
package fss500

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"time"
)

var (
	// ErrShortRead represents a short read when transferring data.
	ErrShortRead = errors.New("short read")

	// ErrEndOfPaper is returned when no more data is to be read for
	// this page.
	ErrEndOfPaper = errors.New("end of paper")

	// ErrTemporaryNoData is returned when data was requested, but
	// there is no data to be read from the scanner.
	ErrTemporaryNoData = errors.New("temporary no data")

	// ErrHopperEmpty is returned when no paper is in the document
	// hopper.
	ErrHopperEmpty = errors.New("hopper empty")
)

// usbcmd embeds scsiCmd in a command structure for the Fujitsu
// ScanSnap iX500, to be sent via USB bulk transfer.
func usbcmd(scsiCmd []byte) []byte {
	// All usb commands are 31 bytes in length, padded with zeros. The
	// actual command starts at offset 0x13.
	result := make([]byte, 31)
	result[0] = 0x43 // usb command
	copy(result[0x13:], scsiCmd)
	return result
}

type request struct {
	cmd     []byte
	extra   []byte
	respLen int
}

// Response contains the raw response and extra bytes read from the
// device.
type Response struct {
	// Raw contains the raw response bytes for the SCSI command.
	Raw []byte
	// Extra contains the extra bytes, if any, for the SCSI command.
	Extra []byte
}

func doWithoutRequestSense(dev io.ReadWriter, r *request) (*Response, error) {
	if _, err := dev.Write(usbcmd(r.cmd)); err != nil {
		return nil, err
	}

	if r.extra != nil {
		//log.Printf("writing extra %x", r.extra)
		if _, err := dev.Write(r.extra); err != nil {
			return nil, err
		}
	}

	var resp Response

	if r.respLen > 0 {
		resp.Extra = make([]byte, r.respLen)
		num, err := dev.Read(resp.Extra)
		if err != nil {
			return nil, err
		}
		resp.Extra = resp.Extra[:num]
		max := len(resp.Extra)
		if max > 10 {
			max = 10
		}
		//log.Printf("read extra %x", resp.Extra[:max])
	}

	resp.Raw = make([]byte, 32)
	num, err := dev.Read(resp.Raw)
	if err != nil {
		return nil, err
	}
	resp.Raw = resp.Raw[:num]
	//log.Printf("read %x", resp.Raw)
	return &resp, err
}

// do sends the specified request, reads the response as instructed,
// and reports any errors by sending an additional REQUEST SENSE
// command.
func do(dev io.ReadWriter, r *request) (*Response, error) {
	resp, err := doWithoutRequestSense(dev, r)
	if err != nil {
		return nil, err
	}

	const usbStatusOffset = 9
	if resp.Raw[usbStatusOffset] == 0 {
		return resp, nil
	}

	rsResp, err := doWithoutRequestSense(dev, &request{
		cmd: []byte{
			// see http://self.gutenberg.org/articles/scsi_request_sense_command
			0x03, // SCSI opcode: REQUEST SENSE
			0x00, // byte 7, 6, 5: LUN. rest: reserved
			0x00, // reserved
			0x00, // reserved
			0x12, // allocation length
			0x00, // control
		},
		respLen: 18,
	})
	if err != nil {
		return nil, err
	}

	// TODO: verify rsResp.raw itself is successful?

	// 000: f0 00 03 00 00 00 00 0a 00 00 00 00 80 13 00 00 ................
	// 010: 00 00                                           ..

	sense := rsResp.Extra[2] & 0x0F
	asc := rsResp.Extra[12]
	ascq := rsResp.Extra[13]
	rsInfo := rsResp.Extra[3 : 3+4]
	rsEom := (rsResp.Extra[2]>>6)&0x1 == 0x1
	rsIli := (rsResp.Extra[2]>>5)&0x1 == 0x1
	//log.Printf("sense = %v, asc = %v, ascq = %v, rsInfo = %v, rsEom = %v, rsIli = %v",
	//sense, asc, ascq, rsInfo, rsEom, rsIli)

	if rsIli {
		n := len(resp.Extra) - int(binary.BigEndian.Uint32(rsInfo))
		resp.Extra = resp.Extra[:n]
	}

	return resp, requestSenseToError(sense, asc, ascq, rsInfo, rsEom, rsIli)
}

// Inquire requests the scanner make and model.
func Inquire(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 12 00 00 00 60 00 00 00 00 00 00 00    .......`.......

	// TODO: verify the scanner make and model in resp.extra?
	_, err := do(dev, &request{
		cmd: []byte{
			0x12, // SCSI opcode: INQUIRY
			0x00, // EVPD (enable vital product data): disabled
			0x00, // page code (for EVPD)
			0x00, // reserved
			0x60, // allocation length: 96 bytes
			0x00, // control
		},
		respLen: 96,
	})

	// response:
	// 000: 06 00 92 02 5b 00 00 10 46 55 4a 49 54 53 55 20 ....[...FUJITSU
	// 010: 53 63 61 6e 53 6e 61 70 20 69 58 35 30 30 20 20 ScanSnap iX500
	// 020: 30 4d 30 30 00 00 00 00 00 00 00 00 03 01 00 00 0M00............
	// 030: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 ................
	// 040: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 ................
	// 050: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 ................

	return err
}

// Preread switches the scanner into 600 dpi scan mode.
func Preread(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 1d 00 00 00 20 00 00 00 00 00 00 00    ....... .......

	// extra request:
	// 000: 53 45 54 20 50 52 45 20 52 45 41 44 4d 4f 44 45 SET PRE READMODE
	// 010: 02 58 02 58 00 00 26 c3 00 00 36 d1 05 7f 00 00 .X.X..&...6.....

	extra := append([]byte("SET PRE READMODE"),
		0x02, // x resolution: hi byte
		0x58, // x resolution: lo byte (→ 600 dpi)

		0x02, // y resolution: hi byte
		0x58, // y resolution: lo byte (→ 600 dpi)

		0x00, // paper width
		0x00, // paper width
		0x26, // paper width
		0xc3, // paper width

		0x00, // paper length
		0x00, // paper length
		0x36, // paper length
		0xd1, // paper length

		0x05, // composition
		0x7f, // TODO: where does this come from/what does it mean?
		0x00,
		0x00,
	)

	_, err := do(dev, &request{
		cmd: []byte{
			0x1d, // SCSI opcode: SEND_DIAGNOSTIC
			0x00, // page format (bit 4): disabled
			// self test (bit 2): disabled,
			// device offline (bit 1): disabled,
			// unit offline (bit 0): disabled
			0x00, // reserved
			0x00, // parameter list length (MSB)
			0x20, // parameter list length (LSB)
			0x00, // control
		},
		extra: extra,
	})

	return err
}

// ModeSelectAuto enables automatic paper feed.
func ModeSelectAuto(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 15 10 00 00 0c 00 00 00 00 00 00 00    ...............

	// extra request:
	// 000: 00 00 00 00 3c 06 00 00 00 00 00 00             ....<.......

	_, err := do(dev, &request{
		cmd: []byte{
			0x15, // SCSI opcode: mode select
			0x10, // page format (bit 4): enabled
			// save pages (bit 0): disabled
			0x00, // reserved
			0x00, // reserved
			0x0c, // parameter list length
			0x00, // control
		},
		extra: []byte{
			0x00, // pc?
			0x00, // page len?
			0x00, // awd? crop?
			0x00, // ald?
			0x3c, // deskew
			0x06, // overscan
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
		},
	})

	return err
}

// ModeSelectDoubleFeed enables double feed detection.
func ModeSelectDoubleFeed(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 15 10 00 00 0c 00 00 00 00 00 00 00    ...............

	// extra request:
	// 000: 00 00 00 00 38 06 00 00 00 00 00 00             ....8.......

	_, err := do(dev, &request{
		cmd: []byte{
			0x15, // SCSI opcode: mode select
			0x10, // page format (bit 4): enabled
			// save pages (bit 0): disabled
			0x00, // reserved
			0x00, // reserved
			0x0c, // parameter list length
			0x00, // control
		},
		extra: []byte{
			0x00, // pc?
			0x00, // page len?
			0x00,
			0x00,
			0x38,
			0x06,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
		},
	})

	return err
}

// ModeSelectBackground sets the background color setting
func ModeSelectBackground(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 15 10 00 00 0c 00 00 00 00 00 00 00    ...............

	// extra request:
	// 000: 00 00 00 00 37 06 00 00 00 00 00 00             ....7.......

	_, err := do(dev, &request{
		cmd: []byte{
			0x15, // SCSI opcode: mode select
			0x10, // page format (bit 4): enabled
			// save pages (bit 0): disabled
			0x00, // reserved
			0x00, // reserved
			0x0c, // parameter list length
			0x00, // control
		},
		extra: []byte{
			0x00,
			0x00,
			0x00,
			0x00,
			0x37,
			0x06,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
		},
	})

	return err
}

// ModeSelectDropout sets the dropout color.
func ModeSelectDropout(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 15 10 00 00 0e 00 00 00 00 00 00 00    ...............

	// extra request:
	// 000: 00 00 00 00 39 08 00 00 00 00 00 00 00 00       ....9.........

	_, err := do(dev, &request{
		cmd: []byte{
			0x15, // SCSI opcode: mode select
			0x10, // page format (bit 4): enabled
			// save pages (bit 0): disabled
			0x00, // reserved
			0x00, // reserved
			0x0e, // parameter list length
			0x00, // control
		},
		extra: []byte{
			0x00,
			0x00,
			0x00,
			0x00,
			0x39,
			0x08,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
		},
	})

	return err
}

// ModeSelectBuffering enables buffering.
func ModeSelectBuffering(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 15 10 00 00 0c 00 00 00 00 00 00 00    ...............

	// extra request:
	// 000: 00 00 00 00 3a 06 80 c0 00 00 00 00             ....:.......

	_, err := do(dev, &request{
		cmd: []byte{
			0x15, // SCSI opcode: mode select
			0x10, // page format (bit 4): enabled
			// save pages (bit 0): disabled
			0x00, // reserved
			0x00, // reserved
			0x0c, // parameter list length
			0x00, // control
		},
		extra: []byte{
			0x00,
			0x00,
			0x00,
			0x00,
			0x3a,
			0x06,
			0x80,
			0xc0,
			0x00,
			0x00,
			0x00,
			0x00,
		},
	})

	return err
}

// ModeSelectPrepick enables prepick.
func ModeSelectPrepick(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 15 10 00 00 0c 00 00 00 00 00 00 00    ...............

	// extra request:
	// 000: 00 00 00 00 33 06 00 00 00 00 00 00             ....3.......

	_, err := do(dev, &request{
		cmd: []byte{
			0x15, // SCSI opcode: mode select
			0x10, // page format (bit 4): enabled
			// save pages (bit 0): disabled
			0x00, // reserved
			0x00, // reserved
			0x0c, // parameter list length
			0x00, // control
		},
		extra: []byte{
			0x00,
			0x00,
			0x00,
			0x00,
			0x33,
			0x06,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
		},
	})

	return err
}

// SetWindow sets a window for scanning, specifying parameters such as
// the brightness, threshold, contrast, compression type, etc.
func SetWindow(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 24 00 00 00 00 00 00 00 88 00 00 00    ...$...........

	// extra request:
	// 000: 00 00 00 00 00 00 00 40 00 00 02 58 02 58 00 00 .......@...X.X..
	// 010: 00 00 00 00 00 00 00 00 26 c0 00 00 36 d0 00 00 ........&...6...
	// 020: 00 05 08 00 00 00 00 00 00 00 00 00 00 00 00 00 ................
	// 030: c1 80 01 00 00 00 00 00 00 00 00 00 00 c0 00 00 ................
	// 040: 26 c3 00 00 36 d1 00 00 80 00 02 58 02 58 00 00 &...6......X.X..
	// 050: 00 00 00 00 00 00 00 00 26 c0 00 00 36 d0 00 00 ........&...6...
	// 060: 00 05 08 00 00 00 00 00 00 00 00 00 00 00 00 00 ................
	// 070: c1 80 01 00 00 00 00 00 00 00 00 00 00 00 00 00 ................
	// 080: 00 00 00 00 00 00 00 00                         ........

	_, err := do(dev, &request{
		cmd: []byte{
			0x24, // SCSI opcode: set window
			0x00, // reserved
			0x00, // reserved
			0x00, // reserved
			0x00, // reserved

			0x00, // reserved
			0x00, // transfer length (MSB)
			0x00, // transfer length
			0x88, // transfer length (LSB): 136 bytes */
			0x00, // control
		},
		extra: []byte{ // window descriptor, see also https://www.staff.uni-mainz.de/tacke/scsi/SCSI2-15.html#tab282
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x02, 0x58, 0x02, 0x58, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x26, 0xc0, 0x00, 0x00, 0x36, 0xd0, 0x00, 0x00,
			0x00, 0x05, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0xc1, 0x80, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xc0, 0x00, 0x00,
			0x26, 0xc3, 0x00, 0x00, 0x36, 0xd1, 0x00, 0x00, 0x80, 0x00, 0x02, 0x58, 0x02, 0x58, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x26, 0xc0, 0x00, 0x00, 0x36, 0xd0, 0x00, 0x00,
			0x00, 0x05, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0xc1, 0x80, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		},
	})

	return err
}

// TODO: document SendLut. Does lut stand for lookup table?
func SendLut(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 2a 00 83 00 00 00 00 01 0a 00 00 00    ...*...........

	// extra request:
	// 000: 00 00 10 00 01 00 01 00 00 00 00 00 01 02 03 04 ................
	// 010: 05 06 07 08 09 0a 0b 0c 0d 0e 0f 10 11 12 13 14 ................
	// 020: 15 16 17 18 19 1a 1b 1c 1d 1e 1f 20 21 22 23 24 ........... !"#$
	// 030: 25 26 27 28 29 2a 2b 2c 2d 2e 2f 30 31 32 33 34 %&'()*+,-./01234
	// 040: 35 36 37 38 39 3a 3b 3c 3d 3e 3f 40 41 42 43 44 56789:;<=>?@ABCD
	// 050: 45 46 47 48 49 4a 4b 4c 4d 4e 4f 50 51 52 53 54 EFGHIJKLMNOPQRST
	// 060: 55 56 57 58 59 5a 5b 5c 5d 5e 5f 60 61 62 63 64 UVWXYZ[\]^_`abcd
	// 070: 65 66 67 68 69 6a 6b 6c 6d 6e 6f 70 71 72 73 74 efghijklmnopqrst
	// 080: 75 76 77 78 79 7a 7b 7c 7d 7e 7f 80 81 82 83 84 uvwxyz{|}~......
	// 090: 85 86 87 88 89 8a 8b 8c 8d 8e 8f 90 91 92 93 94 ................
	// 0a0: 95 96 97 98 99 9a 9b 9c 9d 9e 9f a0 a1 a2 a3 a4 ................
	// 0b0: a5 a6 a7 a8 a9 aa ab ac ad ae af b0 b1 b2 b3 b4 ................
	// 0c0: b5 b6 b7 b8 b9 ba bb bc bd be bf c0 c1 c2 c3 c4 ................
	// 0d0: c5 c6 c7 c8 c9 ca cb cc cd ce cf d0 d1 d2 d3 d4 ................
	// 0e0: d5 d6 d7 d8 d9 da db dc dd de df e0 e1 e2 e3 e4 ................
	// 0f0: e5 e6 e7 e8 e9 ea eb ec ed ee ef f0 f1 f2 f3 f4 ................
	// 100: f5 f6 f7 f8 f9 fa fb fc fd fe                   ..........

	_, err := do(dev, &request{
		cmd: []byte{
			0x2a, // SCSI opcode: SEND
			0x00, // reserved
			0x83, // data type code: vendor-specific: lut table
			0x00, // reserved
			0x00, // data type qualifier (MSB)

			0x00, // data type qualifier (LSB)
			0x00, // transfer length (MSB)
			0x01, // transfer length
			0x0a, // transfer length (LSB)
			0x00, // control
		},
		extra: []byte{
			0x00, 0x00, 0x10, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04,
			0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14,
			0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21, 0x22, 0x23, 0x24,
			0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0x34,
			0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f, 0x40, 0x41, 0x42, 0x43, 0x44,
			0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f, 0x50, 0x51, 0x52, 0x53, 0x54,
			0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e, 0x5f, 0x60, 0x61, 0x62, 0x63, 0x64,
			0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74,
			0x75, 0x76, 0x77, 0x78, 0x79, 0x7a, 0x7b, 0x7c, 0x7d, 0x7e, 0x7f, 0x80, 0x81, 0x82, 0x83, 0x84,
			0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90, 0x91, 0x92, 0x93, 0x94,
			0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0x9b, 0x9c, 0x9d, 0x9e, 0x9f, 0xa0, 0xa1, 0xa2, 0xa3, 0xa4,
			0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0, 0xb1, 0xb2, 0xb3, 0xb4,
			0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xbb, 0xbc, 0xbd, 0xbe, 0xbf, 0xc0, 0xc1, 0xc2, 0xc3, 0xc4,
			0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf, 0xd0, 0xd1, 0xd2, 0xd3, 0xd4,
			0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf, 0xe0, 0xe1, 0xe2, 0xe3, 0xe4,
			0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xf0, 0xf1, 0xf2, 0xf3, 0xf4,
			0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xfb, 0xfc, 0xfd, 0xfe,
		},
	})

	return err
}

// TODO: document SendQtable
func SendQtable(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 2a 00 88 00 00 00 00 00 8a 00 00 00    ...*...........

	// extra request:
	// 000: 00 00 00 00 00 40 00 40 00 00 04 03 03 04 03 03 .....@.@........
	// 010: 04 04 03 04 05 05 04 05 07 0c 07 07 06 06 07 0e ................
	// 020: 0a 0b 08 0c 11 0f 12 12 11 0f 10 10 13 15 1b 17 ................
	// 030: 13 14 1a 14 10 10 18 20 18 1a 1c 1d 1e 1f 1e 12 ....... ........
	// 040: 17 21 24 21 1e 24 1b 1e 1e 1d 05 05 05 07 06 07 .!$!.$..........
	// 050: 0e 07 07 0e 1d 13 10 13 1d 1d 1d 1d 1d 1d 1d 1d ................
	// 060: 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d ................
	// 070: 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d ................
	// 080: 1d 1d 1d 1d 1d 1d 1d 1d 1d 1d                   ..........

	_, err := do(dev, &request{
		cmd: []byte{
			0x2a, // SCSI opcode: SEND
			0x00, // reserved
			0x88, // data type code: vendor-specific: qtable
			0x00, // reserved
			0x00, // data type qualifier (MSB)

			0x00, // data type qualifier (LSB)
			0x00, // transfer length (MSB)
			0x00, // transfer length
			0x8a, // transfer length (MSB)
			0x00, // control
		},
		extra: []byte{
			0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x40, 0x00, 0x00, 0x04, 0x03, 0x03, 0x04, 0x03, 0x03,
			0x04, 0x04, 0x03, 0x04, 0x05, 0x05, 0x04, 0x05, 0x07, 0x0c, 0x07, 0x07, 0x06, 0x06, 0x07, 0x0e,
			0x0a, 0x0b, 0x08, 0x0c, 0x11, 0x0f, 0x12, 0x12, 0x11, 0x0f, 0x10, 0x10, 0x13, 0x15, 0x1b, 0x17,
			0x13, 0x14, 0x1a, 0x14, 0x10, 0x10, 0x18, 0x20, 0x18, 0x1a, 0x1c, 0x1d, 0x1e, 0x1f, 0x1e, 0x12,
			0x17, 0x21, 0x24, 0x21, 0x1e, 0x24, 0x1b, 0x1e, 0x1e, 0x1d, 0x05, 0x05, 0x05, 0x07, 0x06, 0x07,
			0x0e, 0x07, 0x07, 0x0e, 0x1d, 0x13, 0x10, 0x13, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d,
			0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d,
			0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d,
			0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d,
		},
	})

	return err
}

// LampOn turns on the scanner’s lamp.
func LampOn(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 f1 05 00 00 00 00 00 00 00 00 00 00    ...............

	_, err := do(dev, &request{
		cmd: []byte{
			0xf1, // SCSI opcode: SCANNER_CONTROL
			0x05, // scan control function: lamp on
			0x00,
			0x00,
			0x00,

			0x00,
			0x00,
			0x00,
			0x00,
			0x00,
		},
	})

	return err
}

// HardwareStatus contains status bits for individual features of the
// scanner, e.g. whether paper is inserted into the hopper, and
// whether the scan button was pressed.
type HardwareStatus struct {
	top bool
	a3  bool
	b4  bool
	a4  bool
	b5  bool

	Hopper  bool
	omr     bool
	adfOpen bool

	sleep      bool
	sendSw     bool
	manualFeed bool
	ScanSw     bool

	function byte
	inkEmpty bool

	doubleFeed bool

	errorCode byte

	skewAngle uint16
}

func hardwareStatusFromBytes(b []byte) HardwareStatus {
	return HardwareStatus{
		top: (b[2]>>7)&1 == 1,
		a3:  (b[2]>>3)&1 == 1,
		b4:  (b[2]>>2)&1 == 1,
		a4:  (b[2]>>1)&1 == 1,
		b5:  (b[2]>>0)&1 == 1,

		Hopper:  (b[3]>>7)&1 == 1,
		omr:     (b[3]>>6)&1 == 1,
		adfOpen: (b[3]>>5)&1 == 1,

		sleep:      (b[4]>>7)&1 == 1,
		sendSw:     (b[4]>>2)&1 == 1,
		manualFeed: (b[4]>>1)&1 == 1,
		ScanSw:     (b[4]>>0)&1 == 1,

		function: (b[5] >> 0) & 0xf,
		inkEmpty: (b[6]>>7)&1 == 1,

		doubleFeed: (b[6]>>0)&1 == 1,

		errorCode: b[7],

		skewAngle: (uint16(b[8]) << 8) | uint16(b[9]),
	}
}

// GetHardwareStatus retrieves the hardware status (including whether
// paper is inserted into the hopper, and whether the scan button was
// pressed) from the device.
func GetHardwareStatus(dev io.ReadWriter) (HardwareStatus, error) {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 c2 00 00 00 00 00 00 00 0c 00 00 00    ...............

	// extra request:
	// 000: 00 00 00 00 00 01 00 00 00 00 00 00             ............

	resp, err := do(dev, &request{
		cmd: []byte{
			0xc2, // SCSI opcode: GET_HW_STATUS
			0x00,
			0x00,
			0x00,
			0x00,

			0x00,
			0x00,
			0x00,
			0x0c,
			0x00,
		},
		respLen: 12,
	})
	if err != nil {
		return HardwareStatus{}, err
	}
	return hardwareStatusFromBytes(resp.Extra), nil
}

// ObjectPosition loads an object (paper) into the scanner, returning
// an error if no more paper is found in the document feeder.
func ObjectPosition(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 31 01 00 00 00 00 00 00 00 00 00 00    ...1...........

	_, err := do(dev, &request{
		cmd: []byte{
			0x31, // SCSI opcode: OBJECT POSITION
			0x01, // load object
			0x00, // count (MSB)
			0x00, // count
			0x00, // count (LSB)

			0x00, // reserved
			0x00, // reserved
			0x00, // reserved
			0x00, // reserved
			0x00, // control
		},
	})
	return err
}

// StartScan instructs the scanner to start scanning.
func StartScan(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 1b 00 00 00 02 00 00 00 00 00 00 00    ...............

	_, err := do(dev, &request{
		cmd: []byte{
			0x1b, // SCSI opcode: SCAN
			0x00, // reserved
			0x00, // reserved
			0x00, // reserved
			0x02, // transfer length
			0x00, // control
		},
		extra: []byte{ // window ID list
			0x00, // front
			0x80, // back
		},
	})
	return err
}

// GetPixelSize requests the pixel size of the object being scanned.
func GetPixelSize(dev io.ReadWriter) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 28 00 80 00 00 00 00 00 20 00 00 00    ...(....... ...

	// extra request:
	// 000: 00 00 13 60 00 00 1b 68 00 00 13 60 00 00 1b 68 ...`...h...`...h
	// 010: 00 00 00 00 00 00 00 00 13 60 00 00 1b 68 00 00 .........`...h..

	// TODO: interpret pixel size?
	// first 4 bytes: scan_x = 4960
	// next 4 bytes: scan_y = 7016

	// next 4 bytes: paper_w = 4960
	// next 4 bytes: paper_l = 7016

	// → bytes per line = scan_x * 3 (for color) = 14880

	_, err := do(dev, &request{
		cmd: []byte{
			0x28, // SCSI opcode: READ
			0x00, // reserved
			0x80, // data type: vendor-specific
			0x00, // reserved
			0x00, // data type qualifier (MSB)

			0x00, // data type qualifier (LSB)
			0x00, // transfer length (MSB)
			0x00, // transfer length
			0x20, // transfer length (LSB)
			0x00, // control
		},
		respLen: 32,
	})
	return err
}

// TODO: document Ric
func Ric(dev io.ReadWriter, side int) error {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 f1 10 00 00 00 00 03 dc 20 00 00 00    ........... ...

	windowID := byte(0x00)
	if side == 1 {
		windowID = 0x80
	}
	var err error
	for tries := 0; tries < 120; tries++ {
		_, err = do(dev, &request{
			cmd: []byte{
				0xf1, // SCSI opcode: SCANNER_CONTROL
				0x10,
				windowID, // window id: front (back is 0x80)
				0x00,
				0x00,

				0x00,
				0x03,
				0xdc,
				0x20,
				0x00,
			},
		})
		if err == nil {
			break
		}
		log.Printf("error = %v, retrying (%d of 120)", err, tries)
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// ReadData reads data from the front of the page (side == 0) or the
// back of the page (side == 1).
func ReadData(dev io.ReadWriter, side int) (*Response, error) {
	// request:
	// 000: 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 C...............
	// 010: 00 00 00 28 00 00 00 00 00 03 dc 20 00 00 00    ...(....... ...

	windowID := byte(0x00)
	if side == 1 {
		windowID = 0x80
	}

	resp, err := do(dev, &request{
		cmd: []byte{
			0x28, // SCSI opcode: READ
			0x00, // reserved
			0x00, // data type code: image
			0x00, // reserved
			0x00, // data type qualifier (MSB)

			windowID, // data type qualifier (LSB): window id
			0x03,     // transfer length (MSB)
			0xdc,     // transfer length
			0x20,     // transfer length (LSB)
			0x00,     // control
		},
		respLen: 252960,
	})
	if err != nil {
		return resp, err
	}

	for i := 0; i < len(resp.Extra); i++ {
		resp.Extra[i] = 255 - resp.Extra[i] // invert
	}

	return resp, err
}
