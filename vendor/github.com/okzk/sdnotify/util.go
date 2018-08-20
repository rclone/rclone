package sdnotify

import (
	"errors"
	"fmt"
)

// ErrSdNotifyNoSocket is the error returned when the NOTIFY_SOCKET does not exist.
var ErrSdNotifyNoSocket = errors.New("No socket")

// Ready sends READY=1 to the systemd notify socket.
func Ready() error {
	return SdNotify("READY=1")
}

// Stopping sends STOPPING=1 to the systemd notify socket.
func Stopping() error {
	return SdNotify("STOPPING=1")
}

// Reloading sends RELOADING=1 to the systemd notify socket.
func Reloading() error {
	return SdNotify("RELOADING=1")
}

// Errno sends ERRNO=? to the systemd notify socket.
func Errno(errno int) error {
	return SdNotify(fmt.Sprintf("ERRNO=%d", errno))
}

// Status sends STATUS=? to the systemd notify socket.
func Status(status string) error {
	return SdNotify("STATUS=" + status)
}

// Watchdog sends WATCHDOG=1 to the systemd notify socket.
func Watchdog() error {
	return SdNotify("WATCHDOG=1")
}
