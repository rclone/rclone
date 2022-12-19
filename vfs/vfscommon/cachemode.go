// Package vfscommon provides utilities for VFS.
package vfscommon

import (
	"fmt"

	"github.com/rclone/rclone/fs"
)

// CacheMode controls the functionality of the cache
type CacheMode byte

// CacheMode options
const (
	CacheModeOff     CacheMode = iota // cache nothing - return errors for writes which can't be satisfied
	CacheModeMinimal                  // cache only the minimum, e.g. read/write opens
	CacheModeWrites                   // cache all files opened with write intent
	CacheModeFull                     // cache all files opened in any mode
)

var cacheModeToString = []string{
	CacheModeOff:     "off",
	CacheModeMinimal: "minimal",
	CacheModeWrites:  "writes",
	CacheModeFull:    "full",
}

// String turns a CacheMode into a string
func (l CacheMode) String() string {
	if l >= CacheMode(len(cacheModeToString)) {
		return fmt.Sprintf("CacheMode(%d)", l)
	}
	return cacheModeToString[l]
}

// Set a CacheMode
func (l *CacheMode) Set(s string) error {
	for n, name := range cacheModeToString {
		if s != "" && name == s {
			*l = CacheMode(n)
			return nil
		}
	}
	return fmt.Errorf("unknown cache mode level %q", s)
}

// Type of the value
func (l *CacheMode) Type() string {
	return "CacheMode"
}

// UnmarshalJSON makes sure the value can be parsed as a string or integer in JSON
func (l *CacheMode) UnmarshalJSON(in []byte) error {
	return fs.UnmarshalJSONFlag(in, l, func(i int64) error {
		if i < 0 || i >= int64(len(cacheModeToString)) {
			return fmt.Errorf("unknown cache mode level %d", i)
		}
		*l = CacheMode(i)
		return nil
	})
}
