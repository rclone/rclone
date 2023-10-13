package nfs

import "time"

// FSStat returns metadata about a file system
type FSStat struct {
	TotalSize      uint64
	FreeSize       uint64
	AvailableSize  uint64
	TotalFiles     uint64
	FreeFiles      uint64
	AvailableFiles uint64
	// CacheHint is called "invarsec" in the nfs standard
	CacheHint time.Duration
}
