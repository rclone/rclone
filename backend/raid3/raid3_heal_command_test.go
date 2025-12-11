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

	// Upload a test file
	data := []byte("heal-command-test-data")
	remote := createTestFile(t, ctx, f, "heal-file.txt", data)

	// Sanity: file should exist on all three backends initially
	require.True(t, fileExistsOnDir(t, evenDir, remote))
	require.True(t, fileExistsOnDir(t, oddDir, remote))

	// Parity may use suffixes; we only care about even/odd presence here.

	// Simulate loss of the even particle
	require.NoError(t, os.Remove(filepath.Join(evenDir, remote)))
	require.False(t, fileExistsOnDir(t, evenDir, remote))

	// Run heal command (no args – heals entire remote)
	out, err := f.Command(ctx, "heal", nil, nil)
	require.NoError(t, err)
	_ = out // For now we only assert on side effects

	// After heal, even particle should be restored
	require.True(t, fileExistsOnDir(t, evenDir, remote), "even particle should be restored by heal command")
}
