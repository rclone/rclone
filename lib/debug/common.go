// Package debug contains functions for dealing with runtime/debug functions across go versions
package debug

import (
	"runtime/debug"
)

// SetGCPercent calls the runtime/debug.SetGCPercent function to set the garbage
// collection percentage.
func SetGCPercent(percent int) int {
	return debug.SetGCPercent(percent)
}
