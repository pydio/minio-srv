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

import "io"

// AnonGetObject - Get object anonymously
func (l *gcsGateway) AnonGetObject(bucket string, key string, startOffset int64, length int64, writer io.Writer) error {
	return NotImplemented{}
}

// AnonGetObjectInfo - Get object info anonymously
func (l *gcsGateway) AnonGetObjectInfo(bucket string, object string) (ObjectInfo, error) {
	return ObjectInfo{}, NotImplemented{}
}

// AnonListObjects - List objects anonymously
func (l *gcsGateway) AnonListObjects(bucket string, prefix string, marker string, delimiter string, maxKeys int) (ListObjectsInfo, error) {
	return ListObjectsInfo{}, NotImplemented{}
}

// AnonListObjectsV2 - List objects in V2 mode, anonymously
func (l *gcsGateway) AnonListObjectsV2(bucket, prefix, continuationToken string, fetchOwner bool, delimiter string, maxKeys int) (ListObjectsV2Info, error) {
	// Request V1 List Object to the backend
	result, err := l.anonClient.ListObjects(bucket, prefix, continuationToken, delimiter, maxKeys)
	if err != nil {
		return ListObjectsV2Info{}, s3ToObjectError(traceError(err), bucket)
	}
	// translate V1 Result to V2Info
	return fromMinioClientListBucketResultToV2Info(bucket, result), nil
}

// AnonGetBucketInfo - Get bucket metadata anonymously.
func (l *gcsGateway) AnonGetBucketInfo(bucket string) (BucketInfo, error) {
	return BucketInfo{}, NotImplemented{}
}
