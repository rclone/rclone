package log

import (
	"runtime"
	"strings"
)

func getSingleCallerPc(skip int) uintptr {
	var pc [1]uintptr
	runtime.Callers(skip+2, pc[:])
	return pc[0]
}

type Loc struct {
	Package  string
	Function string
	File     string
	Line     int
}

func locFromPc(pc uintptr) Loc {
	f, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	lastSlash := strings.LastIndexByte(f.Function, '/')
	firstDot := strings.IndexByte(f.Function[lastSlash+1:], '.')
	return Loc{
		Package:  f.Function[:lastSlash+1+firstDot],
		Function: f.Function,
		File:     f.File,
		Line:     f.Line,
	}
}

func getMsgLogLoc(msg Msg) Loc {
	var pc [1]uintptr
	msg.Callers(1, pc[:])
	return locFromPc(pc[0])

}
