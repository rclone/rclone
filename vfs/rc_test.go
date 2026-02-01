// Test the VFS remote control commands

package vfs

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getInt is a helper to extract integer-like values safely from RC results.
// JSON decoding may produce different numeric types (int, int64, float64), so
// we need to handle them all.
func getInt(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int32:
		return int64(n), true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case float32:
		return int64(n), true
	default:
		return 0, false
	}
}

func TestRCStatus(t *testing.T) {
	// Create VFS with test files using standard test helper
	r, vfs := newTestVFS(t)

	// Create a test file
	r.WriteFile("test.txt", "test content", time.Now())

	// Clear any existing VFS instances to avoid conflicts
	snapshotAndClearActiveCache(t)
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

	// Verify types using robust helper to handle various numeric types from JSON
	if n, ok := getInt(result["totalFiles"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
	} else {
		require.FailNow(t, "totalFiles has unexpected type")
	}

	if n, ok := getInt(result["fullCount"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
	} else {
		require.FailNow(t, "fullCount has unexpected type")
	}

	if n, ok := getInt(result["partialCount"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
	} else {
		require.FailNow(t, "partialCount has unexpected type")
	}

	if n, ok := getInt(result["noneCount"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
	} else {
		require.FailNow(t, "noneCount has unexpected type")
	}

	if n, ok := getInt(result["dirtyCount"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
	} else {
		require.FailNow(t, "dirtyCount has unexpected type")
	}

	if n, ok := getInt(result["uploadingCount"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
	} else {
		require.FailNow(t, "uploadingCount has unexpected type")
	}

	if n, ok := getInt(result["totalCachedBytes"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
	} else {
		require.FailNow(t, "totalCachedBytes has unexpected type")
	}

	if n, ok := getInt(result["averageCachePercentage"]); ok {
		assert.GreaterOrEqual(t, n, int64(0))
		assert.LessOrEqual(t, n, int64(100))
	} else {
		require.FailNow(t, "averageCachePercentage has unexpected type")
	}
}

func TestRCFileStatus(t *testing.T) {
	// Create VFS with test files using standard test helper
	r, vfs := newTestVFS(t)

	// Create a test file
	r.WriteFile("test.txt", "test content", time.Now())

	// Clear any existing VFS instances to avoid conflicts
	snapshotAndClearActiveCache(t)
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
	snapshotAndClearActiveCache(t)
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
	assert.Equal(t, 2, totalFiles, "Expected to find 2 files in testdir directory")
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

// Helper function to clear active cache for testing (state is saved and restored by caller)
func snapshotAndClearActiveCache(t *testing.T) map[string][]*VFS {
	activeMu.Lock()
	defer activeMu.Unlock()
	// Snapshot current state
	prev := make(map[string][]*VFS, len(active))
	for k, v := range active {
		cp := make([]*VFS, len(v))
		copy(cp, v)
		prev[k] = cp
	}
	// Clear for isolated test
	active = make(map[string][]*VFS)
	t.Cleanup(func() {
		activeMu.Lock()
		defer activeMu.Unlock()
		active = prev
	})
	return prev
}
