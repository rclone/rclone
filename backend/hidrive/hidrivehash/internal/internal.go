// Package internal provides utilities for HiDrive.
package internal

import (
	"encoding"
	"hash"
)

// LevelHash is an internal interface for level-hashes.
type LevelHash interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	hash.Hash
	// Add takes a position-embedded checksum and adds it to the level.
	Add(sum []byte)
	// IsFull returns whether the number of checksums added to this level reached its capacity.
	IsFull() bool
}
