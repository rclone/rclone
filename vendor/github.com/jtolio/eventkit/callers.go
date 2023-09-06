package eventkit

import (
	"runtime"
	"strings"
)

func callerPackage(frames int) string {
	var pc [1]uintptr
	if runtime.Callers(frames+2, pc[:]) != 1 {
		return "unknown"
	}
	frame, _ := runtime.CallersFrames(pc[:]).Next()
	if frame.Func == nil {
		return "unknown"
	}
	slash_pieces := strings.Split(frame.Func.Name(), "/")
	dot_pieces := strings.SplitN(slash_pieces[len(slash_pieces)-1], ".", 2)
	return strings.Join(slash_pieces[:len(slash_pieces)-1], "/") + "/" + dot_pieces[0]
}
