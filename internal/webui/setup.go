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

package webui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"text/template"
)

func writeSecretFile(clientid, clientsecret, clientSecretPath string) error {
	if clientid == "" || clientsecret == "" {
		return fmt.Errorf("either clientid=%q or clientsecret=%q are empty", clientid, clientsecret)
	}
	// Use a temporary file to rule out clobbering *clientSecretPath,
	// should it in the meantime have been created.
	f, err := ioutil.TempFile(filepath.Dir(clientSecretPath), "scan2drive")
	if err != nil {
		return err
	}
	defer f.Close()
	type installed struct {
		ClientId                string   `json:"client_id"`
		ClientSecret            string   `json:"client_secret"`
		AuthUri                 string   `json:"auth_uri"`
		TokenUri                string   `json:"token_uri"`
		AuthProviderX509CertUrl string   `json:"auth_provider_x509_cert_url"`
		RedirectUris            []string `json:"redirect_uris"`
	}
	if err := json.NewEncoder(f).Encode(&struct {
		Installed installed `json:"installed"`
	}{
		Installed: installed{
			ClientId:                clientid,
			ClientSecret:            clientsecret,
			AuthUri:                 "https://accounts.google.com/o/oauth2/auth",
			TokenUri:                "https://accounts.google.com/o/oauth2/token",
			AuthProviderX509CertUrl: "https://www.googleapis.com/oauth2/v1/certs",
			RedirectUris: []string{
				"urn:ietf:wg:oauth:2.0:oob",
				"postmessage",
			},
		},
	}); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), clientSecretPath)
}

func setupHandlerMux(clientSecretPath string) (*http.ServeMux, error) {
	var canWriteClientSecret bool
	if err := ioutil.WriteFile(clientSecretPath, nil, 0600); err == nil {
		canWriteClientSecret = true
		os.Remove(clientSecretPath)
	} else {
		return nil, fmt.Errorf("Cannot offer a web form for storing the client secret: writing test file failed: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.FileServer(http.FS(assetsDir)))
	mux.HandleFunc("/setup", func(w http.ResponseWriter, r *http.Request) {
		clientid := r.FormValue("clientid")
		clientsecret := r.FormValue("clientsecret")
		if err := writeSecretFile(clientid, clientsecret, clientSecretPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// TODO: deliver an HTML page that does a meta refresh so that we can do
		// the syscall.Exec in the background without interrupting the
		// connection to the browser

		// TODO: refactor the web interface so that it can start working
		// gracefully without needing this drastic re-exec:
		if err := syscall.Exec(os.Args[0], os.Args[1:], os.Environ()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})
	tmpl, err := template.ParseFS(assetsDir, "assets/*.tmpl")
	if err != nil {
		log.Fatal(err)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "Setup.html.tmpl", map[string]interface{}{
			"ClientSecretPath":     clientSecretPath,
			"CanWriteClientSecret": canWriteClientSecret,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		io.Copy(w, &buf)
	})
	return mux, nil
}
