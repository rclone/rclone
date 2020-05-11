// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"strings"
)

//
// To avoid confusion about when paths are encrypted, unencrypted, empty or
// non existent, we create some wrapper types so that the compiler will complain
// if someone attempts to use one in the wrong context.
//

// Unencrypted is an opaque type representing an unencrypted path.
type Unencrypted struct {
	raw string
}

// Encrypted is an opaque type representing an encrypted path.
type Encrypted struct {
	raw string
}

//
// unencrypted paths
//

// NewUnencrypted takes a raw unencrypted path and returns it wrapped.
func NewUnencrypted(raw string) Unencrypted {
	return Unencrypted{raw: raw}
}

// Valid returns if the unencrypted path is valid, which is the same as not being empty.
func (path Unencrypted) Valid() bool {
	return path.raw != ""
}

// Raw returns the original raw path for the Unencrypted.
func (path Unencrypted) Raw() string {
	return path.raw
}

// String returns a human readable form of the Unencrypted.
func (path Unencrypted) String() string {
	return path.Raw()
}

// Consume attempts to remove the prefix from the Unencrypted path and
// reports a boolean indicating if it was able to do so.
func (path Unencrypted) Consume(prefix Unencrypted) (Unencrypted, bool) {
	if len(path.raw) >= len(prefix.raw) && path.raw[:len(prefix.raw)] == prefix.raw {
		return NewUnencrypted(path.raw[len(prefix.raw):]), true
	}
	return Unencrypted{}, false
}

// Iterator returns an iterator over the components of the Unencrypted.
func (path Unencrypted) Iterator() Iterator {
	return NewIterator(path.raw)
}

// Less returns true if 'path' should be sorted earlier than 'other'
func (path Unencrypted) Less(other Unencrypted) bool {
	return path.raw < other.raw
}

//
// encrypted path
//

// NewEncrypted takes a raw encrypted path and returns it wrapped.
func NewEncrypted(raw string) Encrypted {
	return Encrypted{raw: raw}
}

// Valid returns if the encrypted path is valid, which is the same as not being empty.
func (path Encrypted) Valid() bool {
	return path.raw != ""
}

// Raw returns the original path for the Encrypted.
func (path Encrypted) Raw() string {
	return path.raw
}

// String returns a human readable form of the Encrypted.
func (path Encrypted) String() string {
	return path.Raw()
}

// Consume attempts to remove the prefix from the Encrypted path and
// reports a boolean indicating if it was able to do so.
func (path Encrypted) Consume(prefix Encrypted) (Encrypted, bool) {
	if len(path.raw) >= len(prefix.raw) && path.raw[:len(prefix.raw)] == prefix.raw {
		return NewEncrypted(path.raw[len(prefix.raw):]), true
	}
	return Encrypted{}, false
}

// Iterator returns an iterator over the components of the Encrypted.
func (path Encrypted) Iterator() Iterator {
	return NewIterator(path.raw)
}

// Less returns true if 'path' should be sorted earlier than 'other'
func (path Encrypted) Less(other Encrypted) bool {
	return path.raw < other.raw
}

//
// path component iteration
//

// Iterator allows one to efficiently iterate over components of a path.
type Iterator struct {
	raw       string
	consumed  int
	lastEmpty bool
}

// NewIterator returns an Iterator for components of the provided raw path.
func NewIterator(raw string) Iterator {
	return Iterator{raw: raw, lastEmpty: raw != ""}
}

// Consumed reports how much of the path has been consumed (if any).
func (pi Iterator) Consumed() string { return pi.raw[:pi.consumed] }

// Remaining reports how much of the path is remaining.
func (pi Iterator) Remaining() string { return pi.raw[pi.consumed:] }

// Done reports if the path has been fully consumed.
func (pi Iterator) Done() bool { return len(pi.raw) == pi.consumed && !pi.lastEmpty }

// Next returns the first component of the path, consuming it.
func (pi *Iterator) Next() string {
	if pi.Done() {
		return ""
	}

	rem := pi.Remaining()
	index := strings.IndexByte(rem, '/')
	if index == -1 {
		pi.consumed += len(rem)
		pi.lastEmpty = false
		return rem
	}
	pi.consumed += index + 1
	pi.lastEmpty = index == len(rem)-1
	return rem[:index]
}
