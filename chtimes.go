// Default implementation of Chtimes

// +build !linux

package main

import (
	"os"
	"time"
)

func Chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}
