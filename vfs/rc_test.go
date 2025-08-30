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

	"io"
	"os"
	"time"
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

// newTestVFS is defined in vfs_test.go and returns (r *fstest.Run, vfs *VFS, cleanup func())

// TestRcFileAndDirStatus verifies that the vfs/file-status and vfs/dir-status
// rc calls work as expected.
//
// This test performs the following steps:
// 1. Tests vfs/file-status for an UNCACHED file:
//   - Creates a file in the remote.
//   - Calls the rc command "vfs/file-status" for the uncached file.
//   - Verifies that the returned FileStatus has status "uncached", 0 percentage, and the correct size.
//
// 2. Opens the file to cache it:
//   - Opens the file in read-only mode.
//   - Reads the entire file to ensure it's fully cached.
//   - Closes the file.
//
// 3. Tests vfs/file-status for a CACHED file:
//   - Calls the rc command "vfs/file-status" again.
//   - Verifies that the returned FileStatus has status "cached", 100 percentage, and the correct cached size.
//
// 4. Tests vfs/dir-status:
//   - Calls the rc command "vfs/dir-status" for the root directory.
//   - Verifies that the returned DirStatusList contains one element.
//   - Verifies that the element's name matches the filename and its status is "cached".
func TestRcFileAndDirStatus(t *testing.T) {
	// Create a test file
	filePath := "test_file.txt"
	fileContent := "hello world"

	// 1. Test vfs/file-status for an UNCACHED file
	//   - Create a file in the remote
	//   - Test the rc call "vfs/file-status" for an uncached file
	//   - Verify it returns FileStatus with status "uncached", percentage 0, and size of the file
	r, vfs := newTestVFS(t)
	require.NotNil(t, r)
	require.NotNil(t, vfs)
	defer vfs.CleanUp()

	r.WriteFile(filePath, fileContent, time.Now())

	callOpt := rc.Params{
		"fs":   fs.ConfigString(r.Fremote),
		"path": filePath,
	}
	call := rc.Calls.Get("vfs/file-status")
	require.NotNil(t, call)

	res, err := call.Fn(context.Background(), callOpt)
	require.NoError(t, err)

	var fileStatus vfscommon.FileStatus
	rawStatus := res["status"].(map[string]interface{})
	fileStatus.Name = rawStatus["name"].(string)
	fileStatus.Path = rawStatus["path"].(string)
	fileStatus.Status = rawStatus["status"].(string)
	fileStatus.Percentage = int(rawStatus["percentage"].(float64))
	fileStatus.Size = int64(rawStatus["size"].(float64))
	fileStatus.CachedSize = int64(rawStatus["cachedSize"].(float64))
	assert.Equal(t, vfscommon.StatusUncached, fileStatus.Status)
	assert.Equal(t, 0, fileStatus.Percentage)
	assert.Equal(t, int64(len(fileContent)), fileStatus.Size)

	// 2. Open the file to cache it
	//   - Open the file in read only mode
	//   - Read the entire file to ensure it's fully cached
	//   - Close the file
	fd, err := vfs.OpenFile(filePath, os.O_RDONLY, 0777)
	require.NoError(t, err)

	_, err = io.ReadAll(fd)
	require.NoError(t, err)
	require.NoError(t, fd.Close())

	// 3. Test vfs/file-status for a CACHED file
	//   - Test the rc call "vfs/file-status" again
	//   - Verify it returns FileStatus with status "cached", percentage 100, and size of the file
	res, err = call.Fn(context.Background(), callOpt)
	require.NoError(t, err)

	rawStatus = res["status"].(map[string]interface{})
	fileStatus.Name = rawStatus["name"].(string)
	fileStatus.Path = rawStatus["path"].(string)
	fileStatus.Status = rawStatus["status"].(string)
	fileStatus.Percentage = int(rawStatus["percentage"].(float64))
	fileStatus.Size = int64(rawStatus["size"].(float64))
	fileStatus.CachedSize = int64(rawStatus["cachedSize"].(float64))
	assert.Equal(t, vfscommon.StatusCached, fileStatus.Status)
	assert.Equal(t, 100, fileStatus.Percentage)
	assert.Equal(t, int64(len(fileContent)), fileStatus.CachedSize)

	// 4. Test vfs/dir-status
	//   - Test the rc call "vfs/dir-status" for the root directory
	//   - Verify it returns a slice of DirStatus with one element
	//   - Verify the element has name equal to the filename and status "cached"
	dirPath := ""
	dirCallOpt := rc.Params{"fs": fs.ConfigString(r.Fremote), "dir": dirPath}
	dirCall := rc.Calls.Get("vfs/dir-status")
	require.NotNil(t, dirCall)
	res, err = dirCall.Fn(context.Background(), dirCallOpt)
	require.NoError(t, err)

	var dirStatus vfscommon.DirStatusList
	rawDirStatusList := res["dirStatus"].([]interface{})
	for _, item := range rawDirStatusList {
		rawItem := item.(map[string]interface{})
		var fs vfscommon.FileStatus
		fs.Name = rawItem["name"].(string)
		fs.Path = rawItem["path"].(string)
		fs.Status = rawItem["status"].(string)
		fs.Percentage = int(rawItem["percentage"].(float64))
		fs.Size = int64(rawItem["size"].(float64))
		fs.CachedSize = int64(rawItem["cachedSize"].(float64))
		dirStatus = append(dirStatus, fs)
	}

	require.Equal(t, 1, len(dirStatus))
	assert.Equal(t, filePath, dirStatus[0].Name)
	assert.Equal(t, vfscommon.StatusCached, dirStatus[0].Status)
}
