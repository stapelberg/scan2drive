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

// Package webui implements the scan2drive web user interface using
// materializecss.com and jQuery.
package webui

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"

	gorilla_context "github.com/gorilla/context"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/stapelberg/scan2drive"
	"github.com/stapelberg/scan2drive/internal/httperr"
	"github.com/stapelberg/scan2drive/internal/scaningest"
	"github.com/stapelberg/scan2drive/internal/user"
	"golang.org/x/net/trace"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v2"
)

var oauthConfig *oauth2.Config

// oauthConfigFromJSON is like google.ConfigFromJSON, but overrides the
// RedirectURIs key which cannot be set to “postmessage” in the web interface.
func oauthConfigFromJSON(jsonKey []byte) (*oauth2.Config, error) {
	type cred struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		RedirectURIs []string `json:"redirect_uris"`
		AuthURI      string   `json:"auth_uri"`
		TokenURI     string   `json:"token_uri"`
	}
	var j struct {
		Web       *cred `json:"web"`
		Installed *cred `json:"installed"`
	}
	if err := json.Unmarshal(jsonKey, &j); err != nil {
		return nil, err
	}
	var c *cred
	switch {
	case j.Web != nil:
		c = j.Web
	case j.Installed != nil:
		c = j.Installed
	default:
		return nil, fmt.Errorf("oauth2/google: no credentials found")
	}

	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  "postmessage",
		Scopes:       []string{drive.DriveScope, "profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.AuthURI,
			TokenURL: c.TokenURI,
		},
	}, nil
}

func loadSessionStore(stateDir string) (sessions.Store, error) {
	cookieSecretPath := filepath.Join(stateDir, "cookies.key")
	secret, err := ioutil.ReadFile(cookieSecretPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(cookieSecretPath), 0755); err != nil {
			return nil, err
		}
		// NOTE: Not much thought went into chosing 32 bytes as the length of
		// cookie keys. In case there are any arguments for a different number,
		// I’m happy to be convinced.
		secret = securecookie.GenerateRandomKey(32)
		if err := ioutil.WriteFile(cookieSecretPath, secret, 0600); err != nil {
			return nil, err
		}
	}

	sessionsPath := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsPath, 0700); err != nil {
		return nil, err
	}
	return sessions.NewFilesystemStore(sessionsPath, secret), nil
}

func Init(clientSecretPath string, lockedUsers *user.Locked, scansDir, stateDir string, finders []scan2drive.ScanSourceFinder, ingesterFor func(uid string) *scaningest.Ingester) (http.Handler, *oauth2.Config, error) {
	store, err := loadSessionStore(stateDir)
	if err != nil {
		return nil, nil, err
	}

	tmpl, err := template.ParseFS(assetsDir, "assets/*.tmpl")
	if err != nil {
		return nil, nil, err
	}

	ui := &UI{
		lockedUsers: lockedUsers,
		store:       store,
		tmpl:        tmpl,
		scansDir:    scansDir,
		stateDir:    stateDir,
		finders:     finders,
		ingesterFor: ingesterFor,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/constants.js", constantsHandler)
	mux.Handle("/assets/", http.FileServer(http.FS(assetsDir)))
	// TODO: verify method (POST) in each handler
	mux.HandleFunc("/scans_dir/", ui.scansDirHandler)
	mux.Handle("/oauth", httperr.Handle(ui.oauthHandler))
	mux.Handle("/signout", httperr.Handle(ui.signoutHandler))
	mux.Handle("/storedrivefolder", httperr.Handle(ui.storeDriveFolder))
	mux.Handle("/storedefaultuser", httperr.Handle(ui.storeDefaultUser))
	mux.Handle("/startscan/", httperr.Handle(ui.startScanHandler))
	// def.HandleFunc("/scanstatus", scanStatusHandler)
	mux.Handle("/scanicon/", http.StripPrefix("/scanicon/", httperr.Handle(ui.scanIconHandler)))
	// def.HandleFunc("/renamescan", renameScanHandler)

	// TODO: display remaining drive quota in the user interface
	// TODO: subscribe the UI to a web events stream for incoming scans, focus the latest scan, specifically the rename icon
	mux.HandleFunc("/", ui.indexHandler)

	mux.HandleFunc("/debug/requests", trace.Traces)

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	b, err := ioutil.ReadFile(clientSecretPath)
	if os.IsNotExist(err) {
		handler, err := setupHandlerMux(clientSecretPath)
		return handler, nil, err
	}
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to read client secret file: %v", err)
	}

	oauthConfig, err = oauthConfigFromJSON(b)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	ui.oauthConfig = oauthConfig
	return gorilla_context.ClearHandler(mux), oauthConfig, nil
}

type UI struct {
	lockedUsers *user.Locked
	store       sessions.Store
	tmpl        *template.Template
	scansDir    string
	stateDir    string
	oauthConfig *oauth2.Config
	finders     []scan2drive.ScanSourceFinder
	ingesterFor func(uid string) *scaningest.Ingester
}

func (ui *UI) updateUsers() error {
	return ui.lockedUsers.UpdateFromDir(ui.stateDir, ui.scansDir, ui.oauthConfig)
}
