//+build !windows,!plan9

package atexit

import (
	"os"
	"syscall"
)

var exitSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM} // Not syscall.SIGQUIT as we want the default behaviour
