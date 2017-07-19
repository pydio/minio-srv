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

import "github.com/minio/minio-go/pkg/policy"

// MakeBucket creates a new container on S3 backend.
func (l *pydioObjects) MakeBucketWithLocation(bucket, location string) error {
	return traceError(NotImplemented{})
}

// DeleteBucket deletes a bucket on S3
func (l *pydioObjects) DeleteBucket(bucket string) error {
	return traceError(NotImplemented{})
}

// SetBucketPolicies sets policy on bucket
func (l *pydioObjects) SetBucketPolicies(bucket string, policyInfo policy.BucketAccessPolicy) error {
	return traceError(NotImplemented{})}

// GetBucketPolicies will get policy on bucket
func (l *pydioObjects) GetBucketPolicies(bucket string) (bap policy.BucketAccessPolicy, e error) {
	return bap, traceError(NotImplemented{})}

// DeleteBucketPolicies deletes all policies on bucket
func (l *pydioObjects) DeleteBucketPolicies(bucket string) error {
	return traceError(NotImplemented{})}

// HealBucket - Not relevant.
func (l *pydioObjects) HealBucket(bucket string) error {
	return traceError(NotImplemented{})
}

// ListBucketsHeal - Not relevant.
func (l *pydioObjects) ListBucketsHeal() (buckets []BucketInfo, err error) {
	return []BucketInfo{}, traceError(NotImplemented{})
}

// HealObject - Not relevant.
func (l *pydioObjects) HealObject(bucket string, object string) (int, int, error) {
	return 0, 0, traceError(NotImplemented{})
}

// ListObjectsHeal - Not relevant.
func (l *pydioObjects) ListObjectsHeal(bucket string, prefix string, marker string, delimiter string, maxKeys int) (loi ListObjectsInfo, e error) {
	return loi, traceError(NotImplemented{})
}

// ListUploadsHeal - Not relevant.
func (l *pydioObjects) ListUploadsHeal(bucket string, prefix string, marker string, uploadIDMarker string, delimiter string, maxUploads int) (lmi ListMultipartsInfo, e error) {
	return lmi, traceError(NotImplemented{})
}