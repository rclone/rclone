package vfs

import (
	"context"
	"io"
	"os"
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

// TestRcFileAndDirStatus tests the vfs/file-status and vfs/dir-status
// commandss
//
// This test case will cover the following scenarios:
// - Test file-status command for uncached file
// - Test file-status command for cached file
// - Test dir-status command for a directory with cached file
func TestRcFileAndDirStatus(t *testing.T) {
	// --- Setup Compartilhado ---
	filePath := "test_file.txt"
	fileContent := "hello world"

	r, vfs := newTestVFS(t)
	require.NotNil(t, r)
	require.NotNil(t, vfs)
	defer vfs.CleanUp()

	r.WriteFile(filePath, fileContent, time.Now())
	time.Sleep(100 * time.Millisecond)

	mapToFileStatus := func(t *testing.T, data interface{}) vfscommon.FileStatus {
		rawStatus, ok := data.(map[string]interface{})
		require.True(t, ok, "A resposta da API não é um mapa válido")

		var status vfscommon.FileStatus
		status.Name = rawStatus["name"].(string)
		status.Path = rawStatus["path"].(string)
		status.Status = rawStatus["status"].(string)
		status.Percentage = int(rawStatus["percentage"].(float64))
		status.Size = int64(rawStatus["size"].(float64))
		status.CachedSize = int64(rawStatus["cachedSize"].(float64))
		return status
	}

	t.Run("FileStatusUncached", func(t *testing.T) {
		// Check if the file exists in VFS before calling the RC command
		node, err := vfs.Stat(filePath)
		require.NoError(t, err, "File should exist in VFS")
		require.True(t, node.IsFile(), "Node should be a file")

		callOpt := rc.Params{"fs": fs.ConfigString(r.Fremote), "path": filePath}
		call := rc.Calls.Get("vfs/file-status")
		require.NotNil(t, call)

		res, err := call.Fn(context.Background(), callOpt)
		require.NoError(t, err)

		fileStatus := mapToFileStatus(t, res["status"])

		assert.Equal(t, "test_file.txt", fileStatus.Name)
		assert.Equal(t, vfscommon.StatusUncached, fileStatus.Status)
		assert.Equal(t, 0, fileStatus.Percentage)
		assert.Equal(t, int64(len(fileContent)), fileStatus.Size)
	})

	fd, err := vfs.OpenFile(filePath, os.O_RDONLY, 0777)
	require.NoError(t, err)
	_, err = io.ReadAll(fd)
	require.NoError(t, err)
	require.NoError(t, fd.Close())

	t.Run("FileStatusCached", func(t *testing.T) {
		callOpt := rc.Params{"fs": fs.ConfigString(r.Fremote), "path": filePath}
		call := rc.Calls.Get("vfs/file-status")
		require.NotNil(t, call)

		res, err := call.Fn(context.Background(), callOpt)
		require.NoError(t, err)

		fileStatus := mapToFileStatus(t, res["status"])

		assert.Equal(t, vfscommon.StatusCached, fileStatus.Status)
		assert.Equal(t, 100, fileStatus.Percentage)
		assert.Equal(t, int64(len(fileContent)), fileStatus.CachedSize)
	})

	t.Run("DirStatusWithCachedFile", func(t *testing.T) {
		dirCallOpt := rc.Params{"fs": fs.ConfigString(r.Fremote), "dir": ""}
		dirCall := rc.Calls.Get("vfs/dir-status")
		require.NotNil(t, dirCall)

		res, err := dirCall.Fn(context.Background(), dirCallOpt)
		require.NoError(t, err)

		rawDirStatusList, ok := res["dirStatus"].([]interface{})
		require.True(t, ok)

		var dirStatus vfscommon.DirStatusList
		for _, item := range rawDirStatusList {
			dirStatus = append(dirStatus, mapToFileStatus(t, item))
		}

		require.Equal(t, 1, len(dirStatus))
		assert.Equal(t, filePath, dirStatus[0].Name)
		assert.Equal(t, vfscommon.StatusCached, dirStatus[0].Status)
	})
}
