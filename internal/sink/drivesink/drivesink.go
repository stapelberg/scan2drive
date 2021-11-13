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

// Package drivesink implements a sink to write scans to Google Drive.
package drivesink

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/stapelberg/scan2drive/internal/jobqueue"
	"github.com/stapelberg/scan2drive/internal/user"
	"golang.org/x/net/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

func getCurrentParentDir(u *user.Account) (string, error) {
	year := fmt.Sprintf("%d", time.Now().Year())
	query := fmt.Sprintf("'%s' in parents and name = '%s' and trashed=false", u.Folder().Id, year)
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
		Parents:  []string{u.Folder().Id},
	}).Fields("id").Do()
	if err != nil {
		return "", err
	}
	return rc.Id, nil
}

func UploadOriginals(ctx context.Context, u *user.Account, j *jobqueue.Job) error {
	tr, _ := trace.FromContext(ctx)

	dir := j.Id()

	filenames, err := j.Filenames()
	if err != nil {
		return err
	}

	driveSrv := u.Drive
	parentId, err := getCurrentParentDir(u)
	if err != nil {
		return err
	}

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
	var eg errgroup.Group
	for _, filename := range filenames {
		filename := filename // copy
		name := filepath.Base(filename)
		if filepath.Ext(name) != ".jpg" {
			continue
		}

		eg.Go(func() error {
			f, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer f.Close()

			ru, err := driveSrv.Files.Create(&drive.File{
				Name:    name,
				Parents: []string{originalsId},
			}).Media(f, googleapi.ContentType("image/jpeg")).Do()
			if err != nil {
				return err
			}
			tr.LazyPrintf("Uploaded %q to Google Drive as id %q", name, ru.Id)
			return nil
		})
	}
	return eg.Wait()
}

func UploadPDF(ctx context.Context, u *user.Account, j *jobqueue.Job) error {
	tr, _ := trace.FromContext(ctx)

	// TODO: make OcrLanguage configurable
	driveSrv := u.Drive
	parentId, err := getCurrentParentDir(u)
	if err != nil {
		return err
	}

	filenames, err := j.Filenames()
	if err != nil {
		return err
	}
	for _, filename := range filenames {
		if filepath.Base(filename) != "scan.pdf" {
			continue
		}

		rd, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer rd.Close()

		r, err := driveSrv.Files.Create(&drive.File{
			Name:    j.Id() + ".pdf",
			Parents: []string{parentId},
		}).Media(rd, googleapi.ContentType("application/pdf")).OcrLanguage("de").Do()
		if err != nil {
			return err
		}
		tr.LazyPrintf("Uploaded file to Google Drive as id %q", r.Id)

		if err := j.WritePDFDriveID(r.Id); err != nil {
			return err
		}

		break
	}
	return nil
}
