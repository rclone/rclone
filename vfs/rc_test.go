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

	// Test direct ID lookup
	vfs, err = getVFS(rc.Params{"fs": vfs2.ID})
	require.NoError(t, err)
	assert.True(t, vfs == vfs2)

	// Test ID suffix lookup
	vfs, err = getVFS(rc.Params{"fs": fs.ConfigString(r.Fremote) + "[" + vfs2.ID + "]"})
	require.NoError(t, err)
	assert.True(t, vfs == vfs2)

	inWrong := rc.Params{"fs": fs.ConfigString(r.Fremote) + "notfound"}
	vfs, err = getVFS(inWrong)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no VFS found with name or ID")
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

	// Test lookups with multiple VFSes active
	vfs, err = getVFS(rc.Params{"fs": vfs2.ID})
	require.NoError(t, err)
	assert.True(t, vfs == vfs2)

	vfs, err = getVFS(rc.Params{"fs": vfs3.ID})
	require.NoError(t, err)
	assert.True(t, vfs == vfs3)

	vfs, err = getVFS(rc.Params{"fs": fs.ConfigString(r.Fremote) + "[" + vfs3.ID + "]"})
	require.NoError(t, err)
	assert.True(t, vfs == vfs3)
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
	r, vfs2, call := rcNewRun(t, "vfs/list")

	out, err := call.Fn(context.Background(), nil)
	require.NoError(t, err)

	assert.Equal(t, []string{fs.ConfigString(r.Fremote)}, out["vfses"])
	assert.Equal(t, []Info{
		{ID: vfs2.ID, Fs: fs.ConfigString(r.Fremote)},
	}, out["list"])

	// Add a second VFS
	opt := vfscommon.Opt
	opt.NoModTime = true
	vfs3 := New(context.Background(), r.Fremote, &opt)
	defer vfs3.Shutdown()

	out, err = call.Fn(context.Background(), nil)
	require.NoError(t, err)

	vfses := out["vfses"].([]string)
	assert.Contains(t, vfses, fs.ConfigString(r.Fremote)+"["+vfs2.ID+"]")
	assert.Contains(t, vfses, fs.ConfigString(r.Fremote)+"["+vfs3.ID+"]")

	list := out["list"].([]Info)
	assert.Len(t, list, 2)
	assert.Contains(t, list, Info{ID: vfs2.ID, Fs: fs.ConfigString(r.Fremote)})
	assert.Contains(t, list, Info{ID: vfs3.ID, Fs: fs.ConfigString(r.Fremote)})
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
