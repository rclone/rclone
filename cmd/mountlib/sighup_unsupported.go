//go:build plan9 || js || wasm

package mountlib

import (
	"os"
)

// NotifyOnSigHup makes SIGHUP notify given channel on supported systems
func NotifyOnSigHup(sighupChan chan os.Signal) {}
