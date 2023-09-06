// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package tracing

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
)

type contextKey int

var (
	excludeFromTracing contextKey = 1
)

// WithoutDistributedTracing disables distributed tracing for the current span.
func WithoutDistributedTracing(ctx context.Context) context.Context {
	return context.WithValue(ctx, excludeFromTracing, true)
}

// IsExcluded check if span shouldn't be reported to remote location.
func IsExcluded(span *monkit.Span) bool {
	val, ok := span.Value(excludeFromTracing).(bool)
	return ok && val
}
