// +build !linux !arm64
// +build !windows
// +build !go1.7

package daemon

import (
	"syscall"
)

func syscallDup(oldfd int, newfd int) (err error) {
	return syscall.Dup2(oldfd, newfd)
}
