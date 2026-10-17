// Package khash provides the kDrive specific hash handling.
//
// kDrive uses XXH3-64 for file hashing. Files uploaded in chunks (by
// rclone, or by the kDrive apps which each use their own chunk size)
// get a "nested" hash: the XXH3 of the concatenated lowercase hex
// strings of the individual chunk hashes. Such a nested hash can't be
// compared with a plain XXH3 of the file contents.
//
// Hash formats returned by the kDrive API:
//   - "xxh3:HASH" for a plain XXH3 of the file contents
//   - "N:xxh3:HASH" for the nested hash of a chunked upload
package khash

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/zeebo/xxh3"
)

// ParseHash parses a kDrive hash string from the API and returns the hex hash value
// along with whether it is a nested (chunked) hash.
func ParseHash(hashStr string) (hexHash string, isNested bool, err error) {
	if hashStr == "" {
		return "", false, nil
	}

	// Check for nested hash format: "N:xxh3:HASH"
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

// NestedChunkHash computes the kDrive nested hash from the hashes of the
// file chunks, in order.
//
// kDrive stores the nested hash of a chunked file as the XXH3 of the
// concatenated lowercase hex strings of the individual chunk hashes.
// Each chunk hash may be passed with or without the "xxh3:" prefix.
func NestedChunkHash(chunkHashes []string) (string, error) {
	if len(chunkHashes) == 0 {
		return "", errors.New("no chunk hashes to nest")
	}
	hexHashes := make([]string, len(chunkHashes))
	for i, chunkHash := range chunkHashes {
		hexHash, _, err := ParseHash(chunkHash)
		if err != nil {
			return "", fmt.Errorf("invalid chunk hash %q: %w", chunkHash, err)
		}
		hexHashes[i] = hexHash
	}
	hasher := xxh3.New()
	_, _ = hasher.Write([]byte(strings.Join(hexHashes, "")))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
