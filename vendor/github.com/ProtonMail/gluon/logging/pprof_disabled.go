//go:build gluon_pprof_disabled

package logging

import (
	"context"
	"runtime/pprof"
)

func pprofDo(ctx context.Context, labels pprof.LabelSet, fn func(context.Context)) {
	fn(ctx)
}
