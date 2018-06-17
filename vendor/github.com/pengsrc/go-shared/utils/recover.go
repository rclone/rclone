package utils

import (
	"context"
	"runtime/debug"

	"github.com/pengsrc/go-shared/log"
)

// Recover is a utils that recovers from panics, logs the panic (and a
// backtrace) for functions in goroutine.
func Recover(ctx context.Context) {
	if x := recover(); x != nil {
		log.Errorf(ctx, "Caught panic: %v, Trace: %s", x, debug.Stack())
	}
}
