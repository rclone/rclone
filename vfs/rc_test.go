package vfs

import (
	"context"
	"testing"
	"time"

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

func TestRcPollIntervalShutdown(t *testing.T) {
	r := fstest.NewRun(t)
	features := r.Fremote.Features()
	originalChangeNotify := features.ChangeNotify
	t.Cleanup(func() {
		features.ChangeNotify = originalChangeNotify
	})

	initialIntervalReceived := make(chan struct{})
	features.ChangeNotify = func(_ context.Context, _ func(string, fs.EntryType), pollInterval <-chan time.Duration) {
		go func() {
			<-pollInterval
			close(initialIntervalReceived)
		}()
	}

	vfs := New(context.Background(), r.Fremote, nil)
	t.Cleanup(func() {
		if vfs.inUse.Load() > 0 {
			vfs.Shutdown()
		}
	})
	<-initialIntervalReceived

	call := rc.Calls.Get("vfs/poll-interval")
	require.NotNil(t, call)
	originalInterval := vfs.Opt.PollInterval

	type result struct {
		out rc.Params
		err error
	}
	resultCh := make(chan result, 1)
	go func() {
		out, err := call.Fn(context.Background(), rc.Params{
			"fs":       fs.ConfigString(r.Fremote),
			"interval": "1h",
		})
		resultCh <- result{out: out, err: err}
	}()

	select {
	case got := <-resultCh:
		t.Fatalf("poll interval update returned before shutdown: out=%v err=%v", got.out, got.err)
	case <-time.After(100 * time.Millisecond):
	}

	shutdownDone := make(chan struct{})
	go func() {
		vfs.Shutdown()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("VFS shutdown blocked behind poll interval update")
	}

	select {
	case got := <-resultCh:
		require.EqualError(t, got.err, "VFS is shutting down")
		assert.Nil(t, got.out)
	case <-time.After(time.Second):
		t.Fatal("poll interval update did not return after shutdown")
	}
	assert.Equal(t, originalInterval, vfs.Opt.PollInterval)
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
