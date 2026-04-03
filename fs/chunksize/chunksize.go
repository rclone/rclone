// Package chunksize calculates a suitable chunk size for large uploads
package chunksize

import (
	"github.com/rclone/rclone/fs"
)

// Calculator calculates the minimum chunk size needed to fit within
// the maximum number of parts, rounded up to the nearest fs.Mebi.
//
// For most backends, (chunk_size) * (concurrent_upload_routines)
// memory will be required so we want to use the smallest possible
// chunk size that's going to allow the upload to proceed. Rounding up
// to the nearest fs.Mebi on the assumption that some backends may
// only allow integer type parameters when specifying the chunk size.
//
// Returns the default chunk size if it is sufficiently large enough
// to support the given file size otherwise returns the smallest chunk
// size necessary to allow the upload to proceed.
func Calculator(o any, size int64, maxParts int, defaultChunkSize fs.SizeSuffix) fs.SizeSuffix {
	// If streaming then use default chunk size
	if size < 0 {
		fs.Debugf(o, "Streaming upload with chunk_size %s allows uploads of up to %s and will fail only when that limit is reached.", defaultChunkSize, fs.SizeSuffix(maxParts)*defaultChunkSize)

		return defaultChunkSize
	}
	fileSize := fs.SizeSuffix(size)
	requiredChunks := fileSize / defaultChunkSize
	if requiredChunks < fs.SizeSuffix(maxParts) || (requiredChunks == fs.SizeSuffix(maxParts) && fileSize%defaultChunkSize == 0) {
		return defaultChunkSize
	}

	minChunk := fileSize / fs.SizeSuffix(maxParts)
	remainder := minChunk % fs.Mebi
	if remainder != 0 {
		minChunk += fs.Mebi - remainder
	}
	if fileSize/minChunk == fs.SizeSuffix(maxParts) && fileSize%fs.SizeSuffix(maxParts) != 0 { // when right on the boundary, we need to add a MiB
		minChunk += fs.Mebi
	}

	fs.Debugf(o, "size: %v, parts: %v, default: %v, new: %v; default chunk size insufficient, returned new chunk size", fileSize, maxParts, defaultChunkSize, minChunk)
	return minChunk
}
