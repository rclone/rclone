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
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/require"
)

var _ fstests.InternalTester = (*Fs)(nil)

// InternalTest runs rs-specific checks during the fstest Internal subtest.
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("FooterOnParticles", func(t *testing.T) { testInternalFooterOnParticles(t, f) })
	t.Run("QuorumList", func(t *testing.T) { testInternalQuorumList(t, f) })
	t.Run("Degraded", func(t *testing.T) { testInternalDegraded(t, f) })
	t.Run("Heal", func(t *testing.T) { testInternalHeal(t, f) })
}

const internalTestDir = "internal-test"

func testInternalFooterOnParticles(t *testing.T, f *Fs) {
	ctx := context.Background()
	require.NoError(t, f.Mkdir(context.Background(), internalTestDir))
	remote := internalTestDir + "/footer.bin"
	data := []byte("internal-footer-payload")
	putInternalTestObject(ctx, t, f, remote, data)

	var ref *Footer
	total := len(f.backends)
	for i := 0; i < total; i++ {
		particle := readShardParticle(ctx, t, f, i, remote)
		payload, ft, err := ExtractParticlePayload(particle, i)
		require.NoError(t, err, "shard %d", i)
		require.Equal(t, int(ft.CurrentShard), i)
		require.Equal(t, crc32cChecksum(payload), ft.PayloadCRC32C)
		if ref == nil {
			ref = ft
		} else {
			require.Equal(t, ref.ContentLength, ft.ContentLength)
			require.Equal(t, ref.MD5, ft.MD5)
			require.Equal(t, ref.SHA256, ft.SHA256)
			require.Equal(t, ref.NumStripes, ft.NumStripes)
			require.Equal(t, ref.StripeSize, ft.StripeSize)
			require.Equal(t, ref.DataShards, ft.DataShards)
			require.Equal(t, ref.ParityShards, ft.ParityShards)
		}
	}
	require.NotNil(t, ref)
	require.Equal(t, int64(len(data)), ref.ContentLength)
}

func testInternalQuorumList(t *testing.T, f *Fs) {
	ctx := context.Background()
	require.NoError(t, f.Mkdir(ctx, internalTestDir))
	keep := internalTestDir + "/keep.bin"
	drop := internalTestDir + "/drop.bin"
	putInternalTestObject(ctx, t, f, keep, []byte("keep"))
	putInternalTestObject(ctx, t, f, drop, []byte("drop"))

	deleteShardParticle(ctx, t, f, 2, drop)
	deleteShardParticle(ctx, t, f, 3, drop)

	entries, err := f.List(ctx, internalTestDir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Remote())
	}
	require.Equal(t, []string{keep}, names)
}

func testInternalDegraded(t *testing.T, f *Fs) {
	ctx := context.Background()
	require.NoError(t, f.Mkdir(ctx, internalTestDir))
	remote := internalTestDir + "/degraded.bin"
	putInternalTestObject(ctx, t, f, remote, []byte("degraded"))

	deleteShardParticle(ctx, t, f, 2, remote)
	deleteShardParticle(ctx, t, f, 3, remote)

	out, err := f.Command(ctx, "degraded", []string{"summary"}, nil)
	require.NoError(t, err)
	require.Contains(t, out.(string), "Degraded: 1")

	out, err = f.Command(ctx, "degraded", []string{"ls"}, nil)
	require.NoError(t, err)
	require.Contains(t, out.(string), "DEGRADED "+remote)
}

func testInternalHeal(t *testing.T, f *Fs) {
	ctx := context.Background()
	require.NoError(t, f.Mkdir(ctx, internalTestDir))
	remote := internalTestDir + "/heal.bin"
	data := []byte("heal-me-internal")
	putInternalTestObject(ctx, t, f, remote, data)

	lastShard := len(f.backends) - 1
	deleteShardParticle(ctx, t, f, lastShard, remote)

	out, err := f.healCommand(ctx, []string{remote}, nil)
	require.NoError(t, err)
	require.Contains(t, out.(string), "restored 1 shard(s)")

	total := len(f.backends)
	for i := 0; i < total; i++ {
		particle := readShardParticle(ctx, t, f, i, remote)
		_, ft, err := ExtractParticlePayload(particle, i)
		require.NoError(t, err, "shard %d after heal", i)
		require.Equal(t, int64(len(data)), ft.ContentLength)
	}
}

func putInternalTestObject(ctx context.Context, t *testing.T, f *Fs, remote string, data []byte) {
	t.Helper()
	info := object.NewStaticObjectInfo(remote, time.Unix(1700008000, 0), int64(len(data)), true, nil, nil)
	obj, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = obj.Remove(ctx)
	})
}

func readShardParticle(ctx context.Context, t *testing.T, f *Fs, shard int, remote string) []byte {
	t.Helper()
	obj, err := f.backends[shard].NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	return b
}

func deleteShardParticle(ctx context.Context, t *testing.T, f *Fs, shard int, remote string) {
	t.Helper()
	obj, err := f.backends[shard].NewObject(ctx, remote)
	if errors.Is(err, fs.ErrorObjectNotFound) {
		return
	}
	require.NoError(t, err)
	require.NoError(t, obj.Remove(ctx))
}
