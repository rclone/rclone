//go:build !linux && !darwin && !freebsd && !windows
// +build !linux,!darwin,!freebsd,!windows

package vfstest

import (
	"errors"
	"runtime"
	"testing"
)

// TestWriteFileDoubleClose tests double close on write
func TestWriteFileDoubleClose(t *testing.T) {
	t.Skip("not supported on " + runtime.GOOS)
}

// writeTestDup performs the platform-specific implementation of the dup() unix
func writeTestDup(oldfd uintptr) (uintptr, error) {
	return 0, errors.New("not supported on " + runtime.GOOS)
}
