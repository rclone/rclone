// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package batchaggregator

import (
	"context"
	"fmt"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/testuplink"
)

var mon = monkit.Package()

// Aggregator aggregates batch items to reduce round trips.
type Aggregator struct {
	batcher metaclient.Batcher

	mu        sync.Mutex
	scheduled []metaclient.BatchItem
}

// New returns a new aggregator that will aggregate batch items to be issued
// by the batcher.
func New(batcher metaclient.Batcher) *Aggregator {
	return &Aggregator{
		batcher: batcher,
	}
}

// Schedule schedules a batch item to be issued at the next flush.
func (a *Aggregator) Schedule(batchItem metaclient.BatchItem) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.scheduled = append(a.scheduled, batchItem)
}

// ScheduleAndFlush schedules a batch item and immediately issues all
// scheduled batch items. It returns the response to the batch item scheduled
// with the call.
func (a *Aggregator) ScheduleAndFlush(ctx context.Context, batchItem metaclient.BatchItem) (_ *metaclient.BatchResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	a.mu.Lock()
	defer a.mu.Unlock()

	a.scheduled = append(a.scheduled, batchItem)

	resp, err := a.issueBatchLocked(ctx)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, errs.New("missing batch responses")
	}
	return &resp[len(resp)-1], nil
}

// Flush issues all scheduled batch items.
func (a *Aggregator) Flush(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	a.mu.Lock()
	defer a.mu.Unlock()

	_, err = a.issueBatchLocked(ctx)
	return err
}

func (a *Aggregator) issueBatchLocked(ctx context.Context) (_ []metaclient.BatchResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	batchItems := a.scheduled
	a.scheduled = a.scheduled[:0]

	if len(batchItems) == 0 {
		return nil, nil
	}

	for _, batchItem := range batchItems {
		testuplink.Log(ctx, "Flush batch item:", batchItemTypeName(batchItem))
	}

	return a.batcher.Batch(ctx, batchItems...)
}

func batchItemTypeName(batchItem metaclient.BatchItem) string {
	return fmt.Sprintf("%T", batchItem.BatchItem().Request)
}
