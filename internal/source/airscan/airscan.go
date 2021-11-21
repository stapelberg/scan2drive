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

// Package airscan implements a scan source from AirScan devices discovered on
// the local network.
package airscan

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brutella/dnssd"
	"github.com/stapelberg/airscan"
	"github.com/stapelberg/airscan/preset"
	"github.com/stapelberg/scan2drive"
	"github.com/stapelberg/scan2drive/internal/mayqtt"
	"github.com/stapelberg/scan2drive/internal/page"
	"github.com/stapelberg/scan2drive/internal/scaningest"
	"golang.org/x/net/context"
	"golang.org/x/net/trace"

	// We request a jpeg image from the scanner, so make sure the image package
	// can decode jpeg images in later processing stages.
	_ "image/jpeg"
)

type AirscanSource struct {
	mu      sync.Mutex
	client  *airscan.Client
	name    string
	host    string
	iconURL string
}

func (a *AirscanSource) Transport() http.RoundTripper {
	return a.client.HTTPClient.(*http.Client).Transport
}

// implements scan2drive.ScanSource
func (a *AirscanSource) Metadata() scan2drive.ScanSourceMetadata {
	return scan2drive.ScanSourceMetadata{
		Id:      "airscan!" + a.host,
		Name:    a.name,
		IconURL: a.iconURL,
	}
}

// implements scan2drive.ScanSource
func (a *AirscanSource) CanProcess(r *scan2drive.ScanRequest) error {
	if r.Source != "airscan" {
		return fmt.Errorf("requested source is not airscan")
	}
	return nil // any!
}

// implements scan2drive.ScanSource
func (a *AirscanSource) ScanTo(ingester *scaningest.Ingester) (string, error) {
	tr := trace.New("AirScan", "ScanTo")
	defer tr.Finish()
	start := time.Now()
	defer func() {
		tr.LazyPrintf("scan done in %v", time.Since(start))
	}()
	mayqtt.Publishf("scanning...")

	a.mu.Lock()
	defer a.mu.Unlock()
	return scan1(tr, a.client, ingester)
}

// NOTE: We currently call out to the generic ProcessScan() implementation,
// whereas the Fujitsu ScanSnap iX500 (fss500) specific implementation did a
// lot of the work itself, resulting in significant speed-ups on the
// Raspberry Pi 3.
//
// We should investigate whether similar speed-ups are required/achievable
// with the Raspberry Pi 4 and AirScan.

func scan1(tr trace.Trace, cl *airscan.Client, ingester *scaningest.Ingester) (string, error) {
	status, err := cl.ScannerStatus()
	if err != nil {
		return "", err
	}
	tr.LazyPrintf("status: %+v", status)
	if got, want := status.State, "Idle"; got != want {
		return "", fmt.Errorf("scanner not ready: in state %q, want %q", got, want)
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
		return "", err
	}
	defer func() {
		tr.LazyPrintf("Deleting ScanJob %s", scan)
		if err := scan.Close(); err != nil {
			log.Printf("error deleting AirScan job (probably harmless): %v", err)
		}
	}()
	tr.LazyPrintf("ScanJob created: %s", scan)

	ingestJob, err := ingester.NewJob()
	if err != nil {
		return "", err
	}

	for scan.ScanPage() {
		b, err := io.ReadAll(scan.CurrentPage())
		if err != nil {
			return "", err
		}
		if err := ingestJob.AddPage(page.JPEGPageFromBytes(b)); err != nil {
			return "", err
		}
	}
	if err := scan.Err(); err != nil {
		return "", err
	}

	return ingestJob.Ingest()
}

type AirscanSourceFinder struct {
	mu      sync.Mutex
	sources map[string]*AirscanSource
}

// implements scan2drive.ScanSourceFinder
func (a *AirscanSourceFinder) CurrentScanSources() []scan2drive.ScanSource {
	a.mu.Lock()
	defer a.mu.Unlock()
	sources := make([]scan2drive.ScanSource, 0, len(a.sources))
	for _, source := range a.sources {
		sources = append(sources, source)
	}
	return sources
}

func SourceFinder() scan2drive.ScanSourceFinder {
	tr := trace.New("AirScan", "DNSSD")

	log.Printf("Searching for AirScan scanners via DNSSD")
	tr.LazyPrintf("Searching for AirScan scanners via DNSSD")
	sourceFinder := &AirscanSourceFinder{
		sources: make(map[string]*AirscanSource),
	}

	addFn := func(srv dnssd.Service) {
		tr.LazyPrintf("DNSSD service discovered: %v", srv)

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

		sourceFinder.mu.Lock()
		defer sourceFinder.mu.Unlock()
		// TODO: use srv.Text["ty"] if non-empty once
		// https://github.com/brutella/dnssd/pull/19 was merged
		sourceFinder.sources[srv.Host] = &AirscanSource{
			client:  airscan.NewClientForService(&srv),
			name:    unescapedName,
			host:    srv.Host,
			iconURL: srv.Text["representation"],
		}
	}
	rmvFn := func(srv dnssd.Service) {
		tr.LazyPrintf("DNSSD service disappeared: %v", srv)
		sourceFinder.mu.Lock()
		defer sourceFinder.mu.Unlock()
		delete(sourceFinder.sources, srv.Host)
	}
	go func() {
		defer tr.Finish()
		const service = "_uscan._tcp.local." // AirScan DNSSD service name
		// addFn and rmvFn are always called (sequentially) from the same goroutine,
		// i.e. no locking is required.
		if err := dnssd.LookupType(context.Background(), service, addFn, rmvFn); err != nil {
			log.Printf("DNSSD init failed: %v", err)
			return
		}
	}()
	return sourceFinder
}
