// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"
	"time"

	"storj.io/common/time2"
)

// Sleep implements sleeping with cancellation.
func Sleep(ctx context.Context, duration time.Duration) bool {
	return time2.Sleep(ctx, duration)
}
