/*
 * Minio Cloud Storage (C) 2017 Minio, Inc.
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
	"github.com/anaskhan96/soup"
	"golang.org/x/net/html"
)

type htmlNode struct {
	*html.Node
}

func (h htmlNode) GetName() string {
	for _, attr := range h.Attr {
		if attr.Key == "name" {
			return attr.Val
		}
	}
	return ""
}

func (h htmlNode) GetValue() string {
	for _, attr := range h.Attr {
		if attr.Key == "value" {
			return attr.Val
		}
	}
	return ""
}

func extractPayload(root soup.Root) (payload map[string]string) {
	payload = make(map[string]string)
	for _, input := range root.FindAll("input") {
		n := htmlNode{input.Pointer}
		name := n.GetName()
		value := n.GetValue()
		if name == "password" {
			payload[name] = ""
		} else if name == "username" {
			payload[name] = ""
		} else {
			payload[name] = value
		}
	}
	return payload
}

func extractSAMLAssertion(root soup.Root) (samlAssertion string) {
	for _, input := range root.FindAll("input") {
		n := htmlNode{input.Pointer}
		if n.GetName() == "SAMLResponse" {
			samlAssertion = n.GetValue()
			break
		}
	}
	return samlAssertion
}
