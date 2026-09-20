//go:build !linux && !darwin && !freebsd && !openbsd

package vfstest

import (
	"runtime"
	"testing"
)

// TestMknod is not supported on this platform.
func TestMknod(t *testing.T) {
	t.Skip("not supported on " + runtime.GOOS)
}
