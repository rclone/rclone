//go:build !plan9

package fserrors

import (
	"syscall"

	liberrors "github.com/rclone/rclone/lib/errors"
)

// IsErrNoSpace checks a possibly wrapped error to
// see if it contains a ENOSPC or EDQUOT error
func IsErrNoSpace(cause error) (isNoSpc bool) {
	liberrors.Walk(cause, func(c error) bool {
		switch c {
		case syscall.ENOSPC:
			isNoSpc = true
			return true
		case syscall.EDQUOT:
			isNoSpc = true
			return true
		default:
			isNoSpc = false
			return false
		}
	})
	return
}
