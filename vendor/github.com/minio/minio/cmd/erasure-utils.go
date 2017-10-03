/*
 * Minio Cloud Storage, (C) 2016 Minio, Inc.
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
	"bytes"
	"io"

	"github.com/klauspost/reedsolomon"
)

// getDataBlockLen - get length of data blocks from encoded blocks.
func getDataBlockLen(enBlocks [][]byte, dataBlocks int) int {
	size := 0
	// Figure out the data block length.
	for _, block := range enBlocks[:dataBlocks] {
		size += len(block)
	}
	return size
}

// Writes all the data blocks from encoded blocks until requested
// outSize length. Provides a way to skip bytes until the offset.
func writeDataBlocks(dst io.Writer, enBlocks [][]byte, dataBlocks int, offset int64, length int64) (int64, error) {
	// Offset and out size cannot be negative.
	if offset < 0 || length < 0 {
		return 0, traceError(errUnexpected)
	}

	// Do we have enough blocks?
	if len(enBlocks) < dataBlocks {
		return 0, traceError(reedsolomon.ErrTooFewShards)
	}

	// Do we have enough data?
	if int64(getDataBlockLen(enBlocks, dataBlocks)) < length {
		return 0, traceError(reedsolomon.ErrShortData)
	}

	// Counter to decrement total left to write.
	write := length

	// Counter to increment total written.
	var totalWritten int64

	// Write all data blocks to dst.
	for _, block := range enBlocks[:dataBlocks] {
		// Skip blocks until we have reached our offset.
		if offset >= int64(len(block)) {
			// Decrement offset.
			offset -= int64(len(block))
			continue
		} else {
			// Skip until offset.
			block = block[offset:]

			// Reset the offset for next iteration to read everything
			// from subsequent blocks.
			offset = 0
		}
		// We have written all the blocks, write the last remaining block.
		if write < int64(len(block)) {
			n, err := io.Copy(dst, bytes.NewReader(block[:write]))
			if err != nil {
				return 0, traceError(err)
			}
			totalWritten += n
			break
		}
		// Copy the block.
		n, err := io.Copy(dst, bytes.NewReader(block))
		if err != nil {
			return 0, traceError(err)
		}

		// Decrement output size.
		write -= n

		// Increment written.
		totalWritten += n
	}

	// Success.
	return totalWritten, nil
}

// chunkSize is roughly BlockSize/DataBlocks.
// chunkSize is calculated such that chunkSize*DataBlocks accommodates BlockSize bytes.
// So chunkSize*DataBlocks can be slightly larger than BlockSize if BlockSize is not divisible by
// DataBlocks. The extra space will have 0-padding.
func getChunkSize(blockSize int64, dataBlocks int) int64 {
	return (blockSize + int64(dataBlocks) - 1) / int64(dataBlocks)
}
