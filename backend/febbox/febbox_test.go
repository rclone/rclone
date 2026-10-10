package febbox

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/require"
)

const integrationSourceURL = "https://raw.githubusercontent.com/rclone/rclone/master/COPYING"

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()

	if *fstest.RemoteName == "" {
		*fstest.RemoteName = "TestFebBox:"
	}
	f, err := fs.NewFs(ctx, *fstest.RemoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skipf("Couldn't create FebBox backend - skipping tests: %v", err)
	}
	require.NoError(t, err)

	testDir := "rclone-test-" + random.String(24)
	t.Cleanup(func() {
		cleanupIntegrationDir(ctx, t, f, testDir)
	})

	require.NoError(t, f.Mkdir(ctx, testDir))

	t.Run("PutUnsupported", func(t *testing.T) {
		src := object.NewStaticObjectInfo("direct-upload", time.Now(), 4, true, nil, f)
		_, err := f.Put(ctx, strings.NewReader("data"), src)
		require.ErrorIs(t, err, fs.ErrorNotImplemented)
		require.ErrorContains(t, err, "accepts only HTTP source URLs")
	})

	commandFs, err := fs.NewFs(ctx, fspath.JoinRootPath(*fstest.RemoteName, testDir))
	require.NoError(t, err)
	command := commandFs.Features().Command
	require.NotNil(t, command)
	_, err = command(ctx, "addurl", []string{integrationSourceURL}, nil)
	require.NoError(t, err)

	obj := waitForIntegrationObject(ctx, t, f, testDir)

	t.Run("UpdateUnsupported", func(t *testing.T) {
		src := object.NewStaticObjectInfo(obj.Remote(), time.Now(), 4, true, nil, f)
		err := obj.Update(ctx, strings.NewReader("data"), src)
		require.ErrorIs(t, err, fs.ErrorNotImplemented)
		require.ErrorContains(t, err, "accepts only HTTP source URLs")
	})

	t.Run("Open", func(t *testing.T) {
		rc, err := obj.Open(ctx)
		if errors.Is(err, fs.ErrorPermissionDenied) {
			return
		}
		require.NoError(t, err)
		defer func() {
			require.NoError(t, rc.Close())
		}()
		contents, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NotEmpty(t, contents)
	})

	t.Run("MoveAndRemove", func(t *testing.T) {
		move := f.Features().Move
		require.NotNil(t, move)
		movedRemote := path.Join(testDir, "moved-"+path.Base(obj.Remote()))
		moved, err := move(ctx, obj, movedRemote)
		require.NoError(t, err)
		require.Equal(t, movedRemote, moved.Remote())
		require.NoError(t, moved.Remove(ctx))
	})

	require.NoError(t, f.Rmdir(ctx, testDir))
}

func waitForIntegrationObject(ctx context.Context, t *testing.T, f fs.Fs, dir string) fs.Object {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		entries, err := f.List(ctx, dir)
		require.NoError(t, err)
		for _, entry := range entries {
			if obj, ok := entry.(fs.Object); ok {
				return obj
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("URL ingestion did not create a file in %q", dir)
	return nil
}

func cleanupIntegrationDir(ctx context.Context, t *testing.T, f fs.Fs, dir string) {
	t.Helper()
	entries, err := f.List(ctx, dir)
	if errors.Is(err, fs.ErrorDirNotFound) {
		return
	}
	if err != nil {
		t.Errorf("list integration directory for cleanup: %v", err)
		return
	}
	for _, entry := range entries {
		obj, ok := entry.(fs.Object)
		if !ok {
			continue
		}
		if err := obj.Remove(ctx); err != nil {
			t.Errorf("remove integration object %q: %v", obj.Remote(), err)
		}
	}
	if err := f.Rmdir(ctx, dir); err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
		t.Errorf("remove integration directory %q: %v", dir, err)
	}
}
