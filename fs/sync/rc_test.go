package sync

import (
	"context"
	"sort"
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
	assert.Equal(t, rc.Params{}, out)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file2, file3)
}

// sync/move: move a directory from source remote to destination remote
func TestRcMove(t *testing.T) {
	r, call := rcNewRun(t, "sync/move")
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
	assert.Equal(t, rc.Params{}, out)

	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file1, file2, file3)
}

// sync/sync: sync a directory from source remote to destination remote
func TestRcSync(t *testing.T) {
	r, call := rcNewRun(t, "sync/sync")
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
	assert.Equal(t, rc.Params{}, out)

	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file2)
}

// sync/copy: check the reports are returned when requested
func TestRcCopyReports(t *testing.T) {
	r, call := rcNewRun(t, "sync/copy")
	r.Mkdir(context.Background(), r.Fremote)

	file1 := r.WriteBoth(context.Background(), "file1", "file1 contents", t1)
	file2 := r.WriteFile("subdir/file2", "file2 contents", t2)
	file3 := r.WriteObject(context.Background(), "subdir/subsubdir/file3", "file3 contents", t3)
	file4 := r.WriteFile("file4", "file4 contents", t1)
	file4dst := r.WriteObject(context.Background(), "file4", "different contents", t1)

	r.CheckLocalItems(t, file1, file2, file4)
	r.CheckRemoteItems(t, file1, file3, file4dst)

	in := rc.Params{
		"srcFs":        r.LocalName,
		"dstFs":        r.FremoteName,
		"combined":     true,
		"missingOnSrc": true,
		"missingOnDst": true,
		"match":        true,
		"differ":       true,
		"destAfter":    true,
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)

	sorted := func(name string) []string {
		result, ok := out[name].(*[]string)
		require.True(t, ok, name)
		sort.Strings(*result)
		return *result
	}
	assert.Equal(t, []string{"* file4", "+ subdir/file2", "- subdir/subsubdir/file3", "= file1"}, sorted("combined"))
	assert.Equal(t, []string{"subdir/subsubdir/file3"}, sorted("missingOnSrc"))
	assert.Equal(t, []string{"subdir/file2"}, sorted("missingOnDst"))
	assert.Equal(t, []string{"file1"}, sorted("match"))
	assert.Equal(t, []string{"file4"}, sorted("differ"))
	assert.Equal(t, []string{"file1", "file4", "subdir/file2", "subdir/subsubdir/file3"}, sorted("destAfter"))
	assert.NotContains(t, out, "error")

	r.CheckLocalItems(t, file1, file2, file4)
	r.CheckRemoteItems(t, file1, file2, file3, file4)
}
