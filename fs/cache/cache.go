// Package cache implements the Fs cache
package cache

import (
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/cache"
)

var (
	c = cache.New()
)

// GetFn gets a fs.Fs named fsString either from the cache or creates
// it afresh with the create function
func GetFn(fsString string, create func(fsString string) (fs.Fs, error)) (f fs.Fs, err error) {
	value, err := c.Get(fsString, func(fsString string) (value interface{}, ok bool, error error) {
		f, err := create(fsString)
		ok = err == nil || err == fs.ErrorIsFile
		return f, ok, err
	})
	if err != nil {
		return nil, err
	}
	return value.(fs.Fs), nil
}

// Get gets a fs.Fs named fsString either from the cache or creates it afresh
func Get(fsString string) (f fs.Fs, err error) {
	return GetFn(fsString, fs.NewFs)
}

// Put puts an fs.Fs named fsString into the cache
func Put(fsString string, f fs.Fs) {
	c.Put(fsString, f)
}

// Clear removes everything from the cahce
func Clear() {
	c.Clear()
}
