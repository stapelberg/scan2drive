// Package httpscaningest implements an HTTP API around the scaningest API.
//
// Example Usage
//
// You can use this API with curl on the command line like so:
//
//   jobid=$(curl -s -X CREATE http://localhost:7120/ingestjob | jq -r .job)
//   curl --request POST --data-binary "@internal/neonjpeg/testdata/page2.jpg" http://localhost:7120/job/$jobid/addpage
//   curl --request POST http://localhost:7120/job/$jobid/ingest
package httpscaningest

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/stapelberg/scan2drive/internal/scaningest"
)

type httpErr struct {
	code int
	err  error
}

func (h *httpErr) Error() string {
	return h.err.Error()
}

func httpError(code int, err error) error {
	return &httpErr{code, err}
}

func handleError(h func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: r.URL.Path is already processed when we do a nested call to
		// handleError() for the job handler.
		path := r.URL.Path // will be modified during request processing
		err := h(w, r)
		if err == nil {
			return
		}
		if err == context.Canceled {
			return // client canceled the request
		}
		code := http.StatusInternalServerError
		unwrapped := err
		if he, ok := err.(*httpErr); ok {
			code = he.code
			unwrapped = he.err
		}
		log.Printf("%s: HTTP %d %s", path, code, unwrapped)
		http.Error(w, unwrapped.Error(), code)
	})
}

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
			return httpError(
				http.StatusMethodNotAllowed,
				fmt.Errorf("unexpected HTTP method: got %v, want PUT or POST", got))
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		return h.job.AddPage(b)

	case "ingest":
		return h.job.Ingest()
	}
	return httpError(
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

	serveMux.Handle("/ingestjob", handleError(func(w http.ResponseWriter, r *http.Request) error {
		if got, want := r.Method, "CREATE"; got != want {
			return httpError(
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

	serveMux.Handle("/job/", handleError(func(w http.ResponseWriter, r *http.Request) error {
		var jobId string
		jobId, r.URL.Path = shiftPath(strings.TrimPrefix(r.URL.Path, "/job/"))
		job := getJob(jobId)
		if job == nil {
			return httpError(
				http.StatusNotFound,
				fmt.Errorf("job not found"))
		}
		hdl := jobHandler{job: job}
		httpHdl := handleError(hdl.ServeHTTPError)
		httpHdl.ServeHTTP(w, r)
		return nil
	}))

	return serveMux
}
