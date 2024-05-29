//go:build windows || plan9

package atexit

import (
	"os"

	"github.com/rclone/rclone/lib/exitcode"
)

var exitSignals = []os.Signal{os.Interrupt}

func exitCode(_ os.Signal) int {
	return exitcode.UncategorizedError
}
