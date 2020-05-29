// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"time"

	"github.com/zeebo/errs"
)

var (
	// ErrNoPath is an error class for using empty path.
	ErrNoPath = errs.Class("no path specified")

	// ErrObjectNotFound is an error class for non-existing object.
	ErrObjectNotFound = errs.Class("object not found")
)

// Object contains information about a specific object.
type Object struct {
	Version  uint32
	Bucket   Bucket
	Path     Path
	IsPrefix bool

	Metadata map[string]string

	ContentType string
	Created     time.Time
	Modified    time.Time
	Expires     time.Time

	Stream
}

// ObjectInfo contains information about a specific object.
type ObjectInfo struct {
	Version  uint32
	Bucket   string
	Path     Path
	IsPrefix bool

	StreamID StreamID

	Metadata []byte

	ContentType string
	Created     time.Time
	Modified    time.Time
	Expires     time.Time

	Stream
}

// Stream is information about an object stream.
type Stream struct {
	ID StreamID

	// Size is the total size of the stream in bytes
	Size int64
	// Checksum is the checksum of the segment checksums
	Checksum []byte

	// SegmentCount is the number of segments
	SegmentCount int64
	// FixedSegmentSize is the size of each segment,
	// when all segments have the same size. It is -1 otherwise.
	FixedSegmentSize int64

	// RedundancyScheme specifies redundancy strategy used for this stream
	RedundancyScheme
	// EncryptionParameters specifies encryption strategy used for this stream
	EncryptionParameters

	LastSegment LastSegment // TODO: remove
}

// LastSegment contains info about last segment.
//
// TODO: remove.
type LastSegment struct {
	Size              int64
	EncryptedKeyNonce Nonce
	EncryptedKey      EncryptedPrivateKey
}

// Segment is full segment information.
type Segment struct {
	Index int64
	// Size is the size of the content in bytes
	Size int64
	// Checksum is the checksum of the content
	Checksum []byte
	// Local data
	Inline []byte
	// Remote data
	PieceID PieceID
	Pieces  []Piece
	// Encryption
	EncryptedKeyNonce Nonce
	EncryptedKey      EncryptedPrivateKey
}

// Piece is information where a piece is located.
type Piece struct {
	Number   byte
	Location NodeID
}
