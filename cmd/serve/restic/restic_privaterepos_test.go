package restic

import (
	"crypto/rand"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/lib/http/auth"
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
	buf := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, buf)
	require.NoError(t, err)

	// setup rclone with a local backend in a temporary directory
	tempdir, err := ioutil.TempDir("", "rclone-restic-test-")
	require.NoError(t, err)

	// make sure the tempdir is properly removed
	defer func() {
		err := os.RemoveAll(tempdir)
		require.NoError(t, err)
	}()

	// globally set private-repos mode & test user
	prev := privateRepos
	prevUser := auth.Opt.BasicUser
	prevPassword := auth.Opt.BasicPass
	privateRepos = true
	auth.Opt.BasicUser = "test"
	auth.Opt.BasicPass = "password"
	// reset when done
	defer func() {
		privateRepos = prev
		auth.Opt.BasicUser = prevUser
		auth.Opt.BasicPass = prevPassword
	}()

	// make a new file system in the temp dir
	f := cmd.NewFsSrc([]string{tempdir})
	srv := newServer(f)
	router := chi.NewRouter()
	srv.Bind(router)

	// Requesting /test/ should allow access
	reqs := []*http.Request{
		newAuthenticatedRequest(t, "POST", "/test/?create=true", nil, auth.Opt.BasicUser, auth.Opt.BasicPass),
		newAuthenticatedRequest(t, "POST", "/test/config", strings.NewReader("foobar test config"), auth.Opt.BasicUser, auth.Opt.BasicPass),
		newAuthenticatedRequest(t, "GET", "/test/config", nil, auth.Opt.BasicUser, auth.Opt.BasicPass),
	}
	for _, req := range reqs {
		checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusOK)})
	}

	// Requesting with bad credentials should raise unauthorised errors
	reqs = []*http.Request{
		newRequest(t, "GET", "/test/config", nil),
		newAuthenticatedRequest(t, "GET", "/test/config", nil, auth.Opt.BasicUser, ""),
	}
	for _, req := range reqs {
		checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusUnauthorized)})
	}

	// Requesting everything else should raise forbidden errors
	reqs = []*http.Request{
		newAuthenticatedRequest(t, "GET", "/", nil, auth.Opt.BasicUser, auth.Opt.BasicPass),
		newAuthenticatedRequest(t, "POST", "/other_user", nil, auth.Opt.BasicUser, auth.Opt.BasicPass),
		newAuthenticatedRequest(t, "GET", "/other_user/config", nil, auth.Opt.BasicUser, auth.Opt.BasicPass),
	}
	for _, req := range reqs {
		checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusForbidden)})
	}

}
