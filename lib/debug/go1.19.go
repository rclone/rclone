//go:build go1.19
// +build go1.19

package debug

import (
	"runtime/debug"
)

// SetMemoryLimit calls the runtime/debug.SetMemoryLimit function to set the
// soft-memory limit.
func SetMemoryLimit(limit int64) (int64, error) {
	return debug.SetMemoryLimit(limit), nil
}
