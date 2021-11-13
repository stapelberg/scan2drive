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
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stapelberg/scan2drive"
	"github.com/stapelberg/scan2drive/internal/httperr"
	"github.com/stapelberg/scan2drive/internal/jobqueue"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jws"
)

const sessionCookieName = "scan2drive"

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

func constantsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `var clientID = "%s";`, oauthConfig.ClientID)
}

func (ui *UI) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	session, err := ui.store.Get(r, sessionCookieName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var scans map[string]*jobqueue.Job
	sub, _ := session.Values["sub"].(string)
	account := ui.lockedUsers.User(sub)
	if !account.LoggedIn() {
		sub = ""
		account = nil
	} else {
		var err error
		scans, err = account.Queue.Scans()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var keys []string
	for key := range scans {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

	type user struct {
		Sub        string
		FullName   string
		PictureURL string
	}
	var defaultSub string
	users := ui.lockedUsers.Users()
	tusers := make([]user, 0, len(users))
	subs := make([]string, 0, len(users))

	for sub, state := range users {
		if state.Default {
			defaultSub = sub
		}
		subs = append(subs, sub)
		tusers = append(tusers, user{
			Sub:        sub,
			FullName:   state.Name,
			PictureURL: state.Picture,
		})
	}
	sort.Strings(subs)

	var scanSources []scan2drive.ScanSourceMetadata
	for _, finder := range ui.finders {
		srcs := finder.CurrentScanSources()
		for _, src := range srcs {
			metadata := src.Metadata()

			if metadata.IconURL != "" && !strings.HasPrefix(metadata.IconURL, "/") {
				if u, err := url.Parse(metadata.IconURL); err == nil {
					u.Host = r.Host
					u.Path = "/scanicon/" + metadata.Id + "/" + path.Base(u.Path)
					metadata.IconURL = u.String()
				}
			}

			scanSources = append(scanSources, metadata)
		}
	}

	var buf bytes.Buffer
	err = ui.tmpl.ExecuteTemplate(&buf, "Index.html.tmpl", map[string]interface{}{
		// TODO: show drive connection status in settings drop-down
		"sub":         sub,
		"user":        account,
		"scans":       scans,
		"keys":        keys,
		"subs":        subs,
		"users":       tusers,
		"defaultsub":  defaultSub,
		"scansources": scanSources,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.Copy(w, &buf)
}

func (ui *UI) writeJsonForUser(sub, name string, value interface{}) error {
	dir := filepath.Join(ui.stateDir, "users", sub)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(dir, name), bytes, 0600)
}

func (ui *UI) writeToken(sub string, token *oauth2.Token) error {
	return ui.writeJsonForUser(sub, "token.json", token)
}

func (ui *UI) writeDriveFolder(sub string, folder scan2drive.DriveFolder) error {
	return ui.writeJsonForUser(sub, "drive_folder.json", &folder)
}

func (ui *UI) writeDefault(sub string, isDefault bool) error {
	fn := filepath.Join(ui.stateDir, "users", sub, "is_default")
	if isDefault {
		return ioutil.WriteFile(fn, []byte("true"), 0644)
	}
	err := os.Remove(fn)
	if os.IsNotExist(err) {
		return nil // desired state
	}
	return err
}

func (ui *UI) oauthHandler(w http.ResponseWriter, r *http.Request) error {
	code, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	token, err := oauthConfig.Exchange(oauth2.NoContext, string(code))
	if err != nil {
		return err
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return fmt.Errorf("id_token is not a string TODO better msg")
	}
	claimset, err := jws.Decode(idToken)
	if err != nil {
		return err
	}
	session, err := ui.store.Get(r, sessionCookieName)
	if err != nil {
		return err
	}
	session.Values["sub"] = claimset.Sub
	if err := session.Save(r, w); err != nil {
		return err
	}
	if token.RefreshToken == "" {
		// Only on the first login, we get a RefreshToken. All subsequent
		// logins will just have an AccessToken. In order to force a first
		// login, the user needs to revoke the scan2drive permissions on
		// https://myaccount.google.com/u/0/permissions
		account := ui.lockedUsers.User(claimset.Sub)
		if account == nil || account.Token == nil {
			return fmt.Errorf("Could not merge old RefreshToken into new token: no old refresh token found. If you re-installed scan2drive, revoke the permission on https://security.google.com/settings/u/0/security/permissions ")
		}
		token.RefreshToken = account.Token.RefreshToken
	}
	if err := ui.writeToken(claimset.Sub, token); err != nil {
		return fmt.Errorf("writeToken: %v", err)
	}
	if err := ui.updateUsers(); err != nil {
		return fmt.Errorf("updateUsers: %v", err)
	}
	return nil
}

func (ui *UI) requireAuth(w http.ResponseWriter, r *http.Request) (string, error) {
	session, err := ui.store.Get(r, sessionCookieName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", err
	}
	if v, ok := session.Values["sub"].(string); ok {
		return v, nil
	}
	http.Error(w, "Not logged in yet", http.StatusForbidden)
	return "", fmt.Errorf(`"sub" not found in session`)
}

func (ui *UI) signoutHandler(w http.ResponseWriter, r *http.Request) error {
	sub, err := ui.requireAuth(w, r)
	if err != nil {
		return httperr.Error(
			http.StatusForbidden,
			err)
	}
	var token string
	account := ui.lockedUsers.User(sub)
	if account != nil && account.Token != nil {
		token = account.Token.RefreshToken
	}
	dir := filepath.Join(ui.stateDir, "users", sub)
	if err := os.Remove(filepath.Join(dir, "token.json")); err != nil {
		return fmt.Errorf("Error deleting token: %v", err)
	}

	if err := ui.updateUsers(); err != nil {
		return fmt.Errorf("updateUsers: %v", err)
	}

	if token == "" {
		log.Printf("skipping revoke: no refresh token present to begin with")
		return nil
	}
	// Revoke the token so that subsequent logins will yield a refresh
	// token without clients having to explicitly revoke permission:
	// https://developers.google.com/identity/protocols/OAuth2WebServer#tokenrevoke
	resp, err := http.PostForm("https://accounts.google.com/o/oauth2/revoke", url.Values{
		"token": []string{token},
	})
	if err != nil {
		return fmt.Errorf("revoking token: %v", err)
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected HTTP status code: got %d, want %d (body: %q)", got, want, string(b))
	}
	return nil
}

//go:embed assets/*
var assetsDir embed.FS

func (ui *UI) scansDirHandler(w http.ResponseWriter, r *http.Request) {
	sub, err := ui.requireAuth(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	// TODO: is this safe with regards to path traversal attacks?
	dir := filepath.Join(ui.scansDir, sub)
	http.StripPrefix("/scans_dir/", http.FileServer(http.Dir(dir))).ServeHTTP(w, r)
}

func (ui *UI) storeDriveFolder(w http.ResponseWriter, r *http.Request) error {
	sub, err := ui.requireAuth(w, r)
	if err != nil {
		return nil // requireAuth handles the error
	}
	var folder scan2drive.DriveFolder
	if err := json.NewDecoder(r.Body).Decode(&folder); err != nil {
		return err
	}
	if err := ui.writeDriveFolder(sub, folder); err != nil {
		return fmt.Errorf("writeDriveFolder: %v", err)
	}
	if err := ui.updateUsers(); err != nil {
		return fmt.Errorf("updateUsers: %v", err)
	}
	return nil
}

func (ui *UI) storeDefaultUser(w http.ResponseWriter, r *http.Request) error {
	_, err := ui.requireAuth(w, r)
	if err != nil {
		return nil // requireAuth handles the error
	}

	var storeDefaultReq struct {
		DefaultSub string
	}

	if err := json.NewDecoder(r.Body).Decode(&storeDefaultReq); err != nil {
		return err
	}

	users := ui.lockedUsers.Users()
	for sub, state := range users {
		state.Default = (sub == storeDefaultReq.DefaultSub)
		if err := ui.writeDefault(sub, state.Default); err != nil {
			return err
		}
	}
	if err := ui.updateUsers(); err != nil {
		return fmt.Errorf("updateUsers: %v", err)
	}
	return nil
}

func (ui *UI) startScanHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		return httperr.Error(
			http.StatusBadRequest,
			fmt.Errorf("Bad Request"))
	}

	sub, err := ui.requireAuth(w, r)
	if err != nil {
		return nil // requireAuth handles the error
	}

	srcId := path.Base(r.URL.Path)
	scanSource := ui.getScanSource(srcId)
	if scanSource == nil {
		return httperr.Error(
			http.StatusNotFound,
			fmt.Errorf("scan source %q not found", srcId))
	}

	ingester := ui.ingesterFor(sub)
	if ingester == nil {
		return httperr.Error(
			http.StatusNotFound,
			fmt.Errorf("ingester for user %q not found", sub))
	}
	jobId, err := scanSource.ScanTo(ingester)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&struct {
		Name string
	}{
		Name: jobId,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

// func scanStatusHandler(w http.ResponseWriter, r *http.Request) {
// 	sub, err := requireAuth(w, r)
// 	if err != nil {
// 		return
// 	}

// 	var scanStatusReq struct {
// 		Name string
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&scanStatusReq); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	scansMu.RLock()
// 	state, ok := scans[scanStatusReq.Name]
// 	scansMu.RUnlock()
// 	if !ok {
// 		http.Error(w, "Not found", http.StatusNotFound)
// 		return
// 	}
// 	if state.Sub != sub {
// 		http.Error(w, "Permission Denied", http.StatusForbidden)
// 		return
// 	}

// 	status := "Scanning pages…"
// 	if state.Scanned {
// 		status = "Converting…"
// 	}
// 	if state.Converted {
// 		status = "Uploading…"
// 	}
// 	if state.UploadedPDF {
// 		status = "Done"
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(&struct {
// 		Status string
// 		Done   bool
// 	}{
// 		Status: status,
// 		Done:   state.UploadedPDF,
// 	}); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 	}
// }

func (ui *UI) getScanSource(srcId string) scan2drive.ScanSource {
	for _, finder := range ui.finders {
		srcs := finder.CurrentScanSources()
		for _, src := range srcs {
			if src.Metadata().Id == srcId {
				return src
			}
		}
	}
	return nil
}

func (ui *UI) scanIconHandler(w http.ResponseWriter, r *http.Request) error {
	if _, err := ui.requireAuth(w, r); err != nil {
		return nil // requireAuth handles the error
	}

	var srcId string
	srcId, r.URL.Path = shiftPath(r.URL.Path)
	scanSource := ui.getScanSource(srcId)
	if scanSource == nil {
		return httperr.Error(
			http.StatusNotFound,
			fmt.Errorf("scan source %q not found", srcId))
	}

	// Remove the .local. avahi suffix to go through local DNS, as there is no
	// avahi on gokrazy (yet?).
	iconURL := strings.ReplaceAll(scanSource.Metadata().IconURL, ".local.", "")
	u, err := url.Parse(iconURL)
	if err != nil {
		return err
	}
	u.Path = path.Dir(u.Path)
	httputil.NewSingleHostReverseProxy(u).ServeHTTP(w, r)
	return nil
}

// func renameScanHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != "POST" {
// 		http.Error(w, "Bad Request", http.StatusBadRequest)
// 		return
// 	}

// 	sub, err := requireAuth(w, r)
// 	if err != nil {
// 		return
// 	}

// 	var renameScanReq struct {
// 		Name    string
// 		NewName string
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&renameScanReq); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// TODO: is filepath.Join() safe w.r.t. directory traversal?
// 	scanDir := filepath.Join(*scansDir, sub, renameScanReq.Name)
// 	if err := os.Remove(filepath.Join(scanDir, "COMPLETE.rename")); err != nil {
// 		if os.IsNotExist(err) {
// 			http.Error(w, "Scan not found", http.StatusNotFound)
// 		} else {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 		}
// 		return
// 	}

// 	if err := ioutil.WriteFile(filepath.Join(scanDir, "rename"), []byte(renameScanReq.NewName), 0600); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// TODO: if uploaddone, then start processscan. otherwise, the file will be considered anyway
// }
