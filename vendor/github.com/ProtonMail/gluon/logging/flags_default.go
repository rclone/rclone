//go:build !debug

package logging

import "runtime"

func getDefaultLabels(pc uintptr, file string, line int) Labels {
	return Labels{
		FnKey:   runtime.FuncForPC(pc).Name(),
		FileKey: file,
		LineKey: line,
	}
}
