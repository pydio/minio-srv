/*
 * Minio Cloud Storage, (C) 2016, 2017 Minio, Inc.
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
	"runtime"
	"sync"
	"testing"

	"github.com/minio/dsync"
)

// Helper function to test equality of locks (without taking timing info into account)
func testLockEquality(lriLeft, lriRight []lockRequesterInfo) bool {
	if len(lriLeft) != len(lriRight) {
		return false
	}

	for i := 0; i < len(lriLeft); i++ {
		if lriLeft[i].writer != lriRight[i].writer ||
			lriLeft[i].node != lriRight[i].node ||
			lriLeft[i].serviceEndpoint != lriRight[i].serviceEndpoint ||
			lriLeft[i].uid != lriRight[i].uid {
			return false
		}
	}
	return true
}

// Helper function to create a lock server for testing
func createLockTestServer(t *testing.T) (string, *lockRPCReceiver, string) {
	obj, fsDir, err := prepareFS()
	if err != nil {
		t.Fatal(err)
	}
	if err = newTestConfig(globalMinioDefaultRegion, obj); err != nil {
		t.Fatalf("unable initialize config file, %s", err)
	}

	locker := &lockRPCReceiver{
		ll: localLocker{
			mutex:           sync.Mutex{},
			serviceEndpoint: "rpc-path",
			lockMap:         make(map[string][]lockRequesterInfo),
		},
	}
	creds := globalServerConfig.GetCredential()
	token, err := authenticateNode(creds.AccessKey, creds.SecretKey)
	if err != nil {
		t.Fatal(err)
	}
	return fsDir, locker, token
}

// Test Lock functionality
func TestLockRpcServerLock(t *testing.T) {
	testPath, locker, token := createLockTestServer(t)
	defer os.RemoveAll(testPath)

	la := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "0123-4567",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	// Claim a lock
	var result bool
	err := locker.Lock(&la, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else {
		if !result {
			t.Errorf("Expected %#v, got %#v", true, result)
		} else {
			gotLri, _ := locker.ll.lockMap["name"]
			expectedLri := []lockRequesterInfo{
				{
					writer:          true,
					node:            "node",
					serviceEndpoint: "rpc-path",
					uid:             "0123-4567",
				},
			}
			if !testLockEquality(expectedLri, gotLri) {
				t.Errorf("Expected %#v, got %#v", expectedLri, gotLri)
			}
		}
	}

	// Try to claim same lock again (will fail)
	la2 := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "89ab-cdef",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	err = locker.Lock(&la2, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else {
		if result {
			t.Errorf("Expected %#v, got %#v", false, result)
		}
	}
}

// Test Unlock functionality
func TestLockRpcServerUnlock(t *testing.T) {
	testPath, locker, token := createLockTestServer(t)
	defer os.RemoveAll(testPath)

	la := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "0123-4567",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	// First test return of error when attempting to unlock a lock that does not exist
	var result bool
	err := locker.Unlock(&la, &result)
	if err == nil {
		t.Errorf("Expected error, got %#v", nil)
	}

	// Create lock (so that we can release)
	err = locker.Lock(&la, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else if !result {
		t.Errorf("Expected %#v, got %#v", true, result)
	}

	// Finally test successful release of lock
	err = locker.Unlock(&la, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else {
		if !result {
			t.Errorf("Expected %#v, got %#v", true, result)
		} else {
			gotLri, _ := locker.ll.lockMap["name"]
			expectedLri := []lockRequesterInfo(nil)
			if !testLockEquality(expectedLri, gotLri) {
				t.Errorf("Expected %#v, got %#v", expectedLri, gotLri)
			}
		}
	}
}

// Test RLock functionality
func TestLockRpcServerRLock(t *testing.T) {
	testPath, locker, token := createLockTestServer(t)
	defer os.RemoveAll(testPath)

	la := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "0123-4567",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	// Claim a lock
	var result bool
	err := locker.RLock(&la, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else {
		if !result {
			t.Errorf("Expected %#v, got %#v", true, result)
		} else {
			gotLri, _ := locker.ll.lockMap["name"]
			expectedLri := []lockRequesterInfo{
				{
					writer:          false,
					node:            "node",
					serviceEndpoint: "rpc-path",
					uid:             "0123-4567",
				},
			}
			if !testLockEquality(expectedLri, gotLri) {
				t.Errorf("Expected %#v, got %#v", expectedLri, gotLri)
			}
		}
	}

	// Try to claim same again (will succeed)
	la2 := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "89ab-cdef",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	err = locker.RLock(&la2, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else {
		if !result {
			t.Errorf("Expected %#v, got %#v", true, result)
		}
	}
}

// Test RUnlock functionality
func TestLockRpcServerRUnlock(t *testing.T) {
	testPath, locker, token := createLockTestServer(t)
	defer os.RemoveAll(testPath)

	la := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "0123-4567",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	// First test return of error when attempting to unlock a read-lock that does not exist
	var result bool
	err := locker.Unlock(&la, &result)
	if err == nil {
		t.Errorf("Expected error, got %#v", nil)
	}

	// Create first lock ... (so that we can release)
	err = locker.RLock(&la, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else if !result {
		t.Errorf("Expected %#v, got %#v", true, result)
	}

	// Try to claim same again (will succeed)
	la2 := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "89ab-cdef",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	// ... and create a second lock on same resource
	err = locker.RLock(&la2, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else if !result {
		t.Errorf("Expected %#v, got %#v", true, result)
	}

	// Test successful release of first read lock
	err = locker.RUnlock(&la, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else {
		if !result {
			t.Errorf("Expected %#v, got %#v", true, result)
		} else {
			gotLri, _ := locker.ll.lockMap["name"]
			expectedLri := []lockRequesterInfo{
				{
					writer:          false,
					node:            "node",
					serviceEndpoint: "rpc-path",
					uid:             "89ab-cdef",
				},
			}
			if !testLockEquality(expectedLri, gotLri) {
				t.Errorf("Expected %#v, got %#v", expectedLri, gotLri)
			}

		}
	}

	// Finally test successful release of second (and last) read lock
	err = locker.RUnlock(&la2, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else {
		if !result {
			t.Errorf("Expected %#v, got %#v", true, result)
		} else {
			gotLri, _ := locker.ll.lockMap["name"]
			expectedLri := []lockRequesterInfo(nil)
			if !testLockEquality(expectedLri, gotLri) {
				t.Errorf("Expected %#v, got %#v", expectedLri, gotLri)
			}
		}
	}
}

// Test ForceUnlock functionality
func TestLockRpcServerForceUnlock(t *testing.T) {
	testPath, locker, token := createLockTestServer(t)
	defer os.RemoveAll(testPath)

	laForce := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "1234-5678",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	// First test that UID should be empty
	var result bool
	err := locker.ForceUnlock(&laForce, &result)
	if err == nil {
		t.Errorf("Expected error, got %#v", nil)
	}

	// Then test force unlock of a lock that does not exist (not returning an error)
	laForce.LockArgs.UID = ""
	err = locker.ForceUnlock(&laForce, &result)
	if err != nil {
		t.Errorf("Expected no error, got %#v", err)
	}

	la := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "0123-4567",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	// Create lock ... (so that we can force unlock)
	err = locker.Lock(&la, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else if !result {
		t.Errorf("Expected %#v, got %#v", true, result)
	}

	// Forcefully unlock the lock (not returning an error)
	err = locker.ForceUnlock(&laForce, &result)
	if err != nil {
		t.Errorf("Expected no error, got %#v", err)
	}

	// Try to get lock again (should be granted)
	err = locker.Lock(&la, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else if !result {
		t.Errorf("Expected %#v, got %#v", true, result)
	}

	// Finally forcefully unlock the lock once again
	err = locker.ForceUnlock(&laForce, &result)
	if err != nil {
		t.Errorf("Expected no error, got %#v", err)
	}
}

// Test Expired functionality
func TestLockRpcServerExpired(t *testing.T) {
	testPath, locker, token := createLockTestServer(t)
	defer os.RemoveAll(testPath)

	la := LockArgs{
		AuthArgs: AuthArgs{
			Token:       token,
			RPCVersion:  globalRPCAPIVersion,
			RequestTime: UTCNow(),
		},
		LockArgs: dsync.LockArgs{
			UID:             "0123-4567",
			Resource:        "name",
			ServerAddr:      "node",
			ServiceEndpoint: "rpc-path",
		}}

	// Unknown lock at server will return expired = true
	var expired bool
	err := locker.Expired(&la, &expired)
	if err != nil {
		t.Errorf("Expected no error, got %#v", err)
	} else {
		if !expired {
			t.Errorf("Expected %#v, got %#v", true, expired)
		}
	}

	// Create lock (so that we can test that it is not expired)
	var result bool
	err = locker.Lock(&la, &result)
	if err != nil {
		t.Errorf("Expected %#v, got %#v", nil, err)
	} else if !result {
		t.Errorf("Expected %#v, got %#v", true, result)
	}

	err = locker.Expired(&la, &expired)
	if err != nil {
		t.Errorf("Expected no error, got %#v", err)
	} else {
		if expired {
			t.Errorf("Expected %#v, got %#v", false, expired)
		}
	}
}

// Test initialization of lock server.
func TestLockServerInit(t *testing.T) {
	if runtime.GOOS == globalWindowsOSName {
		return
	}

	obj, fsDir, err := prepareFS()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fsDir)
	if err = newTestConfig(globalMinioDefaultRegion, obj); err != nil {
		t.Fatalf("unable initialize config file, %s", err)
	}

	currentIsDistXL := globalIsDistXL
	currentLockServer := globalLockServer
	defer func() {
		globalIsDistXL = currentIsDistXL
		globalLockServer = currentLockServer
	}()

	case1Endpoints := mustGetNewEndpointList(
		"http://localhost:9000/mnt/disk1",
		"http://1.1.1.2:9000/mnt/disk2",
		"http://1.1.2.1:9000/mnt/disk3",
		"http://1.1.2.2:9000/mnt/disk4",
	)
	for i := range case1Endpoints {
		if case1Endpoints[i].Host == "localhost:9000" {
			case1Endpoints[i].IsLocal = true
		}
	}

	case2Endpoints := mustGetNewEndpointList(
		"http://localhost:9000/mnt/disk1",
		"http://localhost:9000/mnt/disk2",
		"http://1.1.2.1:9000/mnt/disk3",
		"http://1.1.2.2:9000/mnt/disk4",
	)
	for i := range case2Endpoints {
		if case2Endpoints[i].Host == "localhost:9000" {
			case2Endpoints[i].IsLocal = true
		}
	}

	globalMinioHost = ""
	testCases := []struct {
		isDistXL  bool
		endpoints EndpointList
	}{
		// Test - 1 one lock server initialized.
		{true, case1Endpoints},
		// Test - similar endpoint hosts should
		// converge to single lock server
		// initialized.
		{true, case2Endpoints},
	}

	// Validates lock server initialization.
	for i, testCase := range testCases {
		globalIsDistXL = testCase.isDistXL
		globalLockServer = nil
		_, _ = newDsyncNodes(testCase.endpoints)
		if err != nil {
			t.Fatalf("Got unexpected error initializing lock servers: %v", err)
		}
		if globalLockServer == nil && testCase.isDistXL {
			t.Errorf("Test %d: Expected initialized lock RPC receiver, but got uninitialized", i+1)
		}
	}
}
