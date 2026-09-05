//go:build !plan9

package fserrors

import (
	"slices"
	"syscall"

	liberrors "github.com/rclone/rclone/lib/errors"
)

// noSpaceErrors are the errors which mean the disk is full.
//
// Platform specific files add to this list in their init functions.
var noSpaceErrors = []error{
	syscall.ENOSPC,
}

// IsErrNoSpace checks a possibly wrapped error to
// see if it contains an out of space error.
func IsErrNoSpace(cause error) (isNoSpc bool) {
	liberrors.Walk(cause, func(c error) bool {
		if slices.Contains(noSpaceErrors, c) {
			isNoSpc = true
			return true
		}
		isNoSpc = false
		return false
	})
	return
}
