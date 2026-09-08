package rs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/require"
)

func encodeShardsForTest(t *testing.T, remote string, data []byte, k, m, stripeS int) ([]*bytes.Buffer, *BuildResult) {
	t.Helper()
	src := object.NewStaticObjectInfo(remote, time.Unix(1700003000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, k+m)
	ios := make([]io.Writer, k+m)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	bres, err := BuildRSShardsToWriters(context.Background(), bytes.NewReader(data), src, k, m, stripeS, ios, true)
	require.NoError(t, err)
	return writers, bres
}

func uploadShardBuffers(ctx context.Context, t *testing.T, backends []fs.Fs, remote string, bufs []*bytes.Buffer, mod time.Time) {
	t.Helper()
	for i := range backends {
		blob := bufs[i].Bytes()
		info := object.NewStaticObjectInfo(remote, mod, int64(len(blob)), true, nil, nil)
		_, err := backends[i].Put(ctx, bytes.NewReader(blob), info)
		require.NoError(t, err)
	}
}

func cloneShardBuffers(src []*bytes.Buffer) []*bytes.Buffer {
	out := make([]*bytes.Buffer, len(src))
	for i, b := range src {
		out[i] = bytes.NewBuffer(append([]byte(nil), b.Bytes()...))
	}
	return out
}

func writeIDFsForTest(t *testing.T, backends []fs.Fs, k, m int) *Fs {
	t.Helper()
	return &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         k,
			ParityShards:       m,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
}

func TestPutAssignsSharedWriteID(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-writeid-put")
	f := writeIDFsForTest(t, backends, 3, 1)
	data := bytes.Repeat([]byte("shared-id"), 80)
	info := object.NewStaticObjectInfo("shared.bin", time.Unix(1700003000, 0), int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	var ids []uint64
	for i := range backends {
		obj, err := backends[i].NewObject(ctx, "shared.bin")
		require.NoError(t, err)
		ft, err := readFooterFromParticle(ctx, obj)
		require.NoError(t, err)
		require.NotZero(t, ft.WriteID)
		ids = append(ids, ft.WriteID)
	}
	require.Equal(t, ids[0], ids[1])
	require.Equal(t, ids[0], ids[2])
	require.Equal(t, ids[0], ids[3])
}

func TestWriteIDMixedReadNeverJoinsAcrossWrites(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-writeid-torn")
	f := writeIDFsForTest(t, backends, 3, 1)
	remote := "torn.bin"

	dataA := bytes.Repeat([]byte("AAAA"), 64)
	dataB := bytes.Repeat([]byte("BBBB"), 64)
	bufsA, _ := encodeShardsForTest(t, remote, dataA, 3, 1, 64)
	bufsB, _ := encodeShardsForTest(t, remote, dataB, 3, 1, 64)

	mixed := cloneShardBuffers(bufsA)
	mixed[0] = bytes.NewBuffer(bufsB[0].Bytes())
	uploadShardBuffers(ctx, t, backends, remote, mixed, time.Unix(1700003000, 0))

	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.Equal(t, dataA, got)
}

func TestWriteIDParityAssistedReconstruct(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-writeid-parity")
	f := writeIDFsForTest(t, backends, 3, 1)
	remote := "parity.bin"

	dataWin := bytes.Repeat([]byte("WINN"), 64)
	dataLose := bytes.Repeat([]byte("LOSE"), 64)
	bufsWin, _ := encodeShardsForTest(t, remote, dataWin, 3, 1, 64)
	bufsLose, _ := encodeShardsForTest(t, remote, dataLose, 3, 1, 64)

	mixed := cloneShardBuffers(bufsWin)
	mixed[2] = bytes.NewBuffer(bufsLose[2].Bytes())
	uploadShardBuffers(ctx, t, backends, remote, mixed, time.Unix(1700003000, 0))

	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.Equal(t, dataWin, got)
}

func TestWriteIDSkewKMinusOneReturnsError(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-writeid-kminus1")
	f := writeIDFsForTest(t, backends, 3, 1)
	remote := "kminus1.bin"

	dataWin := bytes.Repeat([]byte("WINN"), 64)
	dataLose := bytes.Repeat([]byte("LOSE"), 64)
	bufsWin, bresWin := encodeShardsForTest(t, remote, dataWin, 3, 1, 64)
	bufsLose, _ := encodeShardsForTest(t, remote, dataLose, 3, 1, 64)

	// Two WIN shards + two LOSE shards: no group reaches k=3.
	mixed := cloneShardBuffers(bufsWin)
	mixed[2] = bytes.NewBuffer(bufsLose[2].Bytes())
	mixed[3] = bytes.NewBuffer(bufsLose[3].Bytes())
	uploadShardBuffers(ctx, t, backends, remote, mixed, time.Unix(1700003000, 0))

	layoutRef := footerFromBuildResult(bresWin, 3, 1)
	_, err := probeAndSelectWriteIDGroup(ctx, f, remote, layoutRef, 3, 1)
	require.ErrorIs(t, err, errWriteIDSkew)

	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	_, err = obj.Open(ctx)
	require.Error(t, err)
	require.True(t, errors.Is(err, errWriteIDSkew) || containsWriteIDSkew(err), "got %v", err)
}

func TestWriteIDSkewSplitGroupsReturnError(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-writeid-split")
	f := writeIDFsForTest(t, backends, 3, 1)
	remote := "split.bin"

	dataA := bytes.Repeat([]byte("AAAA"), 64)
	dataB := bytes.Repeat([]byte("BBBB"), 64)
	bufsA, bresA := encodeShardsForTest(t, remote, dataA, 3, 1, 64)
	bufsB, _ := encodeShardsForTest(t, remote, dataB, 3, 1, 64)

	// Exactly k-1 shards per write (2 each when k=3 is wrong - use 2+2 with none reaching 3).
	mixed := cloneShardBuffers(bufsA)
	mixed[2] = bytes.NewBuffer(bufsB[2].Bytes())
	mixed[3] = bytes.NewBuffer(bufsB[3].Bytes())
	uploadShardBuffers(ctx, t, backends, remote, mixed, time.Unix(1700003000, 0))

	layoutRef := footerFromBuildResult(bresA, 3, 1)
	_, err := probeAndSelectWriteIDGroup(ctx, f, remote, layoutRef, 3, 1)
	require.ErrorIs(t, err, errWriteIDSkew)
}

func TestHealConvergesWriteIDSkew(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-writeid-heal")
	f := writeIDFsForTest(t, backends, 3, 1)
	remote := "heal-skew.bin"

	dataWin := bytes.Repeat([]byte("HEAL"), 64)
	dataLose := bytes.Repeat([]byte("LOSE"), 64)
	bufsWin, _ := encodeShardsForTest(t, remote, dataWin, 3, 1, 64)
	bufsLose, _ := encodeShardsForTest(t, remote, dataLose, 3, 1, 64)

	mixed := cloneShardBuffers(bufsWin)
	mixed[2] = bytes.NewBuffer(bufsLose[2].Bytes())
	uploadShardBuffers(ctx, t, backends, remote, mixed, time.Unix(1700003000, 0))

	winID := parseFooterFromBuffer(t, bufsWin[0].Bytes()).WriteID
	loseID := parseFooterFromBuffer(t, bufsLose[2].Bytes()).WriteID
	require.NotEqual(t, winID, loseID)

	outAny, err := f.healCommand(ctx, []string{remote}, nil)
	require.NoError(t, err)
	require.Contains(t, outAny.(string), "restored 1 shard(s)")

	var ids []uint64
	for i := range backends {
		obj, err := backends[i].NewObject(ctx, remote)
		require.NoError(t, err)
		ft, err := readFooterFromParticle(ctx, obj)
		require.NoError(t, err)
		ids = append(ids, ft.WriteID)
	}
	require.Equal(t, winID, ids[0])
	require.Equal(t, winID, ids[1])
	require.Equal(t, winID, ids[2])
	require.Equal(t, winID, ids[3])

	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.Equal(t, dataWin, got)
}

func TestHealUsesSharedWriteIDSelector(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-writeid-heal-sel")
	f := writeIDFsForTest(t, backends, 3, 1)
	remote := "selector.bin"

	dataWin := bytes.Repeat([]byte("SEL"), 64)
	dataLose := bytes.Repeat([]byte("XXX"), 64)
	bufsWin, bres := encodeShardsForTest(t, remote, dataWin, 3, 1, 64)
	bufsLose, _ := encodeShardsForTest(t, remote, dataLose, 3, 1, 64)

	mixed := cloneShardBuffers(bufsWin)
	mixed[2] = bytes.NewBuffer(bufsLose[2].Bytes())
	uploadShardBuffers(ctx, t, backends, remote, mixed, time.Unix(1700003000, 0))

	layoutRef := footerFromBuildResult(bres, 3, 1)
	sel, err := probeAndSelectWriteIDGroup(ctx, f, remote, layoutRef, 3, 1)
	require.NoError(t, err)

	meta, missing, err := f.discoverHealShardPresence(ctx, remote, 4)
	require.NoError(t, err)
	require.Equal(t, sel.refFooter.WriteID, meta.WriteID)
	require.Equal(t, sel.present, invertMissing(missing))
}

func footerFromBuildResult(b *BuildResult, k, m int) *Footer {
	return &Footer{
		ContentLength: b.ContentLength,
		MD5:           b.MD5,
		SHA256:        b.SHA256,
		Mtime:         b.Mtime.UnixNano(),
		StripeSize:    b.StripeSize,
		NumStripes:    b.NumStripes,
		DataShards:    uint8(k),
		ParityShards:  uint8(m),
		Algorithm:     AlgorithmSYMM,
		WriteID:       b.WriteID,
	}
}

func parseFooterFromBuffer(t *testing.T, particle []byte) *Footer {
	t.Helper()
	ft, err := ParseFooter(particle[len(particle)-FooterSize:])
	require.NoError(t, err)
	return ft
}

func invertMissing(missing []bool) []bool {
	out := make([]bool, len(missing))
	for i, m := range missing {
		out[i] = !m
	}
	return out
}

func containsWriteIDSkew(err error) bool {
	for err != nil {
		if errors.Is(err, errWriteIDSkew) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}
