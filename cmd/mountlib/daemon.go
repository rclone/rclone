// Daemonization interface for non-Unix variants only

// +build windows plan9 js

package mountlib

import (
	"log"
	"runtime"
)

func startBackgroundMode() bool {
	log.Fatalf("background mode not supported on %s platform", runtime.GOOS)
	return false
}
