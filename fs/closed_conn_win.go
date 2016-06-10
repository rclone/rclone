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
	if oe, ok := err.(*net.OpError); ok {
		if se, ok := oe.Err.(*os.SyscallError); ok {
			if errno, ok := se.Err.(syscall.Errno); ok {
				const (
					WSAECONNABORTED   syscall.Errno = 10053
					WSAHOST_NOT_FOUND syscall.Errno = 11001
					WSATRY_AGAIN      syscall.Errno = 11002
					WSAENETRESET      syscall.Errno = 10052
					WSAETIMEDOUT      syscall.Errno = 10060
				)
				switch errno {
				case syscall.WSAECONNRESET, WSAECONNABORTED, WSAHOST_NOT_FOUND, WSATRY_AGAIN, WSAENETRESET, WSAETIMEDOUT:
					return true
				}
			}
		}
	}
	return false
}
