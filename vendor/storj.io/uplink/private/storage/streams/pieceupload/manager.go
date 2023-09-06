// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package pieceupload

import (
	"context"
	"io"
	"sort"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

var (
	// ErrDone is returned from the Manager NextPiece when all of the piece
	// uploads have completed.
	ErrDone = errs.New("all pieces have been uploaded")
)

// PieceReader provides a reader for a piece with the given number.
type PieceReader interface {
	PieceReader(num int) io.Reader
}

// LimitsExchanger exchanges piece upload limits.
type LimitsExchanger interface {
	ExchangeLimits(ctx context.Context, segmentID storj.SegmentID, pieceNumbers []int) (storj.SegmentID, []*pb.AddressedOrderLimit, error)
}

// Manager tracks piece uploads for a segment. It provides callers with piece
// data and limits and tracks which uploads have been successful (or not). It
// also manages obtaining new piece upload limits for failed uploads to
// add resiliency to segment uploads. The manager also keeps track of the
// upload results, which the caller can use to commit the segment, including
// the segment ID, which is updated as limits are exchanged.
type Manager struct {
	exchanger   LimitsExchanger
	pieceReader PieceReader

	mu         sync.Mutex
	retries    int
	segmentID  storj.SegmentID
	limits     []*pb.AddressedOrderLimit
	next       chan int
	exchange   chan struct{}
	done       chan struct{}
	xchgFailed chan struct{}
	xchgError  error
	failed     []int
	results    []*pb.SegmentPieceUploadResult
}

// NewManager returns a new piece upload manager.
func NewManager(exchanger LimitsExchanger, pieceReader PieceReader, segmentID storj.SegmentID, limits []*pb.AddressedOrderLimit) *Manager {
	next := make(chan int, len(limits))
	for num := 0; num < len(limits); num++ {
		next <- num
	}
	return &Manager{
		exchanger:   exchanger,
		pieceReader: pieceReader,
		segmentID:   segmentID,
		limits:      limits,
		next:        next,
		exchange:    make(chan struct{}, 1),
		done:        make(chan struct{}),
		xchgFailed:  make(chan struct{}),
	}
}

// NextPiece returns a reader and limit for the next piece to upload. It also
// returns a callback that the caller uses to indicate success (along with the
// results) or not. NextPiece may return data with a new limit for a piece that
// was previously attempted but failed. It will return ErrDone when enough
// pieces have finished successfully to satisfy the optimal threshold. If
// NextPiece is unable to exchange limits for failed pieces, it will return
// an error.
func (mgr *Manager) NextPiece(ctx context.Context) (_ io.Reader, _ *pb.AddressedOrderLimit, _ func(hash *pb.PieceHash, uploaded bool), err error) {
	var num int
	for acquired := false; !acquired; {
		// If NextPiece is called with a cancelled context, we want to ensure
		// that we return before hitting the select and possibly picking up
		// another piece to upload.
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}

		select {
		case num = <-mgr.next:
			acquired = true
		case <-mgr.exchange:
			if err := mgr.exchangeLimits(ctx); err != nil {
				return nil, nil, nil, err
			}
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		case <-mgr.done:
			return nil, nil, nil, ErrDone
		case <-mgr.xchgFailed:
			return nil, nil, nil, mgr.xchgError
		}
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	limit := mgr.limits[num]
	piece := mgr.pieceReader.PieceReader(num)

	invoked := false
	done := func(hash *pb.PieceHash, uploaded bool) {
		mgr.mu.Lock()
		defer mgr.mu.Unlock()

		// Protect against the callback being invoked twice.
		if invoked {
			return
		}
		invoked = true

		if uploaded {
			mgr.results = append(mgr.results, &pb.SegmentPieceUploadResult{
				PieceNum: int32(num),
				NodeId:   limit.Limit.StorageNodeId,
				Hash:     hash,
			})
		} else {
			mgr.failed = append(mgr.failed, num)
		}

		if len(mgr.results)+len(mgr.failed) < len(mgr.limits) {
			return
		}

		// All of the uploads are accounted for. If there are failed pieces
		// then signal that an exchange should take place so that the
		// uploads can hopefully continue.
		if len(mgr.failed) > 0 {
			select {
			case mgr.exchange <- struct{}{}:
			default:
			}
			return
		}

		// Otherwise, all piece uploads have finished and we can signal the
		// other callers that we are done.
		close(mgr.done)
	}

	return piece, limit, done, nil
}

// Results returns the results of each piece successfully updated as well as
// the segment ID, which may differ from that passed into NewManager if piece
// limits needed to be exchanged for failed piece uploads.
func (mgr *Manager) Results() (storj.SegmentID, []*pb.SegmentPieceUploadResult) {
	mgr.mu.Lock()
	segmentID := mgr.segmentID
	results := append([]*pb.SegmentPieceUploadResult(nil), mgr.results...)
	mgr.mu.Unlock()

	// The satellite expects the results to be sorted by piece number...
	sort.Slice(results, func(i, j int) bool {
		return results[i].PieceNum < results[j].PieceNum
	})

	return segmentID, results
}

func (mgr *Manager) exchangeLimits(ctx context.Context) (err error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// any error in exchangeLimits is permanently fatal because the
	// api call should have retries in it already.
	defer func() {
		if err != nil && mgr.xchgError == nil {
			mgr.xchgError = err
			close(mgr.xchgFailed)
		}
	}()

	if len(mgr.failed) == 0 {
		// purely defensive: shouldn't happen.
		return errs.New("failed piece list is empty")
	}

	if mgr.retries > 10 {
		return errs.New("too many retries: are any nodes reachable?")
	}
	mgr.retries++

	segmentID, limits, err := mgr.exchanger.ExchangeLimits(ctx, mgr.segmentID, mgr.failed)
	if err != nil {
		return errs.New("piece limit exchange failed: %w", err)
	}
	mgr.segmentID = segmentID
	mgr.limits = limits
	for _, num := range mgr.failed {
		mgr.next <- num
	}
	mgr.failed = mgr.failed[:0]
	return nil
}
