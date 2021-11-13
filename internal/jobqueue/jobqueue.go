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

// Package jobqueue implements a reliable job queue that is persisted to the
// file system.
package jobqueue

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/stapelberg/scan2drive/internal/page"
)

type Queue struct {
	Dir string
}

type State int

func (s State) String() string {
	switch s {
	case Canceled:
		return "Canceled"
	case InProgress:
		return "InProgress"
	case Done:
		return "Done"
	default:
		return "<unknown>"
	}
}

const (
	Canceled State = iota
	InProgress
	Done
)

type CompletionMarkers struct {
	UploadedOriginals bool
	Converted         bool
	UploadedPDF       bool
	Renamed           bool
}

type Job struct {
	id         string
	dir        string
	state      State
	curpage    int
	pages      []*page.Any
	Markers    CompletionMarkers
	NewName    string
	PDFDriveId string
}

func (q *Queue) AddJob(pages []*page.Any) (*Job, error) {
	id := time.Now().Format(time.RFC3339)
	dir := filepath.Join(q.Dir, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	job := &Job{id: id, dir: dir}
	for _, page := range pages {
		if err := job.addPage(page); err != nil {
			return nil, err
		}
	}
	if err := job.commit(); err != nil {
		return nil, err
	}

	return job, nil
}

func (q *Queue) Scans() (map[string]*Job, error) {
	entries, err := ioutil.ReadDir(q.Dir)
	if err != nil {
		return nil, err
	}
	jobs := make(map[string]*Job)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		job, err := q.JobById(entry.Name())
		if err != nil {
			return nil, err
		}
		jobs[job.Id()] = job
	}
	return jobs, nil
}

func (q *Queue) JobById(id string) (*Job, error) {
	dir := filepath.Join(q.Dir, id)
	job := &Job{
		id:  id,
		dir: dir,
	}
	if err := job.readStateFromDir(); err != nil {
		return nil, err
	}

	if job.Markers.UploadedPDF {
		job.state = Done
	}

	// load pages back into memory if the job is still in progress
	if job.state == InProgress {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if filepath.Ext(file.Name()) != ".jpg" {
				continue
			}
			if file.Size() == 0 {
				continue
			}
			b, err := ioutil.ReadFile(filepath.Join(dir, file.Name()))
			if err != nil {
				return nil, err
			}
			job.pages = append(job.pages, page.JPEGPageFromBytes(b))
		}
	}
	return job, nil
}

func (j *Job) readStateFromDir() error {
	entries, err := ioutil.ReadDir(j.dir)
	if err != nil {
		return err
	}
	j.state = Canceled // zero value
	for _, entry := range entries {
		if entry.Name() == "COMPLETE.scan" {
			j.state = InProgress
		} else if entry.Name() == "COMPLETE.convert" {
			j.Markers.Converted = true
		} else if entry.Name() == "COMPLETE.uploadoriginals" {
			j.Markers.UploadedOriginals = true
		} else if entry.Name() == "COMPLETE.uploadpdf" {
			j.Markers.UploadedPDF = true
		} else if entry.Name() == "COMPLETE.rename" {
			j.Markers.Renamed = true
		} else if entry.Name() == "rename" {
			content, err := os.ReadFile(filepath.Join(j.dir, "rename"))
			if err != nil {
				return err
			}
			j.NewName = string(content)
		} else if entry.Name() == "pdf.drive_id" {
			content, err := os.ReadFile(filepath.Join(j.dir, "pdf.drive_id"))
			if err != nil {
				return err
			}
			j.PDFDriveId = string(content)
		}
	}
	return nil
}

func (j *Job) Id() string {
	return j.id
}

func (j *Job) State() State {
	return j.state
}

func (j *Job) Pages() []*page.Any {
	return j.pages
}

func (j *Job) addPage(page *page.Any) error {
	j.curpage++
	fn := filepath.Join(j.dir, fmt.Sprintf("page%d.jpg", j.curpage))
	b, err := page.JPEGBytes()
	if err != nil {
		return err
	}
	if err := os.WriteFile(fn, b, 0600); err != nil {
		return err
	}
	j.pages = append(j.pages, page)
	return nil
}

func (j *Job) Filenames() ([]string, error) {
	entries, err := ioutil.ReadDir(j.dir)
	if err != nil {
		return nil, err
	}
	filenames := make([]string, 0, len(entries))
	for _, entry := range entries {
		filenames = append(filenames, filepath.Join(j.dir, entry.Name()))
	}
	return filenames, nil
}

func (j *Job) AddDerivedFile(name string, contents []byte) error {
	fn := filepath.Join(j.dir, name)
	if err := os.WriteFile(fn, contents, 0600); err != nil {
		return err
	}
	return nil
}

func (j *Job) CommitMarker(name string) error {
	if err := os.WriteFile(filepath.Join(j.dir, "COMPLETE."+name), nil, 0600); err != nil {
		return err
	}
	return j.readStateFromDir()
}

func (j *Job) commit() error {
	if err := j.CommitMarker("scan"); err != nil {
		return err
	}

	j.state = InProgress
	return nil
}

func (j *Job) WritePDFDriveID(driveId string) error {
	fn := filepath.Join(j.dir, "pdf.drive_id")
	if err := os.WriteFile(fn, []byte(driveId), 0600); err != nil {
		return err
	}
	return j.readStateFromDir()
}
