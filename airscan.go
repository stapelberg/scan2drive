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
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brutella/dnssd"
	"github.com/stapelberg/scan2drive/internal/airscan"
	"github.com/stapelberg/scan2drive/internal/atomic/write"
	"github.com/stapelberg/scan2drive/internal/dispatch"
	"github.com/stapelberg/scan2drive/proto"
	"golang.org/x/net/context"
	"golang.org/x/net/trace"

	// We request a jpeg image from the scanner, so make sure the image package
	// can decode jpeg images in later processing stages.
	_ "image/jpeg"
)

type pushToAirscan struct {
	name    string
	host    string
	iconURL string
}

func (p *pushToAirscan) Display() dispatch.Display {
	name := p.name
	if name == "" {
		name = "AirScan: unknown target"
	}
	return dispatch.Display{
		Name:    name,
		IconURL: p.iconURL,
	}
}

func (p *pushToAirscan) Scan(user string) (string, error) {
	tr := trace.New("AirScan", p.host)
	defer tr.Finish()
	tr.LazyPrintf("Starting AirScan at host %s.local", p.host)
	return scanA(tr, p.host)
}

func Airscan() {
	tr := trace.New("AirScan", "DNSSD")
	defer tr.Finish()

	log.Printf("Searching for AirScan scanners via DNSSD")

	actionByHost := make(map[string]*pushToAirscan)

	addFn := func(srv dnssd.Service) {
		log.Printf("DNSSD service discovered: %v", srv)

		// 2020/07/16 13:54:34 add: {
		// Name: Brother\ MFC-L2750DW\ series
		// Type: _uscan._tcp
		// Domain: local
		// Host: BRN3C2xxxxxxxxx
		// Text: map[
		//   UUID:yyyyyyyy-xxxx-zzzz-aaaa-bbbbbbbbbbbb
		//   adminurl:http://BRN3C2xxxxxxxxx.local./net/net/airprint.html
		//   cs:binary,grayscale,color
		//   duplex:T
		//   is:adf,platen
		//   note:
		//   pdl:application/pdf,image/jpeg
		//   representation:http://BRN3C2xxxxxxxxx.local./icons/device-icons-128.png
		//   rs:eSCL
		//   txtvers:1
		//   ty:Brother
		//   vers:2.63]
		// TTL: 1h15m0s
		// Port: 80
		// IPs: [10.0.0.12 fe80::3e2a:aaaa:bbbb:cccc]
		// IfaceIPs:map[]

		// TODO: why is the name escaped? i see backslashes in the UI

		if action, ok := actionByHost[srv.Host]; !ok {
			action = &pushToAirscan{
				name:    srv.Name,
				host:    srv.Host,
				iconURL: srv.Text["representation"],
			}
			actionByHost[srv.Host] = action
			// Register the default scan action (physical hardware button, or web
			// interface scan button) to be scanning from this scanner via AirScan:
			dispatch.Register(action)
		}
	}
	rmvFn := func(srv dnssd.Service) {
		log.Printf("remove: %v", srv)
		if action, ok := actionByHost[srv.Host]; ok {
			dispatch.Unregister(action)
			delete(actionByHost, srv.Host)
		}
	}
	// TODO: where is _uscan._tcp canonically defined?
	const service = "_uscan._tcp.local."
	// TODO: are the functions always called in the same goroutine?
	if err := dnssd.LookupType(context.Background(), service, addFn, rmvFn); err != nil {
		log.Printf("DNSSD init failed: %v", err)
		return
	}

}

func scanA(tr trace.Trace, host string) (string, error) {
	start := time.Now()
	client := proto.NewScanClient(scanConn)
	resp, err := client.DefaultUser(context.Background(), &proto.DefaultUserRequest{})
	if err != nil {
		return "", err
	}

	relName := time.Now().Format(time.RFC3339)
	scanDir := filepath.Join(*scansDir, resp.User, relName)

	if err := os.MkdirAll(scanDir, 0700); err != nil {
		return "", err
	}

	if err := scan1(tr, host, scanDir, relName, resp.User); err != nil {
		return "", err
	}

	tr.LazyPrintf("scan done in %v", time.Since(start))

	// NOTE: We currently call out to the generic ProcessScan() implementation,
	// whereas the Fujitsu ScanSnap iX500 (fss500) specific implementation did a
	// lot of the work itself, resulting in significant speed-ups on the
	// Raspberry Pi 3.
	//
	// We should investigate whether similar speed-ups are required/achievable
	// with the Raspberry Pi 4 and AirScan.

	tr.LazyPrintf("processing scan")
	if _, err := client.ProcessScan(context.Background(), &proto.ProcessScanRequest{User: resp.User, Dir: relName}); err != nil {
		return "", err
	}
	tr.LazyPrintf("scan processed")

	return relName, nil
}

func scan1(tr trace.Trace, host, scanDir, relName, respUser string) error {
	cl := airscan.NewClient(host)

	status, err := cl.ScannerStatus()
	if err != nil {
		return err
	}
	tr.LazyPrintf("status: %+v", status)
	if got, want := status.State, "Idle"; got != want {
		return fmt.Errorf("scanner not ready: in state %q, want %q", got, want)
	}
	if status.ADFState != "" {
		if got, want := status.ADFState, "ScannerAdfLoaded"; got != want {
			return fmt.Errorf("scanner feeder contains no documents: status %q, want %q", got, want)
		}
	}

	// NOTE: With the Fujitsu ScanSnap iX500 (fss500), we always scanned 600
	// dpi color full-duplex and retained the highest-quality
	// originals. Other scanners (e.g. the Brother MFC-L2750DW) take A LOT
	// longer than the fss500 for scanning duplex color originals, so with
	// AirScan, we play it safe and only request what we really need:
	// grayscale at 300 dpi.

	// Same contents (aside from whitespace differences) as Apple’s
	// AirScanScanner/41 (Image Preview app on Mac OS X 10.11):
	// A4 at 300 dpi is 2480 x 3508 as per https://www.papersizes.org/a-sizes-in-pixels.htm
	const settings = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<scan:ScanSettings xmlns:scan="http://schemas.hp.com/imaging/escl/2011/05/03" xmlns:pwg="http://www.pwg.org/schemas/2010/12/sm">
  <pwg:Version>2.0</pwg:Version>
  <pwg:ScanRegions pwg:MustHonor="true">
    <pwg:ScanRegion>
      <pwg:ContentRegionUnits>escl:ThreeHundredthsOfInches</pwg:ContentRegionUnits>
      <pwg:Width>2480</pwg:Width>
      <pwg:Height>3508</pwg:Height>
      <pwg:XOffset>0</pwg:XOffset>
      <pwg:YOffset>0</pwg:YOffset>
    </pwg:ScanRegion>
  </pwg:ScanRegions>
  <pwg:DocumentFormat>image/jpeg</pwg:DocumentFormat>
  <pwg:InputSource>Feeder</pwg:InputSource>
  <scan:ColorMode>Grayscale8</scan:ColorMode>
  <scan:XResolution>300</scan:XResolution>
  <scan:YResolution>300</scan:YResolution>
  <scan:Duplex>true</scan:Duplex>
</scan:ScanSettings>`
	loc, err := cl.CreateScanJob(settings)
	if err != nil {
		return err
	}
	defer func() {
		tr.LazyPrintf("Deleting ScanJob %s", loc)
		if err := cl.DeleteScanJob(loc); err != nil {
			log.Printf("error deleting AirScan job (probably harmless): %v", err)
		}
	}()
	tr.LazyPrintf("ScanJob created: %s", loc)

	for pagenum := 1; ; pagenum++ {
		// TODO: correct URL path manipulation
		resp, err := http.Get(loc.String() + "/NextDocument")
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusNotFound {
			tr.LazyPrintf("NotFound: all pages received")
			break // all pages received
		}
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			b, _ := ioutil.ReadAll(resp.Body)
			// TODO: only include b if it is text!
			return fmt.Errorf("unexpected HTTP status: got %v (%s), want %v", resp.Status, strings.TrimSpace(string(b)), want)
		}

		tr.LazyPrintf("receiving page %d", pagenum)
		fn := filepath.Join(scanDir, fmt.Sprintf("page%d.jpg", pagenum))
		o, err := write.TempFile(fn)
		if err != nil {
			return err
		}
		defer o.Cleanup()

		if _, err := io.Copy(o, resp.Body); err != nil {
			return err
		}

		if err := o.CloseAtomicallyReplace(); err != nil {
			return err
		}
	}

	createCompleteMarker(respUser, relName, "scan")

	return nil
}
