//go:build !linux && !darwin && !freebsd

package vfstest

import (
	"runtime"
	"testing"
)

// TestReadFileDoubleClose tests double close on read
func TestReadFileDoubleClose(t *testing.T) {
	t.Skip("not supported on " + runtime.GOOS)
}
