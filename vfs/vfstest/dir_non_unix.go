//go:build !linux && !darwin && !freebsd

package vfstest

import (
	"runtime"
	"testing"
)

// TestDirRewind checks that re-reading a rewound directory works
func TestDirRewind(t *testing.T) {
	t.Skip("not supported on " + runtime.GOOS)
}
