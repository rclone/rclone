// Accounting and limiting reader
// Unix specific functions.

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package accounting

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rclone/rclone/fs"
)

// startSignalHandler() sets a signal handler to catch SIGUSR2 and toggle throttling.
func (tb *tokenBucket) startSignalHandler() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR2)

	go func() {
		// This runs forever, but blocks until the signal is received.
		for {
			<-signals

			func() {
				tb.mu.Lock()
				defer tb.mu.Unlock()

				// if there's no bandwidth limit configured now, do nothing
				if !tb.currLimit.Bandwidth.IsSet() {
					fs.Debugf(nil, "SIGUSR2 received but no bandwidth limit configured right now, ignoring")
					return
				}

				tb.toggledOff = !tb.toggledOff
				tb.curr, tb.prev = tb.prev, tb.curr
				s := "disabled"
				if !tb.curr._isOff() {
					s = "enabled"
				}

				fs.Logf(nil, "Bandwidth limit %s by user", s)
			}()
		}
	}()
}
