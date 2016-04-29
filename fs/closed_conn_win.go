// +build windows

package fs

import (
	"net"
	"os"
	"syscall"
)

// isClosedConnErrorPlatform reports whether err is an error from use
// of a closed network connection using platform specific error codes.
//
// Code adapted from net/http
func isClosedConnErrorPlatform(err error) bool {
	if oe, ok := err.(*net.OpError); ok && oe.Op == "read" {
		if se, ok := oe.Err.(*os.SyscallError); ok && se.Syscall == "wsarecv" {
			if errno, ok := se.Err.(syscall.Errno); ok {
				const WSAECONNABORTED syscall.Errno = 10053
				if errno == syscall.WSAECONNRESET || errno == WSAECONNABORTED {
					return true
				}
			}
		}
	}
	return false
}
