//go:build !linux && !darwin && !freebsd
// +build !linux,!darwin,!freebsd

package vfstest

import (
	"runtime"
	"testing"

	"golang.org/x/sys/windows"
)

// TestWriteFileDoubleClose tests double close on write
func TestWriteFileDoubleClose(t *testing.T) {
	t.Skip("not supported on " + runtime.GOOS)
}

// writeTestDup performs the platform-specific implementation of the dup() syscall
func writeTestDup(oldfd uintptr) (uintptr, error) {
	p := windows.CurrentProcess()
	var h windows.Handle
	return uintptr(h), windows.DuplicateHandle(p, windows.Handle(oldfd), p, &h, 0, true, windows.DUPLICATE_SAME_ACCESS)
}
