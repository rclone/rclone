//go:build !plan9 && !js && !wasm

package mountlib

import (
	"os"
	"os/signal"
	"syscall"
)

// NotifyOnSigHup makes SIGHUP notify given channel on supported systems
func NotifyOnSigHup(sighupChan chan os.Signal) {
	signal.Notify(sighupChan, syscall.SIGHUP)
}
