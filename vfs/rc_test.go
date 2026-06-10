package vfs

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscache"
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

func rcNewManualWriteBackRun(t *testing.T, method string) (r *fstest.Run, vfs *VFS, call *rc.Call) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeWrites
	opt.ManualWriteBack = true
	opt.WriteBack = 0
	opt.HandleCaching = 0
	r, vfs = newTestVFSOpt(t, &opt)
	call = rc.Calls.Get(method)
	assert.NotNil(t, call)
	return r, vfs, call
}

func rcWriteManualDirty(t *testing.T, vfs *VFS, name, contents string) {
	h, err := vfs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	require.NoError(t, err)
	n, err := h.Write([]byte(contents))
	require.NoError(t, err)
	assert.Equal(t, len(contents), n)
	require.NoError(t, h.Close())
	vfs.WaitForWriters(waitForWritersDelay)
}

func rcCheckRemoteItems(t *testing.T, r *fstest.Run, items ...fstest.Item) {
	fstest.CheckListingWithPrecision(t, r.Fremote, items, nil, fs.ModTimeNotSupported)
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
	vfs3 := New(context.Background(), r.Fremote, &opt)
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

func TestRcDirtyAndPushManualWriteBack(t *testing.T) {
	r, vfs, dirtyCall := rcNewManualWriteBackRun(t, "vfs/dirty")
	pushCall := rc.Calls.Get("vfs/push")
	require.NotNil(t, pushCall)

	require.NoError(t, vfs.MkdirAll("dir", 0777))
	rcWriteManualDirty(t, vfs, "dir/file1", "hello")
	rcWriteManualDirty(t, vfs, "dir/file2", "bye")
	rcCheckRemoteItems(t, r)

	fsString := fs.ConfigString(r.Fremote)
	out, err := dirtyCall.Fn(context.Background(), rc.Params{"fs": fsString, "dir": "dir"})
	require.NoError(t, err)
	assert.Equal(t, 2, out["count"])
	assert.Equal(t, int64(8), out["bytes"])
	infos := out["files"].([]vfscache.DirtyInfo)
	require.Len(t, infos, 2)
	assert.Equal(t, "dir/file1", infos[0].Name)
	assert.Equal(t, "dir/file2", infos[1].Name)

	_, err = pushCall.Fn(context.Background(), rc.Params{"fs": fsString})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "need at least one")

	out, err = pushCall.Fn(context.Background(), rc.Params{"fs": fsString, "file": "dir/file1"})
	require.NoError(t, err)
	assert.Equal(t, []manualResult{{Name: "dir/file1"}}, out["pushed"])
	assert.Equal(t, []manualResult{}, out["failed"])
	rcCheckRemoteItems(t, r, fstest.NewItem("dir/file1", "hello", t1))

	out, err = dirtyCall.Fn(context.Background(), rc.Params{"fs": fsString})
	require.NoError(t, err)
	infos = out["files"].([]vfscache.DirtyInfo)
	require.Len(t, infos, 1)
	assert.Equal(t, "dir/file2", infos[0].Name)
}

func TestRcRevertManualWriteBack(t *testing.T) {
	r, vfs, revertCall := rcNewManualWriteBackRun(t, "vfs/revert")
	old := r.WriteObject(context.Background(), "existing", "old", t1)
	rcCheckRemoteItems(t, r, old)

	rcWriteManualDirty(t, vfs, "existing", "new")
	rcWriteManualDirty(t, vfs, "new", "staged")
	rcCheckRemoteItems(t, r, old)

	out, err := revertCall.Fn(context.Background(), rc.Params{"fs": fs.ConfigString(r.Fremote), "all": true})
	require.NoError(t, err)
	assert.Equal(t, []manualResult{{Name: "existing"}, {Name: "new"}}, out["reverted"])
	assert.Equal(t, []manualResult{}, out["failed"])
	rcCheckRemoteItems(t, r, old)

	node, err := vfs.Stat("existing")
	require.NoError(t, err)
	assert.Equal(t, int64(3), node.Size())

	_, err = vfs.Stat("new")
	assert.True(t, errors.Is(err, os.ErrNotExist))
}
