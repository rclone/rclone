package vfscommon

import (
	"fmt"
	"runtime/debug"

	"github.com/rclone/rclone/fs"
)

// RecoverPanic recovers a panic, logs it against o with a stack trace and,
// if err is not nil, stores it there. Use it as `defer RecoverPanic(o, &err)`.
func RecoverPanic(o any, err *error) {
	if r := recover(); r != nil {
		fs.Errorf(o, "panic: %v\n%s", r, debug.Stack())
		if err != nil {
			*err = fmt.Errorf("panic: %v", r)
		}
	}
}

// RecoverCall calls fn, converting a panic into an error.
//
// Prefer this to recovering a whole goroutine where the caller already
// handles an error, so a panic is retried or reported the same way a failure
// is, rather than abandoning the surrounding work part way through.
func RecoverCall(o any, fn func() error) (err error) {
	defer RecoverPanic(o, &err)
	return fn()
}
