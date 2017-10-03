/*
 * Minio Cloud Storage, (C) 2016, 2017, 2017 Minio, Inc.
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
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"path"
	"runtime"
	"sort"
	"sync"
	"time"

	"golang.org/x/crypto/blake2b"
)

const erasureAlgorithmKlauspost = "klauspost/reedsolomon/vandermonde"

// DefaultBitrotAlgorithm is the default algorithm used for bitrot protection.
var DefaultBitrotAlgorithm = BLAKE2b512

func init() {
	newBLAKE2b := func() hash.Hash {
		b2, _ := blake2b.New512(nil) // New512 never returns an error if the key is nil
		return b2
	}
	crypto.RegisterHash(crypto.Hash(SHA256), sha256.New)
	crypto.RegisterHash(crypto.Hash(BLAKE2b512), newBLAKE2b)
	crypto.RegisterHash(crypto.Hash(HighwayHash256), nil) // TODO(aead): currently not supported, waiting for google to finish algorithm spec.

	if runtime.GOARCH == "arm64" { // use SHA256 hardware implementation of arm64
		DefaultBitrotAlgorithm = SHA256
	}
}

// BitrotAlgorithm specifies a algorithm used for bitrot protection.
type BitrotAlgorithm crypto.Hash

const (
	// SHA256 represents the SHA-256 hash function
	SHA256 = BitrotAlgorithm(crypto.SHA256)

	// HighwayHash256 represents the HighwayHash-256 hash function
	HighwayHash256 = BitrotAlgorithm(crypto.SHA3_256) // we must define that HighwayHash-256 is SHA3-256 because there is no HighwayHash constant in golang/crypto yet.

	// BLAKE2b512 represents the BLAKE2b-256 hash function
	BLAKE2b512 = BitrotAlgorithm(crypto.SHA3_512) // we must define that BLAKE2b-512 is SHA3-512 because there is no BLAKE2b-512 constant in golang/crypto yet - FIXME: Go1.9 has BLAKE2 constants
)

var bitrotAlgorithms = map[BitrotAlgorithm]string{
	SHA256:         "sha256",
	BLAKE2b512:     "blake2b",
	HighwayHash256: "highwayhash256",
}

// New returns a new hash.Hash calculating the given bitrot algorithm. New panics
// if the algorithm is not supported or not linked into the binary.
func (a BitrotAlgorithm) New() hash.Hash {
	if _, ok := bitrotAlgorithms[a]; !ok {
		panic(fmt.Sprintf("bitrot algorithm #%d is not supported", a))
	}
	return crypto.Hash(a).New()
}

// Available reports whether the given algorihm is a supported and linked into the binary.
func (a BitrotAlgorithm) Available() bool {
	_, ok := bitrotAlgorithms[a]
	return ok && crypto.Hash(a).Available()
}

// String returns the string identifier for a given bitrot algorithm.
// If the algorithm is not supported String panics.
func (a BitrotAlgorithm) String() string {
	if name, ok := bitrotAlgorithms[a]; ok {
		return name
	}
	panic(fmt.Sprintf("bitrot algorithm #%d is not supported", a))
}

// BitrotAlgorithmFromString returns a bitrot algorithm from the given string representation.
// It returns 0 if the string representation does not match any supported algorithm.
// The zero value of a bitrot algorithm is never supported.
func BitrotAlgorithmFromString(s string) (a BitrotAlgorithm) {
	for alg, name := range bitrotAlgorithms {
		if name == s {
			return alg
		}
	}
	return
}

// objectPartInfo Info of each part kept in the multipart metadata
// file after CompleteMultipartUpload() is called.
type objectPartInfo struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
	ETag   string `json:"etag"`
	Size   int64  `json:"size"`
}

// byObjectPartNumber is a collection satisfying sort.Interface.
type byObjectPartNumber []objectPartInfo

func (t byObjectPartNumber) Len() int           { return len(t) }
func (t byObjectPartNumber) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t byObjectPartNumber) Less(i, j int) bool { return t[i].Number < t[j].Number }

// ChecksumInfo - carries checksums of individual scattered parts per disk.
type ChecksumInfo struct {
	Name      string
	Algorithm BitrotAlgorithm
	Hash      []byte
}

// MarshalJSON marshals the ChecksumInfo struct
func (c ChecksumInfo) MarshalJSON() ([]byte, error) {
	type checksuminfo struct {
		Name      string `json:"name"`
		Algorithm string `json:"algorithm"`
		Hash      string `json:"hash"`
	}

	info := checksuminfo{
		Name:      c.Name,
		Algorithm: c.Algorithm.String(),
		Hash:      hex.EncodeToString(c.Hash),
	}
	return json.Marshal(info)
}

// UnmarshalJSON unmarshals the the given data into the ChecksumInfo struct
func (c *ChecksumInfo) UnmarshalJSON(data []byte) error {
	type checksuminfo struct {
		Name      string `json:"name"`
		Algorithm string `json:"algorithm"`
		Hash      string `json:"hash"`
	}

	var info checksuminfo
	err := json.Unmarshal(data, &info)
	if err != nil {
		return err
	}
	c.Algorithm = BitrotAlgorithmFromString(info.Algorithm)
	if !c.Algorithm.Available() {
		return errBitrotHashAlgoInvalid
	}
	c.Hash, err = hex.DecodeString(info.Hash)
	if err != nil {
		return err
	}
	c.Name = info.Name
	return nil
}

// ErasureInfo holds erasure coding and bitrot related information.
type ErasureInfo struct {
	// Algorithm is the string representation of erasure-coding-algorithm
	Algorithm string `json:"algorithm"`
	// DataBlocks is the number of data blocks for erasure-coding
	DataBlocks int `json:"data"`
	// ParityBlocks is the number of parity blocks for erasure-coding
	ParityBlocks int `json:"parity"`
	// BlockSize is the size of one erasure-coded block
	BlockSize int64 `json:"blockSize"`
	// Index is the index of the current disk
	Index int `json:"index"`
	// Distribution is the distribution of the data and parity blocks
	Distribution []int `json:"distribution"`
	// Checksums holds all bitrot checksums of all erasure encoded blocks
	Checksums []ChecksumInfo `json:"checksum,omitempty"`
}

// AddChecksumInfo adds a checksum of a part.
func (e *ErasureInfo) AddChecksumInfo(ckSumInfo ChecksumInfo) {
	for i, sum := range e.Checksums {
		if sum.Name == ckSumInfo.Name {
			e.Checksums[i] = ckSumInfo
			return
		}
	}
	e.Checksums = append(e.Checksums, ckSumInfo)
}

// GetChecksumInfo - get checksum of a part.
func (e ErasureInfo) GetChecksumInfo(partName string) (ckSum ChecksumInfo) {
	// Return the checksum.
	for _, sum := range e.Checksums {
		if sum.Name == partName {
			return sum
		}
	}
	return ChecksumInfo{}
}

// statInfo - carries stat information of the object.
type statInfo struct {
	Size    int64     `json:"size"`    // Size of the object `xl.json`.
	ModTime time.Time `json:"modTime"` // ModTime of the object `xl.json`.
}

// A xlMetaV1 represents `xl.json` metadata header.
type xlMetaV1 struct {
	Version string   `json:"version"` // Version of the current `xl.json`.
	Format  string   `json:"format"`  // Format of the current `xl.json`.
	Stat    statInfo `json:"stat"`    // Stat of the current object `xl.json`.
	// Erasure coded info for the current object `xl.json`.
	Erasure ErasureInfo `json:"erasure"`
	// Minio release tag for current object `xl.json`.
	Minio struct {
		Release string `json:"release"`
	} `json:"minio"`
	// Metadata map for current object `xl.json`.
	Meta map[string]string `json:"meta,omitempty"`
	// Captures all the individual object `xl.json`.
	Parts []objectPartInfo `json:"parts,omitempty"`
}

// XL metadata constants.
const (
	// XL meta version.
	xlMetaVersion = "1.0.1"

	// XL meta version.
	xlMetaVersion100 = "1.0.0"

	// XL meta format string.
	xlMetaFormat = "xl"

	// Add new constants here.
)

// newXLMetaV1 - initializes new xlMetaV1, adds version, allocates a fresh erasure info.
func newXLMetaV1(object string, dataBlocks, parityBlocks int) (xlMeta xlMetaV1) {
	xlMeta = xlMetaV1{}
	xlMeta.Version = xlMetaVersion
	xlMeta.Format = xlMetaFormat
	xlMeta.Minio.Release = ReleaseTag
	xlMeta.Erasure = ErasureInfo{
		Algorithm:    erasureAlgorithmKlauspost,
		DataBlocks:   dataBlocks,
		ParityBlocks: parityBlocks,
		BlockSize:    blockSizeV1,
		Distribution: hashOrder(object, dataBlocks+parityBlocks),
	}
	return xlMeta
}

// IsValid - tells if the format is sane by validating the version
// string and format style.
func (m xlMetaV1) IsValid() bool {
	return isXLMetaValid(m.Version, m.Format)
}

// Verifies if the backend format metadata is sane by validating
// the version string and format style.
func isXLMetaValid(version, format string) bool {
	return ((version == xlMetaVersion || version == xlMetaVersion100) &&
		format == xlMetaFormat)
}

// Converts metadata to object info.
func (m xlMetaV1) ToObjectInfo(bucket, object string) ObjectInfo {
	objInfo := ObjectInfo{
		IsDir:           false,
		Bucket:          bucket,
		Name:            object,
		Size:            m.Stat.Size,
		ModTime:         m.Stat.ModTime,
		ContentType:     m.Meta["content-type"],
		ContentEncoding: m.Meta["content-encoding"],
	}

	// Extract etag from metadata.
	objInfo.ETag = extractETag(m.Meta)

	// etag/md5Sum has already been extracted. We need to
	// remove to avoid it from appearing as part of
	// response headers. e.g, X-Minio-* or X-Amz-*.
	objInfo.UserDefined = cleanMetaETag(m.Meta)

	// Success.
	return objInfo
}

// objectPartIndex - returns the index of matching object part number.
func objectPartIndex(parts []objectPartInfo, partNumber int) int {
	for i, part := range parts {
		if partNumber == part.Number {
			return i
		}
	}
	return -1
}

// AddObjectPart - add a new object part in order.
func (m *xlMetaV1) AddObjectPart(partNumber int, partName string, partETag string, partSize int64) {
	partInfo := objectPartInfo{
		Number: partNumber,
		Name:   partName,
		ETag:   partETag,
		Size:   partSize,
	}

	// Update part info if it already exists.
	for i, part := range m.Parts {
		if partNumber == part.Number {
			m.Parts[i] = partInfo
			return
		}
	}

	// Proceed to include new part info.
	m.Parts = append(m.Parts, partInfo)

	// Parts in xlMeta should be in sorted order by part number.
	sort.Sort(byObjectPartNumber(m.Parts))
}

// ObjectToPartOffset - translate offset of an object to offset of its individual part.
func (m xlMetaV1) ObjectToPartOffset(offset int64) (partIndex int, partOffset int64, err error) {
	if offset == 0 {
		// Special case - if offset is 0, then partIndex and partOffset are always 0.
		return 0, 0, nil
	}
	partOffset = offset
	// Seek until object offset maps to a particular part offset.
	for i, part := range m.Parts {
		partIndex = i
		// Offset is smaller than size we have reached the proper part offset.
		if partOffset < part.Size {
			return partIndex, partOffset, nil
		}
		// Continue to towards the next part.
		partOffset -= part.Size
	}
	// Offset beyond the size of the object return InvalidRange.
	return 0, 0, traceError(InvalidRange{})
}

// pickValidXLMeta - picks one valid xlMeta content and returns from a
// slice of xlmeta content. If no value is found this function panics
// and dies.
func pickValidXLMeta(metaArr []xlMetaV1, modTime time.Time) (xmv xlMetaV1, e error) {
	// Pick latest valid metadata.
	for _, meta := range metaArr {
		if meta.IsValid() && meta.Stat.ModTime.Equal(modTime) {
			return meta, nil
		}
	}
	return xmv, traceError(errors.New("No valid xl.json present"))
}

// list of all errors that can be ignored in a metadata operation.
var objMetadataOpIgnoredErrs = append(baseIgnoredErrs, errDiskAccessDenied, errVolumeNotFound, errFileNotFound, errFileAccessDenied, errCorruptedFormat)

// readXLMetaParts - returns the XL Metadata Parts from xl.json of one of the disks picked at random.
func (xl xlObjects) readXLMetaParts(bucket, object string) (xlMetaParts []objectPartInfo, err error) {
	var ignoredErrs []error
	for _, disk := range xl.getLoadBalancedDisks() {
		if disk == nil {
			ignoredErrs = append(ignoredErrs, errDiskNotFound)
			continue
		}
		xlMetaParts, err = readXLMetaParts(disk, bucket, object)
		if err == nil {
			return xlMetaParts, nil
		}
		// For any reason disk or bucket is not available continue
		// and read from other disks.
		if isErrIgnored(err, objMetadataOpIgnoredErrs...) {
			ignoredErrs = append(ignoredErrs, err)
			continue
		}
		// Error is not ignored, return right here.
		return nil, err
	}
	// If all errors were ignored, reduce to maximal occurrence
	// based on the read quorum.
	return nil, reduceReadQuorumErrs(ignoredErrs, nil, xl.readQuorum)
}

// readXLMetaStat - return xlMetaV1.Stat and xlMetaV1.Meta from  one of the disks picked at random.
func (xl xlObjects) readXLMetaStat(bucket, object string) (xlStat statInfo, xlMeta map[string]string, err error) {
	var ignoredErrs []error
	for _, disk := range xl.getLoadBalancedDisks() {
		if disk == nil {
			ignoredErrs = append(ignoredErrs, errDiskNotFound)
			continue
		}
		// parses only xlMetaV1.Meta and xlMeta.Stat
		xlStat, xlMeta, err = readXLMetaStat(disk, bucket, object)
		if err == nil {
			return xlStat, xlMeta, nil
		}
		// For any reason disk or bucket is not available continue
		// and read from other disks.
		if isErrIgnored(err, objMetadataOpIgnoredErrs...) {
			ignoredErrs = append(ignoredErrs, err)
			continue
		}
		// Error is not ignored, return right here.
		return statInfo{}, nil, err
	}
	// If all errors were ignored, reduce to maximal occurrence
	// based on the read quorum.
	return statInfo{}, nil, reduceReadQuorumErrs(ignoredErrs, nil, xl.readQuorum)
}

// deleteXLMetadata - deletes `xl.json` on a single disk.
func deleteXLMetdata(disk StorageAPI, bucket, prefix string) error {
	jsonFile := path.Join(prefix, xlMetaJSONFile)
	return traceError(disk.DeleteFile(bucket, jsonFile))
}

// writeXLMetadata - writes `xl.json` to a single disk.
func writeXLMetadata(disk StorageAPI, bucket, prefix string, xlMeta xlMetaV1) error {
	jsonFile := path.Join(prefix, xlMetaJSONFile)

	// Marshal json.
	metadataBytes, err := json.Marshal(&xlMeta)
	if err != nil {
		return traceError(err)
	}
	// Persist marshalled data.
	return traceError(disk.AppendFile(bucket, jsonFile, metadataBytes))
}

// deleteAllXLMetadata - deletes all partially written `xl.json` depending on errs.
func deleteAllXLMetadata(disks []StorageAPI, bucket, prefix string, errs []error) {
	var wg = &sync.WaitGroup{}
	// Delete all the `xl.json` left over.
	for index, disk := range disks {
		if disk == nil {
			continue
		}
		// Undo rename object in parallel.
		wg.Add(1)
		go func(index int, disk StorageAPI) {
			defer wg.Done()
			if errs[index] != nil {
				return
			}
			_ = deleteXLMetdata(disk, bucket, prefix)
		}(index, disk)
	}
	wg.Wait()
}

// Rename `xl.json` content to destination location for each disk in order.
func renameXLMetadata(disks []StorageAPI, srcBucket, srcEntry, dstBucket, dstEntry string, quorum int) ([]StorageAPI, error) {
	isDir := false
	srcXLJSON := path.Join(srcEntry, xlMetaJSONFile)
	dstXLJSON := path.Join(dstEntry, xlMetaJSONFile)
	return rename(disks, srcBucket, srcXLJSON, dstBucket, dstXLJSON, isDir, quorum)
}

// writeUniqueXLMetadata - writes unique `xl.json` content for each disk in order.
func writeUniqueXLMetadata(disks []StorageAPI, bucket, prefix string, xlMetas []xlMetaV1, quorum int) ([]StorageAPI, error) {
	var wg = &sync.WaitGroup{}
	var mErrs = make([]error, len(disks))

	// Start writing `xl.json` to all disks in parallel.
	for index, disk := range disks {
		if disk == nil {
			mErrs[index] = traceError(errDiskNotFound)
			continue
		}
		wg.Add(1)
		// Write `xl.json` in a routine.
		go func(index int, disk StorageAPI) {
			defer wg.Done()

			// Pick one xlMeta for a disk at index.
			xlMetas[index].Erasure.Index = index + 1

			// Write unique `xl.json` for a disk at index.
			err := writeXLMetadata(disk, bucket, prefix, xlMetas[index])
			if err != nil {
				mErrs[index] = err
			}
		}(index, disk)
	}

	// Wait for all the routines.
	wg.Wait()

	err := reduceWriteQuorumErrs(mErrs, objectOpIgnoredErrs, quorum)
	if errorCause(err) == errXLWriteQuorum {
		// Delete all `xl.json` successfully renamed.
		deleteAllXLMetadata(disks, bucket, prefix, mErrs)
	}
	return evalDisks(disks, mErrs), err
}

// writeSameXLMetadata - write `xl.json` on all disks in order.
func writeSameXLMetadata(disks []StorageAPI, bucket, prefix string, xlMeta xlMetaV1, writeQuorum, readQuorum int) ([]StorageAPI, error) {
	var wg = &sync.WaitGroup{}
	var mErrs = make([]error, len(disks))

	// Start writing `xl.json` to all disks in parallel.
	for index, disk := range disks {
		if disk == nil {
			mErrs[index] = traceError(errDiskNotFound)
			continue
		}
		wg.Add(1)
		// Write `xl.json` in a routine.
		go func(index int, disk StorageAPI, metadata xlMetaV1) {
			defer wg.Done()

			// Save the disk order index.
			metadata.Erasure.Index = index + 1

			// Write xl metadata.
			err := writeXLMetadata(disk, bucket, prefix, metadata)
			if err != nil {
				mErrs[index] = err
			}
		}(index, disk, xlMeta)
	}

	// Wait for all the routines.
	wg.Wait()

	err := reduceWriteQuorumErrs(mErrs, objectOpIgnoredErrs, writeQuorum)
	if errorCause(err) == errXLWriteQuorum {
		// Delete all `xl.json` successfully renamed.
		deleteAllXLMetadata(disks, bucket, prefix, mErrs)
	}
	return evalDisks(disks, mErrs), err
}
