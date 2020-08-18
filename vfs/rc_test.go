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

func rcNewRun(t *testing.T, method string) (r *fstest.Run, vfs *VFS, cleanup func(), call *rc.Call) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	r, vfs, cleanup = newTestVFS(t)
	call = rc.Calls.Get(method)
	assert.NotNil(t, call)
	return r, vfs, cleanup, call
}

func TestRcGetVFS(t *testing.T) {
	in := rc.Params{}
	vfs, err := getVFS(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no VFS active")
	assert.Nil(t, vfs)

	r, vfs2, cleanup := newTestVFS(t)
	defer cleanup()

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

	opt := vfscommon.DefaultOpt
	opt.NoModTime = true
	vfs3 := New(r.Fremote, &opt)
	defer vfs3.Shutdown()

	vfs, err = getVFS(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than one VFS active - need")
	assert.Nil(t, vfs)

	vfs, err = getVFS(inPresent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than one VFS active with name")
	assert.Nil(t, vfs)
}

func TestRcForget(t *testing.T) {
	r, vfs, cleanup, call := rcNewRun(t, "vfs/forget")
	defer cleanup()
	_, _ = r, vfs
	out, err := call.Fn(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"forgotten": []string{},
	}, out)
	// FIXME needs more tests
}

func TestRcRefresh(t *testing.T) {
	r, vfs, cleanup, call := rcNewRun(t, "vfs/refresh")
	defer cleanup()
	_, _ = r, vfs
	out, err := call.Fn(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"result": map[string]string{
			"": "OK",
		},
	}, out)
	// FIXME needs more tests
}

func TestRcPollInterval(t *testing.T) {
	r, vfs, cleanup, call := rcNewRun(t, "vfs/poll-interval")
	defer cleanup()
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
	r, vfs, cleanup, call := rcNewRun(t, "vfs/list")
	defer cleanup()
	_ = vfs

	out, err := call.Fn(context.Background(), nil)
	require.NoError(t, err)

	assert.Equal(t, rc.Params{
		"vfses": []string{
			fs.ConfigString(r.Fremote),
		},
	}, out)
}
