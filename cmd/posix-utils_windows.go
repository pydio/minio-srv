// +build windows

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
	"strings"

	os2 "github.com/pydio/minio-srv/pkg/x/os"
)

// Wrapper around safe stat implementation to avoid windows bugs.
func osStat(name string) (os.FileInfo, error) {
	return os2.Stat(name)
}

// isValidVolname verifies a volname name in accordance with object
// layer requirements.
func isValidVolname(volname string) bool {
	if len(volname) < 3 || len(volname) > 63 {
		return false
	}
	// Volname shouldn't have reserved characters on windows in it.
	return !strings.ContainsAny(volname, `\:*?\"<>|`)
}
