//go:build !plan9

package fserrors

import (
	"syscall"
)

func init() {
	retriableErrors = append(retriableErrors,
		syscall.EPIPE,
		syscall.ETIMEDOUT,
		syscall.ECONNREFUSED,
		syscall.EHOSTDOWN,
		syscall.EHOSTUNREACH,
		syscall.ENETDOWN,
		syscall.ENETUNREACH,
		syscall.ECONNABORTED,
		syscall.EAGAIN,
		syscall.EWOULDBLOCK,
		syscall.ECONNRESET,
	)
}
