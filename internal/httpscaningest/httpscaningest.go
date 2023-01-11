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

// Package httpscaningest implements an HTTP API around the scaningest API.
//
// # Example Usage
//
// You can use this API with curl on the command line like so:
//
//	jobid=$(curl -s -X CREATE http://localhost:7120/ingestjob | jq -r .job)
//	curl --request POST --data-binary "@internal/neonjpeg/testdata/page2.jpg" http://localhost:7120/job/$jobid/addpage
//	curl --request POST http://localhost:7120/job/$jobid/ingest
package httpscaningest

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/stapelberg/scan2drive/internal/httperr"
	"github.com/stapelberg/scan2drive/internal/page"
	"github.com/stapelberg/scan2drive/internal/scaningest"
)

// shiftPath from
// https://blog.merovius.de/2017/06/18/how-not-to-use-an-http-router.html:

// shiftPath splits off the first component of p, which will be cleaned of
// relative components before processing. head will never contain a slash and
// tail will always be a rooted path without trailing slash.
func shiftPath(p string) (head, tail string) {
	p = path.Clean("/" + p)
	i := strings.Index(p[1:], "/") + 1
	if i <= 0 {
		return p[1:], "/"
	}
	return p[1:i], p[i:]
}

type jobHandler struct {
	job *scaningest.Job
}

func (h *jobHandler) ServeHTTPError(w http.ResponseWriter, r *http.Request) error {
	var verb string
	verb, r.URL.Path = shiftPath(r.URL.Path)
	switch verb {
	case "addpage":
		if got := r.Method; got != "PUT" && got != "POST" {
			return httperr.Error(
				http.StatusMethodNotAllowed,
				fmt.Errorf("unexpected HTTP method: got %v, want PUT or POST", got))
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		return h.job.AddPage(page.JPEGPageFromBytes(b))

	case "ingest":
		jobId, err := h.job.Ingest()
		_ = jobId
		return err
	}
	return httperr.Error(
		http.StatusNotFound,
		fmt.Errorf("verb %q not found", verb))
}

func ServeMux(ingester *scaningest.Ingester) *http.ServeMux {
	var (
		// TODO: switch to an LRU cache so that we can bound the number of
		// concurrent requests and turn a runaway job request loop into a
		// non-event.
		currentJobsMu sync.Mutex
		currentJobs   = make(map[string]*scaningest.Job)
	)
	getJob := func(jobId string) *scaningest.Job {
		currentJobsMu.Lock()
		defer currentJobsMu.Unlock()
		return currentJobs[jobId]
	}
	serveMux := http.NewServeMux()

	serveMux.Handle("/ingestjob", httperr.Handle(func(w http.ResponseWriter, r *http.Request) error {
		if got, want := r.Method, "CREATE"; got != want {
			return httperr.Error(
				http.StatusMethodNotAllowed,
				fmt.Errorf("unexpected HTTP method: got %v, want %v", got, want))
		}

		job, err := ingester.NewJob()
		if err != nil {
			return err
		}

		jobId := uuid.NewString()

		currentJobsMu.Lock()
		defer currentJobsMu.Unlock()
		currentJobs[jobId] = job
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"job":"%s"}`, jobId)
		return nil
	}))

	serveMux.Handle("/job/", httperr.Handle(func(w http.ResponseWriter, r *http.Request) error {
		var jobId string
		jobId, r.URL.Path = shiftPath(strings.TrimPrefix(r.URL.Path, "/job/"))
		job := getJob(jobId)
		if job == nil {
			return httperr.Error(
				http.StatusNotFound,
				fmt.Errorf("job not found"))
		}
		hdl := jobHandler{job: job}
		httpHdl := httperr.Handle(hdl.ServeHTTPError)
		httpHdl.ServeHTTP(w, r)
		return nil
	}))

	return serveMux
}
