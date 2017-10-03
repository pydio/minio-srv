/*
 * Minio Cloud Storage, (C) 2016 Minio, Inc.
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
 *
 */

// Package madmin_test
package madmin_test

import (
	"testing"

	"github.com/pydio/minio-priv/pkg/madmin"
)

func TestMinioAdminClient(t *testing.T) {
	_, err := madmin.New("localhost:9000", "food", "food123", true)
	if err != nil {
		t.Fatal(err)
	}
}
