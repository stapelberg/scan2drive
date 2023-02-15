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

// Program scan2drive scans your physical documents as PDF files to Google
// Drive, where you can full-text search them.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/stapelberg/scan2drive"
	"github.com/stapelberg/scan2drive/internal/httpscaningest"
	"github.com/stapelberg/scan2drive/internal/jobqueue"
	"github.com/stapelberg/scan2drive/internal/legacyconvert"
	"github.com/stapelberg/scan2drive/internal/mayqtt"
	"github.com/stapelberg/scan2drive/internal/scaningest"
	"github.com/stapelberg/scan2drive/internal/sink/drivesink"
	"github.com/stapelberg/scan2drive/internal/source/airscan"
	"github.com/stapelberg/scan2drive/internal/source/fss500"
	"github.com/stapelberg/scan2drive/internal/user"
	"github.com/stapelberg/scan2drive/internal/webui"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/trace"
	"golang.org/x/sync/errgroup"

	_ "image/jpeg"
	_ "net/http/pprof"
)

func convert(ctx context.Context, j *jobqueue.Job) error {
	tr, _ := trace.FromContext(ctx)
	pdf, thumb, err := legacyconvert.ConvertLogic(tr, j.Pages())
	if err != nil {
		return err
	}
	tr.LazyPrintf("Converted. Writing scan.pdf (%d bytes)", len(pdf))
	if err := j.AddDerivedFile("scan.pdf", pdf); err != nil {
		return err
	}
	if err := j.AddDerivedFile("thumb.png", thumb); err != nil {
		return err
	}
	return nil
}

func processScan(ctx context.Context, u *user.Account, j *jobqueue.Job) (err error) {
	tr := trace.New("ProcessScan", "job id "+j.Id())
	defer tr.Finish()
	ctx = trace.NewContext(ctx, tr)

	tr.LazyPrintf("Processing job %v (state %v)", j.Id(), j.State())
	defer func() {
		tr.LazyPrintf("-> return err=%v", err)
		if err != nil {
			tr.SetError()
		}
	}()
	tr.LazyPrintf("job markers: %+v", j.Markers)

	// TODO(later): instead of this hard-coded conversion, perhaps allow
	// configuring a processing graph of sorts?

	// TODO(later): make the sinks pluggable, start with an external process
	// sink

	// Upload the originals in the background while converting (which takes the
	// bulk of the time).
	uploadOriginalsRes := make(chan error)
	if !j.Markers.UploadedOriginals {
		mayqtt.Publishf("processing %d pages", len(j.Pages()))
		go func() {
			uploadOriginalsRes <- drivesink.UploadOriginals(ctx, u, j)
		}()
	}

	if !j.Markers.Converted {
		// convert does binarize, rotate (make conditional!), g3 encoding, PDF
		// writing, and PNG thumbnail creation.
		if err := convert(ctx, j); err != nil {
			// TODO: remove this duplication with the lines below:
			if !j.Markers.UploadedOriginals {
				if err := <-uploadOriginalsRes; err != nil {
					return fmt.Errorf("Uploading originals: %v", err)
				}
				if err := j.CommitMarker("uploadoriginals"); err != nil {
					return err
				}
				tr.LazyPrintf("job markers now: %+v", j.Markers)
			}
			return err
		}
		if err := j.CommitMarker("convert"); err != nil {
			return err
		}
		tr.LazyPrintf("job markers now: %+v", j.Markers)
	}

	if !j.Markers.UploadedOriginals {
		if err := <-uploadOriginalsRes; err != nil {
			return fmt.Errorf("Uploading originals: %v", err)
		}
		if err := j.CommitMarker("uploadoriginals"); err != nil {
			return err
		}
		tr.LazyPrintf("job markers now: %+v", j.Markers)
	}

	if !j.Markers.UploadedPDF {
		mayqtt.Publishf("uploading PDF")
		if err := drivesink.UploadPDF(ctx, u, j); err != nil {
			return fmt.Errorf("Uploading PDF: %v", err)
		}
		if err := j.CommitMarker("uploadpdf"); err != nil {
			return err
		}
		tr.LazyPrintf("job markers now: %+v", j.Markers)
	}

	mayqtt.Publishf("scanner ready")

	// newName, err := ioutil.ReadFile(filepath.Join(*scansDir, sub, dir, "rename"))
	// if err != nil && !os.IsNotExist(err) {
	// 	return nil, err
	// }
	// if err == nil {
	// 	if err := rename(ctx, sub, dir, string(newName)); err != nil {
	// 		return nil, err
	// 	}
	// }

	return nil
}

func dispatchScanRequest(ingester *scaningest.Ingester, finders []scan2drive.ScanSourceFinder, scanRequest *scan2drive.ScanRequest) error {
	tr := trace.New("scan2drive", "DispatchScanRequest")
	defer tr.Finish()

	for _, finder := range finders {
		srcs := finder.CurrentScanSources()
		tr.LazyPrintf("finder discovered %d scan sources", len(srcs))
		for _, src := range srcs {
			if err := src.CanProcess(scanRequest); err != nil {
				tr.LazyPrintf("skipping source: %v", err)
				continue
			}
			tr.LazyPrintf("scanning to source")
			if _, err := src.ScanTo(ingester); err != nil {
				return fmt.Errorf("scan failed: %v", err)
			}
			return nil
		}
	}
	return fmt.Errorf("no scan source found")
}

func logic() error {
	stateDir := flag.String("state_dir",
		"/perm/scan2drive-state",
		"Directory containing state such as session data and OAuth tokens for communicating with Google Drive or the destination folder id. If wiped, users will need to re-login and chose their drive folder again.")

	scansDir := flag.String("scans_dir",
		"/perm/scans",
		"Directory in which -scan_script places scanned documents.")

	clientSecretPath := flag.String("client_secret_path",
		"/perm/client_secret.json",
		"Path to a client secret JSON file as described in https://developers.google.com/drive/v3/web/quickstart/go#step_1_turn_on_the_api_name")

	httpListenAddr := flag.String("http_listen_address",
		"localhost:7120",
		"[host]:port to listen on for HTTP requests")

	httpsListenAddr := flag.String("https_listen_address",
		":https",
		"[host]:port to listen on for HTTPS requests. This is a no-op unless -tls_autocert_hosts is non-empty.")

	autocertHostList := flag.String("tls_autocert_hosts",
		"",
		"If non-empty, a comma-separated list of hostnames to obtain TLS certificates for. If non-empty, a TLS listener will be enabled on -https_listen_address")

	allowedUsersList := flag.String("allowed_users_list",
		"",
		"If non-empty, a comma-separated list of users who are permitted to log in")

	tailscaleHostname := flag.String("tailscale_hostname", "scan2drive", "tailscale hostname")
	tailscaleAllowedUser := flag.String("tailscale_allowed_user", "", "the name of a tailscale user to allow")

	flag.Parse()
	_ = tailscaleHostname
	_ = tailscaleAllowedUser

	log.Printf("scan2drive starting")

	mqttScanRequests := make(chan *scan2drive.ScanRequest, 1)
	// makes mayqtt.Publishf() work as a side effect:
	mayqtt.MQTT(mqttScanRequests)

	// We have one sequential job queue runner, as the resources on the
	// Raspberry Pi are constrained enough that multiple concurrent scan jobs
	// are not a great idea.
	type workJob struct {
		user *user.Account
		job  *jobqueue.Job
	}
	workJobs := make(chan workJob)
	eg, ctx := errgroup.WithContext(context.Background())
	eg.Go(func() error {
		for workJob := range workJobs {
			if err := processScan(ctx, workJob.user, workJob.job); err != nil {
				log.Printf("job %v failed: %v", workJob.job.Id(), err)
			}
		}
		return nil
	})

	lockedUsers := user.NewLocked()

	ingesterFor := func(uid string) *scaningest.Ingester {
		user := lockedUsers.User(uid)
		if user == nil {
			return nil
		}
		return &scaningest.Ingester{
			IngestCallback: func(j *scaningest.Job) (string, error) {
				// Persist the job into the reliable job queue (on persistent
				// storage) and let the queue worker take it from here.

				log.Printf("ingest(%d pages)", len(j.Pages))
				job, err := user.Queue.AddJob(j.Pages)
				if err != nil {
					return "", err
				}
				log.Printf("enqueuing job %v", job.Id())
				workJobs <- workJob{
					user: user,
					job:  job,
				}
				log.Printf("job %v enqueued!", job.Id())
				return job.Id(), nil
			},
		}
	}

	ingesterForDefault := func() *scaningest.Ingester {
		users := lockedUsers.Users()
		for uid, user := range users {
			if user.Default {
				return ingesterFor(uid)
			}
		}
		for uid := range users {
			return ingesterFor(uid)
		}
		return nil
	}

	var finders []scan2drive.ScanSourceFinder
	finders = append(finders, airscan.SourceFinder())
	if runtime.GOOS == "linux" {
		finders = append(finders, fss500.SourceFinder(ingesterForDefault))
	}

	// email to authorized
	allowedUsers := make(map[string]bool)
	for _, email := range strings.Split(*allowedUsersList, ",") {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		allowedUsers[email] = true
	}

	var listenURLs []string

	type serveFunc struct {
		serve    func() error
		shutdown func() error
	}
	var serveFuncs []serveFunc

	if *autocertHostList != "" {
		// Start HTTPS listener with autocert
		var hosts []string
		for _, host := range strings.Split(*autocertHostList, ",") {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}
			hosts = append(hosts, host)
		}

		m := &autocert.Manager{
			Cache:      autocert.DirCache(filepath.Join(*stateDir, "autocert")),
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(hosts...),
		}
		s := &http.Server{
			Addr:      *httpsListenAddr,
			TLSConfig: m.TLSConfig(),
		}
		for _, host := range hosts {
			log.Printf("listening on https://%s", host)
			listenURLs = append(listenURLs, "https://"+host)
		}

		ln, err := net.Listen("tcp", s.Addr)
		if err != nil {
			return err
		}
		serveFuncs = append(serveFuncs, serveFunc{
			serve: func() error {
				defer ln.Close()

				return s.ServeTLS(ln, "", "")
			},
			shutdown: func() error {
				timeout, canc := context.WithTimeout(context.Background(), 250*time.Millisecond)
				defer canc()
				return s.Shutdown(timeout)
			},
		})
	}

	// HTTP listener (local network)
	ln, err := net.Listen("tcp", *httpListenAddr)
	if err != nil {
		return err
	}
	addr := ln.Addr().String()
	if host, port, err := net.SplitHostPort(addr); err == nil {
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			addr = "localhost"
			if port != "" {
				addr += ":" + port
			}
		}
	} else if strings.HasPrefix(addr, "[::]") {
		host, _ := os.Hostname()
		if host == "" {
			host = "localhost"
		}
		addr = host + strings.TrimPrefix(addr, "[::]")
	}
	log.Printf("listening on http://%s", addr)
	listenURLs = append(listenURLs, "http://"+addr)
	serveFuncs = append(serveFuncs, serveFunc{
		serve: func() error {
			return http.Serve(ln, nil)
		},
		shutdown: func() error {
			// TODO: Shutdown()
			return nil
		},
	})

	// - TODO(later): tailscale listener

	webuiHandler, oauthConfig, err := webui.Init(&webui.Config{
		ClientSecretPath: *clientSecretPath,
		LockedUsers:      lockedUsers,
		ScansDir:         *scansDir,
		StateDir:         *stateDir,
		Finders:          finders,
		IngesterFor:      ingesterFor,
		AllowedUsers:     allowedUsers,
		ListenURLs:       listenURLs,
	})
	if err != nil {
		return err
	}

	if err := lockedUsers.UpdateFromDir(*stateDir, *scansDir, oauthConfig); err != nil {
		return err
	}
	go func() {
		// start after a brief delay to not slow down startup
		time.Sleep(5 * time.Second)
		// Try to resume incomplete jobs:
		for {
			users := lockedUsers.Users()
			for _, user := range users {
				// Read all scan jobs from disk:
				scans, err := user.Queue.Scans()
				if err != nil {
					log.Print(err)
					continue
				}
				for _, job := range scans {
					if job.State() == jobqueue.Done {
						continue
					}
					log.Printf("enqueuing unfinished job %s", job.Id())
					workJobs <- workJob{
						user: user,
						job:  job,
					}
				}
			}
			log.Printf("waiting 1 hour before retrying any unfinished jobs")
			time.Sleep(1 * time.Hour)
		}
	}()

	// Impulse/Trigger: MQTT
	{
		go func() {
			for scanRequest := range mqttScanRequests {
				user := lockedUsers.UserByName(scanRequest.User)
				ingester := ingesterFor(user.Sub)
				if ingester == nil {
					log.Printf("user %q not found", scanRequest.User)
					continue
				}
				if err := dispatchScanRequest(ingester, finders, scanRequest); err != nil {
					log.Printf("dispatchScanRequest: %v", err)
				}
			}
		}()
	}

	// - TODO(later): grpc scaningest API

	// Impulse/Trigger: HTTP API
	{
		defaultIngester := ingesterForDefault()
		serveMux := httpscaningest.ServeMux(defaultIngester)
		http.Handle("/api/", http.StripPrefix("/api", serveMux))
	}

	// Web user interface (can trigger scans, too)
	http.Handle("/", webuiHandler)

	// for /debug/requests:
	trace.AuthRequest = func(req *http.Request) (bool, bool) {
		// RemoteAddr is commonly in the form "IP" or "IP:port".
		// If it is in the form "IP:port", split off the port.
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			host = req.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return false, false
		}
		if ip.IsPrivate() {
			return true, true
		}
		return false, false
	}

	for _, sf := range serveFuncs {
		sf := sf // copy
		eg.Go(func() error {
			errC := make(chan error)
			go func() {
				errC <- sf.serve()
			}()
			select {
			case err := <-errC:
				return err
			case <-ctx.Done():
				if err := sf.shutdown(); err != nil {
					log.Printf("shutting down listener: %v", err)
				}
				return ctx.Err()
			}
		})
	}

	return eg.Wait()
}

func main() {
	gokrazyInit()
	if err := logic(); err != nil {
		log.Fatal(err)
	}
}
