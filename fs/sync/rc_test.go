package sync

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rcNewRun(t *testing.T, method string) (*fstest.Run, *rc.Call) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	r := fstest.NewRun(t)
	call := rc.Calls.Get(method)
	assert.NotNil(t, call)
	cache.Put(r.LocalName, r.Flocal)
	cache.Put(r.FremoteName, r.Fremote)
	return r, call
}

// sync/copy: copy a directory from source remote to destination remote
func TestRcCopy(t *testing.T) {
	r, call := rcNewRun(t, "sync/copy")
	defer r.Finalise()
	r.Mkdir(context.Background(), r.Fremote)

	file1 := r.WriteBoth(context.Background(), "file1", "file1 contents", t1)
	file2 := r.WriteFile("subdir/file2", "file2 contents", t2)
	file3 := r.WriteObject(context.Background(), "subdir/subsubdir/file3", "file3 contents", t3)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file3)

	in := rc.Params{
		"srcFs": r.LocalName,
		"dstFs": r.FremoteName,
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file2, file3)
}

// sync/move: move a directory from source remote to destination remote
func TestRcMove(t *testing.T) {
	r, call := rcNewRun(t, "sync/move")
	defer r.Finalise()
	r.Mkdir(context.Background(), r.Fremote)

	file1 := r.WriteBoth(context.Background(), "file1", "file1 contents", t1)
	file2 := r.WriteFile("subdir/file2", "file2 contents", t2)
	file3 := r.WriteObject(context.Background(), "subdir/subsubdir/file3", "file3 contents", t3)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file3)

	in := rc.Params{
		"srcFs": r.LocalName,
		"dstFs": r.FremoteName,
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file1, file2, file3)
}

// sync/sync: sync a directory from source remote to destination remote
func TestRcSync(t *testing.T) {
	r, call := rcNewRun(t, "sync/sync")
	defer r.Finalise()
	r.Mkdir(context.Background(), r.Fremote)

	file1 := r.WriteBoth(context.Background(), "file1", "file1 contents", t1)
	file2 := r.WriteFile("subdir/file2", "file2 contents", t2)
	file3 := r.WriteObject(context.Background(), "subdir/subsubdir/file3", "file3 contents", t3)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file3)

	in := rc.Params{
		"srcFs": r.LocalName,
		"dstFs": r.FremoteName,
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file2)
}
