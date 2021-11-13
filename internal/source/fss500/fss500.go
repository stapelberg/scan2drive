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

// Package fss500 implements a scan source for a Fujitsu ScanSnap iX500 document
// scanner connected via USB.
package fss500

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stapelberg/scan2drive"
	"github.com/stapelberg/scan2drive/internal/fss500"
	"github.com/stapelberg/scan2drive/internal/fss500/usb"
	"github.com/stapelberg/scan2drive/internal/mayqtt"
	"github.com/stapelberg/scan2drive/internal/scaningest"
	"golang.org/x/net/trace"
)

type lockedDev struct {
	mu  sync.Mutex
	dev *usb.Device
}

type FSS500SourceFinder struct {
	mu     sync.Mutex
	source scan2drive.ScanSource
}

// implements scan2drive.ScanSourceFinder
func (f *FSS500SourceFinder) CurrentScanSources() []scan2drive.ScanSource {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.source != nil {
		return []scan2drive.ScanSource{f.source}
	}
	return nil
}

func (f *FSS500SourceFinder) findDevice(tr trace.Trace, ingesterForDefault func() *scaningest.Ingester) error {
	var l lockedDev
	{
		dev, err := usb.FindDevice()
		if err != nil {
			mayqtt.Publishf("powersave")
			return fmt.Errorf("device not found: %v", err)
		}
		l.dev = dev
	}

	// Make the source finder return a source for this device:
	f.mu.Lock()
	source := &FSS500Source{dev: &l}
	f.source = source
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.source = nil
		f.mu.Unlock()
	}()

	tr.LazyPrintf("device opened, waiting for scan button press")
	mayqtt.Publishf("scanner ready")
	var lastChange time.Time
	for {
		// showReady()
		l.mu.Lock()
		hwStatus, err := fss500.GetHardwareStatus(l.dev)
		l.mu.Unlock()
		if err != nil {
			tr.LazyPrintf("hardware status request failed: %v", err)
			break
		}
		if hwStatus.ScanSw && time.Since(lastChange) > 5*time.Second {
			lastChange = time.Now()
			tr.LazyPrintf("scan button pressed, scanning")

			ingester := ingesterForDefault()
			jobId, err := source.ScanTo(ingester)
			if err != nil {
				tr.LazyPrintf("scan failed: %v", err)
			} else {
				tr.LazyPrintf("scan completed: %s", jobId)
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

	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.dev.Close(); err != nil {
		tr.LazyPrintf("device close failed: %v", err)
	}
	l.dev = nil

	return nil
}

func SourceFinder(ingesterForDefault func() *scaningest.Ingester) scan2drive.ScanSourceFinder {
	tr := trace.New("FSS500", "USB")

	log.Printf("Searching for FSS500 scanners via USB")
	tr.LazyPrintf("Searching for FSS500 scanners via USB")
	sourceFinder := &FSS500SourceFinder{}
	go func() {
		defer tr.Finish()
		for {
			if err := sourceFinder.findDevice(tr, ingesterForDefault); err != nil {
				tr.LazyPrintf("device not found: %v", err)
				time.Sleep(1 * time.Second)
			}
		}
	}()
	return sourceFinder
}

type FSS500Source struct {
	dev *lockedDev
}

// implements scan2drive.ScanSource
func (f *FSS500Source) Metadata() scan2drive.ScanSourceMetadata {
	return scan2drive.ScanSourceMetadata{
		Id:      "fss500!usb",
		Name:    "Fujitsu ScanSnap 500",
		IconURL: "/assets/scannerprinter.svg",
	}
}

// implements scan2drive.ScanSource
func (f *FSS500Source) CanProcess(r *scan2drive.ScanRequest) error {
	if r.Source != "usb" {
		return fmt.Errorf("requested source is not usb")
	}
	return nil // any!
}

// implements scan2drive.ScanSource
func (f *FSS500Source) ScanTo(ingester *scaningest.Ingester) (string, error) {
	tr := trace.New("FSS500", "ScanTo")
	defer tr.Finish()
	start := time.Now()
	defer func() {
		tr.LazyPrintf("scan done in %v", time.Since(start))
	}()
	mayqtt.Publishf("scanning...")

	f.dev.mu.Lock()
	defer f.dev.mu.Unlock()
	if f.dev.dev == nil {
		return "", fmt.Errorf("scanner disappeared")
	}
	return scan(tr, ingester, f.dev.dev)
}
