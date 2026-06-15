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
