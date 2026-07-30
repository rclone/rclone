//go:build !plan9 && !solaris && !js

// Parallel listing support for azureblob, built on the Apache Arrow
// listing format.
//
// The Arrow listing exposes the server-side startFrom (inclusive) and endBefore
// (exclusive) name-range parameters, which let a listing be sharded by
// blob-name range and listed concurrently.

package azureblob

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/rclone/rclone/backend/azureblob/arrowlist"
	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

// listArrowParallel lists (containerName, directory) by splitting the blob name
// keyspace into shards and listing them concurrently. opts must already request
// the Arrow format; it is cloned per shard with StartFrom/EndBefore set. It
// returns the number of raw items seen. directory/prefix are already normalised
// by the caller.
func (f *Fs) listArrowParallel(ctx context.Context, containerName, directory, prefix string, addContainer bool, opts *arrowlist.ListBlobsHierarchyOptions, delimiter string, fn listFn) (int, error) {
	// Parallel listing needs the arrowlist client (server-side endBefore is
	// only honoured on the Arrow path) - without it fall back to sequential.
	if _, err := f.arrowCntSVC(containerName); err != nil {
		fs.Debugf(f, "Not using parallel Arrow listing: %v", err)
		return f.listBlobsPager(ctx, containerName, directory, prefix, addContainer, opts, delimiter, fn)
	}

	// Split the keyspace by a single-case character ladder (see
	// arrowLadderBoundaries for why single-case matters).
	points := arrowLadderBoundaries(directory, arrowShardTarget(f.opt.ListParallelism))
	if len(points) < 1 {
		// Couldn't usefully split - list sequentially.
		return f.listBlobsPager(ctx, containerName, directory, prefix, addContainer, opts, delimiter, fn)
	}

	// Build half-open [startFrom, endBefore) shards from the sorted boundary
	// points p0..pk: ["", p0), [p0, p1), ..., [pk, "").
	type shard struct{ startFrom, endBefore string }
	shards := make([]shard, 0, len(points)+1)
	prev := ""
	for _, p := range points {
		shards = append(shards, shard{prev, p})
		prev = p
	}
	shards = append(shards, shard{prev, ""})

	// fn (and the list.Helper behind it) is not concurrency-safe, so serialise
	// delivery. Only the cheap callback is locked, not the HTTP/decode work.
	var fnMu sync.Mutex
	safeFn := func(remote string, item *container.BlobItem, isDir bool) error {
		fnMu.Lock()
		defer fnMu.Unlock()
		return fn(remote, item, isDir)
	}

	// Schedule every shard; SetLimit bounds concurrency, so more shards than
	// workers gives free load balancing as workers pick up the next shard.
	var total int64
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(f.opt.ListParallelism)
	for _, sh := range shards {
		if gCtx.Err() != nil {
			break
		}
		g.Go(func() error {
			// Clone opts per shard so concurrent shards don't share StartFrom/EndBefore.
			shardOpts := *opts
			if sh.startFrom != "" {
				startFrom := sh.startFrom
				shardOpts.StartFrom = &startFrom
			} else {
				shardOpts.StartFrom = nil
			}
			if sh.endBefore != "" {
				endBefore := sh.endBefore
				shardOpts.EndBefore = &endBefore
			} else {
				shardOpts.EndBefore = nil
			}
			n, err := f.listBlobsPager(gCtx, containerName, directory, prefix, addContainer, &shardOpts, delimiter, safeFn)
			atomic.AddInt64(&total, int64(n))
			return err
		})
	}
	err := g.Wait()
	found := int(atomic.LoadInt64(&total))
	if isEndBeforeUnsupported(err) {
		// The account can't honour the shards' endBefore bounds, so parallel
		// listing can't be used on it.
		f.arrowXMLFallback.Store(true)
		if found == 0 {
			// No shard delivered anything yet (the normal case - Arrow listing
			// enablement is per account so every shard fails on its first
			// page) so it is safe to relist sequentially.
			fs.Debugf(f, "Parallel Arrow listing not supported by this account (%v) - listing sequentially", err)
			return f.listBlobsPager(ctx, containerName, directory, prefix, addContainer, opts, delimiter, fn)
		}
		// Some entries were already delivered so a rerun would duplicate them.
		return found, fmt.Errorf("parallel listing aborted after partial results (retry, or unset list_parallelism): %w", err)
	}
	return found, err
}

// isEndBeforeUnsupported reports whether err means the service can't honour
// the endBefore listing bound, so sharded parallel listing must not be used.
//
// This happens two ways: the arrowlist pager's ErrEndBeforeXMLFallback (the
// server answered XML with endBefore set), or the service rejecting the
// endBefore parameter with 400 OperationNotSupportedWithFeatureMissing "The
// requested operation is not allowed as EndBefore parameter support is
// missing on this account" - endBefore is only supported on the Arrow
// listing path of accounts with Blob Listing with Apache Arrow enabled.
func isEndBeforeUnsupported(err error) bool {
	if errors.Is(err, arrowlist.ErrEndBeforeXMLFallback) {
		return true
	}
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.ErrorCode == "OperationNotSupportedWithFeatureMissing"
}

// arrowShardTarget is the number of ladder boundaries to use for the given
// parallelism (a few per worker for load balancing).
func arrowShardTarget(parallelism int) int {
	return parallelism * 4
}

// arrowLadderBoundaries returns up to target sorted ENCODED-space boundaries of
// the form directory+c, used to split the keyspace into shards.
//
// The partition is always correct (no dup/gap) regardless of the alphabet: keys
// whose next byte is outside the alphabet simply fall into an adjacent shard;
// the alphabet only affects balance. A single-case (digit+lowercase) alphabet
// is deliberate: the boundaries are then monotonic under BOTH byte order (the
// listing order) AND the service's case-insensitive startFrom/endBefore
// validation, so server-side endBefore is accepted. (A mixed-case ladder gives
// pairs like [".../Z", ".../a") which are byte-ascending but rejected 400 as
// "endbefore precedes startfrom".) directory is already encoded; the alphabet
// is ASCII (encoder-invariant) and ascending, so the result is sorted.
func arrowLadderBoundaries(directory string, target int) []string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	if target < 1 {
		return nil
	}
	if target > len(alphabet) {
		target = len(alphabet)
	}
	points := make([]string, 0, target)
	last := -1
	for i := 0; i < target; i++ {
		idx := i * len(alphabet) / target
		if idx == last {
			continue
		}
		last = idx
		points = append(points, directory+string(alphabet[idx]))
	}
	return points
}
