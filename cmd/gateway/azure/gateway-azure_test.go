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

package azure

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/storage"
	minio "github.com/minio/minio/cmd"
)

// Test canonical metadata.
func TestS3MetaToAzureProperties(t *testing.T) {
	headers := map[string]string{
		"accept-encoding":          "gzip",
		"content-encoding":         "gzip",
		"cache-control":            "age: 3600",
		"content-disposition":      "dummy",
		"content-length":           "10",
		"content-type":             "application/javascript",
		"X-Amz-Meta-Hdr":           "value",
		"X-Amz-Meta-X_test_key":    "value",
		"X-Amz-Meta-X__test__key":  "value",
		"X-Amz-Meta-X-Test__key":   "value",
		"X-Amz-Meta-X-Amz-Key":     "hu3ZSqtqwn+aL4V2VhAeov4i+bG3KyCtRMSXQFRHXOk=",
		"X-Amz-Meta-X-Amz-Matdesc": "{}",
		"X-Amz-Meta-X-Amz-Iv":      "eWmyryl8kq+EVnnsE7jpOg==",
	}
	// Only X-Amz-Meta- prefixed entries will be returned in
	// Metadata (without the prefix!)
	expectedHeaders := map[string]string{
		"Hdr":              "value",
		"X__test__key":     "value",
		"X____test____key": "value",
		"X_Test____key":    "value",
		"X_Amz_Key":        "hu3ZSqtqwn+aL4V2VhAeov4i+bG3KyCtRMSXQFRHXOk=",
		"X_Amz_Matdesc":    "{}",
		"X_Amz_Iv":         "eWmyryl8kq+EVnnsE7jpOg==",
	}
	meta, _, err := s3MetaToAzureProperties(context.Background(), headers)
	if err != nil {
		t.Fatalf("Test failed, with %s", err)
	}
	if !reflect.DeepEqual(map[string]string(meta), expectedHeaders) {
		t.Fatalf("Test failed, expected %#v, got %#v", expectedHeaders, meta)
	}
	headers = map[string]string{
		"invalid--meta": "value",
	}
	_, _, err = s3MetaToAzureProperties(context.Background(), headers)
	if err != nil {
		if _, ok := err.(minio.UnsupportedMetadata); !ok {
			t.Fatalf("Test failed with unexpected error %s, expected UnsupportedMetadata", err)
		}
	}

	headers = map[string]string{
		"content-md5": "Dce7bmCX61zvxzP5QmfelQ==",
	}
	_, props, err := s3MetaToAzureProperties(context.Background(), headers)
	if err != nil {
		t.Fatalf("Test failed, with %s", err)
	}
	if props.ContentMD5 != headers["content-md5"] {
		t.Fatalf("Test failed, expected %s, got %s", headers["content-md5"], props.ContentMD5)
	}
}

func TestAzurePropertiesToS3Meta(t *testing.T) {
	// Just one testcase. Adding more test cases does not add value to the testcase
	// as azureToS3Metadata() just adds a prefix.
	metadata := map[string]string{
		"first_name":       "myname",
		"x_test_key":       "value",
		"x_test__key":      "value",
		"x__test__key":     "value",
		"x____test____key": "value",
		"x_amz_key":        "hu3ZSqtqwn+aL4V2VhAeov4i+bG3KyCtRMSXQFRHXOk=",
		"x_amz_matdesc":    "{}",
		"x_amz_iv":         "eWmyryl8kq+EVnnsE7jpOg==",
	}
	expectedMeta := map[string]string{
		"X-Amz-Meta-First-Name":    "myname",
		"X-Amz-Meta-X-Test-Key":    "value",
		"X-Amz-Meta-X-Test_key":    "value",
		"X-Amz-Meta-X_test_key":    "value",
		"X-Amz-Meta-X__test__key":  "value",
		"X-Amz-Meta-X-Amz-Key":     "hu3ZSqtqwn+aL4V2VhAeov4i+bG3KyCtRMSXQFRHXOk=",
		"X-Amz-Meta-X-Amz-Matdesc": "{}",
		"X-Amz-Meta-X-Amz-Iv":      "eWmyryl8kq+EVnnsE7jpOg==",
		"Cache-Control":            "max-age: 3600",
		"Content-Disposition":      "dummy",
		"Content-Encoding":         "gzip",
		"Content-Length":           "10",
		"Content-MD5":              "base64-md5",
		"Content-Type":             "application/javascript",
	}
	actualMeta := azurePropertiesToS3Meta(metadata, storage.BlobProperties{
		CacheControl:       "max-age: 3600",
		ContentDisposition: "dummy",
		ContentEncoding:    "gzip",
		ContentLength:      10,
		ContentMD5:         "base64-md5",
		ContentType:        "application/javascript",
	})
	if !reflect.DeepEqual(actualMeta, expectedMeta) {
		t.Fatalf("Test failed, expected %#v, got %#v", expectedMeta, actualMeta)
	}
}

// Add tests for azure to object error.
func TestAzureToObjectError(t *testing.T) {
	testCases := []struct {
		actualErr      error
		expectedErr    error
		bucket, object string
	}{
		{
			nil, nil, "", "",
		},
		{
			fmt.Errorf("Non azure error"),
			fmt.Errorf("Non azure error"), "", "",
		},
		{
			storage.AzureStorageServiceError{
				Code: "ContainerAlreadyExists",
			}, minio.BucketExists{Bucket: "bucket"}, "bucket", "",
		},
		{
			storage.AzureStorageServiceError{
				Code: "InvalidResourceName",
			}, minio.BucketNameInvalid{Bucket: "bucket."}, "bucket.", "",
		},
		{
			storage.AzureStorageServiceError{
				Code: "RequestBodyTooLarge",
			}, minio.PartTooBig{}, "", "",
		},
		{
			storage.AzureStorageServiceError{
				Code: "InvalidMetadata",
			}, minio.UnsupportedMetadata{}, "", "",
		},
		{
			storage.AzureStorageServiceError{
				StatusCode: http.StatusNotFound,
			}, minio.ObjectNotFound{
				Bucket: "bucket",
				Object: "object",
			}, "bucket", "object",
		},
		{
			storage.AzureStorageServiceError{
				StatusCode: http.StatusNotFound,
			}, minio.BucketNotFound{Bucket: "bucket"}, "bucket", "",
		},
		{
			storage.AzureStorageServiceError{
				StatusCode: http.StatusBadRequest,
			}, minio.BucketNameInvalid{Bucket: "bucket."}, "bucket.", "",
		},
	}
	for i, testCase := range testCases {
		if err := azureToObjectError(testCase.actualErr, testCase.bucket, testCase.object); err != nil {
			if err.Error() != testCase.expectedErr.Error() {
				t.Errorf("Test %d: Expected error %s, got %s", i+1, testCase.expectedErr, err)
			}
		}
	}
}

// Test azureGetBlockID().
func TestAzureGetBlockID(t *testing.T) {
	testCases := []struct {
		partID        int
		subPartNumber int
		uploadID      string
		md5           string
		blockID       string
	}{
		{1, 7, "f328c35cad938137", "d41d8cd98f00b204e9800998ecf8427e", "MDAwMDEuMDcuZjMyOGMzNWNhZDkzODEzNy5kNDFkOGNkOThmMDBiMjA0ZTk4MDA5OThlY2Y4NDI3ZQ=="},
		{2, 19, "abcdc35cad938137", "a7fb6b7b36ee4ed66b5546fac4690273", "MDAwMDIuMTkuYWJjZGMzNWNhZDkzODEzNy5hN2ZiNmI3YjM2ZWU0ZWQ2NmI1NTQ2ZmFjNDY5MDI3Mw=="},
	}
	for _, test := range testCases {
		blockID := azureGetBlockID(test.partID, test.subPartNumber, test.uploadID, test.md5)
		if blockID != test.blockID {
			t.Fatalf("%s is not equal to %s", blockID, test.blockID)
		}
	}
}

// Test azureParseBlockID().
func TestAzureParseBlockID(t *testing.T) {
	testCases := []struct {
		blockID       string
		partID        int
		subPartNumber int
		uploadID      string
		md5           string
		success       bool
	}{
		// Invalid base64.
		{"MDAwMDEuMDcuZjMyOGMzNWNhZDkzODEzNy5kNDFkOGNkOThmMDBiMjA0ZTk4MDA5OThlY2Y4NDI3ZQ=", 0, 0, "", "", false},
		// Invalid number of tokens.
		{"MDAwMDEuQUEuZjMyOGMzNWNhZDkzODEzNwo=", 0, 0, "", "", false},
		// Invalid encoded part ID.
		{"MDAwMGEuMDcuZjMyOGMzNWNhZDkzODEzNy5kNDFkOGNkOThmMDBiMjA0ZTk4MDA5OThlY2Y4NDI3ZQo=", 0, 0, "", "", false},
		// Invalid sub part ID.
		{"MDAwMDEuQUEuZjMyOGMzNWNhZDkzODEzNy5kNDFkOGNkOThmMDBiMjA0ZTk4MDA5OThlY2Y4NDI3ZQo=", 0, 0, "", "", false},
		{"MDAwMDEuMDcuZjMyOGMzNWNhZDkzODEzNy5kNDFkOGNkOThmMDBiMjA0ZTk4MDA5OThlY2Y4NDI3ZQ==", 1, 7, "f328c35cad938137", "d41d8cd98f00b204e9800998ecf8427e", true},
		{"MDAwMDIuMTkuYWJjZGMzNWNhZDkzODEzNy5hN2ZiNmI3YjM2ZWU0ZWQ2NmI1NTQ2ZmFjNDY5MDI3Mw==", 2, 19, "abcdc35cad938137", "a7fb6b7b36ee4ed66b5546fac4690273", true},
	}
	for i, test := range testCases {
		partID, subPartNumber, uploadID, md5, err := azureParseBlockID(test.blockID)
		if err != nil && test.success {
			t.Errorf("Test %d: Expected success but failed %s", i+1, err)
		}
		if err == nil && !test.success {
			t.Errorf("Test %d: Expected to fail but succeeeded insteadl", i+1)
		}
		if err == nil {
			if partID != test.partID {
				t.Errorf("Test %d: %d not equal to %d", i+1, partID, test.partID)
			}
			if subPartNumber != test.subPartNumber {
				t.Errorf("Test %d: %d not equal to %d", i+1, subPartNumber, test.subPartNumber)
			}
			if uploadID != test.uploadID {
				t.Errorf("Test %d: %s not equal to %s", i+1, uploadID, test.uploadID)
			}
			if md5 != test.md5 {
				t.Errorf("Test %d: %s not equal to %s", i+1, md5, test.md5)
			}
		}
	}
}

func TestAnonErrToObjectErr(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		params     []string
		wantErr    error
	}{
		{"ObjectNotFound",
			http.StatusNotFound,
			[]string{"testBucket", "testObject"},
			minio.ObjectNotFound{Bucket: "testBucket", Object: "testObject"},
		},
		{"BucketNotFound",
			http.StatusNotFound,
			[]string{"testBucket", ""},
			minio.BucketNotFound{Bucket: "testBucket"},
		},
		{"ObjectNameInvalid",
			http.StatusBadRequest,
			[]string{"testBucket", "testObject"},
			minio.ObjectNameInvalid{Bucket: "testBucket", Object: "testObject"},
		},
		{"BucketNameInvalid",
			http.StatusBadRequest,
			[]string{"testBucket", ""},
			minio.BucketNameInvalid{Bucket: "testBucket"},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			if err := minio.AnonErrToObjectErr(test.statusCode, test.params...); !reflect.DeepEqual(err, test.wantErr) {
				t.Errorf("anonErrToObjectErr() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestCheckAzureUploadID(t *testing.T) {
	invalidUploadIDs := []string{
		"123456789abcdefg",
		"hello world",
		"0x1234567890",
		"1234567890abcdef1234567890abcdef",
	}

	for _, uploadID := range invalidUploadIDs {
		if err := checkAzureUploadID(context.Background(), uploadID); err == nil {
			t.Fatalf("%s: expected: <error>, got: <nil>", uploadID)
		}
	}

	validUploadIDs := []string{
		"1234567890abcdef",
		"1122334455667788",
	}

	for _, uploadID := range validUploadIDs {
		if err := checkAzureUploadID(context.Background(), uploadID); err != nil {
			t.Fatalf("%s: expected: <nil>, got: %s", uploadID, err)
		}
	}
}
