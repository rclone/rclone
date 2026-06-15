package rs

import (
	"context"
	"errors"
	"fmt"

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

// copyMoveAtomic runs two-phase copy-to-temp + backup + swap for destination overwrite safety.
func (f *Fs) copyMoveAtomic(ctx context.Context, op string, srcObj *Object, remote string, isMove bool) (fs.Object, error) {
	srcFs := srcObj.fs
	nonce, err := newCopyMoveNonce()
	if err != nil {
		if isMove {
			return nil, fs.ErrorCantMove
		}
		return nil, fs.ErrorCantCopy
	}
	paths := copyMoveArtifactPaths(remote, nonce)

	// Phase 1: write temps (never touch dst). Move uses copy-to-temp so src survives until commit.
	err = f.runQuorumTransaction(ctx, quorumTransaction{
		OpName:        op + "-temp",
		Remote:        remote,
		SkipPreflight: true,
		Forward: func(opCtx context.Context, shard int) error {
			return copyShard(opCtx, f, srcFs, srcObj.remote, paths.tmp, shard)
		},
		Rollback: func(opCtx context.Context, shard int) error {
			return removeShardObject(opCtx, f.backends[shard], paths.tmp)
		},
		CommitError: func(result quorumOpResult, required int) error {
			return copyMoveQuorumNotMetErr(op, remote, result, required)
		},
	})
	if err != nil {
		return nil, err
	}

	// Phase 2: backup old dst, install temp -> dst.
	err = f.runQuorumTransaction(ctx, quorumTransaction{
		OpName:        op + "-swap",
		Remote:        remote,
		SkipPreflight: true,
		Forward: func(opCtx context.Context, shard int) error {
			return swapShardForward(opCtx, f, paths, shard)
		},
		Rollback: func(opCtx context.Context, shard int) error {
			return swapShardRollback(opCtx, f, paths, shard)
		},
		CommitError: func(result quorumOpResult, required int) error {
			return copyMoveQuorumNotMetErr(op, remote, result, required)
		},
	})
	if err != nil {
		return nil, err
	}

	// Commit: remove staging artifacts; for Move, quorum-remove src.
	if err := f.copyMoveCommitCleanup(ctx, paths); err != nil {
		fs.Logf(f, "rs: %s %q commit cleanup incomplete: %v", op, remote, err)
	}
	if isMove {
		err = f.runQuorumTransaction(ctx, quorumTransaction{
			OpName:        op + "-src-remove",
			Remote:        srcObj.remote,
			SkipPreflight: true,
			Forward: func(opCtx context.Context, shard int) error {
				return removeShardObject(opCtx, srcFs.backends[shard], srcObj.remote)
			},
		})
		if err != nil {
			return nil, err
		}
	}
	return f.newObjectAfterCopyMove(ctx, remote, srcObj), nil
}

func (f *Fs) copyMoveCommitCleanup(ctx context.Context, paths copyMovePaths) error {
	return f.runQuorumTransaction(ctx, quorumTransaction{
		OpName:        "copymove-cleanup",
		Remote:        paths.remote,
		SkipPreflight: true,
		Forward: func(opCtx context.Context, shard int) error {
			b := f.backends[shard]
			if err := removeShardObject(opCtx, b, paths.bak); err != nil {
				return err
			}
			return removeShardObject(opCtx, b, paths.tmp)
		},
	})
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
	return f.copyMoveAtomic(ctx, "copy", srcObj, remote, false)
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
	return f.copyMoveAtomic(ctx, "move", srcObj, remote, true)
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
