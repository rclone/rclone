// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"strings"
)

// Path represents a object path.
type Path = string

// SplitPath splits path into a slice of path components.
func SplitPath(path Path) []string {
	return strings.Split(path, "/")
}

// JoinPaths concatenates paths to a new single path.
func JoinPaths(paths ...Path) Path {
	return strings.Join(paths, "/")
}
