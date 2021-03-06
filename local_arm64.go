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
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/trace"
	"golang.org/x/sync/errgroup"

	"github.com/augustoroman/serial_lcd"
	"github.com/google/renameio"
	"github.com/stapelberg/scan2drive/internal/dispatch"
	"github.com/stapelberg/scan2drive/internal/fss500"
	"github.com/stapelberg/scan2drive/internal/fss500/usb"
	"github.com/stapelberg/scan2drive/internal/g3"
	"github.com/stapelberg/scan2drive/internal/gpio"
	"github.com/stapelberg/scan2drive/internal/neonjpeg"
	"github.com/stapelberg/scan2drive/proto"
)

var lastStatus string

func publishStatus(status string) {
	// Prevent duplicate messages if status has not changed
	if lastStatus == status {
		return
	}
	lastStatus = status
	mqttPublish <- publishRequest{
		Topic:    "scan2drive/ui/status",
		Retained: true,
		Payload:  []byte(status),
	}
}

func scan(tr trace.Trace, user string, dev io.ReadWriter, display *serial_lcd.LCD) (err error) {
	defer func() {
		tr.LazyPrintf("error: %v", err)
		if err != nil {
			tr.SetError()
		}
	}()

	display.Clear()
	display.MoveTo(1, 1)
	fmt.Fprintf(display, "scanning...")
	publishStatus("scanning...")

	start := time.Now()
	client := proto.NewScanClient(scanConn)

	relName := time.Now().Format(time.RFC3339)
	scanDir := filepath.Join(*scansDir, user, relName)

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

	type numberedPage struct {
		cnt        int
		compressed *bytes.Buffer
		binarized  *image.Gray
	}
	var (
		pages   []numberedPage
		pagesMu sync.Mutex
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
		type pageState struct {
			cnt  int
			out  *renameio.PendingFile
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
			fn := filepath.Join(scanDir, fmt.Sprintf("page%d.jpg", cnt))
			o, err := renameio.TempFile("", fn)
			if err != nil {
				return err
			}
			defer o.Cleanup()
			enc, err := neonjpeg.Encode(o, image.Point{4960, 7016}, &neonjpeg.Options{
				Quality: 75, // like scanimage(1)
			})
			if err != nil {
				return err
			}
			ps := &pageState{
				cnt:  cnt,
				out:  o,
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
			if err := ps.out.CloseAtomicallyReplace(); err != nil {
				return err
			}
			createCompleteMarker(user, relName, "scan")
		}

		tr.LazyPrintf("scan done in %v", time.Since(start))

		var eg errgroup.Group
		for side := range []int{front, back} {
			side := side // copy
			eg.Go(func() error {
				// Skip blank pages
				whitePct := float64(state[side].whitePx) / float64(4960*7016)
				blank := whitePct > 0.99
				tr.LazyPrintf("white percentage of page %d is %f, blank = %v", state[side].cnt, whitePct, blank)
				if blank {
					return nil
				}

				// compress (takes 3.4s on a Raspberry Pi 3)
				var buf bytes.Buffer
				start := time.Now()
				if err := g3.NewEncoder(&buf).Encode(state[side].bin); err != nil {
					return err
				}
				tr.LazyPrintf("compressed in %v", time.Since(start))
				pagesMu.Lock()
				defer pagesMu.Unlock()
				pages = append(pages, numberedPage{
					cnt:        state[side].cnt,
					binarized:  state[side].bin,
					compressed: &buf,
				})
				tr.LazyPrintf("compressed into %d bytes", buf.Len())
				return nil
			})
		} // for side
		if err := eg.Wait(); err != nil {
			return err
		}
	}

	// Prepend (!) the page — the ScanSnap iX500’s
	// document feeder scans the last page first, so we
	// need to reverse the order.
	sort.Slice(pages, func(i, j int) bool { return pages[i].cnt < pages[j].cnt })
	compressed := make([]*bytes.Buffer, 0, len(pages))
	bounds := make([]image.Rectangle, 0, len(pages))
	for i := len(pages) - 1; i >= 0; i-- {
		compressed = append(compressed, pages[i].compressed)
		bounds = append(bounds, image.Rect(0, 0, 4960, 7016))
	}

	display.Clear()
	display.MoveTo(1, 1)
	fmt.Fprintf(display, "writing %d pages", len(pages))
	publishStatus(fmt.Sprintf("writing %d pages", len(pages)))

	// write PDF
	fn := filepath.Join(scanDir, "scan.pdf")
	o, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer o.Close()
	bufw := bufio.NewWriter(o)
	if err := writePDF(bufw, compressed, bounds); err != nil {
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
	for _, ni := range pages {
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

	createCompleteMarker(user, relName, "convert")

	display.Clear()
	display.MoveTo(1, 1)
	fmt.Fprintf(display, "uploading %d pages", len(pages))
	publishStatus(fmt.Sprintf("uploading %d pages", len(pages)))

	tr.LazyPrintf("processing scan")
	if _, err := client.ProcessScan(context.Background(), &proto.ProcessScanRequest{User: user, Dir: relName}); err != nil {
		return err
	}
	tr.LazyPrintf("scan processed")

	return nil
}

type discardLCD struct{}

func (d *discardLCD) Read(p []byte) (n int, err error)  { return 0, nil }
func (d *discardLCD) Write(p []byte) (n int, err error) { return len(p), nil }
func (d *discardLCD) Close() error                      { return nil }

func toggleDefaultUser() {
	if got, want := len(users), 2; got != want {
		log.Printf("unexpected number of users: got %d, want %d", got, want)
		return
	}
	for sub, state := range users {
		state.Default = !state.Default
		if err := writeDefault(sub, state.Default); err != nil {
			log.Println(err)
			return
		}
		users[sub] = state
	}
}

// LocalScanner waits for a locally connected Fujitsu ScanSnap iX500
// to appear, then starts scans whenever the scan button is triggered.
// Running in a goroutine.
func LocalScanner() {
	tr := trace.New("LocalScanner", "fss500")
	defer tr.Finish()

	func() {
		g, err := gpio.NewGPIO()
		if err != nil {
			return
		}
		ch := make(chan gpio.Keypress)
		const pin12 = 18 // BCM18
		if err := g.NotifyKeypresses(pin12, ch); err != nil {
			return
		}

		go func() {
			for range ch {
				log.Printf("Switch User key pressed")
				toggleDefaultUser()
			}
		}()
	}()

	d, err := serial_lcd.Open("/dev/ttyACM0", 115200)
	if err != nil {
		log.Printf("could not open lcd: %v", err)
		// TODO: remove once this PR got merged:
		// https://github.com/augustoroman/serial_lcd/pull/1
		d = serial_lcd.LCD{&discardLCD{}}
	}
	d.SetAutoscroll(false)
	setBacklightColor := func(red, green, blue byte) {
		d.SetBrightness(255)
		if os.Getenv("GOKRAZY_FIRST_START") != "1" {
			d.SetBG(255, 255, 0) // indicate a crash
			return
		}
		d.SetBG(red, green, blue)
	}
	client := proto.NewScanClient(scanConn)
	defaultUser := func() string {
		resp, err := client.DefaultUser(context.Background(), &proto.DefaultUserRequest{})
		if err != nil {
			return err.Error()
		}
		trunc := resp.GetFullName()
		if idx := strings.Index(trunc, " "); idx > -1 {
			trunc = trunc[:idx]
		}
		return trunc
	}
	showNotReady := func() {
		d.SetBrightness(0) // turn off backlight
		d.Clear()
		d.MoveTo(1, 1)
		fmt.Fprintf(d, "scan2"+strings.ToLower(defaultUser()))
		d.MoveTo(1, 2)
		fmt.Fprintf(d, "no scanner found")
		publishStatus("powersave") // magic string that turns off the display
	}
	showNotReady()

	for {
		dev, err := usb.FindDevice()
		if err != nil {
			showNotReady()
			tr.LazyPrintf("device not found: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		tr.LazyPrintf("device opened, waiting for scan button press")

		showReady := func() {
			setBacklightColor(0xFF, 0xFF, 0xFF)
			d.Clear()
			d.MoveTo(1, 1)
			fmt.Fprintf(d, "scan2"+strings.ToLower(defaultUser()))
			d.MoveTo(1, 2)
			fmt.Fprintf(d, "scanner ready")
			publishStatus("scanner ready")
		}

		var lastChange time.Time
		for {
			showReady()
			hwStatus, err := fss500.GetHardwareStatus(dev)
			if err != nil {
				tr.LazyPrintf("hardware status request failed: %v", err)
				break
			}
			select {
			case sr := <-mqttScanRequest:
				func() {
					tr := trace.New("MQTT", "ScanRequest")
					defer tr.Finish()

					user := userByFirstName(sr.User)
					if user == "" {
						publishStatus(fmt.Sprintf("No such user %q", sr.User))
						return
					}

					if sr.Source == "usb" {
						if err := scan(tr, user, dev, &d); err != nil {
							publishStatus(err.Error())
						}
					} else {
						if _, err := dispatch.Scan(user); err != nil {
							publishStatus(err.Error())
						}
					}
				}()
			default:
			}
			if hwStatus.ScanSw && time.Since(lastChange) > 5*time.Second {
				lastChange = time.Now()
				tr.LazyPrintf("scan button pressed, scanning")
				// recover from error backlight color if necessary
				setBacklightColor(0xFF, 0xFF, 0xFF)

				client := proto.NewScanClient(scanConn)
				resp, err := client.DefaultUser(context.Background(), &proto.DefaultUserRequest{})
				if err != nil {
					publishStatus(err.Error())
					continue
				}

				if err := scan(tr, resp.User, dev, &d); err != nil {
					tr.LazyPrintf("scanning failed: %v", err)
					setBacklightColor(0xFF, 0x00, 0x00)
					d.Clear()
					d.MoveTo(1, 1)
					fmt.Fprintf(d, "%v", err)
					publishStatus(err.Error())
				} else {
					showReady()
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
