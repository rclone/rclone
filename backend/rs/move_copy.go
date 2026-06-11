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

// Copy src to this remote using shard-aligned server-side operations where possible.
//
// If it isn't possible then return fs.ErrorCantCopy.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	srcFs := srcObj.fs
	if !f.compatibleLayout(srcFs) {
		return nil, fs.ErrorCantCopy
	}
	if err := f.removeDestinationIfExists(ctx, remote); err != nil {
		return nil, err
	}
	shards := make([]int, len(f.backends))
	for i := range f.backends {
		shards[i] = i
	}
	result := f.runTwoPhaseQuorumOp(ctx, "copy", remote, shards, func(opCtx context.Context, shard int) error {
		srcShardObj, err := srcFs.backends[shard].NewObject(opCtx, srcObj.remote)
		if err != nil {
			return err
		}
		do := f.backends[shard].Features().Copy
		if do == nil {
			return fs.ErrorCantCopy
		}
		_, err = do(opCtx, srcShardObj, remote)
		return err
	})
	if result.Successes < f.writeQuorum() {
		return nil, fmt.Errorf("rs: copy quorum not met for %q: successes=%d required=%d", remote, result.Successes, f.writeQuorum())
	}
	return f.NewObject(ctx, remote)
}

// Move src to this remote using shard-aligned server-side operations where possible.
//
// If it isn't possible then return fs.ErrorCantMove.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	srcFs := srcObj.fs
	if !f.compatibleLayout(srcFs) {
		return nil, fs.ErrorCantMove
	}
	if err := f.removeDestinationIfExists(ctx, remote); err != nil {
		return nil, err
	}
	shards := make([]int, len(f.backends))
	for i := range f.backends {
		shards[i] = i
	}
	result := f.runTwoPhaseQuorumOp(ctx, "move", remote, shards, func(opCtx context.Context, shard int) error {
		srcShardObj, err := srcFs.backends[shard].NewObject(opCtx, srcObj.remote)
		if err != nil {
			return err
		}
		if do := f.backends[shard].Features().Move; do != nil {
			_, err = do(opCtx, srcShardObj, remote)
			return err
		}
		doCopy := f.backends[shard].Features().Copy
		if doCopy == nil {
			return fs.ErrorCantMove
		}
		_, err = doCopy(opCtx, srcShardObj, remote)
		if err != nil {
			return err
		}
		return srcShardObj.Remove(opCtx)
	})
	if result.Successes < f.writeQuorum() {
		return nil, fmt.Errorf("rs: move quorum not met for %q: successes=%d required=%d", remote, result.Successes, f.writeQuorum())
	}
	return f.NewObject(ctx, remote)
}

// DirMove moves srcRemote from src to dstRemote with shard-aligned server-side operations.
//
// If it isn't possible then return fs.ErrorCantDirMove.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok || !f.compatibleLayout(srcFs) {
		return fs.ErrorCantDirMove
	}
	shards := make([]int, len(f.backends))
	for i := range f.backends {
		shards[i] = i
	}
	result := f.runTwoPhaseQuorumOp(ctx, "dirmove", dstRemote, shards, func(opCtx context.Context, shard int) error {
		do := f.backends[shard].Features().DirMove
		if do == nil {
			return fs.ErrorCantDirMove
		}
		return do(opCtx, srcFs.backends[shard], srcRemote, dstRemote)
	})
	if result.Successes < f.writeQuorum() {
		if allShardFailuresMatch(result.Failures, fs.ErrorDirExists) {
			return fs.ErrorDirExists
		}
		if allShardFailuresMatch(result.Failures, fs.ErrorCantDirMove) {
			return fs.ErrorCantDirMove
		}
		return fmt.Errorf("rs: dirmove quorum not met for %q: successes=%d required=%d", dstRemote, result.Successes, f.writeQuorum())
	}
	return nil
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
