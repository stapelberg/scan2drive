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
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	gorilla_context "github.com/gorilla/context"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/stapelberg/scan2drive/proto"
	"golang.org/x/net/context"
	"golang.org/x/net/trace"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc"
)

var (
	rpcListenAddr = flag.String("rpc_listen_address",
		":7119",
		"[host]:port to listen on for RPCs")
	httpListenAddr = flag.String("http_listen_address",
		":7120",
		"[host]:port to listen on for RPCs")
	scanService = flag.String("scan_service",
		"",
		"Optional [host]:port address to use for offloading the conversion of scanned documents. This could be your beefy workstation, while scan2drive itself runs on a Raspberry Pi. If unspecified, conversion will be done locally.")

	staticDir = flag.String("static_dir",
		"static/",
		"Path to the directory containing static assets (JavaScript, images, etc.)")

	// TODO: patch up the file with redirect_uris = postmessage
	clientSecretPath = flag.String("client_secret_path",
		"client_secret_197950901230-jee3asvone1tnh7k2qsshm369723vkun.apps.googleusercontent.com.json",
		"Path to a client secret JSON file as described in https://developers.google.com/drive/v3/web/quickstart/go#step_1_turn_on_the_api_name")

	stateDir = flag.String("state_dir",
		"/tmp/scan2drive-state",
		"Directory containing state such as session data and OAuth tokens for communicating with Google Drive or the destination folder id. If wiped, users will need to re-login and chose their drive folder again.")

	scanScript = flag.String("scan_script",
		"/usr/bin/scan2drive-scan",
		"Script to run when users hit the Scan button in the web interface. This should be a wrapper around scanimage, its exit code is interpreted as SANE_Status type.")
	scansDir = flag.String("scans_dir",
		"/tmp/fin",
		"Directory in which -scan_script places scanned documents.")
	scans   map[string]dirState
	scansMu sync.RWMutex

	processAllInterval = flag.Duration("process_all_interval",
		1*time.Hour,
		"Interval after which to process all scans in -scans_dir. This safety step catches documents which were scanned but not converted and uploaded interactively, e.g. when scan2drive is stopped while you scan.")

	oauthConfig *oauth2.Config

	scanConn      *grpc.ClientConn
	localScanConn *grpc.ClientConn
)

type driveFolder struct {
	Id      string
	IconUrl string
	Url     string
	Name    string
}

type userState struct {
	Token  *oauth2.Token
	Folder driveFolder
	Drive  *drive.Service
}

func (u userState) loggedIn() bool {
	return u.Token != nil
}

func (u userState) getCurrentParentDir() (string, error) {
	year := fmt.Sprintf("%d", time.Now().Year())
	query := fmt.Sprintf("'%s' in parents and name = '%s' and trashed=false", u.Folder.Id, year)
	r, err := u.Drive.Files.List().Q(query).PageSize(1).Fields("files(id, name)").Do()
	if err != nil {
		return "", err
	}

	if len(r.Files) > 1 {
		return "", fmt.Errorf("Files.List: expected at most 1 result, got %d results", len(r.Files))
	}
	if len(r.Files) == 1 {
		return r.Files[0].Id, nil
	}
	rc, err := u.Drive.Files.Create(&drive.File{
		MimeType: "application/vnd.google-apps.folder",
		Name:     year,
		Parents:  []string{u.Folder.Id},
	}).Fields("id").Do()
	if err != nil {
		return "", err
	}
	return rc.Id, nil
}

var (
	users   = make(map[string]userState)
	usersMu sync.RWMutex
)

func readUsers() error {
	newUsers := make(map[string]userState)
	usersPath := filepath.Join(*stateDir, "users")
	entries, err := ioutil.ReadDir(usersPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sub := entry.Name()
		var state userState
		bytes, err := ioutil.ReadFile(filepath.Join(usersPath, sub, "token.json"))
		if err != nil {
			// User directories without token.json are considered logged-out.
			// drive_folder.json is still persisted so that users don’t have to
			// re-select a drive folder when logging in again.
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := json.Unmarshal(bytes, &state.Token); err != nil {
			return err
		}
		// Try to read drive_folder.json if it exists.
		bytes, err = ioutil.ReadFile(filepath.Join(usersPath, sub, "drive_folder.json"))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			if err := json.Unmarshal(bytes, &state.Folder); err != nil {
				return err
			}
		}
		usersMu.RLock()
		oldToken := users[sub].Token
		oldDrive := users[sub].Drive
		usersMu.RUnlock()
		if oldToken != nil &&
			oldToken.AccessToken == state.Token.AccessToken &&
			oldToken.RefreshToken == state.Token.RefreshToken {
			state.Drive = oldDrive
		} else {
			// NOTE: As per https://github.com/golang/oauth2/issues/84,
			// Google’s RefreshTokens stay valid until they are revoked, so
			// there is no need to ever update the token on disk.
			srv, err := drive.New(oauthConfig.Client(context.Background(), state.Token))
			if err != nil {
				log.Printf("Could not create drive client for %q: %v", sub, err)
				continue
			}
			state.Drive = srv
		}
		newUsers[sub] = state
	}
	usersMu.Lock()
	users = newUsers
	usersMu.Unlock()
	return nil
}

func refreshTokens() {
	// TODO: don’t hold mutex the entire time
	usersMu.Lock()
	defer usersMu.Unlock()

	for sub, state := range users {
		if _, err := state.getCurrentParentDir(); err != nil {
			log.Printf("Could not locate destination drive folder for %q: %v", sub, err)
			// TODO: export error status via prometheus
			// TODO: expose this issue via the UI
			continue
		}

		about, err := state.Drive.About.Get().Fields("storageQuota").Do()
		if err != nil {
			log.Printf("Could not get quota for %q: %v", sub, err)
			// TODO: export error status via prometheus
			// TODO: expose this issue via the UI
			continue
		}
		log.Printf("quota = %+v", about.StorageQuota)
		if about.StorageQuota.Limit > 0 &&
			(about.StorageQuota.Limit-about.StorageQuota.Usage) < 100*5*1024*1024 {
			log.Printf("Account %q has less than 500 MiB available", sub)
			// TODO: export error status via prometheus
			// TODO: expose this issue via the UI
		}
	}
}

func writeJsonForUser(sub, name string, value interface{}) error {
	dir := filepath.Join(*stateDir, "users", sub)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(dir, name), bytes, 0600)
}

func writeToken(sub string, token *oauth2.Token) error {
	return writeJsonForUser(sub, "token.json", token)
}

func writeDriveFolder(sub string, folder driveFolder) error {
	return writeJsonForUser(sub, "drive_folder.json", &folder)
}

type server struct {
}

type dirState struct {
	Sub               string
	Scanned           bool
	Converted         bool
	UploadedOriginals bool
	UploadedPDF       bool
	Renamed           bool
	NewName           string
}

func examineScansDir() error {
	dirs := make(map[string][]string)
	usersMu.RLock()
	subs := make([]string, 0, len(users))
	for sub, _ := range users {
		subs = append(subs, sub)
	}
	usersMu.RUnlock()
	for _, sub := range subs {
		entries, err := ioutil.ReadDir(filepath.Join(*scansDir, sub))
		if err != nil {
			// Per-user directories do not exist until the user scans their
			// first document.
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dirs[sub] = append(dirs[sub], entry.Name())
		}
	}
	scansNew := make(map[string]dirState)
	for sub, subdirs := range dirs {
		for _, dir := range subdirs {
			subEntries, err := ioutil.ReadDir(filepath.Join(*scansDir, sub, dir))
			// Ignore “File does not exist” errors, the directory might have been
			// deleted in between our original ReadDir() and now.
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			state := dirState{
				Sub: sub,
			}
			for _, subEntry := range subEntries {
				if subEntry.Name() == "COMPLETE.scan" {
					state.Scanned = true
				} else if subEntry.Name() == "COMPLETE.convert" {
					state.Converted = true
				} else if subEntry.Name() == "COMPLETE.uploadoriginals" {
					state.UploadedOriginals = true
				} else if subEntry.Name() == "COMPLETE.uploadpdf" {
					state.UploadedPDF = true
				} else if subEntry.Name() == "COMPLETE.rename" {
					state.Renamed = true
				} else if subEntry.Name() == "rename" {
					content, err := ioutil.ReadFile(filepath.Join(*scansDir, sub, dir, "rename"))
					if err != nil {
						return err
					}
					state.NewName = string(content)
				}
			}
			scansNew[dir] = state
		}
	}
	scansMu.Lock()
	scans = scansNew
	scansMu.Unlock()
	log.Printf("scans = %+v\n", scans)
	return nil
}

func uploadOriginals(ctx context.Context, sub, dir string) error {
	tr, _ := trace.FromContext(ctx)

	entries, err := ioutil.ReadDir(filepath.Join(*scansDir, sub, dir))
	if err != nil {
		return err
	}

	usersMu.RLock()
	driveSrv := users[sub].Drive
	parentId, err := users[sub].getCurrentParentDir()
	if err != nil {
		return err
	}
	usersMu.RUnlock()

	// Trash any folders which have the same name from earlier partial uploads.
	query := fmt.Sprintf("'%s' in parents and name = '%s' and trashed=false", parentId, dir)
	r, err := driveSrv.Files.List().Q(query).PageSize(1).Fields("files(id, name)").Do()
	if err != nil {
		return err
	}

	for _, file := range r.Files {
		log.Printf("Trashing old folder %q\n", file.Id)
		if _, err := driveSrv.Files.Update(file.Id, &drive.File{
			Trashed: true,
		}).Do(); err != nil {
			return err
		}
	}

	rc, err := driveSrv.Files.Create(&drive.File{
		MimeType: "application/vnd.google-apps.folder",
		Name:     dir,
		Parents:  []string{parentId},
	}).Fields("id").Do()
	if err != nil {
		return err
	}
	originalsId := rc.Id
	var wg sync.WaitGroup
	errors := make(chan error, len(entries))
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".jpg" {
			continue
		}
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			f, err := os.Open(filepath.Join(*scansDir, sub, dir, name))
			if err != nil {
				errors <- err
				return
			}
			defer f.Close()

			ru, err := driveSrv.Files.Create(&drive.File{
				Name:    name,
				Parents: []string{originalsId},
			}).Media(f, googleapi.ContentType("image/jpeg")).Do()
			if err != nil {
				errors <- err
				return
			}
			tr.LazyPrintf("Uploaded %q to Google Drive as id %q", name, ru.Id)

		}(entry.Name())
	}
	wg.Wait()
	select {
	case err := <-errors:
		return err
	default:
		return nil
	}
}

func getScanClient() proto.ScanClient {
	if scanConn != nil {
		state, err := scanConn.State()
		if err != nil {
			log.Printf("Error getting -scan_service connectivity state: %v", err)
		} else {
			if state == grpc.Ready {
				return proto.NewScanClient(scanConn)
			}
		}
	}
	return proto.NewScanClient(localScanConn)
}

func convert(ctx context.Context, sub, dir string) error {
	tr, _ := trace.FromContext(ctx)
	files, err := ioutil.ReadDir(filepath.Join(*scansDir, sub, dir))
	if err != nil {
		return err
	}
	req := &proto.ConvertRequest{}
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".jpg" {
			continue
		}
		path := filepath.Join(*scansDir, sub, dir, file.Name())
		tr.LazyPrintf("Reading %q (%d bytes)", path, file.Size())
		contents, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		// TODO: make this optional
		// Prepend (!) the page — the ScanSnap iX500’s document feeder scans
		// the last page first, so we need to reverse the order.
		req.ScannedPage = append([][]byte{contents}, req.ScannedPage...)
	}

	resp, err := getScanClient().Convert(ctx, req)
	if err != nil {
		return err
	}

	pdfPath := filepath.Join(*scansDir, sub, dir, "scan.pdf")
	thumbPath := filepath.Join(*scansDir, sub, dir, "thumb.png")
	tr.LazyPrintf("Converted. Writing %q (%d bytes)", pdfPath, len(resp.PDF))
	if err := ioutil.WriteFile(pdfPath, resp.PDF, 0644); err != nil {
		return err
	}
	return ioutil.WriteFile(thumbPath, resp.Thumb, 0644)
}

func uploadPDF(ctx context.Context, sub, dir string) error {
	tr, _ := trace.FromContext(ctx)

	rd, err := os.Open(filepath.Join(*scansDir, sub, dir, "scan.pdf"))
	if err != nil {
		return err
	}
	defer rd.Close()

	// TODO: make OcrLanguage configurable
	usersMu.RLock()
	driveSrv := users[sub].Drive
	parentId, err := users[sub].getCurrentParentDir()
	if err != nil {
		return err
	}
	usersMu.RUnlock()

	r, err := driveSrv.Files.Create(&drive.File{
		Name:    dir + ".pdf",
		Parents: []string{parentId},
	}).Media(rd, googleapi.ContentType("application/pdf")).OcrLanguage("de").Do()
	if err != nil {
		return err
	}
	tr.LazyPrintf("Uploaded file to Google Drive as id %q", r.Id)

	return nil
}

func rename(ctx context.Context, sub, dir, newName string) error {
	tr, _ := trace.FromContext(ctx)
	fullNewName := dir + "-" + newName
	tr.LazyPrintf("Renaming scan %q to %q", dir, fullNewName)

	usersMu.RLock()
	driveSrv := users[sub].Drive
	parentId, err := users[sub].getCurrentParentDir()
	if err != nil {
		return err
	}
	usersMu.RUnlock()

	query := fmt.Sprintf("'%s' in parents and name contains '%s' and trashed=false", parentId, dir)
	r, err := driveSrv.Files.List().Q(query).PageSize(1).Fields("files(id, name)").Do()
	if err != nil {
		return err
	}

	if len(r.Files) > 1 {
		return fmt.Errorf("Files.List: expected at most 1 result, got %d results", len(r.Files))
	}
	if len(r.Files) == 0 {
		return fmt.Errorf("Files.List: no files matched query %q", query)
	}

	if _, err := driveSrv.Files.Update(r.Files[0].Id, &drive.File{
		Name: fullNewName,
	}).Do(); err != nil {
		return err
	}

	tr.LazyPrintf("Rename successful")

	return nil
}

// Calls log.Panicf on failure instead of returning an error: when we cannot
// record completion of a task, we might as well exit.
func createCompleteMarker(sub, dir, kind string) {
	completePath := filepath.Join(*scansDir, sub, dir, "COMPLETE."+kind)
	if err := ioutil.WriteFile(completePath, []byte{}, 0644); err != nil {
		log.Panicf("Could not write %q: %v", completePath, err)
	}

	scansMu.Lock()
	if state, ok := scans[dir]; ok {
		if kind == "scan" {
			state.Scanned = true
		} else if kind == "convert" {
			state.Converted = true
		} else if kind == "uploadoriginals" {
			state.UploadedOriginals = true
		} else if kind == "uploadpdf" {
			state.UploadedPDF = true
		} else if kind == "rename" {
			state.Renamed = true
		}
		scans[dir] = state
	}
	scansMu.Unlock()
}

func processAllScans() error {
	if err := examineScansDir(); err != nil {
		return err
	}

	scansMu.RLock()
	scansCopy := make(map[string]dirState)
	for dir, state := range scans {
		scansCopy[dir] = state
	}
	scansMu.RUnlock()

	for dir, state := range scansCopy {
		if !state.Scanned {
			log.Printf("Skipping %q (scan not finished)\n", dir)
			continue
		}

		if !state.UploadedOriginals {
			tr := trace.New("Scand", "uploadOriginals")
			ctx := trace.NewContext(context.Background(), tr)
			if err := uploadOriginals(ctx, state.Sub, dir); err != nil {
				log.Printf("Uploading originals from %q failed: %v\n", dir, err)
			} else {
				createCompleteMarker(state.Sub, dir, "uploadoriginals")
			}
			tr.Finish()
		}

		if !state.Converted {
			log.Printf("Converting %q\n", dir)
			tr := trace.New("Scand", "convert")
			ctx := trace.NewContext(context.Background(), tr)
			err := convert(ctx, state.Sub, dir)
			tr.Finish()
			if err != nil {
				log.Printf("Converting %q failed: %v\n", dir, err)
				// Without a successful conversion, there is no PDF to upload.
				continue
			}
			createCompleteMarker(state.Sub, dir, "convert")
		}

		if !state.UploadedPDF {
			tr := trace.New("Scand", "uploadPDF")
			ctx := trace.NewContext(context.Background(), tr)
			if err := uploadPDF(ctx, state.Sub, dir); err != nil {
				log.Printf("Uploading PDF from %q failed: %v\n", dir, err)
			} else {
				createCompleteMarker(state.Sub, dir, "uploadpdf")
			}
			tr.Finish()
		}

		if !state.Renamed && state.NewName != "" {
			tr := trace.New("Scand", "rename")
			ctx := trace.NewContext(context.Background(), tr)
			if err := rename(ctx, state.Sub, dir, state.NewName); err != nil {
				log.Printf("Renaming scan %q failed: %v\n", dir, err)
			} else {
				createCompleteMarker(state.Sub, dir, "rename")
			}
			tr.Finish()
		}
	}

	return examineScansDir()
}

func (s *server) DefaultUser(ctx context.Context, in *proto.DefaultUserRequest) (*proto.DefaultUserReply, error) {
	usersMu.RLock()
	defer usersMu.RUnlock()
	if len(users) == 0 {
		return nil, fmt.Errorf("No users registered")
	} else if len(users) == 1 {
		for sub, _ := range users {
			return &proto.DefaultUserReply{User: sub}, nil
		}
	}
	// TODO: implement a setting for which user is the default user.
	subs := make([]string, 0, len(users))
	for sub, _ := range users {
		subs = append(subs, sub)
	}
	sort.Strings(subs)
	return &proto.DefaultUserReply{User: subs[0]}, nil
}

func (s *server) ProcessScan(ctx context.Context, in *proto.ProcessScanRequest) (*proto.ProcessScanReply, error) {
	dir := in.Dir
	if dir == "" {
		return nil, fmt.Errorf("ProcessScanRequest.Dir must not be empty")
	}

	sub := in.User
	if _, err := os.Stat(filepath.Join(*scansDir, sub, dir)); err != nil {
		return nil, fmt.Errorf("Stat(%q): %v", dir, err)
	}

	// examineScansDir failing is okay. We can still process the scan, the user
	// just won’t see progress.
	// TODO: add a parameter to restrict examineScansDir to just the scan that came in to avoid excessive disk i/o
	scansMu.RLock()
	_, ok := scans[dir]
	scansMu.RUnlock()
	if !ok {
		examineScansDir()
	}

	// Upload the originals in the background while converting (which takes the
	// bulk of the time).
	uploadOriginalsRes := make(chan error)
	go func() {
		uploadOriginalsRes <- uploadOriginals(ctx, sub, dir)
	}()

	if err := convert(ctx, sub, dir); err != nil {
		return nil, fmt.Errorf("Converting: %v", err)
	}
	createCompleteMarker(sub, dir, "convert")

	if err := <-uploadOriginalsRes; err != nil {
		return nil, fmt.Errorf("Uploading originals: %v", err)
	}
	createCompleteMarker(sub, dir, "uploadoriginals")

	if err := uploadPDF(ctx, sub, dir); err != nil {
		return nil, fmt.Errorf("Uploading PDF: %v", err)
	}
	createCompleteMarker(sub, dir, "uploadpdf")

	newName, err := ioutil.ReadFile(filepath.Join(*scansDir, sub, dir, "rename"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		if err := rename(ctx, sub, dir, string(newName)); err != nil {
			return nil, err
		}
	}

	return &proto.ProcessScanReply{}, nil
}

func maybePrefixLocalhost(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}

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

func main() {
	flag.Parse()

	b, err := ioutil.ReadFile(*clientSecretPath)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	oauthConfig, err = oauthConfigFromJSON(b)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(*stateDir, "users"), 0700); err != nil {
		log.Fatal(err)
	}
	if err := readUsers(); err != nil {
		log.Fatal(err)
	}
	log.Printf("users = %+v\n", users)

	refreshTokens()

	cookieSecretPath := filepath.Join(*stateDir, "cookies.key")
	secret, err := ioutil.ReadFile(cookieSecretPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatal(err)
		}
		// NOTE: Not much thought went into chosing 32 bytes as the length of
		// cookie keys. In case there are any arguments for a different number,
		// I’m happy to be convinced.
		secret = securecookie.GenerateRandomKey(32)
		if err := ioutil.WriteFile(cookieSecretPath, secret, 0600); err != nil {
			log.Fatal(err)
		}
	}

	sessionsPath := filepath.Join(*stateDir, "sessions")
	if err := os.MkdirAll(sessionsPath, 0700); err != nil {
		log.Fatal(err)
	}
	store = sessions.NewFilesystemStore(sessionsPath, secret)

	go func() {
		for {
			processAllScans()
			time.Sleep(*processAllInterval)
		}
	}()

	ln, err := net.Listen("tcp", *rpcListenAddr)
	if err != nil {
		log.Fatal(err)
	}

	if *scanService != "" {
		scanConn, err = grpc.Dial(*scanService, grpc.WithInsecure())
		if err != nil {
			log.Fatal(err)
		}
	}
	localScanConn, err = grpc.Dial(maybePrefixLocalhost(*rpcListenAddr), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Listening on %q (gRPC) and http://%s", *rpcListenAddr, maybePrefixLocalhost(*httpListenAddr))

	// TODO: verify method (POST) in each handler
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(*staticDir))))
	http.HandleFunc("/scans_dir/", scansDirHandler)
	http.HandleFunc("/oauth", oauthHandler)
	http.HandleFunc("/signout", signoutHandler)
	http.HandleFunc("/storedrivefolder", storeDriveFolder)
	http.HandleFunc("/startscan", startScanHandler)
	http.HandleFunc("/scanstatus", scanStatusHandler)
	http.HandleFunc("/renamescan", renameScanHandler)
	http.HandleFunc("/", indexHandler)

	go http.ListenAndServe(*httpListenAddr, gorilla_context.ClearHandler(http.DefaultServeMux))
	s := grpc.NewServer()
	proto.RegisterScanServer(s, &server{})
	s.Serve(ln)
}
