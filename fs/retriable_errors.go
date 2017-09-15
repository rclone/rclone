// +build !plan9

package fs

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
		syscall.ECONNABORTED,
		syscall.EAGAIN,
		syscall.EWOULDBLOCK,
		syscall.ECONNRESET,
	)
}
