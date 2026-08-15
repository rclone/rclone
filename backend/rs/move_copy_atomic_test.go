package rs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/require"
)

type phaseFailCopyToTempFs struct {
	fs.Fs
	fail bool
}

func (f phaseFailCopyToTempFs) Features() *fs.Features {
	features := f.Fs.Features()
	base := *features
	copyFn := features.Copy
	base.Copy = func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		if f.fail && strings.Contains(remote, copyMoveTmpSuffix) {
			return nil, errors.New("phaseFailCopyToTempFs: injected Copy failure")
		}
		if copyFn != nil {
			return copyFn(ctx, src, remote)
		}
		return nil, fs.ErrorCantCopy
	}
	return &base
}

type phaseFailSwapFs struct {
	fs.Fs
	fail bool
}

// noCopyFs hides server-side Copy to simulate Linux local (Move only).
type noCopyFs struct {
	fs.Fs
}

func (f noCopyFs) Features() *fs.Features {
	features := f.Fs.Features()
	base := *features
	base.Copy = nil
	return &base
}

// noMoveFs hides server-side Move.
type noMoveFs struct {
	fs.Fs
}

func (f noMoveFs) Features() *fs.Features {
	features := f.Fs.Features()
	base := *features
	base.Move = nil
	return &base
}

type phaseFailMoveBackupFs struct {
	fs.Fs
	fail bool
}

func (f phaseFailMoveBackupFs) Features() *fs.Features {
	features := f.Fs.Features()
	base := *features
	moveFn := features.Move
	base.Move = func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		if f.fail && strings.Contains(remote, copyMoveBakSuffix) {
			return nil, errors.New("phaseFailMoveBackupFs: injected Move failure")
		}
		if moveFn != nil {
			return moveFn(ctx, src, remote)
		}
		return nil, fs.ErrorCantMove
	}
	return &base
}

type phaseFailMoveSwapFs struct {
	fs.Fs
	fail bool
}

func (f phaseFailMoveSwapFs) Features() *fs.Features {
	features := f.Fs.Features()
	base := *features
	moveFn := features.Move
	base.Move = func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		if f.fail && isMoveOnlySwapInjectFailure(src.Remote(), remote) {
			return nil, errors.New("phaseFailMoveSwapFs: injected Move failure")
		}
		if moveFn != nil {
			return moveFn(ctx, src, remote)
		}
		return nil, fs.ErrorCantMove
	}
	return &base
}

func isMoveOnlySwapInjectFailure(srcRemote, dstRemote string) bool {
	if strings.Contains(srcRemote, copyMoveBakSuffix) || strings.Contains(dstRemote, copyMoveBakSuffix) {
		return false
	}
	return !strings.Contains(srcRemote, copyMoveTmpSuffix) && !strings.Contains(dstRemote, copyMoveTmpSuffix)
}

func (f phaseFailSwapFs) Features() *fs.Features {
	features := f.Fs.Features()
	base := *features
	copyFn := features.Copy
	moveFn := features.Move
	base.Copy = func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		if f.fail && isSwapInjectFailure(src.Remote(), remote) {
			return nil, errors.New("phaseFailSwapFs: injected Copy failure")
		}
		if copyFn != nil {
			return copyFn(ctx, src, remote)
		}
		return nil, fs.ErrorCantCopy
	}
	base.Move = func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		if f.fail && isSwapInjectFailure(src.Remote(), remote) {
			return nil, errors.New("phaseFailSwapFs: injected Move failure")
		}
		if moveFn != nil {
			return moveFn(ctx, src, remote)
		}
		return nil, fs.ErrorCantMove
	}
	return &base
}

func isSwapInjectFailure(srcRemote, dstRemote string) bool {
	if strings.Contains(dstRemote, copyMoveTmpSuffix) || strings.Contains(dstRemote, copyMoveBakSuffix) {
		return false
	}
	return strings.Contains(srcRemote, copyMoveTmpSuffix)
}

func putLogicalObject(ctx context.Context, t *testing.T, f *Fs, remote string, data []byte, mod time.Time) {
	t.Helper()
	info := object.NewStaticObjectInfo(remote, mod, int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)
}

func readLogicalObject(ctx context.Context, t *testing.T, f *Fs, remote string) []byte {
	t.Helper()
	o, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := o.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	return got
}

func shardHasSuffixObject(ctx context.Context, t *testing.T, backends []fs.Fs, suffix string) bool {
	t.Helper()
	for _, b := range backends {
		found := false
		err := operations.ListFn(ctx, b, func(o fs.Object) {
			if strings.Contains(o.Remote(), suffix) {
				found = true
			}
		})
		require.NoError(t, err)
		if found {
			return true
		}
	}
	return false
}

func TestCopyMovePhase1FailurePreservesDst(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-cm-p1")
	backends[2] = phaseFailCopyToTempFs{Fs: backends[2], fail: true}
	backends[3] = phaseFailCopyToTempFs{Fs: backends[3], fail: true}
	f := copyMoveTestFs(t, backends)

	dstData := []byte("original-destination-content")
	srcData := []byte("new-source-content")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700010000, 0))
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700010100, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	_, err = f.Copy(ctx, srcObj, "dst.bin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "copy quorum not met")

	require.Equal(t, dstData, readLogicalObject(ctx, t, f, "dst.bin"))
	_, err = probeAndSelectWriteIDGroup(ctx, f, "dst.bin", nil, 3, 1)
	require.NoError(t, err)
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveTmpSuffix))
}

func TestCopyMovePhase2FailureRestoresDst(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-cm-p2")
	backends[2] = phaseFailSwapFs{Fs: backends[2], fail: true}
	backends[3] = phaseFailSwapFs{Fs: backends[3], fail: true}
	f := copyMoveTestFs(t, backends)

	dstData := []byte("phase2-original-dst")
	srcData := []byte("phase2-new-src")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700020000, 0))
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700020100, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	_, err = f.Copy(ctx, srcObj, "dst.bin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "copy quorum not met")
	require.Equal(t, dstData, readLogicalObject(ctx, t, f, "dst.bin"))
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveTmpSuffix))
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveBakSuffix))
}

func TestCopyMoveOverwriteSuccess(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-cm-ok")
	f := copyMoveTestFs(t, backends)

	dstData := []byte("old-dst-bytes-for-overwrite")
	srcData := []byte("new-src-bytes-after-overwrite")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700030000, 0))
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700030100, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	srcFooterObj, err := f.backends[0].NewObject(ctx, "src.bin")
	require.NoError(t, err)
	srcFooter, err := readFooterFromParticle(ctx, srcFooterObj)
	require.NoError(t, err)

	_, err = f.Copy(ctx, srcObj, "dst.bin")
	require.NoError(t, err)
	require.Equal(t, srcData, readLogicalObject(ctx, t, f, "dst.bin"))

	for i := range f.backends {
		obj, err := f.backends[i].NewObject(ctx, "dst.bin")
		require.NoError(t, err, "shard %d", i)
		ft, err := readFooterFromParticle(ctx, obj)
		require.NoError(t, err, "shard %d", i)
		require.Equal(t, srcFooter.WriteID, ft.WriteID, "shard %d WriteID", i)
	}
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveTmpSuffix))
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveBakSuffix))
}

func TestMoveOverwriteFailurePreservesDst(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-mv-p2")
	backends[2] = phaseFailSwapFs{Fs: backends[2], fail: true}
	backends[3] = phaseFailSwapFs{Fs: backends[3], fail: true}
	f := copyMoveTestFs(t, backends)

	dstData := []byte("move-dst-original")
	srcData := []byte("move-src-new")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700040000, 0))
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700040100, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	_, err = f.Move(ctx, srcObj, "dst.bin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "move quorum not met")
	require.Equal(t, dstData, readLogicalObject(ctx, t, f, "dst.bin"))
	_, err = f.NewObject(ctx, "src.bin")
	require.NoError(t, err)
}

func TestMoveOverwriteSuccessRemovesSrc(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-mv-ok")
	f := copyMoveTestFs(t, backends)

	dstData := []byte("old-at-dst")
	srcData := []byte("moved-to-dst")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700050000, 0))
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700050100, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	_, err = f.Move(ctx, srcObj, "dst.bin")
	require.NoError(t, err)
	require.Equal(t, srcData, readLogicalObject(ctx, t, f, "dst.bin"))
	_, err = f.NewObject(ctx, "src.bin")
	require.Error(t, err)
}

func TestHealCleansCopyMoveArtifacts(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-cm-heal")
	f := copyMoveTestFs(t, backends)

	dstData := []byte("heal-dst-content")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700060000, 0))

	tmpRemote := "dst.bin.rs-tmp-deadbeef01234567"
	bakRemote := "dst.bin.rs-bak-deadbeef01234567"
	for _, b := range backends {
		info := object.NewStaticObjectInfo(tmpRemote, time.Unix(1700060001, 0), 3, true, nil, nil)
		_, err := b.Put(ctx, bytes.NewReader([]byte("tmp")), info)
		require.NoError(t, err)
	}

	outAny, err := f.healCommand(ctx, []string{"dst.bin"}, nil)
	require.NoError(t, err)
	out := outAny.(string)
	require.Contains(t, out, "PURGED_COPYMOVE_TMP")
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveTmpSuffix))

	// Leave bak while logical dst is healthy — heal should purge bak.
	for _, b := range backends {
		obj, err := b.NewObject(ctx, "dst.bin")
		require.NoError(t, err)
		rc, err := obj.Open(ctx)
		require.NoError(t, err)
		particle, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		info := object.NewStaticObjectInfo(bakRemote, time.Unix(1700060002, 0), int64(len(particle)), true, nil, nil)
		_, err = b.Put(ctx, bytes.NewReader(particle), info)
		require.NoError(t, err)
	}
	outAny, err = f.healCommand(ctx, []string{"dst.bin"}, nil)
	require.NoError(t, err)
	require.Contains(t, outAny.(string), "PURGED_COPYMOVE_BAK")
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveBakSuffix))
}

func TestParseCopyMoveArtifact(t *testing.T) {
	base, kind, nonce, ok := parseCopyMoveArtifact("path/file.bin.rs-tmp-abc123")
	require.True(t, ok)
	require.Equal(t, "path/file.bin", base)
	require.Equal(t, copyMoveArtifactTmp, kind)
	require.Equal(t, "abc123", nonce)

	base, kind, _, ok = parseCopyMoveArtifact("file.rs-bak-ff")
	require.True(t, ok)
	require.Equal(t, "file", base)
	require.Equal(t, copyMoveArtifactBak, kind)

	_, _, _, ok = parseCopyMoveArtifact("normal.bin")
	require.False(t, ok)
}

func noCopyBackends(t *testing.T, n int, prefix string) []fs.Fs {
	t.Helper()
	backends := makeLocalBackends(t, n, prefix)
	for i := range backends {
		backends[i] = noCopyFs{Fs: backends[i]}
	}
	return backends
}

func TestCopyRejectsWhenAnyShardLacksCopy(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-copy-nocopy")
	backends[1] = noCopyFs{Fs: backends[1]}
	f := copyMoveTestFs(t, backends)

	putLogicalObject(ctx, t, f, "src.bin", []byte("src"), time.Unix(1700070000, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	_, err = f.Copy(ctx, srcObj, "dst.bin")
	require.ErrorIs(t, err, fs.ErrorCantCopy)
	_, err = backends[0].NewObject(ctx, "dst.bin")
	require.Error(t, err)
}

func TestMoveRejectsMixedShardCapabilities(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-move-mixed")
	backends[0] = noCopyFs{Fs: backends[0]}
	backends[1] = noMoveFs{Fs: backends[1]}
	f := copyMoveTestFs(t, backends)

	putLogicalObject(ctx, t, f, "src.bin", []byte("src"), time.Unix(1700070100, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	_, err = f.Move(ctx, srcObj, "dst.bin")
	require.ErrorIs(t, err, fs.ErrorCantMove)
	_, err = f.NewObject(ctx, "src.bin")
	require.NoError(t, err)
	_, err = backends[0].NewObject(ctx, "dst.bin")
	require.Error(t, err)
}

func TestMoveOnlyGreenfieldSuccess(t *testing.T) {
	ctx := context.Background()
	backends := noCopyBackends(t, 4, "rs-mvonly-green")
	f := copyMoveTestFs(t, backends)

	srcData := []byte("move-only-greenfield")
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700070200, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	srcFooterObj, err := f.backends[0].NewObject(ctx, "src.bin")
	require.NoError(t, err)
	srcFooter, err := readFooterFromParticle(ctx, srcFooterObj)
	require.NoError(t, err)

	_, err = f.Move(ctx, srcObj, "dst.bin")
	require.NoError(t, err)
	require.Equal(t, srcData, readLogicalObject(ctx, t, f, "dst.bin"))
	_, err = f.NewObject(ctx, "src.bin")
	require.Error(t, err)

	for i := range f.backends {
		obj, err := f.backends[i].NewObject(ctx, "dst.bin")
		require.NoError(t, err, "shard %d", i)
		ft, err := readFooterFromParticle(ctx, obj)
		require.NoError(t, err, "shard %d", i)
		require.Equal(t, srcFooter.WriteID, ft.WriteID, "shard %d WriteID", i)
	}
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveTmpSuffix))
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveBakSuffix))
}

func TestMoveOnlyOverwriteSuccess(t *testing.T) {
	ctx := context.Background()
	backends := noCopyBackends(t, 4, "rs-mvonly-over")
	f := copyMoveTestFs(t, backends)

	dstData := []byte("old-dst-for-move-only")
	srcData := []byte("new-src-for-move-only")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700070300, 0))
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700070310, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	_, err = f.Move(ctx, srcObj, "dst.bin")
	require.NoError(t, err)
	require.Equal(t, srcData, readLogicalObject(ctx, t, f, "dst.bin"))
	_, err = f.NewObject(ctx, "src.bin")
	require.Error(t, err)
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveTmpSuffix))
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveBakSuffix))
}

func TestMoveOnlyPhase1FailurePreservesDstAndSrc(t *testing.T) {
	ctx := context.Background()
	backends := noCopyBackends(t, 4, "rs-mvonly-p1")
	backends[2] = phaseFailMoveBackupFs{Fs: backends[2], fail: true}
	backends[3] = phaseFailMoveBackupFs{Fs: backends[3], fail: true}
	f := copyMoveTestFs(t, backends)

	dstData := []byte("phase1-dst-original")
	srcData := []byte("phase1-src-new")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700070400, 0))
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700070410, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	_, err = f.Move(ctx, srcObj, "dst.bin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "move quorum not met")
	require.Equal(t, dstData, readLogicalObject(ctx, t, f, "dst.bin"))
	_, err = f.NewObject(ctx, "src.bin")
	require.NoError(t, err)
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveBakSuffix))
}

func TestMoveOnlyPhase2FailureRestoresDstAndSrc(t *testing.T) {
	ctx := context.Background()
	backends := noCopyBackends(t, 4, "rs-mvonly-p2")
	backends[2] = phaseFailMoveSwapFs{Fs: backends[2], fail: true}
	backends[3] = phaseFailMoveSwapFs{Fs: backends[3], fail: true}
	f := copyMoveTestFs(t, backends)

	dstData := []byte("phase2-dst-original")
	srcData := []byte("phase2-src-new")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700070500, 0))
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700070510, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	_, err = f.Move(ctx, srcObj, "dst.bin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "move quorum not met")
	require.Equal(t, dstData, readLogicalObject(ctx, t, f, "dst.bin"))
	_, err = f.NewObject(ctx, "src.bin")
	require.NoError(t, err)
	require.False(t, shardHasSuffixObject(ctx, t, backends, copyMoveBakSuffix))
}

func TestMoveOnlyPhase2FailureRollbackDisabledPreservesSrc(t *testing.T) {
	ctx := context.Background()
	backends := noCopyBackends(t, 4, "rs-mvonly-norb")
	backends[1] = phaseFailMoveSwapFs{Fs: backends[1], fail: true}
	backends[2] = phaseFailMoveSwapFs{Fs: backends[2], fail: true}
	backends[3] = phaseFailMoveSwapFs{Fs: backends[3], fail: true}
	f := copyMoveTestFs(t, backends)
	f.opt.Rollback = false

	dstData := []byte("rollback-disabled-dst")
	srcData := []byte("rollback-disabled-src")
	putLogicalObject(ctx, t, f, "dst.bin", dstData, time.Unix(1700070600, 0))
	putLogicalObject(ctx, t, f, "src.bin", srcData, time.Unix(1700070610, 0))
	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	srcFooterObj, err := f.backends[0].NewObject(ctx, "src.bin")
	require.NoError(t, err)
	srcFooter, err := readFooterFromParticle(ctx, srcFooterObj)
	require.NoError(t, err)

	_, err = f.Move(ctx, srcObj, "dst.bin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "move quorum not met")

	require.Equal(t, srcData, readLogicalObject(ctx, t, f, "src.bin"))

	srcParticles := 0
	for i, b := range backends {
		obj, err := b.NewObject(ctx, "src.bin")
		if err != nil {
			continue
		}
		srcParticles++
		if i == 0 {
			continue
		}
		ft, err := readFooterFromParticle(ctx, obj)
		require.NoError(t, err, "shard %d", i)
		require.Equal(t, srcFooter.WriteID, ft.WriteID, "shard %d WriteID", i)
	}
	require.GreaterOrEqual(t, srcParticles, f.readQuorum(), "src path must keep >= k particles")

	movedObj, err := f.backends[0].NewObject(ctx, "dst.bin")
	require.NoError(t, err, "shard 0 should still hold moved-src particle at dst")
	movedFooter, err := readFooterFromParticle(ctx, movedObj)
	require.NoError(t, err)
	require.Equal(t, srcFooter.WriteID, movedFooter.WriteID, "restore pass must not delete moved-src particle on shard 0")
}
