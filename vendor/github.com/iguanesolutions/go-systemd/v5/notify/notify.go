package sysdnotify

import (
	"fmt"
	"net"
)

var socket *net.UnixAddr

// IsEnabled tells if systemd notify socket has been detected or not.
func IsEnabled() bool {
	return socket != nil
}

// Ready sends systemd notify READY=1
func Ready() error {
	return Send("READY=1")
}

// Reloading sends systemd notify RELOADING=1
func Reloading() error {
	return Send("RELOADING=1")
}

// Stopping sends systemd notify STOPPING=1
func Stopping() error {
	return Send("STOPPING=1")
}

// Status sends systemd notify STATUS=%s{status}
func Status(status string) error {
	return Send(fmt.Sprintf("STATUS=%s", status))
}

// ErrNo sends systemd notify ERRNO=%d{errno}
func ErrNo(errno int) error {
	return Send(fmt.Sprintf("ERRNO=%d", errno))
}

// BusError sends systemd notify BUSERROR=%s{buserror}
func BusError(buserror string) error {
	return Send(fmt.Sprintf("BUSERROR=%s", buserror))
}

// MainPID sends systemd notify MAINPID=%d{mainpid}
func MainPID(mainpid int) error {
	return Send(fmt.Sprintf("MAINPID=%d", mainpid))
}

// WatchDog sends systemd notify WATCHDOG=1
func WatchDog() error {
	return Send("WATCHDOG=1")
}

// WatchDogUSec sends systemd notify WATCHDOG_USEC=%d{µsec}
func WatchDogUSec(usec int64) error {
	return Send(fmt.Sprintf("WATCHDOG_USEC=%d", usec))
}

// Send state thru the notify socket if any.
// If the notify socket was not detected, it is a noop call.
// Use IsEnabled() to determine if the notify socket has been detected.
func Send(state string) error {
	if socket == nil {
		return nil
	}
	conn, err := net.DialUnix(socket.Net, nil, socket)
	if err != nil {
		return fmt.Errorf("can't open unix socket: %v", err)
	}
	defer conn.Close()
	if _, err = conn.Write([]byte(state)); err != nil {
		return fmt.Errorf("can't write into the unix socket: %v", err)
	}
	return nil
}
