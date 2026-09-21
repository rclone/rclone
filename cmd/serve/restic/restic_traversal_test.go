package restic

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResticPathTraversal checks that request paths with "." or ".."
// components are rejected before they reach the backend, so they can't be
// joined with the Fs root to reach objects outside the directory served.
//
// The memory backend is used because it resolves ".." by joining paths,
// unlike the local backend which encodes the components as file names.
func TestResticPathTraversal(t *testing.T) {
	ctx := context.Background()

	// outside is the parent of the directory served and holds an object
	// which must stay unreachable through the server.
	outside := cmd.NewFsSrc([]string{":memory:traversal-test"})
	_, err := operations.Rcat(ctx, outside, "outside-secret.txt", io.NopCloser(strings.NewReader("SECRET")), time.Now(), nil)
	require.NoError(t, err)

	f := cmd.NewFsSrc([]string{":memory:traversal-test/served-root"})
	_, err = operations.Rcat(ctx, f, "inside.txt", io.NopCloser(strings.NewReader("INSIDE")), time.Now(), nil)
	require.NoError(t, err)

	opt := newOpt()
	s, err := newServer(ctx, f, &opt)
	require.NoError(t, err)
	router := s.server.Router()

	// A path inside the served root is unaffected
	checkRequest(t, router.ServeHTTP,
		newRequest(t, "GET", "/inside.txt", nil),
		[]wantFunc{wantCode(http.StatusOK), wantBody("INSIDE")})

	for _, urlpath := range []string{
		"..",
		"../",
		"../../",
		"../outside-secret.txt",
		"../../outside-secret.txt",
		"%2e%2e/outside-secret.txt",
		"../outside-write.txt",
		"a/../outside-secret.txt",
		"a/../../outside-secret.txt",
		".",
		"./inside.txt",
	} {
		for _, method := range []string{"GET", "HEAD", "POST", "DELETE"} {
			t.Run(method+" /"+urlpath, func(t *testing.T) {
				req := newRequest(t, method, "/"+urlpath, strings.NewReader("EVIL"))
				checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusBadRequest)})
			})
		}
	}

	// The object outside the served root was neither read nor modified, and
	// no new object was created next to it.
	o, err := outside.NewObject(ctx, "outside-secret.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(len("SECRET")), o.Size())
	_, err = outside.NewObject(ctx, "outside-write.txt")
	assert.Equal(t, fs.ErrorObjectNotFound, err)
}
