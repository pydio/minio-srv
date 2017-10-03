/*
 * Minio Cloud Storage, (C) 2015, 2016, 2017 Minio, Inc.
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
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
)

const (
	// Minimum length for Minio access key.
	accessKeyMinLen = 5

	// Maximum length for Minio access key.
	// There is no max length enforcement for access keys
	accessKeyMaxLen = 20

	// Minimum length for Minio secret key for both server and gateway mode.
	secretKeyMinLen = 8

	// Maximum secret key length for Minio, this
	// is used when autogenerating new credentials.
	// There is no max length enforcement for secret keys
	secretKeyMaxLen = 40

	// Alpha numeric table used for generating access keys.
	alphaNumericTable = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// Total length of the alpha numeric table.
	alphaNumericTableLen = byte(len(alphaNumericTable))
)

// Common errors generated for access and secret key validation.
var (
	errInvalidAccessKeyLength = errors.New("Invalid access key, access key should be minimum 5 characters in length")
	errInvalidSecretKeyLength = errors.New("Invalid secret key, secret key should be minimum 8 characters in length")
)

// isAccessKeyValid - validate access key for right length.
func isAccessKeyValid(accessKey string) bool {
	return len(accessKey) >= accessKeyMinLen
}

// isSecretKeyValid - validate secret key for right length.
func isSecretKeyValid(secretKey string) bool {
	return len(secretKey) >= secretKeyMinLen
}

// credential container for access and secret keys.
type credential struct {
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
}

// IsValid - returns whether credential is valid or not.
func (cred credential) IsValid() bool {
	return isAccessKeyValid(cred.AccessKey) && isSecretKeyValid(cred.SecretKey)
}

// Equals - returns whether two credentials are equal or not.
func (cred credential) Equal(ccred credential) bool {
	if !ccred.IsValid() {
		return false
	}
	return cred.AccessKey == ccred.AccessKey && subtle.ConstantTimeCompare([]byte(cred.SecretKey), []byte(ccred.SecretKey)) == 1
}

// createCredential returns new credentials from the given access key and secret key.
// It returns an error if the access key or secret key are too long or short.
func createCredential(accessKey, secretKey string) (credential, error) {
	if !isAccessKeyValid(accessKey) {
		return credential{}, errInvalidAccessKeyLength
	}
	if !isSecretKeyValid(secretKey) {
		return credential{}, errInvalidSecretKeyLength
	}
	return credential{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}, nil
}

// Initialize a new credential object
func getNewCredential(accessKeyLen, secretKeyLen int) (cred credential, err error) {
	keyBytes := make([]byte, accessKeyLen)
	_, err = rand.Read(keyBytes)
	if err != nil {
		return cred, err
	}
	for i := 0; i < accessKeyLen; i++ {
		keyBytes[i] = alphaNumericTable[keyBytes[i]%alphaNumericTableLen]
	}
	accessKey := string(keyBytes)

	// Generate secret key.
	keyBytes = make([]byte, secretKeyLen)
	_, err = rand.Read(keyBytes)
	if err != nil {
		return cred, err
	}
	secretKey := string([]byte(base64.StdEncoding.EncodeToString(keyBytes))[:secretKeyLen])
	cred, err = createCredential(accessKey, secretKey)

	return cred, err
}

func mustGetNewCredential() credential {
	// Generate Minio credentials with Minio key max lengths.
	cred, err := getNewCredential(accessKeyMaxLen, secretKeyMaxLen)
	fatalIf(err, "Unable to generate new credentials.")
	return cred
}
