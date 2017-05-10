// +build !linux,!darwin,!freebsd

package mounttest

import (
	"runtime"
	"testing"
)

// TestWriteFileDoubleClose tests double close on write
func TestWriteFileDoubleClose(t *testing.T) {
	t.Skip("not supported on " + runtime.GOOS)
}
