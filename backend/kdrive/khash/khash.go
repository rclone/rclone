// Package khash provides the kDrive specific hash implementation.
//
// kDrive uses XXH3-64 for file hashing. For files larger than the chunk size (20MB),
// it computes a "nested" hash which is the XXH3 of all individual chunk hashes.
//
// The chunk size is dynamically calculated based on file size to avoid exceeding
// the maximum chunk count (10000 chunks).
//
// Hash formats returned by kDrive API:
//   - Current format: "xxh3:HASH" (both simple and nested hashes use this format)
//   - Format with chunk count:  "{chunks}:xxh3:HASH" where {chunks} is the actual chunk count
//
// This implementation computes both formats (raw bytes) and can compare with hashes retrieved from the API.
package khash

import (
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	"github.com/rclone/rclone/backend/kdrive/chunksize"
	"github.com/zeebo/xxh3"
)

// digest implements the hash.Hash interface for kDrive's XXH3 hashing
type digest struct {
	chunkSize    *int64       // Target size of each chunk
	bytesInChunk int64        // Bytes written to the current chunk so far
	currentHash  *xxh3.Hasher // Streaming hasher for the current chunk
	chunkHashes  []byte       // Concatenated 8-byte hashes of completed chunks
}

// New creates a new hash.Hash with default configuration (20MB clusters).
// Used by Rclone core for standard checks.
func New() hash.Hash {
	return NewWithChunkSize(chunksize.ChunkSizeConfig.DefaultChunkSize)
}

// NewForSize creates a new hash.Hash optimized for the specific file size.
// This ensures that the chunk size used for hashing matches what the kDrive backend
// would use for this file size (handling the 10000 chunk limit).
func NewForSize(fileSize int64) hash.Hash {
	cs := chunksize.CalculateChunkSize(fileSize, chunksize.ChunkSizeConfig.DefaultChunkSize)
	return NewWithChunkSize(cs)
}

// NewWithChunkSize creates a new hash.Hash with a custom chunk size.
func NewWithChunkSize(chunkSize int64) hash.Hash {
	d := &digest{
		chunkSize: &chunkSize,
	}
	d.Reset()
	return d
}

// Reset resets the Hash to its initial state.
func (d *digest) Reset() {
	d.currentHash = xxh3.New()
	d.bytesInChunk = 0
	d.chunkHashes = make([]byte, 0, 128) // Pre-allocate space for some chunk hashes
}

// Write adds more data to the running hash.
// It streams data into the current chunk hasher without buffering the content itself,
// ensuring O(1) memory usage regardless of chunk size.
func (d *digest) Write(p []byte) (n int, err error) {
	if d.chunkSize == nil {
		return d.currentHash.Write(p)
	}

	n = len(p)

	for len(p) > 0 {
		// How much space is left in the current "virtual" chunk?
		remaining := *d.chunkSize - d.bytesInChunk

		toWrite := int64(len(p))
		if toWrite > remaining {
			toWrite = remaining
		}

		// Stream directly to the hasher
		_, err := d.currentHash.Write(p[:toWrite])
		if err != nil {
			return 0, err
		}

		d.bytesInChunk += toWrite
		p = p[toWrite:]

		// If current chunk is full, finalize it
		if d.bytesInChunk == *d.chunkSize {
			d.finalizeChunk()
		}
	}

	return n, nil
}

// finalizeChunk computes the hash of the current full chunk and prepares for the next.
func (d *digest) finalizeChunk() {
	// sum the current chunk
	h := d.currentHash.Sum(nil)
	d.chunkHashes = append(d.chunkHashes, h...)

	// Reset for next chunk
	d.currentHash.Reset()
	d.bytesInChunk = 0
}

// Sum appends the current hash to b and returns the resulting slice.
// It handles both "Simple Hash" (single chunk) and "Nested Hash" (multi-chunk) logic.
func (d *digest) Sum(b []byte) []byte {
	// Case A: No full chunks yet (chunkHashes is empty)
	// Return the hash of the current partial chunk (Simple Hash)
	if d.chunkSize == nil || len(d.chunkHashes) == 0 {
		return d.currentHash.Sum(b)
	}

	// Case B: Exactly one full chunk and no partial data
	// Return the existing hash of the single chunk (Simple Hash)
	if len(d.chunkHashes) == 8 && d.bytesInChunk == 0 {
		return append(b, d.chunkHashes...)
	}

	// Case C: Multiple chunks (Nested Hash)
	// Build the list of all hashes
	finalHashes := make([]byte, len(d.chunkHashes), len(d.chunkHashes)+8)
	copy(finalHashes, d.chunkHashes)

	// Add the current partial chunk ONLY if it contains data
	if d.bytesInChunk > 0 {
		finalHashes = d.currentHash.Sum(finalHashes)
	}

	// Compute hash of hashes
	nestedHasher := xxh3.New()
	// kDrive nested hash uses the concatenated lowercase hex string of chunk hashes,
	// then applies xxh3 on that ASCII payload.
	_, _ = nestedHasher.Write([]byte(hex.EncodeToString(finalHashes)))
	return nestedHasher.Sum(b)
}

// Size returns the number of bytes Sum will return (8 bytes for XXH3-64).
func (d *digest) Size() int {
	return 8
}

// BlockSize returns the hash's underlying block size.
func (d *digest) BlockSize() int {
	return 64
}

// IsNestedHash checks if the given hash string is in the nested hash format.
func IsNestedHash(hashStr string) bool {
	return strings.HasPrefix(hashStr, "N:xxh3:")
}

// ParseHash parses a kDrive hash string from the API and returns the hex hash value
// along with the chunk count if available.
func ParseHash(hashStr string) (hexHash string, isNested bool, err error) {
	if hashStr == "" {
		return "", false, nil
	}

	// Check for new format with nested hash: "N:xxh3:HASH"
	if strings.HasPrefix(hashStr, "N:xxh3:") {
		hashStr = strings.TrimPrefix(hashStr, "N:xxh3:")
		isNested = true
	} else {
		hashStr = strings.TrimPrefix(hashStr, "xxh3:")
	}

	// Validate hex string
	if _, decodeErr := hex.DecodeString(hashStr); decodeErr != nil {
		return "", false, fmt.Errorf("invalid hash format: %w", decodeErr)
	}

	return hashStr, isNested, nil
}

// ValidateHash validates a local hash against a remote hash from the API.
func ValidateHash(localHashStr string, remoteHashStr string) (bool, error) {
	remoteHash, _, err := ParseHash(remoteHashStr)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(localHashStr, remoteHash), nil
}

// Verify interface compliance
var _ hash.Hash = (*digest)(nil)
