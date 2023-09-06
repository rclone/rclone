// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package fpath

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// FPath is an OS independent path handling structure.
type FPath struct {
	original string // the original URL or local path
	local    bool   // if local path
	bucket   string // only for Storj URL
	path     string // only for Storj URL - the path within the bucket, cleaned from duplicated slashes
}

var parseSchemeRegex = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9+.-]*):(.*)$`)

func parseScheme(o string) (scheme, rest string) {
	found := parseSchemeRegex.FindStringSubmatch(o)

	switch len(found) {
	case 2:
		return strings.ToLower(found[1]), ""
	case 3:
		return strings.ToLower(found[1]), found[2]
	}

	return "", o
}

var parseBucketRegex = regexp.MustCompile(`^/{1,4}([^/]+)(/.*)?$`)

func parseBucket(o string) (bucket, rest string) {
	found := parseBucketRegex.FindStringSubmatch(o)

	switch len(found) {
	case 2:
		return found[1], ""
	case 3:
		return found[1], found[2]
	}

	return "", o
}

// New creates new FPath from the given URL.
func New(p string) (FPath, error) {
	fp := FPath{original: p}

	// Skip processing further if we can determine this is an absolute
	// path to a local file.
	if filepath.IsAbs(p) {
		fp.local = true

		return fp, nil
	}

	// Does the path have a scheme? If not then we treat it as a local
	// path. Otherwise we validate that the scheme is a supported one.
	scheme, rest := parseScheme(p)
	if scheme == "" {
		// Forbid the use of an empty scheme.
		if strings.HasPrefix(rest, ":") {
			return fp, errors.New("malformed URL: missing scheme, use format sj://bucket/")
		}

		fp.local = true

		return fp, nil
	}

	switch scheme {
	case "s3":
	case "sj":
	default:
		return fp, fmt.Errorf("unsupported URL scheme: %s, use format sj://bucket/", scheme)
	}

	// The remaining portion of the path must begin with a bucket.
	bucket, rest := parseBucket(rest)
	if bucket == "" {
		return fp, errors.New("no bucket specified, use format sj://bucket/")
	}

	fp.bucket = bucket

	// We only want to clean the path if it is non-empty. This is because
	// path. Clean will turn an empty path into ".".
	rest = strings.TrimLeft(rest, "/")
	if rest != "" {
		fp.path = path.Clean(rest)
	}

	return fp, nil
}

// Join is appends the given segment to the path.
func (p FPath) Join(segment string) FPath {
	if p.local {
		p.original = filepath.Join(p.original, segment)
		return p
	}

	p.original += "/" + segment
	p.path = path.Join(p.path, segment)
	return p
}

// Base returns the last segment of the path.
func (p FPath) Base() string {
	if p.local {
		return filepath.Base(p.original)
	}
	if p.path == "" {
		return ""
	}
	return path.Base(p.path)
}

// Bucket returns the first segment of path.
func (p FPath) Bucket() string {
	return p.bucket
}

// Path returns the URL path without the scheme.
func (p FPath) Path() string {
	if p.local {
		return p.original
	}
	return p.path
}

// IsLocal returns whether the path refers to local or remote location.
func (p FPath) IsLocal() bool {
	return p.local
}

// String returns the entire URL (untouched).
func (p FPath) String() string {
	return p.original
}
