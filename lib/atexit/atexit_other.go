//+build windows plan9

package atexit

import (
	"os"
)

var exitSignals = []os.Signal{os.Interrupt}
