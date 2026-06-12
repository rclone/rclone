package rs

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rclone/rclone/fs"
)

func (f *Fs) compatibleLayout(other *Fs) bool {
	if other == nil {
		return false
	}
	if f.opt.DataShards != other.opt.DataShards || f.opt.ParityShards != other.opt.ParityShards {
		return false
	}
	return len(f.backends) == len(other.backends)
}

// removeDestinationIfExists clears destination particles on every shard before
// copy/move. Shard backends may still hold a file when rs NewObject cannot
// assemble a logical object (orphan or corrupt particles); those must be
// removed so server-side copy (e.g. local COPYFILE_EXCL) can succeed.
func (f *Fs) removeDestinationIfExists(ctx context.Context, remote string) error {
	var wg sync.WaitGroup
	for i := range f.backends {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			obj, err := f.backends[i].NewObject(ctx, remote)
			if err != nil {
				return
			}
			_ = obj.Remove(ctx)
		}()
	}
	wg.Wait()
	return nil
}

func copyShard(ctx context.Context, dstFs, srcFs *Fs, srcRemote, dstRemote string, shard int) error {
	srcShardObj, err := srcFs.backends[shard].NewObject(ctx, srcRemote)
	if err != nil {
		return err
	}
	do := dstFs.backends[shard].Features().Copy
	if do == nil {
		return fs.ErrorCantCopy
	}
	_, err = do(ctx, srcShardObj, dstRemote)
	return err
}

func moveShard(ctx context.Context, dstFs, srcFs *Fs, srcRemote, dstRemote string, shard int) error {
	srcShardObj, err := srcFs.backends[shard].NewObject(ctx, srcRemote)
	if err != nil {
		return err
	}
	if do := dstFs.backends[shard].Features().Move; do != nil {
		_, err = do(ctx, srcShardObj, dstRemote)
		return err
	}
	doCopy := dstFs.backends[shard].Features().Copy
	if doCopy == nil {
		return fs.ErrorCantMove
	}
	_, err = doCopy(ctx, srcShardObj, dstRemote)
	if err != nil {
		return err
	}
	return srcShardObj.Remove(ctx)
}

func removeShardObject(ctx context.Context, b fs.Fs, remote string) error {
	obj, err := b.NewObject(ctx, remote)
	if err != nil {
		return nil
	}
	return obj.Remove(ctx)
}

// rollbackMoveShard restores srcRemote on srcFs after a successful shard move to dstRemote.
func rollbackMoveShard(ctx context.Context, dstFs, srcFs *Fs, srcRemote, dstRemote string, shard int) error {
	dstShardObj, err := dstFs.backends[shard].NewObject(ctx, dstRemote)
	if err != nil {
		return nil
	}
	if do := dstFs.backends[shard].Features().Move; do != nil {
		_, err = do(ctx, dstShardObj, srcRemote)
		if err == nil {
			return nil
		}
	}
	doCopy := srcFs.backends[shard].Features().Copy
	if doCopy == nil {
		return fs.ErrorCantMove
	}
	_, err = doCopy(ctx, dstShardObj, srcRemote)
	if err != nil {
		return err
	}
	return dstShardObj.Remove(ctx)
}

// Copy src to this remote using shard-aligned server-side operations where possible.
//
// If it isn't possible then return fs.ErrorCantCopy.
// See backend/rs/docs/QUORUM_TRANSACTIONS.md (Copy).
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	srcFs := srcObj.fs
	if !f.compatibleLayout(srcFs) {
		return nil, fs.ErrorCantCopy
	}
	if _, err := f.preflightReachableShards(ctx); err != nil {
		return nil, err
	}
	if err := f.removeDestinationIfExists(ctx, remote); err != nil {
		return nil, err
	}
	err := f.runQuorumTransaction(ctx, quorumTransaction{
		OpName:        "copy",
		Remote:        remote,
		SkipPreflight: true,
		Forward: func(opCtx context.Context, shard int) error {
			return copyShard(opCtx, f, srcFs, srcObj.remote, remote, shard)
		},
		Rollback: func(opCtx context.Context, shard int) error {
			return removeShardObject(opCtx, f.backends[shard], remote)
		},
	})
	if err != nil {
		return nil, err
	}
	return f.newObjectAfterCopyMove(ctx, remote, srcObj), nil
}

// Move src to this remote using shard-aligned server-side operations where possible.
//
// If it isn't possible then return fs.ErrorCantMove.
// See backend/rs/docs/QUORUM_TRANSACTIONS.md (Move).
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	srcFs := srcObj.fs
	if !f.compatibleLayout(srcFs) {
		return nil, fs.ErrorCantMove
	}
	if _, err := f.preflightReachableShards(ctx); err != nil {
		return nil, err
	}
	if err := f.removeDestinationIfExists(ctx, remote); err != nil {
		return nil, err
	}
	err := f.runQuorumTransaction(ctx, quorumTransaction{
		OpName:        "move",
		Remote:        remote,
		SkipPreflight: true,
		Forward: func(opCtx context.Context, shard int) error {
			return moveShard(opCtx, f, srcFs, srcObj.remote, remote, shard)
		},
		Rollback: func(opCtx context.Context, shard int) error {
			return rollbackMoveShard(opCtx, f, srcFs, srcObj.remote, remote, shard)
		},
	})
	if err != nil {
		return nil, err
	}
	return f.newObjectAfterCopyMove(ctx, remote, srcObj), nil
}

// DirMove moves srcRemote from src to dstRemote with shard-aligned server-side operations.
//
// If it isn't possible then return fs.ErrorCantDirMove.
// See backend/rs/docs/QUORUM_TRANSACTIONS.md (DirMove).
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok || !f.compatibleLayout(srcFs) {
		return fs.ErrorCantDirMove
	}
	reachable, err := f.preflightReachableShards(ctx)
	if err != nil {
		return err
	}
	if err := f.dirmovePreflight(ctx, srcFs, srcRemote, dstRemote, reachable); err != nil {
		return err
	}
	return f.runQuorumTransaction(ctx, quorumTransaction{
		OpName:        "dirmove",
		Remote:        dstRemote,
		SkipPreflight: true,
		Forward: func(opCtx context.Context, shard int) error {
			do := f.backends[shard].Features().DirMove
			if do == nil {
				return fs.ErrorCantDirMove
			}
			return do(opCtx, srcFs.backends[shard], srcRemote, dstRemote)
		},
		Rollback: func(opCtx context.Context, shard int) error {
			do := srcFs.backends[shard].Features().DirMove
			if do == nil {
				fs.Logf(f, "rs: dirmove %q rollback shard=%d: DirMove not supported on source", dstRemote, shard)
				return nil
			}
			return do(opCtx, f.backends[shard], dstRemote, srcRemote)
		},
		CommitError: func(result quorumOpResult, required int) error {
			if allShardFailuresMatch(result.Failures, fs.ErrorDirExists) {
				return fs.ErrorDirExists
			}
			if allShardFailuresMatch(result.Failures, fs.ErrorCantDirMove) {
				return fs.ErrorCantDirMove
			}
			return fmt.Errorf("rs: dirmove quorum not met for %q: successes=%d required=%d", dstRemote, result.Successes, required)
		},
	})
}

func allShardFailuresMatch(failures map[int]shardFailure, target error) bool {
	if len(failures) == 0 {
		return false
	}
	for _, failure := range failures {
		if !errors.Is(failure.err, target) {
			return false
		}
	}
	return true
}
