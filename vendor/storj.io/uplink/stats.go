// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/paths"
	"storj.io/common/rpc"
	"storj.io/common/storj"
)

const eventErrorMessageLimit = 64

type operationStats struct {
	start       time.Time
	quicRollout int
	bytes       int64
	working     time.Duration
	failure     []error
	satellite   string
	encPath     paths.Encrypted
}

func newOperationStats(ctx context.Context, satellite storj.NodeURL) (os operationStats) {
	os.start = time.Now()
	os.quicRollout = rpc.QUICRolloutPercent(ctx)
	os.satellite = satellite.String()
	return os
}

func (os *operationStats) trackWorking() func() {
	start := time.Now()
	return func() { os.working += time.Since(start) }
}

func (os *operationStats) flagFailure(err error) {
	if err != nil {
		os.failure = append(os.failure, err)
	}
}

func (os *operationStats) err() (message string, err error) {
	err = errs.Combine(os.failure...)
	if err != nil {
		message = err.Error()
		if len(message) > eventErrorMessageLimit {
			message = message[:eventErrorMessageLimit]
		}
	}

	return message, err
}
