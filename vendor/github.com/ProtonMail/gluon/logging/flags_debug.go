//go:build debug

package logging

import (
	"runtime"
	"runtime/debug"
)

// StackKey refers to the stack trace.
const StackKey = "stack"

func getDefaultLabels(pc uintptr, file string, line int) Labels {
	return Labels{
		FnKey:    runtime.FuncForPC(pc).Name(),
		FileKey:  file,
		LineKey:  line,
		StackKey: string(debug.Stack()),
	}
}
