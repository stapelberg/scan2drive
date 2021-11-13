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

package fss500

import (
	"bytes"
	"fmt"
	"image"
	"log"
	"time"

	"github.com/stapelberg/scan2drive/internal/fss500"
	"github.com/stapelberg/scan2drive/internal/fss500/usb"
	"github.com/stapelberg/scan2drive/internal/neonjpeg"
	"github.com/stapelberg/scan2drive/internal/page"
	"github.com/stapelberg/scan2drive/internal/scaningest"
	"golang.org/x/net/trace"
)

// binarizeRotated is a copy of binarize, processing chunks of a 4960x7016 pixel
// array in RGB format (as returned by the Fujitsu ScanSnap iX500). Assumes the
// input is rotated by 180 degrees.
func binarizeRotated(chunk []byte, height int, bin *image.Gray, offset int) int {
	var white int
	const channels = 3
	var r, g, b uint32
	var i, o int
	var a uint32
	for y := 0; y < height; y++ {
		for x := 0; x < 4960; x++ {
			i = channels*4960*y + channels*x

			r = uint32(chunk[i+0])
			r |= r << 8
			g = uint32(chunk[i+1])
			g |= g << 8
			b = uint32(chunk[i+2])
			b |= b << 8

			a = (19595*r + 38470*g + 7471*b + 1<<15) >> 24
			o = (7016-1-(offset+y))*4960 + (4960 - 1 - x)
			if uint8(a) > 127 {
				bin.Pix[o] = 0xff // white
				white++
			} else {
				bin.Pix[o] = 0x00 // black
			}
		}
	}
	return white
}

func scan(tr trace.Trace, ingester *scaningest.Ingester, dev *usb.Device) (_ string, err error) {
	defer func() {
		tr.LazyPrintf("error: %v", err)
		if err != nil {
			tr.SetError()
		}
	}()

	ingestJob, err := ingester.NewJob()
	if err != nil {
		return "", err
	}

	if err := scan1(tr, ingester, dev, ingestJob); err != nil {
		return "", err
	}

	return ingestJob.Ingest()
}

func scan1(tr trace.Trace, ingester *scaningest.Ingester, dev *usb.Device, ingestJob *scaningest.Job) error {
	start := time.Now()

	if err := fss500.Inquire(dev); err != nil {
		return err
	}

	if err := fss500.Preread(dev); err != nil {
		return err
	}

	// mode_select_auto: overscan/auto detection
	if err := fss500.ModeSelectAuto(dev); err != nil {
		return err
	}

	// mode_select_df: double feed detection
	if err := fss500.ModeSelectDoubleFeed(dev); err != nil {
		return err
	}

	// mode_select_bg: background color setting
	if err := fss500.ModeSelectBackground(dev); err != nil {
		return err
	}

	// mode_select_dropout: dropout color setting
	if err := fss500.ModeSelectDropout(dev); err != nil {
		return err
	}

	if err := fss500.ModeSelectBuffering(dev); err != nil {
		return err
	}

	if err := fss500.ModeSelectPrepick(dev); err != nil {
		return err
	}

	if err := fss500.SetWindow(dev); err != nil {
		return err
	}

	// send_lut (for hardware with no brightness/contrast)
	if err := fss500.SendLut(dev); err != nil {
		return err
	}

	// send_q_table (for JPEG)
	if err := fss500.SendQtable(dev); err != nil {
		return err
	}

	if err := fss500.LampOn(dev); err != nil {
		return err
	}

	if _, err := fss500.GetHardwareStatus(dev); err != nil {
		return err
	}

	var cnt int

	type numberedPage struct {
		cnt        int
		compressed *bytes.Buffer
		binarized  *image.Gray
	}

	for paper := 0; ; paper++ { // pieces of paper (each with a front/back side)
		if err := fss500.ObjectPosition(dev); err != nil {
			if err == fss500.ErrHopperEmpty {
				if paper == 0 {
					return fmt.Errorf("no document inserted")
				}
				break
			}
			return fmt.Errorf("ObjectPosition: %v", err)
		}

		if err := fss500.StartScan(dev); err != nil {
			return fmt.Errorf("StartScan: %v", err)
		}

		if err := fss500.GetPixelSize(dev); err != nil {
			return fmt.Errorf("GetPixelSize: %v", err)
		}

		const (
			front = 0
			back  = 1
		)
		type pageState struct {
			buf  *bytes.Buffer
			cnt  int
			enc  *neonjpeg.Encoder
			rest []byte // buffers pixels until 16 full rows
			ch   chan []byte
			done chan struct{}

			bin     *image.Gray // binarized and rotated full page
			offset  int
			whitePx int // number of white pixels in binarized page
		}
		var state [2]*pageState

		for side := range []int{front, back} {
			cnt++
			var buf bytes.Buffer
			enc, err := neonjpeg.Encode(&buf, image.Point{4960, 7016}, &neonjpeg.Options{
				Quality: 75, // like scanimage(1)
			})
			if err != nil {
				return err
			}
			ps := &pageState{
				buf:  &buf,
				cnt:  cnt,
				enc:  enc,
				rest: make([]byte, 0, 16*3*4960),
				ch:   make(chan []byte),
				done: make(chan struct{}),
				bin:  image.NewGray(image.Rect(0, 0, 4960, 7016)),
			}
			go func() {
				for chunk := range ps.ch {
					height := len(chunk) / 3 / 4960
					if padding := height % 16; padding != 0 {
						chunk = append(chunk, make([]byte, padding*3*4960)...)
					}
					ps.enc.EncodePixels(chunk, height)
					white := binarizeRotated(chunk, height, ps.bin, ps.offset)
					ps.offset += height
					ps.whitePx += white
				}
				ps.done <- struct{}{}
			}()
			state[side] = ps
		}

	ExhaustData:
		for {
			for side := range []int{front, back} {
				if err := fss500.Ric(dev, side); err != nil {
					return fmt.Errorf("Ric: %v", err)
				}

				resp, err := fss500.ReadData(dev, side)
				if err == fss500.ErrTemporaryNoData {
					time.Sleep(500 * time.Millisecond)
					continue ExhaustData
				} else if err == fss500.ErrEndOfPaper {
				} else if err != nil {
					return err
				}

				buf := append(state[side].rest, resp.Extra...)

				// Grab a chunk of 16 full rows (as required by neonjpeg), store
				// the remaining bytes for the next iteration.
				height := len(buf) / 3 / 4960
				chunk := buf[:((height/16)*16)*3*4960]
				state[side].rest = buf[len(chunk):]

				// Copy chunk to safely use it in a separate goroutine.
				tmp := make([]byte, len(chunk))
				copy(tmp, chunk)
				state[side].ch <- tmp

				if err == fss500.ErrEndOfPaper && side == back {
					log.Printf("done!")
					break ExhaustData
				}
			} // for side
		} // for

		for side := range []int{front, back} {
			ps := state[side]
			ps.ch <- ps.rest
			close(ps.ch)
			<-ps.done
			if err := ps.enc.Flush(); err != nil {
				return err
			}

			whitePct := float64(ps.whitePx) / float64(4960*7016)
			pg := page.Binarized(ps.buf.Bytes(), ps.bin, whitePct)
			if err := ingestJob.AddPage(pg); err != nil {
				return err
			}
		}

		tr.LazyPrintf("scan done in %v", time.Since(start))
	}

	// The ScanSnap iX500’s document feeder scans the last page first, so we
	// need to reverse the order:
	ingestJob.ReversePages()

	return nil
}
