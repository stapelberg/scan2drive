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

// Package scan2drive contains domain types for scan2drive, like scan sources or
// scan requests.
package scan2drive

import (
	"github.com/stapelberg/scan2drive/internal/scaningest"
)

// A ScanRequest is received via MQTT, HTTP or gRPC.
type ScanRequest struct {
	User     string `json:"user"`
	Source   string `json:"source"`
	SourceId string `json:"source_id"`
}

type DriveFolder struct {
	Id      string
	IconUrl string
	Url     string
	Name    string
}

type ScanSourceFinder interface {
	CurrentScanSources() []ScanSource
}

// A ScanSource is a specific device, e.g. a Brother printer/scanner that was
// discovered via AirScan.
type ScanSource interface {
	Metadata() ScanSourceMetadata

	// CanProcess returns nil if this source can process the specified scan
	// request, or an error describing what prevents the source from
	// processing.
	CanProcess(*ScanRequest) error

	// the ingester selects the user by virtue of selecting the user’s job
	// queue in the ingestcallback.
	// returns the job id
	ScanTo(*scaningest.Ingester) (string, error)
}

type ScanSourceMetadata struct {
	Id      string
	Name    string
	IconURL string
}
