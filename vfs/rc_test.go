package vfs

import (
	"context"
	"runtime"
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

func newTestPollVFS(t *testing.T, changeNotify func(context.Context, func(string, fs.EntryType), <-chan time.Duration)) (*fstest.Run, *VFS, *rc.Call) {
	t.Helper()
	r := fstest.NewRun(t)
	features := r.Fremote.Features()
	originalChangeNotify := features.ChangeNotify
	features.ChangeNotify = changeNotify
	t.Cleanup(func() {
		features.ChangeNotify = originalChangeNotify
	})
	vfs := New(context.Background(), r.Fremote, nil)
	t.Cleanup(func() {
		if vfs.inUse.Load() > 0 {
			vfs.Shutdown()
		}
	})
	call := rc.Calls.Get("vfs/poll-interval")
	require.NotNil(t, call)
	return r, vfs, call
}

func waitForPollLock(t *testing.T, vfs *VFS) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if vfs.pollMu.TryLock() {
			vfs.pollMu.Unlock()
			runtime.Gosched()
			continue
		}
		return
	}
	t.Fatal("poll interval update did not acquire poll lock")
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
	initialIntervalReceived := make(chan struct{})
	r, vfs, call := newTestPollVFS(t, func(_ context.Context, _ func(string, fs.EntryType), pollInterval <-chan time.Duration) {
		go func() {
			<-pollInterval
			close(initialIntervalReceived)
		}()
	})
	<-initialIntervalReceived

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

	waitForPollLock(t, vfs)

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

func TestSetPollIntervalAfterShutdown(t *testing.T) {
	initialIntervalReceived := make(chan struct{})
	_, vfs, _ := newTestPollVFS(t, func(_ context.Context, _ func(string, fs.EntryType), pollInterval <-chan time.Duration) {
		go func() {
			<-pollInterval
			close(initialIntervalReceived)
			for range pollInterval {
			}
		}()
	})
	<-initialIntervalReceived
	vfs.Shutdown()

	timeoutHit, err := setPollInterval(vfs, time.Hour, 0)
	require.EqualError(t, err, "VFS is shutting down")
	assert.False(t, timeoutHit)
}

func TestRcPollIntervalUpdate(t *testing.T) {
	intervals := make(chan time.Duration, 2)
	r, vfs, call := newTestPollVFS(t, func(_ context.Context, _ func(string, fs.EntryType), pollInterval <-chan time.Duration) {
		go func() {
			for interval := range pollInterval {
				intervals <- interval
			}
		}()
	})

	assert.Equal(t, time.Duration(vfs.Opt.PollInterval), <-intervals)
	status, err := call.Fn(context.Background(), rc.Params{
		"fs": fs.ConfigString(r.Fremote),
	})
	require.NoError(t, err)
	assert.Equal(t, true, status["supported"])
	assert.Equal(t, vfs.Opt.PollInterval != 0, status["enabled"])

	out, err := call.Fn(context.Background(), rc.Params{
		"fs":       fs.ConfigString(r.Fremote),
		"interval": "1h",
	})
	require.NoError(t, err)
	assert.Equal(t, time.Hour, <-intervals)
	assert.Equal(t, fs.Duration(time.Hour), vfs.Opt.PollInterval)
	assert.Equal(t, false, out["timeout"])
}

func TestRcPollIntervalTimeout(t *testing.T) {
	initialIntervalReceived := make(chan struct{})
	r, vfs, call := newTestPollVFS(t, func(_ context.Context, _ func(string, fs.EntryType), pollInterval <-chan time.Duration) {
		go func() {
			<-pollInterval
			close(initialIntervalReceived)
		}()
	})
	<-initialIntervalReceived
	originalInterval := vfs.Opt.PollInterval

	out, err := call.Fn(context.Background(), rc.Params{
		"fs":       fs.ConfigString(r.Fremote),
		"interval": "1h",
		"timeout":  "10ms",
	})
	require.NoError(t, err)
	assert.Equal(t, true, out["timeout"])
	assert.Equal(t, originalInterval, vfs.Opt.PollInterval)
}

func TestRcPollIntervalUnsupported(t *testing.T) {
	r, vfs, call := newTestPollVFS(t, nil)
	out, err := call.Fn(context.Background(), rc.Params{
		"fs":       fs.ConfigString(r.Fremote),
		"interval": "1h",
	})
	require.EqualError(t, err, "poll-interval is not supported by this remote")
	assert.Nil(t, out)
	assert.Nil(t, vfs.pollChan)
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
