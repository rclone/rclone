package restic

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rclone/rclone/cmd"
	"github.com/stretchr/testify/require"
)

// newAuthenticatedRequest returns a new HTTP request with the given params.
func newAuthenticatedRequest(t testing.TB, method, path string, body io.Reader, user, pass string) *http.Request {
	req := newRequest(t, method, path, body)
	req.SetBasicAuth(user, pass)
	req.Header.Add("Accept", resticAPIV2)
	return req
}

// TestResticPrivateRepositories runs tests on the restic handler code for private repositories
func TestResticPrivateRepositories(t *testing.T) {
	ctx := context.Background()
	buf := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, buf)
	require.NoError(t, err)

	// setup rclone with a local backend in a temporary directory
	tempdir := t.TempDir()

	opt := newOpt()

	// set private-repos mode & test user
	opt.PrivateRepos = true
	opt.Auth.BasicUser = "test"
	opt.Auth.BasicPass = "password"

	// make a new file system in the temp dir
	f := cmd.NewFsSrc([]string{tempdir})
	s, err := newServer(ctx, f, &opt)
	require.NoError(t, err)
	router := s.server.Router()

	// Requesting /test/ should allow access
	reqs := []*http.Request{
		newAuthenticatedRequest(t, "POST", "/test/?create=true", nil, opt.Auth.BasicUser, opt.Auth.BasicPass),
		newAuthenticatedRequest(t, "POST", "/test/config", strings.NewReader("foobar test config"), opt.Auth.BasicUser, opt.Auth.BasicPass),
		newAuthenticatedRequest(t, "GET", "/test/config", nil, opt.Auth.BasicUser, opt.Auth.BasicPass),
	}
	for _, req := range reqs {
		checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusOK)})
	}

	// Requesting with bad credentials should raise unauthorised errors
	reqs = []*http.Request{
		newRequest(t, "GET", "/test/config", nil),
		newAuthenticatedRequest(t, "GET", "/test/config", nil, opt.Auth.BasicUser, ""),
		newAuthenticatedRequest(t, "GET", "/test/config", nil, "", opt.Auth.BasicPass),
		newAuthenticatedRequest(t, "GET", "/test/config", nil, opt.Auth.BasicUser+"x", opt.Auth.BasicPass),
		newAuthenticatedRequest(t, "GET", "/test/config", nil, opt.Auth.BasicUser, opt.Auth.BasicPass+"x"),
	}
	for _, req := range reqs {
		checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusUnauthorized)})
	}

	// Requesting everything else should raise forbidden errors
	reqs = []*http.Request{
		newAuthenticatedRequest(t, "GET", "/", nil, opt.Auth.BasicUser, opt.Auth.BasicPass),
		newAuthenticatedRequest(t, "POST", "/other_user", nil, opt.Auth.BasicUser, opt.Auth.BasicPass),
		newAuthenticatedRequest(t, "GET", "/other_user/config", nil, opt.Auth.BasicUser, opt.Auth.BasicPass),
	}
	for _, req := range reqs {
		checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusForbidden)})
	}

}
