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

import "sync"

type serverCredentials struct {
	sync.RWMutex
	Creds map[string]credential
}

var globalServerCreds *serverCredentials

func newServerCredentials() *serverCredentials {
	return &serverCredentials{
		Creds: make(map[string]credential),
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
