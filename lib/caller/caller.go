// Package caller contains functions to examine the call stack.
package caller

import (
	"runtime"
	"strings"
)

// Present looks for functionName in the call stack and return true if found
//
// Note that this ignores the caller.
func Present(functionName string) bool {
	var pcs [48]uintptr
	n := runtime.Callers(3, pcs[:]) // skip runtime.Callers, Present and caller
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if strings.HasSuffix(f.Function, functionName) {
			return true
		}
		if !more {
			break
		}
	}
	return false
}
