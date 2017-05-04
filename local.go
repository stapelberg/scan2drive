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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/net/trace"
	"golang.org/x/sync/errgroup"

	"github.com/stapelberg/scan2drive/internal/fss500"
	"github.com/stapelberg/scan2drive/internal/fss500/usb"
	"github.com/stapelberg/scan2drive/internal/g3"
	"github.com/stapelberg/scan2drive/proto"
)

func scan(tr trace.Trace, dev io.ReadWriter) error {
	client := proto.NewScanClient(scanConn)
	resp, err := client.DefaultUser(context.Background(), &proto.DefaultUserRequest{})
	if err != nil {
		return err
	}

	relName := time.Now().Format(time.RFC3339)
	scanDir := filepath.Join(*scansDir, resp.User, relName)

	if err := os.MkdirAll(scanDir, 0700); err != nil {
		return err
	}

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

	var (
		compressed   []*bytes.Buffer
		compressedMu sync.Mutex
	)

	type numberedImage struct {
		cnt       int
		binarized *image.Gray
	}
	var (
		binarized   []numberedImage
		binarizedMu sync.Mutex
	)

	for paper := 0; ; paper++ { // pieces of paper (each with a front/back side)
		if err := fss500.ObjectPosition(dev); err != nil {
			if err == fss500.ErrHopperEmpty {
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
		var pixels [2][]byte

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
				pixels[side] = append(pixels[side], resp.Extra...)
				if err == fss500.ErrEndOfPaper && side == back {
					log.Printf("done!")
					break ExhaustData
				}
			} // for side
		} // for

		// TODO(later): process interleaved with reading from USB
		var eg errgroup.Group
		for side := range []int{front, back} {
			side := side // copy
			cnt++
			cnt := cnt // copy
			eg.Go(func() error {
				// Convert the pixel data to *image.RGBA, for which image/jpeg has a specialized version.
				// This brings down JPEG encoding times from 30s to 15s.
				start := time.Now()
				m := fss500.ToRGBA(pixels[side])
				tr.LazyPrintf("converted to *image.RGBA in %v", time.Since(start))

				fn := filepath.Join(scanDir, fmt.Sprintf("page%d.jpg", cnt))
				start = time.Now()
				o, err := os.Create(fn)
				if err != nil {
					return err
				}
				defer o.Close()
				bufw := bufio.NewWriter(o)
				if err := jpeg.Encode(bufw, m, &jpeg.Options{
					Quality: 75, // like scanimage(1)
				}); err != nil {
					return err
				}
				if err := bufw.Flush(); err != nil {
					return err
				}
				if err := o.Close(); err != nil {
					return err
				}
				createCompleteMarker(resp.User, relName, "scan")
				tr.LazyPrintf("saved to JPG in %q in %v", fn, time.Since(start))
				return nil
			})

			eg.Go(func() error {
				// binarize (takes 3s on a Raspberry Pi 3)
				start := time.Now()
				bin, whitePct := binarizeFSS500(pixels[side])
				blank := whitePct > 0.99
				tr.LazyPrintf("white percentage of page %d is %f, blank = %v (binarized in %v)", cnt, whitePct, blank, time.Since(start))
				if blank {
					return nil
				}

				start = time.Now()
				// rotate (takes 7.9s on a Raspberry Pi 3)
				bin = rotate180(bin)
				tr.LazyPrintf("rotated in %v", time.Since(start))

				// compress (takes 3.4s on a Raspberry Pi 3)
				var buf bytes.Buffer
				start = time.Now()
				if err := g3.NewEncoder(&buf).Encode(bin); err != nil {
					return err
				}
				tr.LazyPrintf("compressed in %v", time.Since(start))
				// Prepend (!) the page — the ScanSnap iX500’s
				// document feeder scans the last page first, so we
				// need to reverse the order.
				compressedMu.Lock()
				defer compressedMu.Unlock()
				compressed = append([]*bytes.Buffer{&buf}, compressed...)
				binarizedMu.Lock()
				defer binarizedMu.Unlock()
				binarized = append(binarized, numberedImage{
					cnt:       cnt,
					binarized: bin,
				})
				tr.LazyPrintf("compressed into %d bytes", buf.Len())
				return nil
			})
		} // for side
		if err := eg.Wait(); err != nil {
			return err
		}
	}

	// write PDF
	fn := filepath.Join(scanDir, "scan.pdf")
	o, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer o.Close()
	bufw := bufio.NewWriter(o)
	if err := writePDF(bufw, compressed); err != nil {
		return err
	}
	if err := bufw.Flush(); err != nil {
		return err
	}
	if err := o.Close(); err != nil {
		return err
	}

	// write thumb
	var (
		max    int
		maxImg *image.Gray
	)
	for _, ni := range binarized {
		if ni.cnt > max {
			max = ni.cnt
			maxImg = ni.binarized
		}
	}
	if maxImg != nil {
		fn := filepath.Join(scanDir, "thumb.png")
		o, err := os.Create(fn)
		if err != nil {
			return err
		}
		defer o.Close()
		bufw := bufio.NewWriter(o)
		if err := png.Encode(bufw, maxImg); err != nil {
			return err
		}
		if err := bufw.Flush(); err != nil {
			return err
		}
		if err := o.Close(); err != nil {
			return err
		}
	}

	createCompleteMarker(resp.User, relName, "convert")

	tr.LazyPrintf("processing scan")
	if _, err := client.ProcessScan(context.Background(), &proto.ProcessScanRequest{User: resp.User, Dir: relName}); err != nil {
		return err
	}
	tr.LazyPrintf("scan processed")

	return nil
}

// LocalScanner waits for a locally connected Fujitsu ScanSnap iX500
// to appear, then starts scans whenever the scan button is triggered.
// Running in a goroutine.
func LocalScanner() {
	tr := trace.New("LocalScanner", "fss500")
	defer tr.Finish()

	for {
		dev, err := usb.FindDevice()
		if err != nil {
			tr.LazyPrintf("device not found: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		tr.LazyPrintf("device opened, waiting for scan button press")

		var lastChange time.Time
		for {
			hwStatus, err := fss500.GetHardwareStatus(dev)
			if err != nil {
				tr.LazyPrintf("hardware status request failed: %v", err)
				break
			}
			if hwStatus.ScanSw && time.Since(lastChange) > 5*time.Second {
				lastChange = time.Now()
				tr.LazyPrintf("scan button pressed, scanning")
				if err := scan(tr, dev); err != nil {
					tr.LazyPrintf("scanning failed: %v", err)
				}
			}
			if !hwStatus.Hopper {
				// The user inserted paper, so they’re likely about to
				// scan. Poll more frequently.
				time.Sleep(50 * time.Millisecond)
			} else {
				time.Sleep(1 * time.Second)
			}
		}

		if err := dev.Close(); err != nil {
			tr.LazyPrintf("device close failed: %v", err)
		}
	}
}
