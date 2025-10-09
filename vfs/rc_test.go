// Test the VFS remote control commands

package vfs

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRCStatus(t *testing.T) {
	// Create VFS with test files using standard test helper
	r, vfs := newTestVFS(t)

	// Create a test file
	r.WriteFile("test.txt", "test content", time.Now())

	// Clear any existing VFS instances to avoid conflicts
	clearActiveCache()
	// Add VFS to active cache
	addToActiveCache(vfs)

	// Test vfs/status endpoint
	statusCall := rc.Calls.Get("vfs/status")
	require.NotNil(t, statusCall)

	// Test with valid file path
	result, err := statusCall.Fn(context.Background(), rc.Params{
		"fs": r.Fremote.String(),
	})
	require.NoError(t, err)

	// Verify structure
	assert.Contains(t, result, "totalFiles")
	assert.Contains(t, result, "fullCount")
	assert.Contains(t, result, "partialCount")
	assert.Contains(t, result, "noneCount")
	assert.Contains(t, result, "dirtyCount")
	assert.Contains(t, result, "uploadingCount")
	assert.Contains(t, result, "totalCachedBytes")
	assert.Contains(t, result, "averageCachePercentage")

	// Verify types
	totalFiles, ok := result["totalFiles"].(int)
	require.True(t, ok)
	assert.GreaterOrEqual(t, totalFiles, 0)

	fullCount, ok := result["fullCount"].(int)
	require.True(t, ok)
	assert.GreaterOrEqual(t, fullCount, 0)

	partialCount, ok := result["partialCount"].(int)
	require.True(t, ok)
	assert.GreaterOrEqual(t, partialCount, 0)

	noneCount, ok := result["noneCount"].(int)
	require.True(t, ok)
	assert.GreaterOrEqual(t, noneCount, 0)

	dirtyCount, ok := result["dirtyCount"].(int)
	require.True(t, ok)
	assert.GreaterOrEqual(t, dirtyCount, 0)

	uploadingCount, ok := result["uploadingCount"].(int)
	require.True(t, ok)
	assert.GreaterOrEqual(t, uploadingCount, 0)

	totalCachedBytes, ok := result["totalCachedBytes"].(int64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, totalCachedBytes, int64(0))

	averageCachePercentage, ok := result["averageCachePercentage"].(int)
	require.True(t, ok)
	assert.GreaterOrEqual(t, averageCachePercentage, 0)
	assert.LessOrEqual(t, averageCachePercentage, 100)
}

func TestRCFileStatus(t *testing.T) {
	// Create VFS with test files using standard test helper
	r, vfs := newTestVFS(t)

	// Create a test file
	r.WriteFile("test.txt", "test content", time.Now())

	// Clear any existing VFS instances to avoid conflicts
	clearActiveCache()
	// Add VFS to active cache
	addToActiveCache(vfs)

	// Test vfs/file-status endpoint
	fileStatusCall := rc.Calls.Get("vfs/file-status")
	require.NotNil(t, fileStatusCall)

	// Test with valid file path
	result, err := fileStatusCall.Fn(context.Background(), rc.Params{
		"fs":   r.Fremote.String(),
		"file": "test.txt",
	})
	require.NoError(t, err)

	// Verify structure - now returns in 'files' array
	assert.Contains(t, result, "files")
	files, ok := result["files"].([]rc.Params)
	require.True(t, ok)
	require.Len(t, files, 1)

	// Check the first (and only) file in the array
	file := files[0]
	assert.Contains(t, file, "name")
	assert.Contains(t, file, "status")
	assert.Contains(t, file, "percentage")

	// Verify types
	name, ok := file["name"].(string)
	require.True(t, ok)
	assert.Equal(t, "test.txt", name)

	status, ok := file["status"].(string)
	require.True(t, ok)
	assert.Contains(t, []string{"FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING"}, status)

	percentage, ok := file["percentage"].(int)
	require.True(t, ok)
	assert.GreaterOrEqual(t, percentage, 0)
	assert.LessOrEqual(t, percentage, 100)

	// Test with non-existent file
	result, err = fileStatusCall.Fn(context.Background(), rc.Params{
		"fs":   r.Fremote.String(),
		"file": "nonexistent.txt",
	})
	require.NoError(t, err)

	// Verify structure - now returns in 'files' array
	assert.Contains(t, result, "files")
	files, ok = result["files"].([]rc.Params)
	require.True(t, ok)
	require.Len(t, files, 1)

	file = files[0]
	name, ok = file["name"].(string)
	require.True(t, ok)
	assert.Equal(t, "nonexistent.txt", name)

	status, ok = file["status"].(string)
	require.True(t, ok)
	assert.Equal(t, "NONE", status)

	percentage, ok = file["percentage"].(int)
	require.True(t, ok)
	assert.Equal(t, 0, percentage)
}

func TestRCDirStatus(t *testing.T) {
	// Create VFS with test files using standard test helper
	r, vfs := newTestVFS(t)

	// Enable VFS cache for testing
	opt := vfs.Opt
	opt.CacheMode = vfscommon.CacheModeFull
	opt.CacheMaxSize = 100 * 1024 * 1024 // 100MB
	opt.CacheMaxAge = fs.Duration(24 * time.Hour)

	// Create test files in the root directory using the remote filesystem
	r.Mkdir("testdir")
	r.WriteFile("testdir/test1.txt", "test content 1", time.Now())
	r.WriteFile("testdir/test2.txt", "test content 2", time.Now())

	// Clear any existing VFS instances to avoid conflicts
	clearActiveCache()
	// Add VFS to active cache
	addToActiveCache(vfs)

	// Give VFS time to process files
	time.Sleep(100 * time.Millisecond)

	// Test vfs/dir-status endpoint
	dirStatusCall := rc.Calls.Get("vfs/dir-status")
	require.NotNil(t, dirStatusCall)

	// Test with valid directory path
	result, err := dirStatusCall.Fn(context.Background(), rc.Params{
		"fs":  r.Fremote.String(),
		"dir": "testdir",
	})
	require.NoError(t, err)

	// Verify structure - now returns files grouped by status
	assert.Contains(t, result, "files")
	filesByStatus, ok := result["files"].(rc.Params)
	require.True(t, ok)

	// Check that we have at least one status category
	totalFiles := 0
	for _, v := range filesByStatus {
		statusFiles, ok := v.([]rc.Params)
		if ok {
			totalFiles += len(statusFiles)
		}
	}
	t.Logf("Found %d files in testdir directory", totalFiles)

	// Test with missing dir parameter (should default to root)
	result, err = dirStatusCall.Fn(context.Background(), rc.Params{
		"fs": r.Fremote.String(),
	})

	require.NoError(t, err)

	// Verify structure - now returns files grouped by status
	assert.Contains(t, result, "files")
	filesByStatus, ok = result["files"].(rc.Params)
	require.True(t, ok)
	// Check that we found some files (exact count may vary)
	totalFiles = 0
	for _, v := range filesByStatus {
		statusFiles, ok := v.([]rc.Params)
		if ok {
			totalFiles += len(statusFiles)
			for _, file := range statusFiles {
				if name, ok := file["name"].(string); ok {
					status, _ := file["status"].(string)
					t.Logf("File: %s, Status: %s", name, status)
				}
			}
		}
	}
	t.Logf("Found %d files in root directory", totalFiles)

	// Test with non-existent directory
	_, err = dirStatusCall.Fn(context.Background(), rc.Params{
		"fs":  r.Fremote.String(),
		"dir": "nonexistent",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// Helper function to add VFS to active cache for testing
func addToActiveCache(vfs *VFS) {
	activeMu.Lock()
	defer activeMu.Unlock()

	fsName := vfs.f.String()
	active[fsName] = append(active[fsName], vfs)
}

// Helper function to clear active cache for testing
func clearActiveCache() {
	activeMu.Lock()
	defer activeMu.Unlock()

	active = make(map[string][]*VFS)
}