// Package write provides a way to atomically create or replace a file.
//
// Caveat: this package requires the file system rename(2) implementation to be
// atomic. Notably, this is not the case on NFS:
// https://stackoverflow.com/a/41396801
package write

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func tempDir(dest string) string {
	if tmpdir := os.Getenv("TMPDIR"); tmpdir != "" {
		return tmpdir
	}

	// Chose the destination directory as temporary directory so that we
	// definitely can rename the file, for which both temporary and destination
	// file need to point to the same mount point.
	return filepath.Dir(dest)
}

// PendingFile is a pending temporary file, waiting to replace the destination
// path in a call to CloseAtomicallyReplace.
type PendingFile struct {
	*os.File

	path string
	done bool
}

// Chmod wraps os.File.Chmod, adding a sync call: fsync(2) after fchmod(2)
// orders writes as per https://lwn.net/Articles/270891/.
//
// Note that ioutil.TempFile, which TempFile wraps, creates files with mode
// 0600, so changing the mode is usually desired.
//
// You can circumvent Chmod and directly call File.Chmod for performance for
// idempotent applications (which only ever atomically write new files and
// tolerate file loss) writing to an ordered file system. ext3, ext4, xfs,
// btrfs, zfs are ordered by default.
func (t *PendingFile) Chmod(mode os.FileMode) error {
	if err := t.File.Chmod(mode); err != nil {
		return err
	}
	return t.File.Sync()
}

// Cleanup is a no-op if CloseAtomicallyReplace succeeded, and otherwise closes
// and removes the temporary file.
func (t *PendingFile) Cleanup() {
	if t.done {
		return
	}
	// An error occurred. Close and remove the tempfile, ignoring errors as
	// there is nothing to recover here.
	t.Close()
	os.Remove(t.Name())
}

// CloseAtomicallyReplace closes the temporary file and atomatically replaces
// the destination file with it, i.e., a concurrent open(2) call will either
// open the file previously located at the destination path (if any), or the
// just written file, but the file will always be present.
//
// Caveat: if the destination file system does not use write barriers, call
// Sync() before CloseAtomicallyReplace() to ensure correct file system recovery
// in the event of a crash.
func (t *PendingFile) CloseAtomicallyReplace() error {
	if err := t.Close(); err != nil {
		return err
	}
	if err := os.Rename(t.Name(), t.path); err != nil {
		return err
	}
	t.done = true
	return nil
}

// TempFile wraps ioutil.TempFile for the use case of atomically creating or
// replacing the destination file at path.
//
// Example:
//     t, err := write.TempFile("/tmp/bar.txt")
//     if err != nil {
//     	log.Fatal(err)
//     }
//     defer t.Cleanup()
//     if _, err := t.Write([]byte("foo")); err != nil {
//     	log.Fatal(err)
//     }
//     if err := t.CloseAtomicallyReplace(); err != nil {
//     	log.Fatal(err)
//     }
func TempFile(path string) (*PendingFile, error) {
	f, err := ioutil.TempFile(tempDir(path), filepath.Base(path))
	if err != nil {
		return nil, err
	}

	return &PendingFile{File: f, path: path}, nil
}
