// +build !plan9

package fserrors

import (
	"syscall"

	"github.com/rclone/rclone/lib/errors"
)

// IsErrNoSpace checks a possibly wrapped error to
// see if it contains a ENOSPC error
func IsErrNoSpace(cause error) (isNoSpc bool) {
	errors.Walk(cause, func(c error) bool {
		if c == syscall.ENOSPC {
			isNoSpc = true
			return true
		}
		isNoSpc = false
		return false
	})
	return
}
