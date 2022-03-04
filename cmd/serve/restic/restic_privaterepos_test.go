//go:build go1.17
// +build go1.17

package restic

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rclone/rclone/cmd/serve/httplib"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/httplib/httpflags"
	"github.com/stretchr/testify/require"
)

// newAuthenticatedRequest returns a new HTTP request with the given params.
func newAuthenticatedRequest(t testing.TB, method, path string, body io.Reader) *http.Request {
	req := newRequest(t, method, path, body)
	req = req.WithContext(context.WithValue(req.Context(), httplib.ContextUserKey, "test"))
	req.Header.Add("Accept", resticAPIV2)
	return req
}

// TestResticPrivateRepositories runs tests on the restic handler code for private repositories
func TestResticPrivateRepositories(t *testing.T) {
	buf := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, buf)
	require.NoError(t, err)

	// setup rclone with a local backend in a temporary directory
	tempdir := t.TempDir()

	// globally set private-repos mode & test user
	prev := privateRepos
	prevUser := httpflags.Opt.BasicUser
	prevPassword := httpflags.Opt.BasicPass
	privateRepos = true
	httpflags.Opt.BasicUser = "test"
	httpflags.Opt.BasicPass = "password"
	// reset when done
	defer func() {
		privateRepos = prev
		httpflags.Opt.BasicUser = prevUser
		httpflags.Opt.BasicPass = prevPassword
	}()

	// make a new file system in the temp dir
	f := cmd.NewFsSrc([]string{tempdir})
	srv := NewServer(f, &httpflags.Opt)

	// Requesting /test/ should allow access
	reqs := []*http.Request{
		newAuthenticatedRequest(t, "POST", "/test/?create=true", nil),
		newAuthenticatedRequest(t, "POST", "/test/config", strings.NewReader("foobar test config")),
		newAuthenticatedRequest(t, "GET", "/test/config", nil),
	}
	for _, req := range reqs {
		checkRequest(t, srv.ServeHTTP, req, []wantFunc{wantCode(http.StatusOK)})
	}

	// Requesting everything else should raise forbidden errors
	reqs = []*http.Request{
		newAuthenticatedRequest(t, "GET", "/", nil),
		newAuthenticatedRequest(t, "POST", "/other_user", nil),
		newAuthenticatedRequest(t, "GET", "/other_user/config", nil),
	}
	for _, req := range reqs {
		checkRequest(t, srv.ServeHTTP, req, []wantFunc{wantCode(http.StatusForbidden)})
	}

}
