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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/stapelberg/scan2drive/proto"
	"golang.org/x/net/context"
	"golang.org/x/net/trace"
)

func standardDeviation(page []byte) (float64, error) {
	cmd := exec.Command(
		"identify",
		"-format",
		"%[fx:standard_deviation]",
		"/dev/stdin")
	cmd.Stdin = bytes.NewBuffer(page)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(string(out), 64)
	return f, err
}

func tailor(idx int, page []byte, destdir string) error {
	destpath := filepath.Join(destdir, fmt.Sprintf("page%d.tif", idx))

	// scantailor-cli expects to be able to read the input file multiple times,
	// so we need to write to a temporary file instead of feeding the file via
	// stdin.
	tmpfile, err := ioutil.TempFile("", "scan2drive-tailor-input")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(page); err != nil {
		return err
	}
	if err := tmpfile.Close(); err != nil {
		return err
	}

	tmpdir, err := ioutil.TempDir("", "scan2drive-tailor")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	cmd := exec.Command(
		"scantailor-cli",
		"--margins=0",
		"--layout=1",
		"--dpi=600",
		"--content-box=0x0:4960x7016",
		"--rotate=180",
		tmpfile.Name(),
		tmpdir)
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	files, err := ioutil.ReadDir(tmpdir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".tif" {
			return os.Rename(filepath.Join(tmpdir, file.Name()), destpath)
		}
	}
	return fmt.Errorf("Could not find .tif file after running scantailor-cli")
}

func (s *server) Convert(ctx context.Context, in *proto.ConvertRequest) (*proto.ConvertReply, error) {
	tr, _ := trace.FromContext(ctx)

	tmpdir, err := ioutil.TempDir("", "scan2drive-convert")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpdir)

	var wg sync.WaitGroup
	errors := make(chan error, len(in.ScannedPage))
	for idx, page := range in.ScannedPage {
		wg.Add(1)
		go func(idx int, page []byte, tmpdir string) {
			defer wg.Done()
			dev, err := standardDeviation(page)
			if err != nil {
				errors <- err
				return
			}
			blank := dev <= 0.1
			tr.LazyPrintf("standard deviation of page %d is %f, blank = %v", idx, dev, blank)
			if blank {
				return
			}
			if err := tailor(idx, page, tmpdir); err != nil {
				errors <- err
			}
		}(idx, page, tmpdir)
	}
	wg.Wait()
	select {
	case err := <-errors:
		return nil, err
	default:
	}

	files, err := ioutil.ReadDir(tmpdir)
	if err != nil {
		return nil, err
	}
	args := []string{
		"-r",
		"300",
		"--delete",
	}
	for _, file := range files {
		args = append(args, filepath.Join(tmpdir, file.Name()))
	}
	cmd := exec.Command("pdfbeads", args...)
	cmd.Stderr = os.Stderr
	// pdfbeads writes temporary files to $PWD, see
	// https://github.com/ifad/pdfbeads/issues/4
	cmd.Dir = tmpdir
	tr.LazyPrintf("Calling %v", cmd.Args)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	cmd = exec.Command("convert", "/dev/stdin[0]", "png:-")
	cmd.Stdin = bytes.NewBuffer(out)
	cmd.Stderr = os.Stderr
	tr.LazyPrintf("Calling %v", cmd.Args)
	thumb, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return &proto.ConvertReply{PDF: out, Thumb: thumb}, nil
}
