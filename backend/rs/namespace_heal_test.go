package rs

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/require"
)

func namespaceTestFs(t *testing.T, backends []fs.Fs) *Fs {
	t.Helper()
	return &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			Rollback:           true,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
}

func TestDegradedLsdReportsDirSkew(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-ns-lsd-skew")
	for i := 0; i < 3; i++ {
		require.NoError(t, backends[i].Mkdir(ctx, "skewdir"))
	}
	f := namespaceTestFs(t, backends)

	out, err := f.degradedListDirectories(ctx)
	require.NoError(t, err)
	require.Contains(t, out, "SKEW skewdir")
	require.Contains(t, out, "dirVotes=3")
}

func TestDegradedLsdReportsExtraDir(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-ns-lsd-extra")
	require.NoError(t, backends[0].Mkdir(ctx, "extradir"))
	require.NoError(t, backends[1].Mkdir(ctx, "extradir"))
	f := namespaceTestFs(t, backends)

	out, err := f.degradedListDirectories(ctx)
	require.NoError(t, err)
	require.Contains(t, out, "EXTRA extradir")
}

func TestHealNamespaceMkdirSkew(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-ns-heal-mkdir")
	for i := 0; i < 3; i++ {
		require.NoError(t, backends[i].Mkdir(ctx, "healdir"))
	}
	f := namespaceTestFs(t, backends)

	stats, _, err := f.healNamespace(ctx, "healdir", false)
	require.NoError(t, err)
	require.Equal(t, 1, stats.mkdirs)
	require.True(t, backendRootHasDir(ctx, backends[3], "healdir"))
}

func TestHealNamespacePurgeOrphanObject(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-ns-heal-orphan")
	f := namespaceTestFs(t, backends)

	orphan := object.NewStaticObjectInfo("lonely.bin", time.Unix(1700008000, 0), 1, true, nil, nil)
	_, err := backends[0].Put(ctx, bytes.NewReader([]byte("x")), orphan)
	require.NoError(t, err)

	stats, _, err := f.healNamespace(ctx, "", false)
	require.NoError(t, err)
	require.Equal(t, 1, stats.orphansPurged)
	_, err = backends[0].NewObject(ctx, "lonely.bin")
	require.Error(t, err)
}

func TestHealNamespaceRmdirExtraDir(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-ns-heal-rmdir")
	require.NoError(t, backends[0].Mkdir(ctx, "gone"))
	require.NoError(t, backends[1].Mkdir(ctx, "gone"))
	f := namespaceTestFs(t, backends)

	stats, _, err := f.healNamespace(ctx, "gone", false)
	require.NoError(t, err)
	require.Equal(t, 2, stats.rmdirs)
	require.False(t, backendRootHasDir(ctx, backends[0], "gone"))
	require.False(t, backendRootHasDir(ctx, backends[1], "gone"))
}
