package vfs

import (
	"context"
	"os"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestRCStatus(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	ctx := context.Background()

	file, err := vfs.OpenFile("test.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file.Write([]byte("test content"))
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	statusCall := rc.Calls.Get("vfs/status")
	require.NotNil(t, statusCall)

	result, err := statusCall.Fn(context.Background(), rc.Params{
		"fs": fs.ConfigString(r.Fremote),
	})
	require.NoError(t, err)

	assert.Contains(t, result, "totalFiles")
	assert.Contains(t, result, "totalCachedBytes")
	assert.Contains(t, result, "averageCachePercentage")
	assert.Contains(t, result, "counts")

	counts, ok := result["counts"].(rc.Params)
	require.True(t, ok)
	assert.Contains(t, counts, "FULL")
	assert.Contains(t, counts, "PARTIAL")
	assert.Contains(t, counts, "NONE")
	assert.Contains(t, counts, "DIRTY")
	assert.Contains(t, counts, "UPLOADING")
	assert.Contains(t, counts, "ERROR")

	if n, ok := getInt(result["totalFiles"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
	} else {
		require.FailNow(t, "totalFiles has unexpected type")
	}

	if n, ok := getInt(result["averageCachePercentage"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
		assert.LessOrEqual(t, n, int64(100))
	} else {
		require.FailNow(t, "averageCachePercentage has unexpected type")
	}
}

func TestRCStatus_CacheDisabled(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeOff

	vfs := New(r.Fremote, &opt)
	defer vfs.Shutdown()

	prev := snapshotAndClearActiveCache(t)
	addToActiveCache(vfs)

	statusCall := rc.Calls.Get("vfs/status")
	require.NotNil(t, statusCall)

	result, err := statusCall.Fn(context.Background(), rc.Params{
		"fs": fs.ConfigString(r.Fremote),
	})
	require.NoError(t, err)

	assert.Contains(t, result, "totalFiles")
	assert.Equal(t, 0, result["totalFiles"])

	counts, ok := result["counts"].(rc.Params)
	require.True(t, ok)
	for _, status := range []string{"FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING", "ERROR"} {
		assert.Equal(t, 0, counts[status], "status %s should be 0", status)
	}
}

func TestRCFileStatus(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	ctx := context.Background()

	file, err := vfs.OpenFile("test.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file.Write([]byte("test content"))
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	fileStatusCall := rc.Calls.Get("vfs/file-status")
	require.NotNil(t, fileStatusCall)

	result, err := fileStatusCall.Fn(context.Background(), rc.Params{
		"fs":   fs.ConfigString(r.Fremote),
		"file": "test.txt",
	})
	require.NoError(t, err)

	assert.Contains(t, result, "files")
	files, ok := result["files"].([]interface{})
	require.True(t, ok)
	assert.Len(t, files, 1)

	file := files[0].(rc.Params)
	assert.Contains(t, file, "name")
	assert.Contains(t, file, "status")
	assert.Contains(t, file, "percentage")
	assert.Contains(t, file, "size")
	assert.Contains(t, file, "cachedBytes")
	assert.Contains(t, file, "dirty")
	assert.Contains(t, file, "uploading")

	if n, ok := getInt(file["percentage"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
		assert.LessOrEqual(t, n, int64(100))
	} else {
		require.FailNow(t, "percentage has unexpected type")
	}
}

func TestRCFileStatus_MultipleFiles(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	ctx := context.Background()

	file1, err := vfs.OpenFile("file1.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file1.Write([]byte("content 1"))
	require.NoError(t, err)
	err = file1.Close()
	require.NoError(t, err)

	file2, err := vfs.OpenFile("file2.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file2.Write([]byte("content 2"))
	require.NoError(t, err)
	err = file2.Close()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	fileStatusCall := rc.Calls.Get("vfs/file-status")
	require.NotNil(t, fileStatusCall)

	result, err := fileStatusCall.Fn(context.Background(), rc.Params{
		"fs":    fs.ConfigString(r.Fremote),
		"file":   "file1.txt",
		"file1": "file2.txt",
		"file2": "nonexistent.txt",
	})
	require.NoError(t, err)

	assert.Contains(t, result, "files")
	files, ok := result["files"].([]interface{})
	require.True(t, ok)
	assert.Len(t, files, 3)

	file := files[2].(rc.Params)
	assert.Equal(t, "ERROR", file["status"])
	assert.Contains(t, file, "error")
}

func TestRCFileStatus_InvalidPath(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	ctx := context.Background()

	file, err := vfs.OpenFile("test.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file.Write([]byte("test content"))
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	fileStatusCall := rc.Calls.Get("vfs/file-status")
	require.NotNil(t, fileStatusCall)

	result, err := fileStatusCall.Fn(context.Background(), rc.Params{
		"fs":   fs.ConfigString(r.Fremote),
		"file": "nonexistent.txt",
	})
	require.NoError(t, err)

	assert.Contains(t, result, "files")
	files, ok := result["files"].([]interface{})
	require.True(t, ok)
	assert.Len(t, files, 1)

	file := files[0].(rc.Params)
	assert.Equal(t, "ERROR", file["status"])
	assert.Contains(t, file, "error")
}

func TestRCFileStatus_EmptyPath(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	fileStatusCall := rc.Calls.Get("vfs/file-status")
	require.NotNil(t, fileStatusCall)

	_, err := fileStatusCall.Fn(context.Background(), rc.Params{
		"fs":   fs.ConfigString(r.Fremote),
		"file": "",
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "empty")
}

func TestRCFileStatus_NoFiles(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	fileStatusCall := rc.Calls.Get("vfs/file-status")
	require.NotNil(t, fileStatusCall)

	_, err := fileStatusCall.Fn(context.Background(), rc.Params{
		"fs": fs.ConfigString(r.Fremote),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no file parameter")
}

func TestRCFileStatus_TooManyFiles(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	ctx := context.Background()

	file, err := vfs.OpenFile("test.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file.Write([]byte("test content"))
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	params := rc.Params{"fs": fs.ConfigString(r.Fremote), "file": "test.txt"}
	for i := 1; i <= 110; i++ {
		key := "file" + string(rune('0'+i))
		params[key] = "test.txt"
	}

	fileStatusCall := rc.Calls.Get("vfs/file-status")
	require.NotNil(t, fileStatusCall)

	_, err := fileStatusCall.Fn(context.Background(), params)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "too many")
}


func TestRCDirStatus(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	ctx := context.Background()

	err := vfs.Mkdir("testdir", 0755)
	require.NoError(t, err)

	file1, err := vfs.OpenFile("testdir/file1.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file1.Write([]byte("content 1"))
	require.NoError(t, err)
	err = file1.Close()
	require.NoError(t, err)

	file2, err := vfs.OpenFile("testdir/file2.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file2.Write([]byte("content 2"))
	require.NoError(t, err)
	err = file2.Close()
	require.NoError(t, err)

	err = vfs.Mkdir("testdir/subdir", 0755)
	require.NoError(t, err)

	file3, err := vfs.OpenFile("testdir/subdir/file3.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file3.Write([]byte("content 3"))
	require.NoError(t, err)
	err = file3.Close()
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	dirStatusCall := rc.Calls.Get("vfs/dir-status")
	require.NotNil(t, dirStatusCall)

	result, err := dirStatusCall.Fn(context.Background(), rc.Params{
		"fs":  fs.ConfigString(r.Fremote),
		"dir": "testdir",
	})
	require.NoError(t, err)

	assert.Contains(t, result, "dir")
	assert.Contains(t, result, "files")
	assert.Contains(t, result, "fs")

	assert.Equal(t, "testdir", result["dir"])

	files, ok := result["files"].(rc.Params)
	require.True(t, ok)

	for _, status := range []string{"FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING", "ERROR"} {
		assert.Contains(t, files, status, "files should contain status %s", status)
	}

	totalFiles := 0
	for _, status := range []string{"FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING", "ERROR"} {
		statusFiles, ok := files[status].([]interface{})
		if ok {
			totalFiles += len(statusFiles)
		}
	}

	assert.GreaterOrEqual(t, totalFiles, 3, "should have at least 3 files in testdir")
}

func TestRCDirStatus_Recursive(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	ctx := context.Background()

	err := vfs.Mkdir("testdir", 0755)
	require.NoError(t, err)

	file1, err := vfs.OpenFile("testdir/file1.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file1.Write([]byte("content 1"))
	require.NoError(t, err)
	err = file1.Close()
	require.NoError(t, err)

	file2, err := vfs.OpenFile("testdir/file2.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file2.Write([]byte("content 2"))
	require.NoError(t, err)
	err = file2.Close()
	require.NoError(t, err)

	err = vfs.Mkdir("testdir/subdir", 0755)
	require.NoError(t, err)

	file3, err := vfs.OpenFile("testdir/subdir/file3.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file3.Write([]byte("content 3"))
	require.NoError(t, err)
	err = file3.Close()
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	dirStatusCall := rc.Calls.Get("vfs/dir-status")
	require.NotNil(t, dirStatusCall)

	result, err := dirStatusCall.Fn(context.Background(), rc.Params{
		"fs":        fs.ConfigString(r.Fremote),
		"dir":       "testdir",
		"recursive": true,
	})
	require.NoError(t, err)

	assert.Contains(t, result, "dir")
	assert.Contains(t, result, "files")
	assert.Contains(t, result, "recursive")
	assert.Contains(t, result, "fs")

	assert.Equal(t, "testdir", result["dir"])

	files, ok := result["files"].(rc.Params)
	require.True(t, ok)

	for _, status := range []string{"FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING", "ERROR"} {
		assert.Contains(t, files, status, "files should contain status %s", status)
	}

	totalFiles := 0
	for _, status := range []string{"FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING", "ERROR"} {
		statusFiles, ok := files[status].([]interface{})
		if ok {
			totalFiles += len(statusFiles)
		}
	}

	assert.Equal(t, 3, totalFiles, "should have 3 files in testdir with recursive=true")
}

func TestRCDirStatus_NonExistentDirectory(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	dirStatusCall := rc.Calls.Get("vfs/dir-status")
	require.NotNil(t, dirStatusCall)

	result, err := dirStatusCall.Fn(context.Background(), rc.Params{
		"fs": fs.ConfigString(r.Fremote),
		"dir": "nonexistent",
	})
	require.NoError(t, err)

	assert.Contains(t, result, "dir")
	assert.Contains(t, result, "files")
	assert.Contains(t, result, "fs")

	assert.Equal(t, "nonexistent", result["dir"])

	files, ok := result["files"].(rc.Params)
	require.True(t, ok)

	totalFiles := 0
	for _, status := range []string{"FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING", "ERROR"} {
		statusFiles, ok := files[status].([]interface{})
		if ok {
			totalFiles += len(statusFiles)
		}
	}

	assert.Equal(t, 0, totalFiles, "nonexistent directory should have 0 files")
}

func TestRCDirStatus_Root(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	ctx := context.Background()

	file1, err := vfs.OpenFile("file1.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file1.Write([]byte("content 1"))
	require.NoError(t, err)
	err = file1.Close()
	require.NoError(t, err)

	file2, err := vfs.OpenFile("file2.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file2.Write([]byte("content 2"))
	require.NoError(t, err)
	err = file2.Close()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	dirStatusCall := rc.Calls.Get("vfs/dir-status")
	require.NotNil(t, dirStatusCall)

	result, err := dirStatusCall.Fn(context.Background(), rc.Params{
		"fs": fs.ConfigString(r.Fremote),
	})
	require.NoError(t, err)

	assert.Contains(t, result, "dir")
	assert.Contains(t, result, "files")
	assert.Contains(t, result, "fs")

	assert.Equal(t, "", result["dir"], "root directory should be empty string")

	files, ok := result["files"].(rc.Params)
	require.True(t, ok)

	totalFiles := 0
	for _, status := range []string{"FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING", "ERROR"} {
		statusFiles, ok := files[status].([]interface{})
		if ok {
			totalFiles += len(statusFiles)
		}
	}

	assert.Equal(t, 2, totalFiles, "root directory should have 2 files")
}

func TestRCFileStatus_Lifecycle(t *testing.T) {
	r, vfs := newTestVFS(t)
	defer cleanupVFS(t, r, vfs)

	clearActiveCache()
	addToActiveCache(vfs)

	fileStatusCall := rc.Calls.Get("vfs/file-status")
	require.NotNil(t, fileStatusCall)

	ctx := context.Background()

	result1, err := fileStatusCall.Fn(context.Background(), rc.Params{
		"fs":   fs.ConfigString(r.Fremote),
		"file": "lifecycle.txt",
	})
	require.NoError(t, err)

	files1, ok := result1["files"].([]interface{})
	require.True(t, ok)
	assert.Len(t, files1, 1)
	file1 := files1[0].(rc.Params)

	assert.Equal(t, "ERROR", file1["status"], "file should not exist initially")

	time.Sleep(100 * time.Millisecond)

	file, err := vfs.OpenFile("lifecycle.txt", 0, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = file.Write([]byte("test content for lifecycle"))
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	result2, err := fileStatusCall.Fn(context.Background(), rc.Params{
		"fs":   fs.ConfigString(r.Fremote),
		"file": "lifecycle.txt",
	})
	require.NoError(t, err)

	files2, ok := result2["files"].([]interface{})
	require.True(t, ok)
	assert.Len(t, files2, 1)
	file2 := files2[0].(rc.Params)

	status1, _ := file1["status"].(string)
	status2, _ := file2["status"].(string)

	assert.NotEqual(t, "ERROR", status1)
	assert.NotEqual(t, "ERROR", status2)
	assert.NotEqual(t, status1, status2, "status should change after file is created")
}
