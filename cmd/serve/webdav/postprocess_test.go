package webdav

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	fslog "github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPostprocessTestWebDAV(t *testing.T, root string) *WebDAV {
	t.Helper()

	ctx := context.Background()
	f, err := fs.NewFs(ctx, root)
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{"localhost:0"}
	vfsOpt := vfscommon.Opt
	w, err := newWebDAV(ctx, f, &opt, &vfsOpt, &proxy.Opt)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, w.Shutdown())
		w._vfs.Shutdown()
	})

	return w
}

func capturePostprocessLogs(t *testing.T) *strings.Builder {
	t.Helper()

	var logs strings.Builder
	fslog.Handler.SetOutput(func(_ slog.Level, text string) {
		logs.WriteString(text)
		logs.WriteByte('\n')
	})
	t.Cleanup(fslog.Handler.ResetOutput)

	oldHandlerLevel := fslog.Handler.SetLevel(slog.LevelError)
	t.Cleanup(func() {
		fslog.Handler.SetLevel(oldHandlerLevel)
	})

	ci := fs.GetConfig(context.Background())
	oldLogLevel := ci.LogLevel
	ci.LogLevel = fs.LogLevelError
	t.Cleanup(func() {
		ci.LogLevel = oldLogLevel
	})

	return &logs
}

func TestPostprocessMoveWithoutMtimeHeaderDoesNotStatSource(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello\n"), 0o666))

	w := newPostprocessTestWebDAV(t, root)
	require.NoError(t, w.Rename(context.Background(), "test.txt", "renamed.txt"))
	_, err := os.Stat(filepath.Join(root, "test.txt"))
	require.ErrorIs(t, err, os.ErrNotExist)

	logs := capturePostprocessLogs(t)
	req := httptest.NewRequest("MOVE", "/test.txt", nil)
	w.postprocess(req, "test.txt")

	assert.NotContains(t, logs.String(), "Failed to stat node")
	_, err = os.Stat(filepath.Join(root, "renamed.txt"))
	require.NoError(t, err)
}

func TestPostprocessMoveWithMtimeHeaderUsesDestination(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello\n"), 0o666))

	w := newPostprocessTestWebDAV(t, root)
	require.NoError(t, w.Rename(context.Background(), "test.txt", "renamed.txt"))
	_, err := os.Stat(filepath.Join(root, "test.txt"))
	require.ErrorIs(t, err, os.ErrNotExist)

	logs := capturePostprocessLogs(t)
	req := httptest.NewRequest("MOVE", "/test.txt", nil)
	want := time.Unix(1234567890, 0)
	req.Header.Set("Destination", "http://example.com/renamed.txt")
	req.Header.Set("X-OC-Mtime", "1234567890")
	w.postprocess(req, "test.txt")

	assert.NotContains(t, logs.String(), "Failed to stat node")
	info, err := os.Stat(filepath.Join(root, "renamed.txt"))
	require.NoError(t, err)
	assert.WithinDuration(t, want, info.ModTime(), time.Second)
}

func TestPostprocessMoveWithMtimeHeaderUsesDestinationBehindBaseURL(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello\n"), 0o666))

	w := newPostprocessTestWebDAV(t, root)
	w.opt.HTTP.BaseURL = "/prefix"
	require.NoError(t, w.Rename(context.Background(), "test.txt", "renamed.txt"))

	logs := capturePostprocessLogs(t)
	req := httptest.NewRequest("MOVE", "/prefix/test.txt", nil)
	want := time.Unix(1234567890, 0)
	req.Header.Set("Destination", "http://example.com/prefix/renamed.txt")
	req.Header.Set("X-OC-Mtime", "1234567890")
	w.postprocess(req, "test.txt")

	assert.NotContains(t, logs.String(), "Failed to stat node")
	info, err := os.Stat(filepath.Join(root, "renamed.txt"))
	require.NoError(t, err)
	assert.WithinDuration(t, want, info.ModTime(), time.Second)
}

func TestPostprocessPutWithMtimeHeaderSetsModTime(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello\n"), 0o666))

	w := newPostprocessTestWebDAV(t, root)
	req := httptest.NewRequest("PUT", "/test.txt", nil)
	want := time.Unix(1234567890, 0)
	req.Header.Set("X-OC-Mtime", "1234567890")
	w.postprocess(req, "test.txt")

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.WithinDuration(t, want, info.ModTime(), time.Second)
}
