/*
 * Minio Cloud Storage, (C) 2016, 2017, 2018 Minio, Inc.
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
	"context"
	"fmt"
	"path"
	"sync"

	"github.com/pydio/minio-srv/cmd/logger"
	"github.com/pydio/minio-srv/pkg/madmin"
)

func (xl xlObjects) ReloadFormat(ctx context.Context, dryRun bool) error {
	logger.LogIf(ctx, NotImplemented{})
	return NotImplemented{}
}

func (xl xlObjects) HealFormat(ctx context.Context, dryRun bool) (madmin.HealResultItem, error) {
	logger.LogIf(ctx, NotImplemented{})
	return madmin.HealResultItem{}, NotImplemented{}
}

// Heals a bucket if it doesn't exist on one of the disks, additionally
// also heals the missing entries for bucket metadata files
// `policy.json, notification.xml, listeners.json`.
func (xl xlObjects) HealBucket(ctx context.Context, bucket string, dryRun bool) (
	results []madmin.HealResultItem, err error) {

	storageDisks := xl.getDisks()

	// get write quorum for an object
	writeQuorum := len(storageDisks)/2 + 1

	// Heal bucket.
	var result madmin.HealResultItem
	result, err = healBucket(ctx, storageDisks, bucket, writeQuorum, dryRun)
	if err != nil {
		return nil, err
	}
	results = append(results, result)

	// Proceed to heal bucket metadata.
	metaResults, err := healBucketMetadata(xl, bucket, dryRun)
	results = append(results, metaResults...)
	return results, err
}

// Heal bucket - create buckets on disks where it does not exist.
func healBucket(ctx context.Context, storageDisks []StorageAPI, bucket string, writeQuorum int,
	dryRun bool) (res madmin.HealResultItem, err error) {

	// Initialize sync waitgroup.
	var wg = &sync.WaitGroup{}

	// Initialize list of errors.
	var dErrs = make([]error, len(storageDisks))

	// Disk states slices
	beforeState := make([]string, len(storageDisks))
	afterState := make([]string, len(storageDisks))

	// Make a volume entry on all underlying storage disks.
	for index, disk := range storageDisks {
		if disk == nil {
			dErrs[index] = errDiskNotFound
			beforeState[index] = madmin.DriveStateOffline
			afterState[index] = madmin.DriveStateOffline
			continue
		}
		wg.Add(1)

		// Make a volume inside a go-routine.
		go func(index int, disk StorageAPI) {
			defer wg.Done()
			if _, serr := disk.StatVol(bucket); serr != nil {
				if serr == errDiskNotFound {
					beforeState[index] = madmin.DriveStateOffline
					afterState[index] = madmin.DriveStateOffline
					dErrs[index] = serr
					return
				}
				if serr != errVolumeNotFound {
					beforeState[index] = madmin.DriveStateCorrupt
					afterState[index] = madmin.DriveStateCorrupt
					dErrs[index] = serr
					return
				}

				beforeState[index] = madmin.DriveStateMissing
				afterState[index] = madmin.DriveStateMissing

				// mutate only if not a dry-run
				if dryRun {
					return
				}

				makeErr := disk.MakeVol(bucket)
				dErrs[index] = makeErr
				if makeErr == nil {
					afterState[index] = madmin.DriveStateOk
				}
				return
			}
			beforeState[index] = madmin.DriveStateOk
			afterState[index] = madmin.DriveStateOk
		}(index, disk)
	}

	// Wait for all make vol to finish.
	wg.Wait()

	// Initialize heal result info
	res = madmin.HealResultItem{
		Type:      madmin.HealItemBucket,
		Bucket:    bucket,
		DiskCount: len(storageDisks),
	}
	for i, before := range beforeState {
		if storageDisks[i] == nil {
			res.Before.Drives = append(res.Before.Drives, madmin.HealDriveInfo{
				UUID:     "",
				Endpoint: "",
				State:    before,
			})
			res.After.Drives = append(res.After.Drives, madmin.HealDriveInfo{
				UUID:     "",
				Endpoint: "",
				State:    afterState[i],
			})
			continue
		}
		drive := storageDisks[i].String()
		res.Before.Drives = append(res.Before.Drives, madmin.HealDriveInfo{
			UUID:     "",
			Endpoint: drive,
			State:    before,
		})
		res.After.Drives = append(res.After.Drives, madmin.HealDriveInfo{
			UUID:     "",
			Endpoint: drive,
			State:    afterState[i],
		})
	}

	reducedErr := reduceWriteQuorumErrs(ctx, dErrs, bucketOpIgnoredErrs, writeQuorum)
	if reducedErr == errXLWriteQuorum {
		// Purge successfully created buckets if we don't have writeQuorum.
		undoMakeBucket(storageDisks, bucket)
	}
	return res, reducedErr
}

// Heals all the metadata associated for a given bucket, this function
// heals `policy.json`, `notification.xml` and `listeners.json`.
func healBucketMetadata(xl xlObjects, bucket string, dryRun bool) (
	results []madmin.HealResultItem, err error) {

	healBucketMetaFn := func(metaPath string) error {
		reqInfo := &logger.ReqInfo{BucketName: bucket}
		ctx := logger.SetReqInfo(context.Background(), reqInfo)
		result, healErr := xl.HealObject(ctx, minioMetaBucket, metaPath, dryRun)
		// If object is not found, skip the file.
		if isErrObjectNotFound(healErr) {
			return nil
		}
		if healErr != nil {
			return healErr
		}
		result.Type = madmin.HealItemBucketMetadata
		results = append(results, result)
		return nil
	}

	// Heal `policy.json` for missing entries, ignores if
	// `policy.json` is not found.
	policyPath := pathJoin(bucketConfigPrefix, bucket, bucketPolicyConfig)
	err = healBucketMetaFn(policyPath)
	if err != nil {
		return results, err
	}

	// Heal `notification.xml` for missing entries, ignores if
	// `notification.xml` is not found.
	nConfigPath := path.Join(bucketConfigPrefix, bucket,
		bucketNotificationConfig)
	err = healBucketMetaFn(nConfigPath)
	if err != nil {
		return results, err
	}

	// Heal `listeners.json` for missing entries, ignores if
	// `listeners.json` is not found.
	lConfigPath := path.Join(bucketConfigPrefix, bucket, bucketListenerConfig)
	err = healBucketMetaFn(lConfigPath)
	return results, err
}

// listAllBuckets lists all buckets from all disks. It also
// returns the occurrence of each buckets in all disks
func listAllBuckets(storageDisks []StorageAPI) (buckets map[string]VolInfo,
	bucketsOcc map[string]int, err error) {

	buckets = make(map[string]VolInfo)
	bucketsOcc = make(map[string]int)
	for _, disk := range storageDisks {
		if disk == nil {
			continue
		}
		var volsInfo []VolInfo
		volsInfo, err = disk.ListVols()
		if err != nil {
			if IsErrIgnored(err, bucketMetadataOpIgnoredErrs...) {
				continue
			}
			return nil, nil, err
		}
		for _, volInfo := range volsInfo {
			// StorageAPI can send volume names which are
			// incompatible with buckets - these are
			// skipped, like the meta-bucket.
			if !IsValidBucketName(volInfo.Name) ||
				isMinioMetaBucketName(volInfo.Name) {
				continue
			}
			// Increase counter per bucket name
			bucketsOcc[volInfo.Name]++
			// Save volume info under bucket name
			buckets[volInfo.Name] = volInfo
		}
	}
	return buckets, bucketsOcc, nil
}

// Heals an object by re-writing corrupt/missing erasure blocks.
func healObject(ctx context.Context, storageDisks []StorageAPI, bucket string, object string,
	quorum int, dryRun bool) (result madmin.HealResultItem, err error) {

	partsMetadata, errs := readAllXLMetadata(ctx, storageDisks, bucket, object)

	errCount := 0
	for _, err := range errs {
		if err != nil {
			errCount++
		}
	}

	if errCount == len(errs) {
		// Only if we get errors from all the disks we return error. Else we need to
		// continue to return filled madmin.HealResultItem struct which includes info
		// on what disks the file is available etc.
		if reducedErr := reduceReadQuorumErrs(ctx, errs, nil, quorum); reducedErr != nil {
			return defaultHealResult(storageDisks, errs, bucket, object), toObjectErr(reducedErr, bucket, object)
		}
	}

	// List of disks having latest version of the object xl.json
	// (by modtime).
	latestDisks, modTime := listOnlineDisks(storageDisks, partsMetadata, errs)

	// List of disks having all parts as per latest xl.json.
	availableDisks, dataErrs := disksWithAllParts(ctx, latestDisks, partsMetadata, errs, bucket, object)

	// Initialize heal result object
	result = madmin.HealResultItem{
		Type:      madmin.HealItemObject,
		Bucket:    bucket,
		Object:    object,
		DiskCount: len(storageDisks),

		// Initialize object size to -1, so we can detect if we are
		// unable to reliably find the object size.
		ObjectSize: -1,
	}

	// Loop to find number of disks with valid data, per-drive
	// data state and a list of outdated disks on which data needs
	// to be healed.
	outDatedDisks := make([]StorageAPI, len(storageDisks))
	numAvailableDisks := 0
	disksToHealCount := 0
	for i, v := range availableDisks {
		driveState := ""
		switch {
		case v != nil:
			driveState = madmin.DriveStateOk
			numAvailableDisks++
			// If data is sane on any one disk, we can
			// extract the correct object size.
			result.ObjectSize = partsMetadata[i].Stat.Size
			result.ParityBlocks = partsMetadata[i].Erasure.ParityBlocks
			result.DataBlocks = partsMetadata[i].Erasure.DataBlocks
		case errs[i] == errDiskNotFound, dataErrs[i] == errDiskNotFound:
			driveState = madmin.DriveStateOffline
		case errs[i] == errFileNotFound, errs[i] == errVolumeNotFound:
			fallthrough
		case dataErrs[i] == errFileNotFound, dataErrs[i] == errVolumeNotFound:
			driveState = madmin.DriveStateMissing
		default:
			// all remaining cases imply corrupt data/metadata
			driveState = madmin.DriveStateCorrupt
		}

		// an online disk without valid data/metadata is
		// outdated and can be healed.
		if errs[i] != errDiskNotFound && v == nil {
			outDatedDisks[i] = storageDisks[i]
			disksToHealCount++
		}
		var drive string
		if v == nil {
			if errs[i] != errDiskNotFound {
				drive = outDatedDisks[i].String()
			}
			result.Before.Drives = append(result.Before.Drives, madmin.HealDriveInfo{
				UUID:     "",
				Endpoint: drive,
				State:    driveState,
			})
			result.After.Drives = append(result.After.Drives, madmin.HealDriveInfo{
				UUID:     "",
				Endpoint: drive,
				State:    driveState,
			})
			continue
		}
		drive = v.String()
		result.Before.Drives = append(result.Before.Drives, madmin.HealDriveInfo{
			UUID:     "",
			Endpoint: drive,
			State:    driveState,
		})
		result.After.Drives = append(result.After.Drives, madmin.HealDriveInfo{
			UUID:     "",
			Endpoint: drive,
			State:    driveState,
		})
	}

	// If less than read quorum number of disks have all the parts
	// of the data, we can't reconstruct the erasure-coded data.
	if numAvailableDisks < quorum {
		// Default to most common configuration for erasure
		// blocks upon returning quorum error.
		result.ParityBlocks = len(storageDisks) / 2
		result.DataBlocks = len(storageDisks) / 2
		return result, toObjectErr(errXLReadQuorum, bucket, object)
	}

	if disksToHealCount == 0 {
		// Nothing to heal!
		return result, nil
	}

	// After this point, only have to repair data on disk - so
	// return if it is a dry-run
	if dryRun {
		return result, nil
	}

	// Latest xlMetaV1 for reference. If a valid metadata is not
	// present, it is as good as object not found.
	latestMeta, pErr := pickValidXLMeta(ctx, partsMetadata, modTime, quorum)
	if pErr != nil {
		return result, toObjectErr(pErr, bucket, object)
	}

	// Clear data files of the object on outdated disks
	for _, disk := range outDatedDisks {
		// Before healing outdated disks, we need to remove
		// xl.json and part files from "bucket/object/" so
		// that rename(minioMetaBucket, "tmp/tmpuuid/",
		// "bucket", "object/") succeeds.
		if disk == nil {
			// Not an outdated disk.
			continue
		}

		// List and delete the object directory,
		files, derr := disk.ListDir(bucket, object, -1)
		if derr == nil {
			for _, entry := range files {
				_ = disk.DeleteFile(bucket,
					pathJoin(object, entry))
			}
		}
	}

	// Reorder so that we have data disks first and parity disks next.
	latestDisks = shuffleDisks(latestDisks, latestMeta.Erasure.Distribution)
	outDatedDisks = shuffleDisks(outDatedDisks, latestMeta.Erasure.Distribution)
	partsMetadata = shufflePartsMetadata(partsMetadata, latestMeta.Erasure.Distribution)

	// We write at temporary location and then rename to final location.
	tmpID := mustGetUUID()

	// Checksum of the part files. checkSumInfos[index] will
	// contain checksums of all the part files in the
	// outDatedDisks[index]
	checksumInfos := make([][]ChecksumInfo, len(outDatedDisks))

	// Heal each part. erasureHealFile() will write the healed
	// part to .minio/tmp/uuid/ which needs to be renamed later to
	// the final location.
	erasure, err := NewErasure(ctx, latestMeta.Erasure.DataBlocks,
		latestMeta.Erasure.ParityBlocks, latestMeta.Erasure.BlockSize)
	if err != nil {
		return result, toObjectErr(err, bucket, object)
	}

	for partIndex := 0; partIndex < len(latestMeta.Parts); partIndex++ {
		partName := latestMeta.Parts[partIndex].Name
		partSize := latestMeta.Parts[partIndex].Size
		erasureInfo := latestMeta.Erasure
		var algorithm BitrotAlgorithm
		bitrotReaders := make([]*bitrotReader, len(latestDisks))
		for i, disk := range latestDisks {
			if disk == OfflineDisk {
				continue
			}
			info := partsMetadata[i].Erasure.GetChecksumInfo(partName)
			algorithm = info.Algorithm
			endOffset := getErasureShardFileEndOffset(0, partSize, partSize, erasureInfo.BlockSize, erasure.dataBlocks)
			bitrotReaders[i] = newBitrotReader(disk, bucket, pathJoin(object, partName), algorithm, endOffset, info.Hash)
		}
		bitrotWriters := make([]*bitrotWriter, len(outDatedDisks))
		for i, disk := range outDatedDisks {
			if disk == OfflineDisk {
				continue
			}
			bitrotWriters[i] = newBitrotWriter(disk, minioMetaTmpBucket, pathJoin(tmpID, partName), algorithm)
		}
		hErr := erasure.Heal(ctx, bitrotReaders, bitrotWriters, partSize)
		if hErr != nil {
			return result, toObjectErr(hErr, bucket, object)
		}
		// outDatedDisks that had write errors should not be
		// written to for remaining parts, so we nil it out.
		for i, disk := range outDatedDisks {
			if disk == nil {
				continue
			}
			// A non-nil stale disk which did not receive
			// a healed part checksum had a write error.
			if bitrotWriters[i] == nil {
				outDatedDisks[i] = nil
				disksToHealCount--
				continue
			}
			// append part checksums
			checksumInfos[i] = append(checksumInfos[i],
				ChecksumInfo{partName, algorithm, bitrotWriters[i].Sum()})
		}

		// If all disks are having errors, we give up.
		if disksToHealCount == 0 {
			return result, fmt.Errorf("all disks without up-to-date data had write errors")
		}
	}

	// xl.json should be written to all the healed disks.
	for index, disk := range outDatedDisks {
		if disk == nil {
			continue
		}
		partsMetadata[index] = latestMeta
		partsMetadata[index].Erasure.Checksums = checksumInfos[index]
	}

	// Generate and write `xl.json` generated from other disks.
	outDatedDisks, aErr := writeUniqueXLMetadata(ctx, outDatedDisks, minioMetaTmpBucket, tmpID,
		partsMetadata, diskCount(outDatedDisks))
	if aErr != nil {
		return result, toObjectErr(aErr, bucket, object)
	}

	// Rename from tmp location to the actual location.
	for _, disk := range outDatedDisks {
		if disk == nil {
			continue
		}

		// Attempt a rename now from healed data to final location.
		aErr = disk.RenameFile(minioMetaTmpBucket, retainSlash(tmpID), bucket,
			retainSlash(object))
		if aErr != nil {
			logger.LogIf(ctx, aErr)
			return result, toObjectErr(aErr, bucket, object)
		}

		for i, v := range result.Before.Drives {
			if v.Endpoint == disk.String() {
				result.After.Drives[i].State = madmin.DriveStateOk
			}
		}
	}

	// Set the size of the object in the heal result
	result.ObjectSize = latestMeta.Stat.Size

	return result, nil
}

// healObjectDir - heals object directory specifically, this special call
// is needed since we do not have a special backend format for directories.
func (xl xlObjects) healObjectDir(ctx context.Context, bucket, object string, dryRun bool) (hr madmin.HealResultItem, err error) {
	storageDisks := xl.getDisks()

	// Initialize heal result object
	hr = madmin.HealResultItem{
		Type:         madmin.HealItemObject,
		Bucket:       bucket,
		Object:       object,
		DiskCount:    len(storageDisks),
		ParityBlocks: len(storageDisks) / 2,
		DataBlocks:   len(storageDisks) / 2,
		ObjectSize:   0,
	}

	// Prepare object creation in all disks
	for _, disk := range storageDisks {
		if disk == nil {
			hr.Before.Drives = append(hr.Before.Drives, madmin.HealDriveInfo{
				UUID:  "",
				State: madmin.DriveStateOffline,
			})
			hr.After.Drives = append(hr.After.Drives, madmin.HealDriveInfo{
				UUID:  "",
				State: madmin.DriveStateMissing,
			})
			continue
		}

		drive := disk.String()
		hr.Before.Drives = append(hr.Before.Drives, madmin.HealDriveInfo{
			UUID:     "",
			Endpoint: drive,
			State:    madmin.DriveStateMissing,
		})
		hr.After.Drives = append(hr.After.Drives, madmin.HealDriveInfo{
			UUID:     "",
			Endpoint: drive,
			State:    madmin.DriveStateMissing,
		})

		if !dryRun {
			if err := disk.MakeVol(pathJoin(bucket, object)); err != nil && err != errVolumeExists {
				return hr, toObjectErr(err, bucket, object)
			}

			for i, v := range hr.Before.Drives {
				if v.Endpoint == drive {
					hr.After.Drives[i].State = madmin.DriveStateOk
				}
			}
		}
	}
	return hr, nil
}

// Populates default heal result item entries with possible values when we are returning prematurely.
// This is to ensure that in any circumstance we are not returning empty arrays with wrong values.
func defaultHealResult(storageDisks []StorageAPI, errs []error, bucket, object string) madmin.HealResultItem {
	// Initialize heal result object
	result := madmin.HealResultItem{
		Type:      madmin.HealItemObject,
		Bucket:    bucket,
		Object:    object,
		DiskCount: len(storageDisks),

		// Initialize object size to -1, so we can detect if we are
		// unable to reliably find the object size.
		ObjectSize: -1,
	}

	for index, disk := range storageDisks {
		if disk == nil {
			result.Before.Drives = append(result.Before.Drives, madmin.HealDriveInfo{
				UUID:  "",
				State: madmin.DriveStateOffline,
			})
			result.After.Drives = append(result.After.Drives, madmin.HealDriveInfo{
				UUID:  "",
				State: madmin.DriveStateOffline,
			})
			continue
		}
		drive := disk.String()
		driveState := madmin.DriveStateCorrupt
		switch errs[index] {
		case errFileNotFound, errVolumeNotFound:
			driveState = madmin.DriveStateMissing
		}
		result.Before.Drives = append(result.Before.Drives, madmin.HealDriveInfo{
			UUID:     "",
			Endpoint: drive,
			State:    driveState,
		})
		result.After.Drives = append(result.After.Drives, madmin.HealDriveInfo{
			UUID:     "",
			Endpoint: drive,
			State:    driveState,
		})
	}

	// Default to most common configuration for erasure blocks.
	result.ParityBlocks = len(storageDisks) / 2
	result.DataBlocks = len(storageDisks) / 2

	return result
}

// HealObject - heal the given object.
//
// FIXME: If an object object was deleted and one disk was down,
// and later the disk comes back up again, heal on the object
// should delete it.
func (xl xlObjects) HealObject(ctx context.Context, bucket, object string, dryRun bool) (hr madmin.HealResultItem, err error) {
	// Create context that also contains information about the object and bucket.
	// The top level handler might not have this information.
	reqInfo := logger.GetReqInfo(ctx)
	var newReqInfo *logger.ReqInfo
	if reqInfo != nil {
		newReqInfo = logger.NewReqInfo(reqInfo.RemoteHost, reqInfo.UserAgent, reqInfo.RequestID, reqInfo.API, bucket, object)
	} else {
		newReqInfo = logger.NewReqInfo("", "", "", "Heal", bucket, object)
	}
	healCtx := logger.SetReqInfo(context.Background(), newReqInfo)

	// Healing directories handle it separately.
	if hasSuffix(object, slashSeparator) {
		return xl.healObjectDir(healCtx, bucket, object, dryRun)
	}

	storageDisks := xl.getDisks()

	// FIXME: Metadata is read again in the healObject() call below.
	// Read metadata files from all the disks
	partsMetadata, errs := readAllXLMetadata(healCtx, storageDisks, bucket, object)

	latestXLMeta, err := getLatestXLMeta(healCtx, partsMetadata, errs)
	if err != nil {
		return defaultHealResult(storageDisks, errs, bucket, object), toObjectErr(err, bucket, object)
	}

	// Lock the object before healing.
	objectLock := xl.nsMutex.NewNSLock(bucket, object)
	if lerr := objectLock.GetRLock(globalHealingTimeout); lerr != nil {
		return defaultHealResult(storageDisks, errs, bucket, object), lerr
	}
	defer objectLock.RUnlock()

	// Heal the object.
	return healObject(healCtx, xl.getDisks(), bucket, object, latestXLMeta.Erasure.DataBlocks, dryRun)
}
