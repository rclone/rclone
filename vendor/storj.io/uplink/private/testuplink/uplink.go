// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testuplink

import (
	"context"

	"storj.io/common/memory"
)

type segmentSizeKey struct{}

// WithMaxSegmentSize creates context with max segment size for testing purposes.
func WithMaxSegmentSize(ctx context.Context, segmentSize memory.Size) context.Context {
	return context.WithValue(ctx, segmentSizeKey{}, segmentSize)
}

// GetMaxSegmentSize returns max segment size from context if exists.
func GetMaxSegmentSize(ctx context.Context) (memory.Size, bool) {
	segmentSize, ok := ctx.Value(segmentSizeKey{}).(memory.Size)
	return segmentSize, ok
}
