/*
 * Minio Cloud Storage, (C) 2017 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/minio/minio/pkg/quick"
)

type serverCredentials struct {
	sync.RWMutex
	Version string                `json:"version"`
	Creds   map[string]credential `json:"creds"`
}

var globalServerCreds *serverCredentials

// Minio credentials file.
const minioCredsFile = "creds.json"

func newServerCredentials() *serverCredentials {
	return &serverCredentials{
		Version: "1",
		Creds:   make(map[string]credential),
	}
}

func (s *serverCredentials) SetCredential(cred credential) {
	s.Lock()
	defer s.Unlock()

	s.Creds[cred.AccessKey] = cred
}

func (s *serverCredentials) GetCredential(accessKey string) credential {
	s.RLock()
	defer s.RUnlock()

	return s.Creds[accessKey]
}

func (s *serverCredentials) RemoveCredential(accessKey string) {
	s.Lock()
	defer s.Unlock()

	delete(s.Creds, accessKey)
}

func (s *serverCredentials) Load() error {
	_, err := quick.Load(filepath.Join(configDir.Get(), minioCredsFile), s)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// If creds.json doesn't exist, its okay to proceed and ignore.
	return nil
}

func (s *serverCredentials) Save() error {
	s.Lock()
	defer s.Unlock()
	// Purge all the expired entries before saving.
	for k, v := range s.Creds {
		if v.IsExpired() {
			delete(s.Creds, k)
		}
	}
	return quick.Save(filepath.Join(configDir.Get(), minioCredsFile), s)
}
