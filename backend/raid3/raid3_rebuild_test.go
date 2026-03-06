package raid3_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/all" // for S3 (TestRaid3Minio)
	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/raid3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/testserver"
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

func uploadDataset(ctx context.Context, t *testing.T, f *raid3.Fs, files []rebuildFile) {
	t.Helper()
	require.NoError(t, uploadDatasetErr(ctx, f, files))
}

// uploadDatasetErr uploads files to f; returns error for caller to handle (e.g. NoSuchBucket skip).
func uploadDatasetErr(ctx context.Context, f *raid3.Fs, files []rebuildFile) error {
	dirs := make(map[string]struct{})
	for _, file := range files {
		dir := path.Dir(file.remote)
		if dir != "." {
			if _, ok := dirs[dir]; !ok {
				if err := f.Mkdir(ctx, dir); err != nil {
					return err
				}
				dirs[dir] = struct{}{}
			}
		}
		info := object.NewStaticObjectInfo(file.remote, time.Now(), int64(len(file.data)), true, nil, nil)
		if _, err := f.Put(ctx, bytes.NewReader(file.data), info); err != nil {
			return err
		}
	}
	return nil
}

func particlePath(base, remote string) string {
	parts := append([]string{base}, strings.Split(remote, "/")...)
	return filepath.Join(parts...)
}

func TestRebuildEvenBackendSuccess(t *testing.T) {
	// Do not use t.Parallel() - running alongside TestStandard can cause deadlocks.
	ctx := context.Background()

	eveDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()
	sourceDir := t.TempDir()

	fsInterface, err := raid3.NewFs(ctx, "TestRebuildSuccess", "", configmap.Simple{
		"even":   eveDir,
		"odd":    oddDir,
		"parity": parityDir,
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*raid3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	files := defaultRebuildDataset()
	writeSourceFiles(t, sourceDir, files)
	uploadDataset(ctx, t, f, files)

	for _, file := range files {
		_, err := os.Stat(particlePath(parityDir, file.remote))
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

func TestRebuildOddBackendSuccess(t *testing.T) {
	// Do not use t.Parallel() - running alongside TestStandard can cause deadlocks.
	ctx := context.Background()

	eveDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()
	sourceDir := t.TempDir()

	fsInterface, err := raid3.NewFs(ctx, "TestRebuildOddSuccess", "", configmap.Simple{
		"even":   eveDir,
		"odd":    oddDir,
		"parity": parityDir,
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*raid3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	files := defaultRebuildDataset()
	writeSourceFiles(t, sourceDir, files)
	uploadDataset(ctx, t, f, files)

	for _, file := range files {
		_, err := os.Stat(particlePath(parityDir, file.remote))
		require.NoErrorf(t, err, "expected parity particle for %s", file.remote)
	}

	// Simulate disk swap: remove odd backend contents and recreate directory
	require.NoError(t, os.RemoveAll(oddDir))
	require.NoError(t, os.MkdirAll(oddDir, 0o755))

	out, err := f.Command(ctx, "rebuild", []string{"odd"}, nil)
	require.NoError(t, err)
	summary, _ := out.(string)
	assert.Contains(t, summary, "Files rebuilt:")

	for _, file := range files {
		_, err := os.Stat(particlePath(oddDir, file.remote))
		require.NoErrorf(t, err, "expected rebuilt particle for %s", file.remote)
	}

	sourceFsInterface, err := local.NewFs(ctx, "sourceLocal", sourceDir, configmap.Simple{})
	require.NoError(t, err)
	require.NoError(t, operations.Check(ctx, &operations.CheckOpt{Fsrc: sourceFsInterface, Fdst: f}))
}

func TestRebuildOddBackendFailure(t *testing.T) {
	// Do not use t.Parallel() - running alongside TestStandard can cause deadlocks.
	ctx := context.Background()

	eveDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()
	sourceDir := t.TempDir()

	fsInterface, err := raid3.NewFs(ctx, "TestRebuildOddFailure", "", configmap.Simple{
		"even":   eveDir,
		"odd":    oddDir,
		"parity": parityDir,
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*raid3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	files := defaultRebuildDataset()
	writeSourceFiles(t, sourceDir, files)
	uploadDataset(ctx, t, f, files)

	for _, file := range files {
		_, err := os.Stat(particlePath(parityDir, file.remote))
		require.NoErrorf(t, err, "expected parity particle for %s", file.remote)
	}

	// Simulate catastrophic loss: odd backend replaced, parity missing (cannot rebuild odd with only even)
	require.NoError(t, os.RemoveAll(oddDir))
	require.NoError(t, os.MkdirAll(oddDir, 0o755))
	require.NoError(t, os.RemoveAll(parityDir))
	require.NoError(t, os.MkdirAll(parityDir, 0o755))

	out, err := f.Command(ctx, "rebuild", []string{"odd"}, nil)
	require.NoError(t, err)
	summary, _ := out.(string)
	assert.Contains(t, summary, "Files rebuilt: 0/")

	_, err = os.Stat(particlePath(oddDir, files[0].remote))
	assert.Error(t, err)

	_, err = f.NewObject(ctx, files[0].remote)
	assert.Error(t, err)
}

func TestRebuildParityBackendSuccess(t *testing.T) {
	// Do not use t.Parallel() - running alongside TestStandard can cause deadlocks.
	ctx := context.Background()

	eveDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()
	sourceDir := t.TempDir()

	fsInterface, err := raid3.NewFs(ctx, "TestRebuildParitySuccess", "", configmap.Simple{
		"even":   eveDir,
		"odd":    oddDir,
		"parity": parityDir,
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*raid3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	files := defaultRebuildDataset()
	writeSourceFiles(t, sourceDir, files)
	uploadDataset(ctx, t, f, files)

	for _, file := range files {
		_, err := os.Stat(particlePath(eveDir, file.remote))
		require.NoErrorf(t, err, "expected even particle for %s", file.remote)
	}

	// Simulate disk swap: remove parity backend contents and recreate directory
	require.NoError(t, os.RemoveAll(parityDir))
	require.NoError(t, os.MkdirAll(parityDir, 0o755))

	out, err := f.Command(ctx, "rebuild", []string{"parity"}, nil)
	require.NoError(t, err)
	summary, _ := out.(string)
	assert.Contains(t, summary, "Files rebuilt:")

	for _, file := range files {
		_, err := os.Stat(particlePath(parityDir, file.remote))
		require.NoErrorf(t, err, "expected rebuilt parity particle for %s", file.remote)
	}

	sourceFsInterface, err := local.NewFs(ctx, "sourceLocal", sourceDir, configmap.Simple{})
	require.NoError(t, err)
	require.NoError(t, operations.Check(ctx, &operations.CheckOpt{Fsrc: sourceFsInterface, Fdst: f}))
}

func TestRebuildEvenBackendFailure(t *testing.T) {
	// Do not use t.Parallel() - running alongside TestStandard can cause deadlocks.
	ctx := context.Background()

	eveDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()
	sourceDir := t.TempDir()

	fsInterface, err := raid3.NewFs(ctx, "TestRebuildFailure", "", configmap.Simple{
		"even":   eveDir,
		"odd":    oddDir,
		"parity": parityDir,
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*raid3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	files := defaultRebuildDataset()
	writeSourceFiles(t, sourceDir, files)
	uploadDataset(ctx, t, f, files)

	for _, file := range files {
		_, err := os.Stat(particlePath(parityDir, file.remote))
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

func TestRebuildParityBackendFailure(t *testing.T) {
	// Do not use t.Parallel() - running alongside TestStandard can cause deadlocks.
	ctx := context.Background()

	eveDir := t.TempDir()
	oddDir := t.TempDir()
	parityDir := t.TempDir()
	sourceDir := t.TempDir()

	fsInterface, err := raid3.NewFs(ctx, "TestRebuildParityFailure", "", configmap.Simple{
		"even":   eveDir,
		"odd":    oddDir,
		"parity": parityDir,
	})
	require.NoError(t, err)

	f, ok := fsInterface.(*raid3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(context.Background()) })

	files := defaultRebuildDataset()
	writeSourceFiles(t, sourceDir, files)
	uploadDataset(ctx, t, f, files)

	for _, file := range files {
		_, err := os.Stat(particlePath(eveDir, file.remote))
		require.NoErrorf(t, err, "expected even particle for %s", file.remote)
	}

	// Simulate catastrophic loss: parity backend replaced, even missing (cannot rebuild parity with only odd)
	require.NoError(t, os.RemoveAll(parityDir))
	require.NoError(t, os.MkdirAll(parityDir, 0o755))
	require.NoError(t, os.RemoveAll(eveDir))
	require.NoError(t, os.MkdirAll(eveDir, 0o755))

	out, err := f.Command(ctx, "rebuild", []string{"parity"}, nil)
	require.NoError(t, err)
	summary, _ := out.(string)
	assert.Contains(t, summary, "Files rebuilt: 0/")

	_, err = os.Stat(particlePath(parityDir, files[0].remote))
	assert.Error(t, err)

	_, err = f.NewObject(ctx, files[0].remote)
	assert.Error(t, err)
}

// minioRebuildPathPrefix is used for TestRebuildMinioBackendSuccess so that all
// object paths have at least one segment before the filename. S3's bucket.Split
// uses the first segment as the bucket name; without a prefix, root-level files
// (e.g. "even-length.bin") yield an empty key and "Key must not be empty". This
// matches fstest.RandomRemoteName() (e.g. TestS3:rclone-test-xxx) and the bash
// script's create_test_dataset path prefix.
const minioRebuildPathPrefix = "rclone-rebuild-test"

// minioBackendRemote maps raid3 backend name to TestRaid3Minio sub-remote name.
var minioBackendRemote = map[string]string{
	"even": "minioeven", "odd": "minioodd", "parity": "minioparity",
}

// TestRebuildMinioBackendSuccess runs rebuild tests with TestRaid3Minio (MinIO/S3).
// Requires -remote TestRaid3Minio: and Docker. Uses a path prefix (minioRebuildPathPrefix)
// so S3 sees paths like "rclone-rebuild-test/even-length.bin" — first segment = bucket,
// rest = key — avoiding "Key must not be empty". Same idea as fstest.RandomRemoteName()
// and the bash script's create_test_dataset. Simulates empty backend by purging
// the sub-remote under that prefix, then runs rebuild and verifies with operations.Check.
func TestRebuildMinioBackendSuccess(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	if !strings.HasPrefix(*fstest.RemoteName, "TestRaid3Minio") {
		t.Skip("Rebuild MinIO test requires -remote TestRaid3Minio:")
	}
	ctx := context.Background()

	fstest.Initialise()
	remoteName := *fstest.RemoteName
	if !strings.HasSuffix(remoteName, ":") {
		remoteName += ":"
	}
	// Use a path prefix so S3 gets bucket=prefix, key=filename (never empty key).
	remoteName += minioRebuildPathPrefix

	finish, err := testserver.Start(remoteName)
	require.NoError(t, err)
	defer finish()

	if envConfig := os.Getenv("RCLONE_CONFIG"); envConfig != "" {
		require.NoError(t, config.SetConfigPath(envConfig))
	} else {
		// testserver sets RCLONE_CONFIG_<UPPERCASE_NAME>__CONFIG_FILE (see testserver.start)
		configEnv := "RCLONE_CONFIG_" + strings.ToUpper("TestRaid3Minio") + "__CONFIG_FILE"
		if p := os.Getenv(configEnv); p != "" {
			require.NoError(t, config.SetConfigPath(p))
		}
	}

	// MinIO/S3 require the bucket to exist; create it before the raid3 Fs so backends see it.
	// Init script may also create it when rclone is in PATH.
	for _, backend := range []string{"even", "odd", "parity"} {
		subRemote := minioBackendRemote[backend] + ":" + minioRebuildPathPrefix
		subFs, err := fs.NewFs(ctx, subRemote)
		require.NoError(t, err)
		require.NoError(t, subFs.Mkdir(ctx, ""), "create bucket for %s", subRemote)
	}
	// Verify at least one bucket is usable (List at root); MinIO can be eventually consistent.
	var listErr error
	for i := 0; i < 5; i++ {
		time.Sleep(300 * time.Millisecond)
		subFs, _ := fs.NewFs(ctx, minioBackendRemote["even"]+":"+minioRebuildPathPrefix)
		_, listErr = subFs.List(ctx, "")
		if listErr == nil {
			break
		}
	}
	require.NoError(t, listErr, "bucket %q not usable after Mkdir (MinIO not ready?)", minioRebuildPathPrefix)

	fInterface, err := fs.NewFs(ctx, remoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skipf("Remote %q not in config - skipping", remoteName)
		return
	}
	require.NoError(t, err)
	f, ok := fInterface.(*raid3.Fs)
	require.True(t, ok)
	t.Cleanup(func() { _ = f.Shutdown(ctx) })

	sourceDir := t.TempDir()
	files := defaultRebuildDataset()
	writeSourceFiles(t, sourceDir, files)

	if err := uploadDatasetErr(ctx, f, files); err != nil {
		if strings.Contains(err.Error(), "NoSuchBucket") {
			t.Skipf("MinIO buckets not available (run with PATH=$PWD:$PATH after 'go build' so init script can rclone mkdir): %v", err)
		}
		require.NoError(t, err)
	}

	sourceFs, err := local.NewFs(ctx, "sourceLocal", sourceDir, configmap.Simple{})
	require.NoError(t, err)

	for _, backend := range []string{"even", "odd", "parity"} {
		backend := backend
		t.Run(backend, func(t *testing.T) {
			subRemote := minioBackendRemote[backend] + ":" + minioRebuildPathPrefix
			subFs, err := fs.NewFs(ctx, subRemote)
			require.NoError(t, err)
			require.NoError(t, operations.Purge(ctx, subFs, ""))
			// S3 Purge at root removes the bucket; recreate so rebuild can write.
			require.NoError(t, subFs.Mkdir(ctx, ""))

			out, err := f.Command(ctx, "rebuild", []string{backend}, nil)
			require.NoError(t, err)
			summary, _ := out.(string)
			assert.Contains(t, summary, "Files rebuilt:")

			require.NoError(t, operations.Check(ctx, &operations.CheckOpt{Fsrc: sourceFs, Fdst: f}))

			uploadDataset(ctx, t, f, files)
		})
	}
}
