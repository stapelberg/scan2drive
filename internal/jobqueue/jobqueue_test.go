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

package jobqueue_test

import (
	"testing"

	"github.com/stapelberg/scan2drive/internal/jobqueue"
	"github.com/stapelberg/scan2drive/internal/page"
)

func TestJobQueue(t *testing.T) {
	dir := t.TempDir()
	jobId := func() string {
		defaultQueue := &jobqueue.Queue{
			Dir: dir,
		}

		b := []byte("hello world")
		job, err := defaultQueue.AddJob([]*page.Any{page.JPEGPageFromBytes(b)})
		if err != nil {
			t.Fatal(err)
		}
		return job.Id()
	}()
	freshQueue := &jobqueue.Queue{
		Dir: dir,
	}
	job, err := freshQueue.JobById(jobId)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := job.State(), jobqueue.InProgress; got != want {
		t.Fatalf("unexpected job state: got %v, want %v", got, want)
	}
	if got, want := len(job.Pages()), 1; got != want {
		t.Fatalf("unexpected number of pages: got %v, want %v", got, want)
	}
}
