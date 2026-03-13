package vfs

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rcNewRun(t *testing.T, method string) (r *fstest.Run, vfs *VFS, call *rc.Call) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	r, vfs = newTestVFS(t)
	call = rc.Calls.Get(method)
	assert.NotNil(t, call)
	return r, vfs, call
}

func TestRcGetVFS(t *testing.T) {
	in := rc.Params{}
	vfs, err := getVFS(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no VFS active")
	assert.Nil(t, vfs)

	r, vfs2 := newTestVFS(t)

	vfs, err = getVFS(in)
	require.NoError(t, err)
	assert.True(t, vfs == vfs2)

	inPresent := rc.Params{"fs": fs.ConfigString(r.Fremote)}
	vfs, err = getVFS(inPresent)
	require.NoError(t, err)
	assert.True(t, vfs == vfs2)

	inWrong := rc.Params{"fs": fs.ConfigString(r.Fremote) + "notfound"}
	vfs, err = getVFS(inWrong)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no VFS found with name")
	assert.Nil(t, vfs)

	opt := vfscommon.Opt
	opt.NoModTime = true
	vfs3 := New(r.Fremote, &opt)
	defer vfs3.Shutdown()

	vfs, err = getVFS(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than one VFS active - need")
	assert.Nil(t, vfs)

	inPresent = rc.Params{"fs": fs.ConfigString(r.Fremote)}
	vfs, err = getVFS(inPresent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than one VFS active with name")
	assert.Nil(t, vfs)
}

func TestRcForget(t *testing.T) {
	r, vfs, call := rcNewRun(t, "vfs/forget")
	_, _ = r, vfs
	in := rc.Params{"fs": fs.ConfigString(r.Fremote)}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"forgotten": []string{},
	}, out)
	// FIXME needs more tests
}

func TestRcRefresh(t *testing.T) {
	r, vfs, call := rcNewRun(t, "vfs/refresh")
	_, _ = r, vfs
	in := rc.Params{"fs": fs.ConfigString(r.Fremote)}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"result": map[string]string{
			"": "OK",
		},
	}, out)
	// FIXME needs more tests
}

func TestRcPollInterval(t *testing.T) {
	r, vfs, call := rcNewRun(t, "vfs/poll-interval")
	_ = vfs
	if r.Fremote.Features().ChangeNotify == nil {
		t.Skip("ChangeNotify not supported")
	}
	out, err := call.Fn(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{}, out)
	// FIXME needs more tests
}

func TestRcList(t *testing.T) {
	r, vfs, call := rcNewRun(t, "vfs/list")
	_ = vfs

	out, err := call.Fn(context.Background(), nil)
	require.NoError(t, err)

	assert.Equal(t, rc.Params{
		"vfses": []string{
			fs.ConfigString(r.Fremote),
		},
	}, out)
}

func TestRcStats(t *testing.T) {
	r, vfs, call := rcNewRun(t, "vfs/stats")
	out, err := call.Fn(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, fs.ConfigString(r.Fremote), out["fs"])
	assert.Equal(t, int32(1), out["inUse"])
	assert.Equal(t, 0, out["metadataCache"].(rc.Params)["files"])
	assert.Equal(t, 1, out["metadataCache"].(rc.Params)["dirs"])
	assert.Equal(t, vfs.Opt, out["opt"].(vfscommon.Options))
}

func TestRcTransfers(t *testing.T) {
	// Test with default VFS (no cache) - should return empty
	r, vfs, call := rcNewRun(t, "vfs/transfers")
	_, _ = r, vfs
	in := rc.Params{"fs": fs.ConfigString(r.Fremote)}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)

	// With no cache, should return empty transfers and zeroed summary
	transfers := out["transfers"]
	assert.NotNil(t, transfers)

	summary, ok := out["summary"].(rc.Params)
	require.True(t, ok)
	assert.Equal(t, 0, summary["activeReads"])
	assert.Equal(t, 0, summary["activeDownloads"])
	assert.Equal(t, 0, summary["totalOpenFiles"])
	assert.Equal(t, false, summary["outOfSpace"])
}

func TestRcTransfersWithCache(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeFull
	opt.CachePollInterval = 0
	r, vfs := newTestVFSOpt(t, &opt)
	call := rc.Calls.Get("vfs/transfers")
	assert.NotNil(t, call)

	in := rc.Params{"fs": fs.ConfigString(r.Fremote)}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)

	// With cache enabled but no open files, should return empty transfers
	summary, ok := out["summary"].(rc.Params)
	require.True(t, ok)
	assert.Equal(t, 0, summary["activeReads"])
	assert.Equal(t, 0, summary["activeDownloads"])
	assert.Equal(t, 0, summary["totalOpenFiles"])

	// totalCacheFiles and totalCacheBytes should be present
	_, ok = summary["totalCacheFiles"]
	assert.True(t, ok)
	_, ok = summary["totalCacheBytes"]
	assert.True(t, ok)

	_ = vfs
}
