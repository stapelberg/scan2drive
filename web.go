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
// limitations under the License./

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"google.golang.org/grpc"

	"github.com/gorilla/sessions"
	"github.com/stapelberg/scan2drive/proto"
	"github.com/stapelberg/scan2drive/templates"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jws"
)

const sessionCookieName = "scan2drive"

var store sessions.Store

func indexHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, sessionCookieName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var sub string
	var state userState
	if v, ok := session.Values["sub"].(string); ok {
		sub = v
		usersMu.RLock()
		state = users[sub]
		if !state.loggedIn() {
			state = userState{}
			sub = ""
		}
		usersMu.RUnlock()
	}
	scansMu.RLock()
	var keys []string
	for key, _ := range scans {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	err = templates.IndexTpl.ExecuteTemplate(w, "Index", map[string]interface{}{
		// TODO: show drive connection status in settings drop-down
		"sub":   sub,
		"user":  state,
		"scans": scans,
		"keys":  keys,
	})
	scansMu.RUnlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func oauthHandler(w http.ResponseWriter, r *http.Request) {
	code, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	token, err := oauthConfig.Exchange(oauth2.NoContext, string(code))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "id_token is not a string TODO better msg", http.StatusInternalServerError)
		return
	}
	claimset, err := jws.Decode(idToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session, err := store.Get(r, sessionCookieName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Values["sub"] = claimset.Sub
	if err := session.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if token.RefreshToken == "" {
		// Only on the first login, we get a RefreshToken. All subsequent
		// logins will just have an AccessToken. In order to force a first
		// login, the user needs to revoke the scan2drive permissions on
		// https://security.google.com/settings/u/0/security/permissions
		usersMu.RLock()
		oldRefreshToken := ""
		if users[claimset.Sub].Token != nil {
			oldRefreshToken = users[claimset.Sub].Token.RefreshToken
		}
		usersMu.RUnlock()
		if oldRefreshToken == "" {
			log.Printf("Error merging tokens: no old refresh token found")
			http.Error(w, "Old refresh token not found", http.StatusInternalServerError)
			return
		}
		token.RefreshToken = oldRefreshToken
	}
	if err := writeToken(claimset.Sub, token); err != nil {
		log.Printf("Error writing token: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := readUsers(); err != nil {
		log.Printf("Error reading users: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := examineScansDir(); err != nil {
		log.Printf("Error examining scans: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func requireAuth(w http.ResponseWriter, r *http.Request) (string, error) {
	session, err := store.Get(r, sessionCookieName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", err
	}
	if v, ok := session.Values["sub"].(string); ok {
		return v, nil
	}
	return "", fmt.Errorf(`"sub" not found in session`)
}

func signoutHandler(w http.ResponseWriter, r *http.Request) {
	sub, err := requireAuth(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	dir := filepath.Join(*stateDir, "users", sub)
	if err := os.Remove(filepath.Join(dir, "token.json")); err != nil {
		log.Printf("Error deleting token: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := readUsers(); err != nil {
		log.Printf("Error reading users: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func scansDirHandler(w http.ResponseWriter, r *http.Request) {
	sub, err := requireAuth(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	// TODO: is this safe with regards to path traversal attacks?
	dir := filepath.Join(*scansDir, sub)
	http.StripPrefix("/"+filepath.Join("scans_dir", sub), http.FileServer(http.Dir(dir))).ServeHTTP(w, r)
}

func storeDriveFolder(w http.ResponseWriter, r *http.Request) {
	sub, err := requireAuth(w, r)
	if err != nil {
		return
	}
	var folder driveFolder
	if err := json.NewDecoder(r.Body).Decode(&folder); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := writeDriveFolder(sub, folder); err != nil {
		log.Printf("Error writing drive folder: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := readUsers(); err != nil {
		log.Printf("Error reading users: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func startScanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	sub, err := requireAuth(w, r)
	if err != nil {
		return
	}

	output, err := exec.Command(*scanScript).Output()
	if err != nil {
		// Make the exit status available for the client to interpret as
		// SANE_Status.
		if exitErr, ok := err.(*exec.ExitError); ok {
			if waitStatus, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				w.Header().Add("X-Exit-Status", strconv.Itoa(waitStatus.ExitStatus()))
			}
		}
		log.Printf("Error scanning: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	name := strings.TrimSpace(string(output))

	// TODO: connecting to localhost:7119 might break when -rpc_listen_address is specified
	conn, err := grpc.Dial("localhost:7119", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatal(err)
	}
	client := proto.NewScanClient(conn)
	go client.ProcessScan(context.Background(), &proto.ProcessScanRequest{User: sub, Dir: name})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&struct {
		Name string
	}{
		Name: name,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func scanStatusHandler(w http.ResponseWriter, r *http.Request) {
	sub, err := requireAuth(w, r)
	if err != nil {
		return
	}

	var scanStatusReq struct {
		Name string
	}

	if err := json.NewDecoder(r.Body).Decode(&scanStatusReq); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	scansMu.RLock()
	state, ok := scans[scanStatusReq.Name]
	scansMu.RUnlock()
	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if state.Sub != sub {
		http.Error(w, "Permission Denied", http.StatusForbidden)
		return
	}

	status := "Scanning pages…"
	if state.Scanned {
		status = "Converting…"
	}
	if state.Converted {
		status = "Uploading…"
	}
	if state.UploadedPDF {
		status = "Done"
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&struct {
		Status string
		Done   bool
	}{
		Status: status,
		Done:   state.UploadedPDF,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renameScanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	sub, err := requireAuth(w, r)
	if err != nil {
		return
	}

	var renameScanReq struct {
		Name    string
		NewName string
	}

	if err := json.NewDecoder(r.Body).Decode(&renameScanReq); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO: is filepath.Join() safe w.r.t. directory traversal?
	scanDir := filepath.Join(*scansDir, sub, renameScanReq.Name)
	if err := os.Remove(filepath.Join(scanDir, "COMPLETE.rename")); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Scan not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if err := ioutil.WriteFile(filepath.Join(scanDir, "rename"), []byte(renameScanReq.NewName), 0600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO: if uploaddone, then start processscan. otherwise, the file will be considered anyway
}
