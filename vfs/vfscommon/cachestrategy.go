// Package vfscommon provides utilities for VFS.
package vfscommon

import (
	"github.com/rclone/rclone/fs"
)

type cacheStrategyChoices struct{}

func (cacheStrategyChoices) Choices() []string {
	return []string{
		CacheStrategyLRU:   "lru",
		CacheStrategyLFU:   "lfu",
		CacheStrategyLFF:   "lff",
		CacheStrategyLRUSP: "lru-sp",
	}
}

// CacheStrategy controls the eviction strategy of the cache
type CacheStrategy = fs.Enum[cacheStrategyChoices]

// CacheStrategy options
const (
	CacheStrategyLRU   CacheStrategy = iota // least recently used
	CacheStrategyLFU                        // least frequently used
	CacheStrategyLFF                        // largest file first
	CacheStrategyLRUSP                      // least recently used, adjusted for size and popularity
)

// Type of the value
func (cacheStrategyChoices) Type() string {
	return "CacheStrategy"
}
