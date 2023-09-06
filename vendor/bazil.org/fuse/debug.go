package fuse

import (
	"runtime"
)

func stack() string {
	buf := make([]byte, 1024)
	return string(buf[:runtime.Stack(buf, false)])
}

func nop(msg interface{}) {}

// Debug is called to output debug messages, including protocol
// traces. The default behavior is to do nothing.
//
// The messages have human-friendly string representations and are
// safe to marshal to JSON.
//
// Implementations must not retain msg.
var Debug func(msg interface{}) = nop
