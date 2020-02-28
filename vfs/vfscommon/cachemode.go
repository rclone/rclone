package vfscommon

import (
	"fmt"

	"github.com/rclone/rclone/lib/errors"
)

// CacheMode controls the functionality of the cache
type CacheMode byte

// CacheMode options
const (
	CacheModeOff     CacheMode = iota // cache nothing - return errors for writes which can't be satisfied
	CacheModeMinimal                  // cache only the minimum, eg read/write opens
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
	return errors.Errorf("Unknown cache mode level %q", s)
}

// Type of the value
func (l *CacheMode) Type() string {
	return "CacheMode"
}
