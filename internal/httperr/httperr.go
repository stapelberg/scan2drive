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

// Package httperr implements middleware which serves returned errors as HTTP
// internal server errors.
package httperr

import (
	"context"
	"log"
	"net/http"
)

type Err struct {
	Code int
	Err  error
}

func (h *Err) Error() string {
	return h.Err.Error()
}

func Error(code int, err error) error {
	return &Err{code, err}
}

func Handle(h func(http.ResponseWriter, *http.Request) error) http.Handler {
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
		if he, ok := err.(*Err); ok {
			code = he.Code
			unwrapped = he.Err
		}
		log.Printf("%s: HTTP %d %s", path, code, unwrapped)
		http.Error(w, unwrapped.Error(), code)
	})
}
