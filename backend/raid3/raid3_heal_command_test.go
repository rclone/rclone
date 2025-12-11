package raid3_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/raid3"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/require"
)

// createTestFile uploads a single file to the raid3 backend and returns its remote path.
func createTestFile(t *testing.T, ctx context.Context, f *raid3.Fs, name string, data []byte) string {
	t.Helper()
	info := object.NewStaticObjectInfo(name, time.Now(), int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)
	return name
}

// fileExistsOnDir checks if a file exists in the given backend directory.
func fileExistsOnDir(t *testing.T, dir string, remote string) bool {
	t.Helper()
	fullPath := filepath.Join(dir, remote)
	_, err := os.Stat(fullPath)
	return err == nil
}

func TestHealCommandReconstructsMissingParticle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Initialise local backends for particles
	evenFs, err := local.NewFs(ctx, "evenLocal", evenDir, configmap.Simple{})
	require.NoError(t, err)
	oddFs, err := local.NewFs(ctx, "oddLocal", oddDir, configmap.Simple{})
	require.NoError(t, err)
	parityFs, err := local.NewFs(ctx, "parityLocal", parityDir, configmap.Simple{})
	require.NoError(t, err)

	_ = evenFs
	_ = oddFs
	_ = parityFs

	// Create raid3 Fs over the three local backends
	fsInterface, err := raid3.NewFs(ctx, "healTest", "", configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
		// Explicitly disable auto_heal; we want healCommand to do the work.
		"auto_heal": "false",
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*raid3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	// Upload multiple test files
	file1 := createTestFile(t, ctx, f, "file1.txt", []byte("file1-data"))
	file2 := createTestFile(t, ctx, f, "file2.txt", []byte("file2-data"))
	file3 := createTestFile(t, ctx, f, "file3.txt", []byte("file3-data"))

	// Verify all files exist on all backends initially
	require.True(t, fileExistsOnDir(t, evenDir, file1))
	require.True(t, fileExistsOnDir(t, oddDir, file1))
	require.True(t, fileExistsOnDir(t, evenDir, file2))
	require.True(t, fileExistsOnDir(t, oddDir, file2))
	require.True(t, fileExistsOnDir(t, evenDir, file3))
	require.True(t, fileExistsOnDir(t, oddDir, file3))

	// Simulate loss of particles from multiple files
	// File1: missing even particle
	require.NoError(t, os.Remove(filepath.Join(evenDir, file1)))
	require.False(t, fileExistsOnDir(t, evenDir, file1))

	// File2: missing odd particle
	require.NoError(t, os.Remove(filepath.Join(oddDir, file2)))
	require.False(t, fileExistsOnDir(t, oddDir, file2))

	// File3: remains healthy (all particles present)

	// Run heal command (no args – heals entire remote)
	out, err := f.Command(ctx, "heal", nil, nil)
	require.NoError(t, err)
	_ = out // For now we only assert on side effects

	// After heal, all degraded files should be restored
	require.True(t, fileExistsOnDir(t, evenDir, file1), "file1: even particle should be restored by heal command")
	require.True(t, fileExistsOnDir(t, oddDir, file2), "file2: odd particle should be restored by heal command")

	// File3 should remain healthy (unchanged)
	require.True(t, fileExistsOnDir(t, evenDir, file3), "file3: even particle should still exist")
	require.True(t, fileExistsOnDir(t, oddDir, file3), "file3: odd particle should still exist")
}

func TestHealCommandSingleFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	evenDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()

	// Create raid3 Fs over the three local backends
	fsInterface, err := raid3.NewFs(ctx, "healSingleTest", "", configmap.Simple{
		"even":   evenDir,
		"odd":    oddDir,
		"parity": parityDir,
		// Explicitly disable auto_heal; we want healCommand to do the work.
		"auto_heal": "false",
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*raid3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	// Upload multiple test files
	file1 := createTestFile(t, ctx, f, "file1.txt", []byte("file1-data"))
	file2 := createTestFile(t, ctx, f, "file2.txt", []byte("file2-data"))
	file3 := createTestFile(t, ctx, f, "file3.txt", []byte("file3-data"))

	// Verify all files exist on all backends initially
	require.True(t, fileExistsOnDir(t, evenDir, file1))
	require.True(t, fileExistsOnDir(t, oddDir, file1))
	require.True(t, fileExistsOnDir(t, evenDir, file2))
	require.True(t, fileExistsOnDir(t, oddDir, file2))
	require.True(t, fileExistsOnDir(t, evenDir, file3))
	require.True(t, fileExistsOnDir(t, oddDir, file3))

	// Simulate loss of particles from multiple files
	// File1: missing even particle (will be healed)
	require.NoError(t, os.Remove(filepath.Join(evenDir, file1)))
	require.False(t, fileExistsOnDir(t, evenDir, file1))

	// File2: missing odd particle (should NOT be healed - only file1 is specified)
	require.NoError(t, os.Remove(filepath.Join(oddDir, file2)))
	require.False(t, fileExistsOnDir(t, oddDir, file2))

	// File3: remains healthy

	// Run heal command with single file path (only file1 should be healed)
	out, err := f.Command(ctx, "heal", []string{file1}, nil)
	require.NoError(t, err)
	_ = out

	// After heal, only file1 should be restored
	require.True(t, fileExistsOnDir(t, evenDir, file1), "file1: even particle should be restored by single-file heal command")

	// File2 should still be missing its odd particle (not healed)
	require.False(t, fileExistsOnDir(t, oddDir, file2), "file2: odd particle should NOT be restored (not specified in heal command)")

	// File3 should remain healthy (unchanged)
	require.True(t, fileExistsOnDir(t, evenDir, file3), "file3: even particle should still exist")
	require.True(t, fileExistsOnDir(t, oddDir, file3), "file3: odd particle should still exist")
}
