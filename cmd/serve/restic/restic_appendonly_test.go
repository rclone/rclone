// +build go1.9

package restic

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/serve/httplib/httpflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// declare a few helper functions

// wantFunc tests the HTTP response in res and marks the test as errored if something is incorrect.
type wantFunc func(t testing.TB, res *httptest.ResponseRecorder)

// newRequest returns a new HTTP request with the given params. On error, the
// test is marked as failed.
func newRequest(t testing.TB, method, path string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, path, body)
	require.NoError(t, err)
	return req
}

// wantCode returns a function which checks that the response has the correct HTTP status code.
func wantCode(code int) wantFunc {
	return func(t testing.TB, res *httptest.ResponseRecorder) {
		assert.Equal(t, code, res.Code)
	}
}

// wantBody returns a function which checks that the response has the data in the body.
func wantBody(body string) wantFunc {
	return func(t testing.TB, res *httptest.ResponseRecorder) {
		assert.NotNil(t, res.Body)
		assert.Equal(t, res.Body.Bytes(), []byte(body))
	}
}

// checkRequest uses f to process the request and runs the checker functions on the result.
func checkRequest(t testing.TB, f http.HandlerFunc, req *http.Request, want []wantFunc) {
	rr := httptest.NewRecorder()
	f(rr, req)

	for _, fn := range want {
		fn(t, rr)
	}
}

// TestRequest is a sequence of HTTP requests with (optional) tests for the response.
type TestRequest struct {
	req  *http.Request
	want []wantFunc
}

// createOverwriteDeleteSeq returns a sequence which will create a new file at
// path, and then try to overwrite and delete it.
func createOverwriteDeleteSeq(t testing.TB, path string) []TestRequest {
	// add a file, try to overwrite and delete it
	req := []TestRequest{
		{
			req:  newRequest(t, "GET", path, nil),
			want: []wantFunc{wantCode(http.StatusNotFound)},
		},
		{
			req:  newRequest(t, "POST", path, strings.NewReader("foobar test config")),
			want: []wantFunc{wantCode(http.StatusOK)},
		},
		{
			req: newRequest(t, "GET", path, nil),
			want: []wantFunc{
				wantCode(http.StatusOK),
				wantBody("foobar test config"),
			},
		},
		{
			req:  newRequest(t, "POST", path, strings.NewReader("other config")),
			want: []wantFunc{wantCode(http.StatusForbidden)},
		},
		{
			req: newRequest(t, "GET", path, nil),
			want: []wantFunc{
				wantCode(http.StatusOK),
				wantBody("foobar test config"),
			},
		},
		{
			req:  newRequest(t, "DELETE", path, nil),
			want: []wantFunc{wantCode(http.StatusForbidden)},
		},
		{
			req: newRequest(t, "GET", path, nil),
			want: []wantFunc{
				wantCode(http.StatusOK),
				wantBody("foobar test config"),
			},
		},
	}
	return req
}

// TestResticHandler runs tests on the restic handler code, especially in append-only mode.
func TestResticHandler(t *testing.T) {
	buf := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, buf)
	require.NoError(t, err)
	randomID := hex.EncodeToString(buf)

	var tests = []struct {
		seq []TestRequest
	}{
		{createOverwriteDeleteSeq(t, "/config")},
		{createOverwriteDeleteSeq(t, "/data/"+randomID)},
		{
			// ensure we can add and remove lock files
			[]TestRequest{
				{
					req:  newRequest(t, "GET", "/locks/"+randomID, nil),
					want: []wantFunc{wantCode(http.StatusNotFound)},
				},
				{
					req:  newRequest(t, "POST", "/locks/"+randomID, strings.NewReader("lock file")),
					want: []wantFunc{wantCode(http.StatusOK)},
				},
				{
					req: newRequest(t, "GET", "/locks/"+randomID, nil),
					want: []wantFunc{
						wantCode(http.StatusOK),
						wantBody("lock file"),
					},
				},
				{
					req:  newRequest(t, "POST", "/locks/"+randomID, strings.NewReader("other lock file")),
					want: []wantFunc{wantCode(http.StatusForbidden)},
				},
				{
					req:  newRequest(t, "DELETE", "/locks/"+randomID, nil),
					want: []wantFunc{wantCode(http.StatusOK)},
				},
				{
					req:  newRequest(t, "GET", "/locks/"+randomID, nil),
					want: []wantFunc{wantCode(http.StatusNotFound)},
				},
			},
		},
	}

	// setup rclone with a local backend in a temporary directory
	tempdir, err := ioutil.TempDir("", "rclone-restic-test-")
	require.NoError(t, err)

	// make sure the tempdir is properly removed
	defer func() {
		err := os.RemoveAll(tempdir)
		require.NoError(t, err)
	}()

	// globally set append-only mode
	prev := appendOnly
	appendOnly = true
	defer func() {
		appendOnly = prev // reset when done
	}()

	// make a new file system in the temp dir
	f := cmd.NewFsSrc([]string{tempdir})
	srv := newServer(f, &httpflags.Opt)

	// create the repo
	checkRequest(t, srv.handler,
		newRequest(t, "POST", "/?create=true", nil),
		[]wantFunc{wantCode(http.StatusOK)})

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			for i, seq := range test.seq {
				t.Logf("request %v: %v %v", i, seq.req.Method, seq.req.URL.Path)
				checkRequest(t, srv.handler, seq.req, seq.want)
			}
		})
	}
}
