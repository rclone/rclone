// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rpctracing

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/tracing"
)

// WithoutDistributedTracing disables distributed tracing for the current span.
// Deprecated: use tracing.WithoutDistributedTracing.
func WithoutDistributedTracing(ctx context.Context) context.Context {
	return tracing.WithoutDistributedTracing(ctx)
}

// IsExcluded check if span shouldn't be reported to remote location.
// Deprecated: use tracing.IsExcluded.
func IsExcluded(span *monkit.Span) bool {
	return tracing.IsExcluded(span)
}
