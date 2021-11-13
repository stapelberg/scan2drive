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

// Package scaningest implements an API for ingesting scan jobs, to be used with
// a jobqueue.
package scaningest

import "github.com/stapelberg/scan2drive/internal/page"

type Ingester struct {
	IngestCallback func(*Job) (string, error)
}

type Job struct {
	ingester *Ingester
	Pages    []*page.Any
}

func (i *Ingester) NewJob() (*Job, error) {
	return &Job{ingester: i}, nil
}

func (j *Job) AddPage(page *page.Any) error {
	// NOTE: For large jobs, we’d need to spill pages to disk to not exhaust
	// memory. For now, we just assume small-enough jobs, and/or enough RAM.
	j.Pages = append(j.Pages, page)
	return nil
}

func (j *Job) ReversePages() {
	var reversed []*page.Any
	for i := len(j.Pages) - 1; i >= 0; i-- {
		reversed = append(reversed, j.Pages[i])
	}
	j.Pages = reversed
}

func (j *Job) Ingest() (string, error) {
	return j.ingester.IngestCallback(j)
}
