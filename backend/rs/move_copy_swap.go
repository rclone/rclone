package rs

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/rclone/rclone/fs"
)

const (
	copyMoveTmpSuffix = ".rs-tmp-"
	copyMoveBakSuffix = ".rs-bak-"
)

// copyMovePaths holds per-op staging paths for atomic copy/move overwrite.
type copyMovePaths struct {
	remote string
	tmp    string
	bak    string
	nonce  string
}

func newCopyMoveNonce() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("rs: generate copy/move nonce: %w", err)
	}
	return fmt.Sprintf("%016x", binary.LittleEndian.Uint64(b[:])), nil
}

func copyMoveArtifactPaths(remote, nonce string) copyMovePaths {
	return copyMovePaths{
		remote: remote,
		tmp:    remote + copyMoveTmpSuffix + nonce,
		bak:    remote + copyMoveBakSuffix + nonce,
		nonce:  nonce,
	}
}

// copyMoveArtifactKind classifies rs copy/move staging suffix paths.
type copyMoveArtifactKind int

const (
	copyMoveArtifactNone copyMoveArtifactKind = iota
	copyMoveArtifactTmp
	copyMoveArtifactBak
)

// parseCopyMoveArtifact returns base logical path, artifact kind, and nonce for heal cleanup.
func parseCopyMoveArtifact(remote string) (base string, kind copyMoveArtifactKind, nonce string, ok bool) {
	if i := strings.LastIndex(remote, copyMoveTmpSuffix); i >= 0 {
		return remote[:i], copyMoveArtifactTmp, remote[i+len(copyMoveTmpSuffix):], true
	}
	if i := strings.LastIndex(remote, copyMoveBakSuffix); i >= 0 {
		return remote[:i], copyMoveArtifactBak, remote[i+len(copyMoveBakSuffix):], true
	}
	return "", copyMoveArtifactNone, "", false
}

func shardObjectExists(ctx context.Context, b fs.Fs, remote string) bool {
	_, err := b.NewObject(ctx, remote)
	return err == nil
}

func shardCopyObject(ctx context.Context, b fs.Fs, srcRemote, dstRemote string) error {
	srcObj, err := b.NewObject(ctx, srcRemote)
	if err != nil {
		return err
	}
	do := b.Features().Copy
	if do == nil {
		return fs.ErrorCantCopy
	}
	_, err = do(ctx, srcObj, dstRemote)
	return err
}

func shardMoveObject(ctx context.Context, b fs.Fs, srcRemote, dstRemote string) error {
	srcObj, err := b.NewObject(ctx, srcRemote)
	if err != nil {
		return err
	}
	if do := b.Features().Move; do != nil {
		_, err = do(ctx, srcObj, dstRemote)
		return err
	}
	doCopy := b.Features().Copy
	if doCopy == nil {
		return fs.ErrorCantMove
	}
	if _, err = doCopy(ctx, srcObj, dstRemote); err != nil {
		return err
	}
	return srcObj.Remove(ctx)
}

// shardBackupDst moves or copies dst aside to bak when dst exists. dst is kept when Copy is used.
func shardBackupDst(ctx context.Context, b fs.Fs, dst, bak string) error {
	if !shardObjectExists(ctx, b, dst) {
		return nil
	}
	if shardObjectExists(ctx, b, bak) {
		if err := removeShardObject(ctx, b, bak); err != nil {
			return err
		}
	}
	if b.Features().Move != nil {
		return shardMoveObject(ctx, b, dst, bak)
	}
	if b.Features().Copy != nil {
		return shardCopyObject(ctx, b, dst, bak)
	}
	return fs.ErrorCantCopy
}

// shardInstallTemp atomically replaces dst with temp content (bak must exist when dst had content).
func shardInstallTemp(ctx context.Context, b fs.Fs, tmp, dst string, hadDst bool) error {
	if !shardObjectExists(ctx, b, tmp) {
		return fs.ErrorObjectNotFound
	}
	if b.Features().Move != nil && !shardObjectExists(ctx, b, dst) {
		return shardMoveObject(ctx, b, tmp, dst)
	}
	if b.Features().Copy != nil {
		if err := shardCopyObject(ctx, b, tmp, dst); err != nil {
			return err
		}
		return removeShardObject(ctx, b, tmp)
	}
	if hadDst {
		if err := removeShardObject(ctx, b, dst); err != nil {
			return err
		}
	}
	if err := shardCopyObject(ctx, b, tmp, dst); err != nil {
		return err
	}
	return removeShardObject(ctx, b, tmp)
}

// shardRestoreDst rolls back swap: remove new dst and restore from bak when present.
func shardRestoreDst(ctx context.Context, b fs.Fs, dst, bak, tmp string) error {
	_ = removeShardObject(ctx, b, tmp)
	if shardObjectExists(ctx, b, bak) {
		if shardObjectExists(ctx, b, dst) {
			if err := removeShardObject(ctx, b, dst); err != nil {
				return err
			}
		}
		if b.Features().Move != nil {
			return shardMoveObject(ctx, b, bak, dst)
		}
		if b.Features().Copy != nil {
			if err := shardCopyObject(ctx, b, bak, dst); err != nil {
				return err
			}
			return removeShardObject(ctx, b, bak)
		}
		return fs.ErrorCantCopy
	}
	// Greenfield dst (no backup): remove partially installed destination particle.
	return removeShardObject(ctx, b, dst)
}

func swapShardForward(ctx context.Context, f *Fs, paths copyMovePaths, shard int) error {
	b := f.backends[shard]
	hadDst := shardObjectExists(ctx, b, paths.remote)
	if err := shardBackupDst(ctx, b, paths.remote, paths.bak); err != nil {
		return err
	}
	if err := shardInstallTemp(ctx, b, paths.tmp, paths.remote, hadDst); err != nil {
		_ = shardRestoreDst(ctx, b, paths.remote, paths.bak, paths.tmp)
		return err
	}
	return nil
}

func swapShardRollback(ctx context.Context, f *Fs, paths copyMovePaths, shard int) error {
	return shardRestoreDst(ctx, f.backends[shard], paths.remote, paths.bak, paths.tmp)
}

func copyMoveQuorumNotMetErr(op, remote string, result quorumOpResult, required int) error {
	return fmt.Errorf("rs: %s quorum not met for %q: successes=%d required=%d", op, remote, result.Successes, required)
}
