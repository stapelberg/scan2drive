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

package airscan

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type ScannerStatus struct {
	Version  string `xml:"Version"`
	State    string `xml:"State"`
	ADFState string `xml:"AdfState"`
}

type Client struct {
	host string
}

func (c *Client) ScannerStatus() (*ScannerStatus, error) {
	req, err := http.NewRequest("GET", "http://"+c.host+"/eSCL/ScannerStatus", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		b, _ := ioutil.ReadAll(resp.Body)
		// TODO: only include b if it consists entirely of printable runes!
		return nil, fmt.Errorf("unexpected HTTP status: got %v (%s), want %v", resp.Status, strings.TrimSpace(string(b)), want)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var status ScannerStatus
	if err := xml.Unmarshal(b, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) CreateScanJob(settings string) (*url.URL, error) {
	req, err := http.NewRequest("POST", "http://"+c.host+"/eSCL/ScanJobs", strings.NewReader(settings))
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if got, want := resp.StatusCode, http.StatusCreated; got != want {
		b, _ := ioutil.ReadAll(resp.Body)
		// TODO: only include b if it consists entirely of printable runes!
		return nil, fmt.Errorf("unexpected HTTP status: got %v (%s), want %v", resp.Status, strings.TrimSpace(string(b)), want)
	}
	loc, err := resp.Location()
	if err != nil {
		return nil, err
	}
	return loc, nil
}

func (c *Client) DeleteScanJob(loc *url.URL) error {
	req, err := http.NewRequest("DELETE", loc.String(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound {
		b, _ := ioutil.ReadAll(resp.Body)
		// TODO: only include b if it consists entirely of printable runes!
		return fmt.Errorf("unexpected HTTP status: %v (%s)", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

func NewClient(host string) *Client {
	return &Client{host: host}
}
