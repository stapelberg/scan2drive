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
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brutella/dnssd"
	"github.com/google/renameio"
	"github.com/stapelberg/airscan"
	"github.com/stapelberg/airscan/preset"
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

	client := proto.NewScanClient(scanConn)
	resp, err := client.DefaultUser(context.Background(), &proto.DefaultUserRequest{})
	if err != nil {
		return "", err
	}

	tr.LazyPrintf("Starting AirScan at host %s.local for user %s", p.host, resp.User)

	return scanA(tr, resp.User, p.host)
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

		// miekg/dns escapes characters in DNS labels, which as per RFC1034 and
		// RFC1035 does not actually permit whitespace. The purpose of escaping
		// originally appears to be to use these labels in a DNS master file,
		// but for our UI, the backslashes look just wrong.
		unescapedName := strings.ReplaceAll(srv.Name, "\\", "")

		// TODO: use srv.Text["ty"] if non-empty once
		// https://github.com/brutella/dnssd/pull/19 was merged

		if action, ok := actionByHost[srv.Host]; !ok {
			action = &pushToAirscan{
				name:    unescapedName,
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
	const service = "_uscan._tcp.local." // AirScan DNSSD service name
	// addFn and rmvFn are always called (sequentially) from the same goroutine,
	// i.e. no locking is required.
	if err := dnssd.LookupType(context.Background(), service, addFn, rmvFn); err != nil {
		log.Printf("DNSSD init failed: %v", err)
		return
	}

}

func scanA(tr trace.Trace, user, host string) (string, error) {
	start := time.Now()

	relName := time.Now().Format(time.RFC3339)
	scanDir := filepath.Join(*scansDir, user, relName)

	if err := os.MkdirAll(scanDir, 0700); err != nil {
		return "", err
	}

	if err := scan1(tr, host, scanDir, relName, user); err != nil {
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
	client := proto.NewScanClient(scanConn)
	if _, err := client.ProcessScan(context.Background(), &proto.ProcessScanRequest{User: user, Dir: relName}); err != nil {
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

	// NOTE: With the Fujitsu ScanSnap iX500 (fss500), we always scanned 600
	// dpi color full-duplex and retained the highest-quality
	// originals. Other scanners (e.g. the Brother MFC-L2750DW) take A LOT
	// longer than the fss500 for scanning duplex color originals, so with
	// AirScan, we play it safe and only request what we really need:
	// grayscale at 300 dpi.

	settings := preset.GrayscaleA4ADF()
	// For the ADF, the ScanSnap is better.
	// We use the Brother for its flatbed scan only.
	settings.InputSource = "Platen"
	scan, err := cl.Scan(settings)
	if err != nil {
		return err
	}
	defer func() {
		tr.LazyPrintf("Deleting ScanJob %s", scan)
		if err := scan.Close(); err != nil {
			log.Printf("error deleting AirScan job (probably harmless): %v", err)
		}
	}()
	tr.LazyPrintf("ScanJob created: %s", scan)

	pagenum := 1
	for scan.ScanPage() {
		fn := filepath.Join(scanDir, fmt.Sprintf("page%d.jpg", pagenum))

		o, err := renameio.TempFile("", fn)
		if err != nil {
			return err
		}
		defer o.Cleanup()

		if _, err := io.Copy(o, scan.CurrentPage()); err != nil {
			return err
		}

		if err := o.CloseAtomicallyReplace(); err != nil {
			return err
		}

		pagenum++
	}
	if err := scan.Err(); err != nil {
		return err
	}

	createCompleteMarker(respUser, relName, "scan")

	return nil
}
