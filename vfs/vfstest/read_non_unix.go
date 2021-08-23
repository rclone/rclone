//go:build !linux && !darwin && !freebsd
// +build !linux,!darwin,!freebsd

package vfstest

import (
	"runtime"

	"github.com/rclone/rclone/fstest/retesting"
)

// TestReadFileDoubleClose tests double close on read
func TestReadFileDoubleClose(t retesting.T) {
	t.Skip("not supported on " + runtime.GOOS)
}
