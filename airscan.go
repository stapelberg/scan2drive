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

	"github.com/stapelberg/scan2drive/internal/dispatch"
)

type pushToAirscan struct {
	target string
}

func (p *pushToAirscan) TargetName() string {
	if p.target != "" {
		return p.target
	}
	return "AirScan: unknown target"
}

func (p *pushToAirscan) Scan(user string) (string, error) {
	log.Printf("trigger an airscan")
	// TODO: trigger an airscan
	return "", fmt.Errorf("not yet implemented")
}

// TODO: periodically locate all available scanners in the network using DNSSD
// (can we subscribe to changes, are they broadcasted?)
func Airscan() {
	// Register the default scan action (physical hardware button, or web
	// interface scan button) to be scanning from this scanner via AirScan:
	dispatch.Register(&pushToAirscan{})

	// when the physical button is pressed,
}
