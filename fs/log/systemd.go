// Systemd interface for non-Unix variants only

//go:build !unix
// +build !unix

package log

import (
	"log"
	"runtime"
)

// Enables systemd logs if configured or if auto-detected
func startSystemdLog() bool {
	log.Fatalf("--log-systemd not supported on %s platform", runtime.GOOS)
	return false
}

func isJournalStream() bool {
	return false
}
