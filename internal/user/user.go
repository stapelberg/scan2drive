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

// Package user implements a mutex-protected user store (containing metadata,
// credentials, and job queue) that is persisted to disk.
package user

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/stapelberg/scan2drive"
	"github.com/stapelberg/scan2drive/internal/jobqueue"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

type Account struct {
	Queue *jobqueue.Queue
	Sub   string

	// old attributes below:

	Token   *oauth2.Token
	folder  scan2drive.DriveFolder
	Drive   *drive.Service
	Default bool

	Name    string // full name, e.g. “Michael Stapelberg”
	Picture string // profile picture URL
}

func (a *Account) Folder() scan2drive.DriveFolder {
	if a == nil {
		return scan2drive.DriveFolder{}
	}
	return a.folder
}

func (a *Account) LoggedIn() bool {
	return a != nil && a.Token != nil
}

func LoadFromDir(dir string) (*Account, error) {
	var account Account

	{
		bytes, err := os.ReadFile(filepath.Join(dir, "token.json"))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(bytes, &account.Token); err != nil {
			return nil, err
		}
	}

	{
		// Try to read drive_folder.json if it exists.
		bytes, err := os.ReadFile(filepath.Join(dir, "drive_folder.json"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil {
			if err := json.Unmarshal(bytes, &account.folder); err != nil {
				return nil, err
			}
		}
	}

	{
		if _, err := os.Stat(filepath.Join(dir, "is_default")); err == nil {
			account.Default = true
		}
	}

	return &account, nil
}

type Locked struct {
	mu    sync.Mutex
	users map[string]*Account
}

func NewLocked() *Locked {
	return &Locked{}
}

func (l *Locked) User(sub string) *Account {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.users[sub]
}

func (l *Locked) Users() map[string]*Account {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.users
}

func (l *Locked) UserByName(name string) *Account {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, user := range l.users {
		if user.Name == name {
			return user
		}
	}
	return nil
}

func (l *Locked) UpdateFromDir(stateDir, scansDir string, oauthConfig *oauth2.Config) error {
	newUsers := make(map[string]*Account)
	usersPath := filepath.Join(stateDir, "users")
	entries, err := ioutil.ReadDir(usersPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no users, nothing to load
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sub := entry.Name()
		account, err := LoadFromDir(filepath.Join(usersPath, sub))
		if err != nil {
			// User directories without token.json are considered logged-out.
			// drive_folder.json is still persisted so that users don’t have to
			// re-select a drive folder when logging in again.
			if os.IsNotExist(err) {
				continue
			}

			return err
		}
		// TODO: when reading LoadAllUsers after initial startup, don’t
		// re-create this client from scratch but use the existing one.

		// usersMu.RLock()
		// oldToken := users[sub].Token
		// oldDrive := users[sub].Drive
		// usersMu.RUnlock()
		// if oldToken != nil &&
		// 	oldToken.AccessToken == state.Token.AccessToken &&
		// 	oldToken.RefreshToken == state.Token.RefreshToken {
		// 	state.Drive = oldDrive
		// } else {

		// NOTE: As per https://github.com/golang/oauth2/issues/84,
		// Google’s RefreshTokens stay valid until they are revoked, so
		// there is no need to ever update the token on disk.
		ctx := context.Background()
		httpClient := oauthConfig.Client(ctx, account.Token)
		dsrv, err := drive.New(httpClient)
		if err != nil {
			return fmt.Errorf("creating drive client for %q: %v", sub, err)
		}
		account.Drive = dsrv

		osrv, err := oauth2api.NewService(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			return fmt.Errorf("creating oauth2 client for %q: %v", sub, err)
		}
		o, err := osrv.Userinfo.Get().Do()
		if err != nil {
			return fmt.Errorf("getting userinfo for %q: %v", sub, err)
		}
		account.Name = o.Name
		account.Picture = o.Picture
		account.Sub = sub

		dir := filepath.Join(scansDir, sub)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating local scans directory for %q: %v", sub, err)
		}
		account.Queue = &jobqueue.Queue{Dir: dir}

		newUsers[sub] = account
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.users = newUsers
	return nil
}
