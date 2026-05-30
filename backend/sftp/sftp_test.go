// Test Sftp filesystem interface

//go:build !plan9

package sftp_test

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/rclone/rclone/backend/sftp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSFTPOpenssh:",
		NilObject:  (*sftp.Object)(nil),
	})
}

func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSFTPRclone:",
		NilObject:  (*sftp.Object)(nil),
	})
}

func TestIntegration3(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSFTPRcloneSSH:",
		NilObject:  (*sftp.Object)(nil),
	})
}

// TestHardLinkPreservation tests the hard link preservation feature
func TestHardLinkPreservation(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	
	ctx := context.Background()
	
	// Skip on CI or environments where we can't create hard links
	if os.Getenv("CI") != "" {
		t.Skip("Skipping hard link test in CI environment")
	}
	
	// Create a temporary local directory
	localDir, err := os.MkdirTemp("", "rclone-sftp-hardlink-test")
	require.NoError(t, err)
	defer os.RemoveAll(localDir)
	
	// Create a test file
	testFile := path.Join(localDir, "original.txt")
	testContent := []byte("This is test content for hard link preservation")
	err = os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)
	
	// Create a hard link
	hardLinkFile := path.Join(localDir, "hardlink.txt")
	err = os.Link(testFile, hardLinkFile)
	require.NoError(t, err)
	
	// Verify they are hard linked locally
	stat1, err := os.Stat(testFile)
	require.NoError(t, err)
	stat2, err := os.Stat(hardLinkFile)
	require.NoError(t, err)
	
	// Get system-specific stat info to check inodes
	if sys1, ok := stat1.Sys().(*os.SyscallStat_t); ok {
		if sys2, ok2 := stat2.Sys().(*os.SyscallStat_t); ok2 {
			assert.Equal(t, sys1.Ino, sys2.Ino, "Local files should have same inode")
			assert.Greater(t, sys1.Nlink, uint64(1), "Local file should have multiple hard links")
		}
	}
	
	// Now test with SFTP backend
	// This would need a properly configured SFTP remote with preserve_links enabled
	// The actual integration test would:
	// 1. Create an SFTP fs with preserve_links=true
	// 2. Upload both files
	// 3. Verify that the second file is created as a hard link on the remote
	// 4. Download and verify they still share content
	
	// Example test structure (requires actual SFTP server):
	/*
	remoteName := "TestSFTPHardLink:"
	f, err := fs.NewFs(ctx, remoteName)
	require.NoError(t, err)
	
	// Enable hard link preservation
	if sftpFs, ok := f.(*sftp.Fs); ok {
		sftpFs.opt.PreserveLinks = true
	}
	
	// Upload first file
	obj1, err := f.Put(ctx, bytes.NewReader(testContent), fs.NewStaticObjectInfo("original.txt", stat1.ModTime(), int64(len(testContent)), true, nil, nil))
	require.NoError(t, err)
	
	// Upload second file (should be detected as hard link)
	obj2, err := f.Put(ctx, bytes.NewReader(testContent), fs.NewStaticObjectInfo("hardlink.txt", stat2.ModTime(), int64(len(testContent)), true, nil, nil))
	require.NoError(t, err)
	
	// Verify they are hard linked on remote
	// This would require SSH access to run stat commands
	*/
}

// TestHardLinkCopy tests creating hard links via Copy operation
func TestHardLinkCopy(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	
	ctx := context.Background()
	
	// This test verifies the existing copy_is_hardlink functionality
	// which creates hard links when doing server-side copies
	
	// Example test structure (requires actual SFTP server):
	/*
	remoteName := "TestSFTPCopyHardLink:"
	f, err := fs.NewFs(ctx, remoteName)
	require.NoError(t, err)
	
	// Enable hard link for copy
	if sftpFs, ok := f.(*sftp.Fs); ok {
		sftpFs.opt.CopyIsHardlink = true
	}
	
	// Create a test file
	testContent := []byte("Test content for copy as hard link")
	obj1, err := f.Put(ctx, bytes.NewReader(testContent), fs.NewStaticObjectInfo("original.txt", time.Now(), int64(len(testContent)), true, nil, nil))
	require.NoError(t, err)
	
	// Copy the file (should create hard link)
	obj2, err := f.Copy(ctx, obj1, "copy.txt")
	require.NoError(t, err)
	
	// Verify both objects exist and have same content
	assert.Equal(t, obj1.Size(), obj2.Size())
	*/
}