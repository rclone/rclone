// Syslog interface for non-Unix variants only

//go:build windows || nacl || plan9
// +build windows nacl plan9

package log

import (
	"log"
	"runtime"
)

// Starts syslog if configured, returns true if it was started
func startSysLog() bool {
	log.Fatalf("--syslog not supported on %s platform", runtime.GOOS)
	return false
}
