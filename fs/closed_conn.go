// +build !plan9

package fs

import (
	"net"
	"os"
	"syscall"
)

// closedConnErrors indicate a connection is closed or broken and
// should be retried
//
// These are added to in closed_conn_win.go
var closedConnErrors = []syscall.Errno{
	syscall.EPIPE,
	syscall.ETIMEDOUT,
	syscall.ECONNREFUSED,
	syscall.EHOSTDOWN,
	syscall.EHOSTUNREACH,
	syscall.ECONNABORTED,
	syscall.EAGAIN,
	syscall.EWOULDBLOCK,
	syscall.ECONNRESET,
}

// isClosedConnErrorPlatform reports whether err is an error from use
// of a closed network connection using platform specific error codes.
func isClosedConnErrorPlatform(err error) bool {
	// now check whether err is an error from use of a closed
	// network connection using platform specific error codes.
	//
	// Code adapted from net/http
	if oe, ok := err.(*net.OpError); ok {
		if se, ok := oe.Err.(*os.SyscallError); ok {
			if errno, ok := se.Err.(syscall.Errno); ok {
				for _, retriableErrno := range closedConnErrors {
					if errno == retriableErrno {
						return true
					}
				}
			}
		}
	}
	return false
}
