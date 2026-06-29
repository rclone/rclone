// Regression tests for the RFC 4918 Overwrite header default behaviour.
//
// These tests cover plain HTTP semantics and are independent of the
// underlying filesystem's character-handling quirks, so they intentionally
// do not carry the `!windows && !darwin` build constraint that applies to
// the integration tests in webdav_test.go.

package webdav

import (
	"context"
	"net/http"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startWritableServer starts a webdav server backed by a fresh temp
// directory and returns the server URL. It is used by tests that need to
// exercise mutating verbs such as MKCOL and MOVE.
func startWritableServer(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	f, err := fs.NewFs(context.Background(), dir)
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{"localhost:0"}

	w, err := newWebDAV(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err)
	go func() {
		require.NoError(t, w.Serve())
	}()
	t.Cleanup(func() {
		assert.NoError(t, w.Shutdown())
	})

	return w.server.URLs()[0]
}

func mkcol(t *testing.T, baseURL, path string) {
	t.Helper()
	req, err := http.NewRequest("MKCOL", baseURL+path, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "MKCOL %s", path)
}

// TestMoveDefaultsToOverwrite is a regression test for
// https://github.com/rclone/rclone/issues/9496
//
// RFC 4918 section 10.6 requires that when the Overwrite header is omitted
// from a COPY or MOVE request, the resource MUST behave as if Overwrite: T
// had been sent. The upstream golang.org/x/net/webdav library mis-handles
// the MOVE case by checking r.Header.Get("Overwrite") == "T", so an absent
// header is treated as Overwrite: F and the request fails with 412
// Precondition Failed. rclone normalises the header before delegating to
// the library so that the default matches the RFC.
func TestMoveDefaultsToOverwrite(t *testing.T) {
	testURL := startWritableServer(t)

	mkcol(t, testURL, "dir1")
	mkcol(t, testURL, "dir2")

	// MOVE without Overwrite header: per RFC 4918 the default is T, so the
	// existing destination must be replaced and the server must return 2xx.
	req, err := http.NewRequest("MOVE", testURL+"dir2", nil)
	require.NoError(t, err)
	req.Header.Set("Destination", testURL+"dir1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.NotEqual(t, http.StatusPreconditionFailed, resp.StatusCode,
		"MOVE without Overwrite header must not return 412; RFC 4918 default is Overwrite: T")
	assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
		"expected 2xx, got %d", resp.StatusCode)
}

// TestMoveOverwriteFalseStillRejects ensures the rclone normalisation only
// fills in a missing Overwrite header and never overrides an explicit
// Overwrite: F sent by the client.
func TestMoveOverwriteFalseStillRejects(t *testing.T) {
	testURL := startWritableServer(t)

	mkcol(t, testURL, "dir1")
	mkcol(t, testURL, "dir2")

	req, err := http.NewRequest("MOVE", testURL+"dir2", nil)
	require.NoError(t, err)
	req.Header.Set("Destination", testURL+"dir1")
	req.Header.Set("Overwrite", "F")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode,
		"MOVE with explicit Overwrite: F must still return 412 when destination exists")
}
