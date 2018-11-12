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

package logger

// LoggingTarget is the entity that we will receive
// a single log entry and send it to the log target
//   e.g. send the log to a http server
type LoggingTarget interface {
	send(entry interface{}) error
}

// Targets is the set of enabled loggers
var Targets = []LoggingTarget{}

// AddTarget adds a new logger target to the
// list of enabled loggers
func AddTarget(t LoggingTarget) {
	Targets = append(Targets, t)
}
