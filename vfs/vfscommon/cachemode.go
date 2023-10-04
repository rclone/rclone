// Package vfscommon provides utilities for VFS.
package vfscommon

import (
	"github.com/rclone/rclone/fs"
)

type cacheModeChoices struct{}

func (cacheModeChoices) Choices() []string {
	return []string{
		CacheModeOff:     "off",
		CacheModeMinimal: "minimal",
		CacheModeWrites:  "writes",
		CacheModeFull:    "full",
	}
}

// CacheMode controls the functionality of the cache
type CacheMode = fs.Enum[cacheModeChoices]

// CacheMode options
const (
	CacheModeOff     CacheMode = iota // cache nothing - return errors for writes which can't be satisfied
	CacheModeMinimal                  // cache only the minimum, e.g. read/write opens
	CacheModeWrites                   // cache all files opened with write intent
	CacheModeFull                     // cache all files opened in any mode
)

// Type of the value
func (cacheModeChoices) Type() string {
	return "CacheMode"
}
