/*
 * Minio Cloud Storage, (C) 2018 Minio, Inc.
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

package condition

import (
	"encoding/json"
	"fmt"
)

type name string

const (
	stringEquals    name = "StringEquals"
	stringNotEquals      = "StringNotEquals"
	stringLike           = "StringLike"
	stringNotLike        = "StringNotLike"
	ipAddress            = "IpAddress"
	notIPAddress         = "NotIpAddress"
	null                 = "Null"
)

// IsValid - checks if name is valid or not.
func (n name) IsValid() bool {
	switch n {
	case stringEquals, stringNotEquals, stringLike, stringNotLike, ipAddress, notIPAddress, null:
		return true
	}

	return false
}

// MarshalJSON - encodes name to JSON data.
func (n name) MarshalJSON() ([]byte, error) {
	if !n.IsValid() {
		return nil, fmt.Errorf("invalid name %v", n)
	}

	return json.Marshal(string(n))
}

// UnmarshalJSON - decodes JSON data to condition name.
func (n *name) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsedName, err := parseName(s)
	if err != nil {
		return err
	}

	*n = parsedName
	return nil
}

func parseName(s string) (name, error) {
	n := name(s)

	if n.IsValid() {
		return n, nil
	}

	return n, fmt.Errorf("invalid condition name '%v'", s)
}
