package main

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
	"sync"

	"github.com/stapelberg/scan2drive/templates"
)

type setupMux struct {
	setup bool
	mu    sync.RWMutex
	cond  *sync.Cond

	setupMux   *http.ServeMux
	defaultMux *http.ServeMux
}

func newSetupMux() *setupMux {
	m := &setupMux{
		setupMux:   http.NewServeMux(),
		defaultMux: http.NewServeMux(),
	}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (s *setupMux) setSetup(setup bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setup = setup
	s.cond.Broadcast()
}

// setupLeft blocks until setup mode is left
func (s *setupMux) setupLeft() {
	s.cond.L.Lock()
	for s.setup {
		s.cond.Wait()
	}
	s.cond.L.Unlock()
}

// ServeHTTP implements (net/http).Handler
func (s *setupMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mux := s.defaultMux
	if s.setup {
		mux = s.setupMux
	}
	s.mu.RUnlock()

	mux.ServeHTTP(w, r)
}

func writeSecretFile(clientid, clientsecret string) error {
	if clientid == "" || clientsecret == "" {
		return fmt.Errorf("either clientid=%q or clientsecret=%q are empty", clientid, clientsecret)
	}
	// Use a temporary file to rule out clobbering *clientSecretPath,
	// should it in the meantime have been created.
	f, err := ioutil.TempFile(filepath.Dir(*clientSecretPath), "scan2drive")
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
	return os.Rename(f.Name(), *clientSecretPath)
}

func registerSetupHandlers(mux *setupMux) {
	var canWriteClientSecret bool
	if err := ioutil.WriteFile(*clientSecretPath, nil, 0600); err == nil {
		canWriteClientSecret = true
		os.Remove(*clientSecretPath)
	} else {
		log.Printf("Cannot offer a web form for storing the client secret: writing test file failed: %v", err)
	}
	mux.setupMux.HandleFunc("/assets/", assetsDirHandler)
	mux.setupMux.HandleFunc("/setup", func(w http.ResponseWriter, r *http.Request) {
		clientid := r.FormValue("clientid")
		clientsecret := r.FormValue("clientsecret")
		if err := writeSecretFile(clientid, clientsecret); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		mux.setSetup(false)
		http.Redirect(w, r, "/", http.StatusFound)
	})
	mux.setupMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if err := templates.SetupTpl.ExecuteTemplate(&buf, "Setup", map[string]interface{}{
			"ClientSecretPath":     *clientSecretPath,
			"CanWriteClientSecret": canWriteClientSecret,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		io.Copy(w, &buf)
	})
}
