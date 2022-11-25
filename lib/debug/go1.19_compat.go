//go:build !go1.19
// +build !go1.19

package debug

import (
	"fmt"
	"runtime"
)

// SetMemoryLimit is a no-op on Go version < 1.19.
func SetMemoryLimit(limit int64) (int64, error) {
	return limit, fmt.Errorf("not implemented on Go version below 1.19: %s", runtime.Version())
}
