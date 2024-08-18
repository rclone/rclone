//go:build windows

package fserrors

import (
	"syscall"
)

// Windows error code list
// https://docs.microsoft.com/en-us/windows/win32/winsock/windows-sockets-error-codes-2
const (
	WSAENETDOWN       syscall.Errno = 10050
	WSAENETUNREACH    syscall.Errno = 10051
	WSAENETRESET      syscall.Errno = 10052
	WSAECONNABORTED   syscall.Errno = 10053
	WSAECONNRESET     syscall.Errno = 10054
	WSAENOBUFS        syscall.Errno = 10055
	WSAENOTCONN       syscall.Errno = 10057
	WSAESHUTDOWN      syscall.Errno = 10058
	WSAETIMEDOUT      syscall.Errno = 10060
	WSAECONNREFUSED   syscall.Errno = 10061
	WSAEHOSTDOWN      syscall.Errno = 10064
	WSAEHOSTUNREACH   syscall.Errno = 10065
	WSAEDISCON        syscall.Errno = 10101
	WSAEREFUSED       syscall.Errno = 10112
	WSAHOST_NOT_FOUND syscall.Errno = 11001 //nolint:revive // Don't include revive when running golangci-lint to avoid var-naming: don't use ALL_CAPS in Go names; use CamelCase (revive)
	WSATRY_AGAIN      syscall.Errno = 11002 //nolint:revive // Don't include revive when running golangci-lint to avoid var-naming: don't use ALL_CAPS in Go names; use CamelCase (revive)
)

func init() {
	// append some lower level errors since the standardized ones
	// don't seem to happen
	retriableErrors = append(retriableErrors,
		syscall.WSAECONNRESET,
		WSAENETDOWN,
		WSAENETUNREACH,
		WSAENETRESET,
		WSAECONNABORTED,
		WSAECONNRESET,
		WSAENOBUFS,
		WSAENOTCONN,
		WSAESHUTDOWN,
		WSAETIMEDOUT,
		WSAECONNREFUSED,
		WSAEHOSTDOWN,
		WSAEHOSTUNREACH,
		WSAEDISCON,
		WSAEREFUSED,
		WSAHOST_NOT_FOUND,
		WSATRY_AGAIN,
		syscall.ERROR_HANDLE_EOF,
		syscall.ERROR_NETNAME_DELETED,
		syscall.ERROR_BROKEN_PIPE,
	)
}
