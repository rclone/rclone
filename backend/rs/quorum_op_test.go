package rs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/require"
)

// backendRootHasDir reports whether dir exists at the backend root (use local backends in tests).
func backendRootHasDir(ctx context.Context, b fs.Fs, dir string) bool {
	entries, err := b.List(ctx, "")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Remote() == dir {
			return true
		}
	}
	return false
}

func testQuorumFs(t *testing.T, backends []fs.Fs, writeQuorum int) *Fs {
	t.Helper()
	return &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:   2,
			ParityShards: 1,
			WriteQuorum:  writeQuorum,
			Rollback:     true,
			UseSpooling:  true,
		},
		features: (&fs.Features{}),
	}
}

func TestQuorumOpPreflightInsufficientReachable(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-quorum-preflight")
	backends[2] = failListFs{Fs: backends[2], fail: true}
	backends[3] = failListFs{Fs: backends[3], fail: true}
	f := testQuorumFs(t, backends, 3)

	_, err := f.preflightReachableShards(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient reachable remotes")
	require.Contains(t, err.Error(), "available=2")
	require.Contains(t, err.Error(), "required=3")
}

func TestQuorumOpPreflightReachableOK(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-quorum-preflight-ok")
	backends[3] = failListFs{Fs: backends[3], fail: true}
	f := testQuorumFs(t, backends, 3)

	reachable, err := f.preflightReachableShards(ctx)
	require.NoError(t, err)
	require.Len(t, reachable, 3)
}

func TestQuorumOpCommitSuccess(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-quorum-commit")
	f := testQuorumFs(t, backends, 3)

	err := f.runQuorumTransaction(ctx, quorumTransaction{
		OpName: "mkdir",
		Remote: "qdir",
		Forward: func(opCtx context.Context, shard int) error {
			return f.backends[shard].Mkdir(opCtx, "qdir")
		},
	})
	require.NoError(t, err)
	for _, b := range backends {
		require.True(t, backendRootHasDir(ctx, b, "qdir"))
	}
}

func TestQuorumOpRollbackOnCommitFailure(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-quorum-rollback")
	backends[2] = failMkdirFs{Fs: backends[2], fail: true}
	backends[3] = failMkdirFs{Fs: backends[3], fail: true}
	f := testQuorumFs(t, backends, 3)

	var rollbackCalls atomic.Int32
	err := f.runQuorumTransaction(ctx, quorumTransaction{
		OpName: "mkdir",
		Remote: "rbdir",
		Forward: func(opCtx context.Context, shard int) error {
			return f.backends[shard].Mkdir(opCtx, "rbdir")
		},
		Rollback: func(opCtx context.Context, shard int) error {
			rollbackCalls.Add(1)
			return f.backends[shard].Rmdir(opCtx, "rbdir")
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "quorum not met")
	require.Equal(t, int32(2), rollbackCalls.Load(), "rollback should run on successful shards only")

	require.False(t, backendRootHasDir(ctx, backends[0], "rbdir"))
	require.False(t, backendRootHasDir(ctx, backends[1], "rbdir"))
}

func TestQuorumOpRollbackDisabled(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-quorum-no-rollback")
	backends[2] = failMkdirFs{Fs: backends[2], fail: true}
	backends[3] = failMkdirFs{Fs: backends[3], fail: true}
	f := testQuorumFs(t, backends, 3)
	f.opt.Rollback = false

	var rollbackCalls atomic.Int32
	err := f.runQuorumTransaction(ctx, quorumTransaction{
		OpName: "mkdir",
		Remote: "norbdir",
		Forward: func(opCtx context.Context, shard int) error {
			return f.backends[shard].Mkdir(opCtx, "norbdir")
		},
		Rollback: func(opCtx context.Context, shard int) error {
			rollbackCalls.Add(1)
			return f.backends[shard].Rmdir(opCtx, "norbdir")
		},
	})
	require.Error(t, err)
	require.Equal(t, int32(0), rollbackCalls.Load())

	present := 0
	for _, b := range backends {
		if backendRootHasDir(ctx, b, "norbdir") {
			present++
		}
	}
	require.Equal(t, 2, present, "mkdir should remain on shards that succeeded without rollback")
}

func TestQuorumOpRollbackPartialFailure(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-quorum-rb-partial")
	backends[2] = failMkdirFs{Fs: backends[2], fail: true}
	backends[3] = failMkdirFs{Fs: backends[3], fail: true}
	f := testQuorumFs(t, backends, 3)

	err := f.runQuorumTransaction(ctx, quorumTransaction{
		OpName: "mkdir",
		Remote: "partial",
		Forward: func(opCtx context.Context, shard int) error {
			return f.backends[shard].Mkdir(opCtx, "partial")
		},
		Rollback: func(opCtx context.Context, shard int) error {
			if shard == 0 {
				return errors.New("injected rollback failure")
			}
			return f.backends[shard].Rmdir(opCtx, "partial")
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "quorum not met")

	require.True(t, backendRootHasDir(ctx, backends[0], "partial"), "shard 0 rollback failed so mkdir should remain")
	require.False(t, backendRootHasDir(ctx, backends[1], "partial"))
}

func TestQuorumSuccessIndices(t *testing.T) {
	shards := []int{0, 1, 2, 3}
	failures := map[int]shardFailure{2: {err: errors.New("x"), phase: 1}, 3: {err: errors.New("y"), phase: 2}}
	require.Equal(t, []int{0, 1}, quorumSuccessIndices(shards, failures))
}

func TestRmdirQuorumCommitSuccess(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-rmdir-commit")
	f := testQuorumFs(t, backends, 3)

	require.NoError(t, f.Mkdir(ctx, "rmdir-ok"))
	require.NoError(t, f.Rmdir(ctx, "rmdir-ok"))
	for _, b := range backends {
		require.False(t, backendRootHasDir(ctx, b, "rmdir-ok"))
	}
}

func TestRmdirQuorumPreflightInsufficientReachable(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-rmdir-preflight")
	backends[2] = failListFs{Fs: backends[2], fail: true}
	backends[3] = failListFs{Fs: backends[3], fail: true}
	f := testQuorumFs(t, backends, 3)
	require.NoError(t, backends[0].Mkdir(ctx, "gone"))

	err := f.Rmdir(ctx, "gone")
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient reachable remotes")
}

func TestRmdirQuorumPartialHadDirBelowWriteQuorum(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-rmdir-partial")
	f := testQuorumFs(t, backends, 3)
	require.NoError(t, backends[0].Mkdir(ctx, "skew"))
	require.NoError(t, backends[1].Mkdir(ctx, "skew"))

	err := f.Rmdir(ctx, "skew")
	require.Error(t, err)
	require.Contains(t, err.Error(), "rmdir quorum not met")
	require.True(t, backendRootHasDir(ctx, backends[0], "skew"))
	require.True(t, backendRootHasDir(ctx, backends[1], "skew"))
}

func TestRmdirQuorumRollbackOnFailure(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-rmdir-rollback")
	backends[2] = failRmdirFs{Fs: backends[2], fail: true}
	backends[3] = failRmdirFs{Fs: backends[3], fail: true}
	f := testQuorumFs(t, backends, 3)
	require.NoError(t, f.Mkdir(ctx, "rb-rmdir"))

	err := f.Rmdir(ctx, "rb-rmdir")
	require.Error(t, err)
	require.Contains(t, err.Error(), "rmdir quorum not met")
	for _, b := range backends[:2] {
		require.True(t, backendRootHasDir(ctx, b, "rb-rmdir"), "rollback should restore mkdir on successful rmdir shards")
	}
}

func TestRmdirQuorumUnreachableShardAllowed(t *testing.T) {
	ctx := context.Background()
	backends := makeLocalBackends(t, 4, "rs-rmdir-unreach")
	backends[3] = failListFs{Fs: backends[3], fail: true}
	f := testQuorumFs(t, backends, 3)
	for i := 0; i < 3; i++ {
		require.NoError(t, backends[i].Mkdir(ctx, "cohort"))
	}

	require.NoError(t, f.Rmdir(ctx, "cohort"))
	for i := 0; i < 3; i++ {
		require.False(t, backendRootHasDir(ctx, backends[i], "cohort"))
	}
}
