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
	"log"

	"github.com/brutella/dnssd"
	"github.com/stapelberg/scan2drive/internal/dispatch"
	"golang.org/x/net/context"
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
	log.Printf("trigger an airscan at host %q", p.host)
	// TODO: trigger an airscan
	return "", fmt.Errorf("not yet implemented")
}

func Airscan() {
	log.Printf("Searching for AirScan scanners via DNSSD")

	actionByHost := make(map[string]*pushToAirscan)

	addFn := func(srv dnssd.Service) {
		log.Printf("add: %v", srv)

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
