// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"strings"

	"storj.io/common/paths"
	"storj.io/common/storj"
)

// Path is a representation of an object path within a bucket.
type Path struct {
	bucket    string
	unencPath paths.Unencrypted
	raw       []byte
}

// Bucket returns the bucket part of the path.
func (p Path) Bucket() string { return p.bucket }

// UnencryptedPath returns the unencrypted path part of the path.
func (p Path) UnencryptedPath() paths.Unencrypted { return p.unencPath }

// Raw returns the raw data in the path.
func (p Path) Raw() []byte { return append([]byte(nil), p.raw...) }

// String returns the string form of the raw data in the path.
func (p Path) String() string { return string(p.raw) }

// ParsePath returns a new Path with the given raw bytes.
func ParsePath(raw storj.Path) Path {
	// A path may contain a bucket and an unencrypted path.
	parts := strings.SplitN(raw, "/", 2)
	path := Path{}
	path.bucket = parts[0]
	if len(parts) > 1 {
		path.unencPath = paths.NewUnencrypted(parts[1])
	}
	path.raw = []byte(raw)
	return path
}
