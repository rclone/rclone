// Accounting and limiting reader
// Unix specific functions.

// +build darwin dragonfly freebsd linux netbsd openbsd

package fs

import (
	"os"
	"os/signal"
	"syscall"
)

// startSignalHandler() sets a signal handler to catch SIGUSR2 and toggle throttling.
func startSignalHandler() {
	// Don't do anything if no bandwidth limits requested.
	if bwLimit <= 0 {
		return
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR2)

	go func() {
		// This runs forever, but blocks until the signal is received.
		for {
			<-signals
			if tokenBucket == nil {
				tokenBucket = origTokenBucket
			} else {
				tokenBucket = nil
			}
		}
	}()
}
