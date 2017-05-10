// +build !linux,!darwin,!freebsd

package mounttest

import (
	"runtime"
	"testing"
)

// TestReadFileDoubleClose tests double close on read
func TestReadFileDoubleClose(t *testing.T) {
	t.Skip("not supported on " + runtime.GOOS)
}
