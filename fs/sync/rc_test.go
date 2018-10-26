package sync

import (
	"testing"

	"github.com/ncw/rclone/fs/rc"
	"github.com/ncw/rclone/fstest"
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
	rc.PutCachedFs(r.LocalName, r.Flocal)
	rc.PutCachedFs(r.FremoteName, r.Fremote)
	return r, call
}

// sync/copy: copy a directory from source remote to destination remote
func TestRcCopy(t *testing.T) {
	r, call := rcNewRun(t, "sync/copy")
	defer r.Finalise()
	r.Mkdir(r.Fremote)

	file1 := r.WriteBoth("file1", "file1 contents", t1)
	file2 := r.WriteFile("subdir/file2", "file2 contents", t2)
	file3 := r.WriteObject("subdir/subsubdir/file3", "file3 contents", t3)

	fstest.CheckItems(t, r.Flocal, file1, file2)
	fstest.CheckItems(t, r.Fremote, file1, file3)

	in := rc.Params{
		"srcFs": r.LocalName,
		"dstFs": r.FremoteName,
	}
	out, err := call.Fn(in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	fstest.CheckItems(t, r.Flocal, file1, file2)
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)
}

// sync/move: move a directory from source remote to destination remote
func TestRcMove(t *testing.T) {
	r, call := rcNewRun(t, "sync/move")
	defer r.Finalise()
	r.Mkdir(r.Fremote)

	file1 := r.WriteBoth("file1", "file1 contents", t1)
	file2 := r.WriteFile("subdir/file2", "file2 contents", t2)
	file3 := r.WriteObject("subdir/subsubdir/file3", "file3 contents", t3)

	fstest.CheckItems(t, r.Flocal, file1, file2)
	fstest.CheckItems(t, r.Fremote, file1, file3)

	in := rc.Params{
		"srcFs": r.LocalName,
		"dstFs": r.FremoteName,
	}
	out, err := call.Fn(in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)
}

// sync/sync: sync a directory from source remote to destination remote
func TestRcSync(t *testing.T) {
	r, call := rcNewRun(t, "sync/sync")
	defer r.Finalise()
	r.Mkdir(r.Fremote)

	file1 := r.WriteBoth("file1", "file1 contents", t1)
	file2 := r.WriteFile("subdir/file2", "file2 contents", t2)
	file3 := r.WriteObject("subdir/subsubdir/file3", "file3 contents", t3)

	fstest.CheckItems(t, r.Flocal, file1, file2)
	fstest.CheckItems(t, r.Fremote, file1, file3)

	in := rc.Params{
		"srcFs": r.LocalName,
		"dstFs": r.FremoteName,
	}
	out, err := call.Fn(in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	fstest.CheckItems(t, r.Flocal, file1, file2)
	fstest.CheckItems(t, r.Fremote, file1, file2)
}
