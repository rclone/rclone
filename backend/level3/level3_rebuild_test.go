package level3_test

import (
	"bytes"
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/level3"
	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rebuildFile struct {
	remote string
	data   []byte
}

func defaultRebuildDataset() []rebuildFile {
	return []rebuildFile{
		{remote: "even-length.bin", data: []byte("ABCD")},
		{remote: "odd-length.bin", data: []byte("ABCDE")},
		{remote: "dir/nested.txt", data: []byte("nested data")},
		{remote: "dir/subdir/deep.bin", data: []byte("0123456789ABCDEF")},
	}
}

func writeSourceFiles(t *testing.T, root string, files []rebuildFile) {
	t.Helper()
	for _, file := range files {
		fullPath := filepath.Join(append([]string{root}, strings.Split(file.remote, "/")...)...)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, file.data, 0o644))
	}
}

func uploadDataset(t *testing.T, ctx context.Context, f *level3.Fs, files []rebuildFile) {
	t.Helper()
	dirs := make(map[string]struct{})
	for _, file := range files {
		dir := path.Dir(file.remote)
		if dir != "." {
			if _, ok := dirs[dir]; !ok {
				require.NoError(t, f.Mkdir(ctx, dir))
				dirs[dir] = struct{}{}
			}
		}

		info := object.NewStaticObjectInfo(file.remote, time.Now(), int64(len(file.data)), true, nil, nil)
		_, err := f.Put(ctx, bytes.NewReader(file.data), info)
		require.NoError(t, err)
	}
}

func particlePath(base, remote string) string {
	parts := append([]string{base}, strings.Split(remote, "/")...)
	return filepath.Join(parts...)
}

func TestRebuildEvenBackendSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	eveDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()
	sourceDir := t.TempDir()

	fsInterface, err := level3.NewFs(ctx, "TestRebuildSuccess", "", configmap.Simple{
		"even":   eveDir,
		"odd":    oddDir,
		"parity": parityDir,
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*level3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	files := defaultRebuildDataset()
	writeSourceFiles(t, sourceDir, files)
	uploadDataset(t, ctx, f, files)

	for _, file := range files {
		parityName := level3.GetParityFilename(file.remote, len(file.data)%2 == 1)
		_, err := os.Stat(particlePath(parityDir, parityName))
		require.NoErrorf(t, err, "expected parity particle for %s", file.remote)
	}

	// Simulate disk swap: remove even backend contents and recreate directory
	require.NoError(t, os.RemoveAll(eveDir))
	require.NoError(t, os.MkdirAll(eveDir, 0o755))

	out, err := f.Command(ctx, "rebuild", []string{"even"}, nil)
	require.NoError(t, err)
	summary, _ := out.(string)
	assert.Contains(t, summary, "Files rebuilt:")

	for _, file := range files {
		_, err := os.Stat(particlePath(eveDir, file.remote))
		require.NoErrorf(t, err, "expected rebuilt particle for %s", file.remote)
	}

	sourceFsInterface, err := local.NewFs(ctx, "sourceLocal", sourceDir, configmap.Simple{})
	require.NoError(t, err)
	require.NoError(t, operations.Check(ctx, &operations.CheckOpt{Fsrc: sourceFsInterface, Fdst: f}))
}

func TestRebuildEvenBackendFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	eveDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()
	sourceDir := t.TempDir()

	fsInterface, err := level3.NewFs(ctx, "TestRebuildFailure", "", configmap.Simple{
		"even":   eveDir,
		"odd":    oddDir,
		"parity": parityDir,
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*level3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	files := defaultRebuildDataset()
	writeSourceFiles(t, sourceDir, files)
	uploadDataset(t, ctx, f, files)

	for _, file := range files {
		parityName := level3.GetParityFilename(file.remote, len(file.data)%2 == 1)
		_, err := os.Stat(particlePath(parityDir, parityName))
		require.NoErrorf(t, err, "expected parity particle for %s", file.remote)
	}

	// Simulate catastrophic loss: even backend replaced, parity missing
	require.NoError(t, os.RemoveAll(eveDir))
	require.NoError(t, os.MkdirAll(eveDir, 0o755))
	require.NoError(t, os.RemoveAll(parityDir))
	require.NoError(t, os.MkdirAll(parityDir, 0o755))

	out, err := f.Command(ctx, "rebuild", []string{"even"}, nil)
	require.NoError(t, err)
	summary, _ := out.(string)
	assert.Contains(t, summary, "Files rebuilt: 0/")

	// Even backend should still be empty for first file
	_, err = os.Stat(particlePath(eveDir, files[0].remote))
	assert.Error(t, err)

	// Reading from level3 should fail due to insufficient particles
	_, err = f.NewObject(ctx, files[0].remote)
	assert.Error(t, err)
}
