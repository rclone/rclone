// +build windows

package fserrors

import (
	"syscall"
)

const (
	WSAECONNABORTED   syscall.Errno = 10053
	WSAHOST_NOT_FOUND syscall.Errno = 11001
	WSATRY_AGAIN      syscall.Errno = 11002
	WSAENETRESET      syscall.Errno = 10052
	WSAETIMEDOUT      syscall.Errno = 10060
)

func init() {
	// append some lower level errors since the standardized ones
	// don't seem to happen
	retriableErrors = append(retriableErrors,
		syscall.WSAECONNRESET,
		WSAECONNABORTED,
		WSAHOST_NOT_FOUND,
		WSATRY_AGAIN,
		WSAENETRESET,
		WSAETIMEDOUT,
		syscall.ERROR_HANDLE_EOF,
		syscall.ERROR_NETNAME_DELETED,
		syscall.ERROR_BROKEN_PIPE,
	)
}
