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
	"encoding/base64"
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

	"github.com/gorilla/securecookie"
	"github.com/stapelberg/scan2drive"
	"github.com/stapelberg/scan2drive/internal/httperr"
	"github.com/stapelberg/scan2drive/internal/jobqueue"
	"github.com/stapelberg/scan2drive/internal/source/airscan"
	"golang.org/x/oauth2"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
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

func genXSRFToken() string {
	return base64.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32))
}

func (ui *UI) constantsHandler(w http.ResponseWriter, r *http.Request) error {
	session, err := ui.store.Get(r, sessionCookieName)
	if err != nil {
		return err
	}

	xsrftoken, ok := session.Values["xsrftoken"].(string)
	if !ok {
		xsrftoken = genXSRFToken()
		session.Values["xsrftoken"] = xsrftoken
		if err := session.Save(r, w); err != nil {
			return err
		}
	}

	w.Header().Set("Content-Type", "text/javascript")
	fmt.Fprintf(w, "var clientID = %q;\n", oauthConfig.ClientID)
	fmt.Fprintf(w, "var redirectURL = %q;\n", oauthConfig.RedirectURL)
	fmt.Fprintf(w, "var XSRFToken = %q;\n", xsrftoken)
	return nil
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
	accessToken := ""
	if !account.LoggedIn() {
		sub = ""
		account = nil
	} else {
		accessToken = account.Token.AccessToken
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
		"clientid":    oauthConfig.ClientID,
		"accesstoken": accessToken,
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

func redirectToIndexWithError(w http.ResponseWriter, r *http.Request, errorMsg string) {
	q := url.Values{}
	q.Set("error", errorMsg)
	target := url.URL{
		Path:     "/",
		RawQuery: q.Encode(),
	}
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (ui *UI) oauthHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	session, err := ui.store.Get(r, sessionCookieName)
	if err != nil {
		return err
	}

	xsrftoken, ok := session.Values["xsrftoken"].(string)
	if !ok {
		return fmt.Errorf("xsrftoken not found in session")
	}

	if msg := r.FormValue("error"); msg != "" {
		redirectToIndexWithError(w, r, "OAuth error: "+msg)
		return nil
	}

	// Because we directly authorize the user (as opposed to first logging in
	// the user, and then authorizing at a later point), Google’s consent dialog
	// shows a checkbox next to the drive.file scope, allowing the user to grant
	// login permission, but no drive.file permission. In such a case, throw an
	// error here to make the user log in again.
	if !strings.Contains(r.FormValue("scope"), "https://www.googleapis.com/auth/drive.file") {
		redirectToIndexWithError(w, r, "OAuth error: scope does not contain drive.file. sign in again and tick the checkbox")
		return nil
	}

	if state := r.FormValue("state"); state != xsrftoken {
		return fmt.Errorf("XSRF token mismatch")
	}

	code := r.FormValue("code")
	if code == "" {
		return fmt.Errorf("empty code parameter")
	}

	token, err := oauthConfig.Exchange(ctx, string(code))
	if err != nil {
		return err
	}

	httpClient := oauthConfig.Client(ctx, token)
	osrv, err := oauth2api.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return fmt.Errorf("creating oauth2 client: %v", err)
	}
	userinfo, err := osrv.Userinfo.Get().Do()
	if err != nil {
		return fmt.Errorf("getting userinfo: %v", err)
	}
	sub := userinfo.Id
	if sub == "" {
		return fmt.Errorf("oauth2 error: userinfo.Id unexpectedly empty?!")
	}

	if len(ui.allowedUsers) == 0 {
		log.Printf("skipping user validation because -allowed_users_list is not set")
	} else {
		log.Printf("login request from %q", userinfo.Email)
		if !ui.allowedUsers[userinfo.Email] {
			return fmt.Errorf("not listed in -allowed_users_list")
		}
	}

	session.Values["sub"] = sub
	if err := session.Save(r, w); err != nil {
		return err
	}
	if token.RefreshToken == "" {
		// Only on the first login, we get a RefreshToken. All subsequent
		// logins will just have an AccessToken. In order to force a first
		// login, the user needs to revoke the scan2drive permissions on
		// https://myaccount.google.com/permissions
		account := ui.lockedUsers.User(sub)
		if account == nil || account.Token == nil {
			return fmt.Errorf("Could not merge old RefreshToken into new token: no old refresh token found. If you re-installed scan2drive, revoke the permission on https://myaccount.google.com/permissions")
		}
		token.RefreshToken = account.Token.RefreshToken
	}
	if err := ui.writeToken(sub, token); err != nil {
		return fmt.Errorf("writeToken: %v", err)
	}
	if err := ui.updateUsers(); err != nil {
		return fmt.Errorf("updateUsers: %v", err)
	}

	http.Redirect(w, r, "/", http.StatusFound)

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
	// https://developers.google.com/identity/protocols/oauth2/web-server#tokenrevoke
	resp, err := http.PostForm("https://oauth2.googleapis.com/revoke", url.Values{
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
		return // requireAuth handles the error
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
	airscanSource, ok := scanSource.(*airscan.AirscanSource)
	if !ok {
		return httperr.Error(
			http.StatusBadRequest,
			fmt.Errorf("scan source is of type %T, not airscan", scanSource))
	}

	// Remove the .local. avahi suffix to go through local DNS, as there is no
	// avahi on gokrazy (yet?).
	iconURL := strings.ReplaceAll(scanSource.Metadata().IconURL, ".local.", "")
	u, err := url.Parse(iconURL)
	if err != nil {
		return err
	}
	u.Path = path.Dir(u.Path)
	reverseProxy := httputil.NewSingleHostReverseProxy(u)
	reverseProxy.Transport = airscanSource.Transport()
	reverseProxy.ServeHTTP(w, r)
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
