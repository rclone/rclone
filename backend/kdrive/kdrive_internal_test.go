package kdrive

import (
	"bytes"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Set individual mode to prevent automatic cleanup of entire remote
	*fstest.Individual = true
	// Parse flags first
	flag.Parse()
	// Initialise fstest (setup verbose logging, etc.)
	fstest.Initialise()
	// Run tests
	rc := m.Run()
	os.Exit(rc)
}

// setupTestFs creates an isolated test filesystem in a unique subdirectory
// This prevents tests from deleting user's personal files
func setupTestFs(t *testing.T) *Fs {
	ctx := context.Background()
	fs.GetConfig(ctx).LogLevel = fs.LogLevelDebug

	// Create a unique test directory name
	testDir := fmt.Sprintf("rclone-test-%d", time.Now().UnixNano())

	// Step 1: Create fs pointing to root of drive
	fRoot, err := fs.NewFs(ctx, "TestKdrive:")
	require.NoError(t, err, "Failed to create root fs")

	// Step 2: Create the test directory in the root fs
	err = fRoot.Mkdir(ctx, testDir)
	require.NoError(t, err, "Failed to create test directory")

	// Step 3: Create fs pointing specifically to the test subdirectory
	fTest, err := fs.NewFs(ctx, fmt.Sprintf("TestKdrive:%s", testDir))
	require.NoError(t, err, "Failed to create test fs")

	// Step 4: Cleanup - delete the test directory from the root fs
	t.Cleanup(func() {
		// Use Rmdir to delete the test directory and its contents
		err := operations.Purge(ctx, fRoot, testDir)
		if err != nil {
			t.Logf("Failed to remove test directory: %v", err)
		}
	})

	// Cast fTest to *Fs
	fKdrive, ok := fTest.(*Fs)
	require.True(t, ok, "Expected *Fs type")

	return fKdrive
}

// TestPutSmallFile tests the updateDirect path (file < uploadThreshold of 20MB)
// go test -v ./backend/kdrive/ -remote TestKdrive: -run TestPutSmallFile -verbose
func TestPutSmallFile(t *testing.T) {
	ctx := context.Background()
	fRemote := setupTestFs(t)

	// File of 1KB
	size := int64(1024)
	data := make([]byte, size)
	_, err := rand.Read(data)
	require.NoError(t, err)

	remote := fmt.Sprintf("small-file-%d.bin", time.Now().UnixNano())
	src := object.NewStaticObjectInfo(remote, time.Now(), size, true, nil, fRemote)

	obj, err := fRemote.Put(ctx, bytes.NewReader(data), src)
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Verify that the object exists
	obj2, err := fRemote.NewObject(ctx, remote)
	require.NoError(t, err)
	assert.Equal(t, size, obj2.Size())
	assert.Equal(t, remote, obj2.Remote())
}

// TestPutLargeFile tests the updateMultipart path (file > uploadThreshold of 20MB)
// go test -v ./backend/kdrive/ -remote TestKdrive: -run TestPutLargeFile -verbose
func TestPutLargeFile(t *testing.T) {
	ctx := context.Background()
	fRemote := setupTestFs(t)

	// File of 50MB to force chunked mode
	size := int64(50 * 1024 * 1024)
	data := make([]byte, size)
	_, err := rand.Read(data)
	require.NoError(t, err)

	remote := fmt.Sprintf("large-file-%d.bin", time.Now().UnixNano())
	src := object.NewStaticObjectInfo(remote, time.Now(), size, true, nil, fRemote)

	obj, err := fRemote.Put(ctx, bytes.NewReader(data), src)
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Verify that the object exists
	obj2, err := fRemote.NewObject(ctx, remote)
	require.NoError(t, err)
	assert.Equal(t, size, obj2.Size())
	assert.Equal(t, remote, obj2.Remote())
}

func prepareListing(t *testing.T) fs.Fs {
	ctx := context.Background()

	// Use the same isolated test fs setup
	fRemote := setupTestFs(t)

	// Copies the test/test-list folder to the remote (recursive)
	testDirPath := "./test/test-list"
	fLocal, err := fs.NewFs(ctx, testDirPath)
	require.NoError(t, err)

	err = sync.CopyDir(ctx, fRemote, fLocal, true)
	require.NoError(t, err)

	return fRemote
}

// TestListFiles test List without recursion
// go test -v ./backend/kdrive/ -remote TestKdrive: -run TestListFiles -verbose
func TestListFiles(t *testing.T) {
	ctx := context.Background()
	fRemote := prepareListing(t)

	entries, err := fRemote.List(ctx, "")
	require.NoError(t, err)

	// Verify that we have listed the files/directories
	assert.NotEmpty(t, entries)
	assert.Len(t, entries, 3)

	var remoteList []string
	for _, item := range entries {
		fs.Debugf(nil, "Remote file : %s", item.Remote())
		remoteList = append(remoteList, item.Remote())
	}

	assert.Contains(t, remoteList, "test-list-subfolder")
	assert.Contains(t, remoteList, "test-list-file1.txt")
	assert.Contains(t, remoteList, "test-list-file2.txt")
	assert.NotContains(t, remoteList, "test-list-subfolder/test-list-subsubfolder")

	// List subfolder
	entriesSub, err := fRemote.List(ctx, "/test-list-subfolder")
	require.NoError(t, err)

	// Verify that we have listed the files/directories
	assert.NotEmpty(t, entriesSub)
	assert.Len(t, entriesSub, 2)

	var remoteListSub []string
	for _, item := range entriesSub {
		fs.Debugf(nil, "Remote file sub : %s", item.Remote())
		remoteListSub = append(remoteListSub, item.Remote())
	}

	assert.Contains(t, remoteListSub, "/test-list-subfolder/test-list-subsubfolder")
	assert.Contains(t, remoteListSub, "/test-list-subfolder/test-list-subfile.txt")
	assert.NotContains(t, remoteListSub, "test-list-file1.txt")
}

// TestListFiles test List with recursion
// go test -v ./backend/kdrive/ -remote TestKdrive: -run TestListFiles -verbose
func TestListRecursive(t *testing.T) {
	ctx := context.Background()
	fRemote := prepareListing(t)

	if fRemote.Features().ListR == nil {
		t.Skip("ListR not supported")
	}

	var entries fs.DirEntries
	err := fRemote.Features().ListR(ctx, "", func(entry fs.DirEntries) error {
		entries = append(entries, entry...)
		return nil
	})
	require.NoError(t, err)

	// Verify that we have listed the files/directories
	assert.NotEmpty(t, entries)
	assert.Len(t, entries, 6)

	var remoteList []string
	for _, item := range entries {
		fs.Debugf(nil, "Remote file %s", item.Remote())
		remoteList = append(remoteList, item.Remote())
	}

	assert.Contains(t, remoteList, "test-list-subfolder")
	assert.Contains(t, remoteList, "test-list-file1.txt")
	assert.Contains(t, remoteList, "test-list-file2.txt")
	assert.Contains(t, remoteList, "test-list-subfolder/test-list-subfile.txt")
	assert.Contains(t, remoteList, "test-list-subfolder/test-list-subsubfolder")
	assert.Contains(t, remoteList, "test-list-subfolder/test-list-subsubfolder/test-list-subsubfile.txt")
}

// TestPublicLink tests the creation and deletion of public links
// go test -v ./backend/kdrive/ -remote TestKdrive: -run TestPublicLink -verbose
func TestPublicLink(t *testing.T) {
	ctx := context.Background()
	fRemote := setupTestFs(t)

	if fRemote.Features().PublicLink == nil {
		t.Skip("PublicLink not supported")
	}

	// Create a test file
	testContent := []byte("Test content for public link")
	testFile := fmt.Sprintf("test-link-file-%d.txt", time.Now().UnixNano())
	src := object.NewStaticObjectInfo(testFile, time.Now(), int64(len(testContent)), true, nil, fRemote)

	obj, err := fRemote.Put(ctx, bytes.NewReader(testContent), src)
	require.NoError(t, err, "Failed to create test file")
	require.NotNil(t, obj)

	testFile2 := fmt.Sprintf("test-link-file-%d.txt", time.Now().UnixNano())
	src = object.NewStaticObjectInfo(testFile2, time.Now(), int64(len(testContent)), true, nil, fRemote)

	obj, err = fRemote.Put(ctx, bytes.NewReader(testContent), src)
	require.NoError(t, err, "Failed to create test file")
	require.NotNil(t, obj)

	// Test 1: Create public link for file
	t.Run("Create link for file", func(t *testing.T) {
		link, err := fRemote.Features().PublicLink(ctx, testFile, 0, false)
		require.NoError(t, err)
		assert.NotEmpty(t, link)
		assert.Contains(t, link, "infomaniak")
		fs.Debugf(nil, "Created public link: %s", link)
	})

	// Test 2: Get existing link (should return the same link)
	t.Run("Get existing link for file", func(t *testing.T) {
		link, err := fRemote.Features().PublicLink(ctx, testFile, 0, false)
		require.NoError(t, err)
		assert.NotEmpty(t, link)
		fs.Debugf(nil, "Retrieved public link: %s", link)
	})

	// Test 3: Create public link with expiration
	t.Run("Create link with expiration", func(t *testing.T) {
		expire := fs.Duration(24 * time.Hour)
		link, err := fRemote.Features().PublicLink(ctx, testFile, expire, false)
		require.NoError(t, err)
		assert.NotEmpty(t, link)
		fs.Debugf(nil, "Created public link with expiration: %s", link)
	})

	// Test 4: Test with directory
	t.Run("Create link for directory", func(t *testing.T) {
		testDir := fmt.Sprintf("test-link-dir-%d", time.Now().UnixNano())
		err := fRemote.Mkdir(ctx, testDir)
		require.NoError(t, err)

		link, err := fRemote.Features().PublicLink(ctx, testDir, 0, false)
		require.NoError(t, err)
		assert.NotEmpty(t, link)
		assert.Contains(t, link, "infomaniak")
		fs.Debugf(nil, "Created public link for directory: %s", link)

		// Clean up the directory using Rmdir instead of dirCache
		err = fRemote.Rmdir(ctx, testDir)
		if err != nil {
			t.Logf("Warning: failed to remove test directory: %v", err)
		}
	})

	// Test 5: Remove public link
	t.Run("Remove public link", func(t *testing.T) {
		_, err := fRemote.Features().PublicLink(ctx, testFile, 0, true)
		require.NoError(t, err)
		fs.Debugf(nil, "Removed public link for: %s", testFile)
	})

	// Test 6: Try to remove link for non-existent file (should error)
	t.Run("Remove link for non-existent file fails", func(t *testing.T) {
		_, err := fRemote.Features().PublicLink(ctx, "non-existent-file.txt", 0, true)
		assert.Error(t, err)
	})

	// Test 7: Try to non existent link (should error)
	t.Run("Remove non-existent link fails", func(t *testing.T) {
		_, err := fRemote.Features().PublicLink(ctx, testFile2, 0, true)
		assert.Error(t, err)
	})
}
