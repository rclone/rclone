// +build go1.7

package restic

import (
	"bytes"
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
)

// declare a few helper functions

// wantFunc tests the HTTP response in res and calls t.Error() if something is incorrect.
type wantFunc func(t testing.TB, res *httptest.ResponseRecorder)

// newRequest returns a new HTTP request with the given params. On error, t.Fatal is called.
func newRequest(t testing.TB, method, path string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// wantCode returns a function which checks that the response has the correct HTTP status code.
func wantCode(code int) wantFunc {
	return func(t testing.TB, res *httptest.ResponseRecorder) {
		if res.Code != code {
			t.Errorf("wrong response code, want %v, got %v", code, res.Code)
		}
	}
}

// wantBody returns a function which checks that the response has the data in the body.
func wantBody(body string) wantFunc {
	return func(t testing.TB, res *httptest.ResponseRecorder) {
		if res.Body == nil {
			t.Errorf("body is nil, want %q", body)
			return
		}

		if !bytes.Equal(res.Body.Bytes(), []byte(body)) {
			t.Errorf("wrong response body, want:\n  %q\ngot:\n  %q", body, res.Body.Bytes())
		}
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

// TestResticHandler runs tests on the restic handler code, especially in append-only mode.
func TestResticHandler(t *testing.T) {
	// each test is a sequence of HTTP requests with (optional) tests for the response
	type TestRequest struct {
		req  *http.Request
		want []wantFunc
	}

	buf := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		t.Fatal(err)
	}
	randomID := hex.EncodeToString(buf)

	var tests = []struct {
		seq []TestRequest
	}{
		{
			// add a file, try to overwrite and delete it
			[]TestRequest{
				{
					req:  newRequest(t, "GET", "/config", nil),
					want: []wantFunc{wantCode(http.StatusNotFound)},
				},
				{
					req:  newRequest(t, "POST", "/config", strings.NewReader("foobar test config")),
					want: []wantFunc{wantCode(http.StatusOK)},
				},
				{
					req: newRequest(t, "GET", "/config", nil),
					want: []wantFunc{
						wantCode(http.StatusOK),
						wantBody("foobar test config"),
					},
				},
				{
					req:  newRequest(t, "POST", "/config", strings.NewReader("other config")),
					want: []wantFunc{wantCode(http.StatusForbidden)},
				},
				{
					req: newRequest(t, "GET", "/config", nil),
					want: []wantFunc{
						wantCode(http.StatusOK),
						wantBody("foobar test config"),
					},
				},
				{
					req:  newRequest(t, "DELETE", "/config", nil),
					want: []wantFunc{wantCode(http.StatusForbidden)},
				},
				{
					req: newRequest(t, "GET", "/config", nil),
					want: []wantFunc{
						wantCode(http.StatusOK),
						wantBody("foobar test config"),
					},
				},
			},
		},
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
	if err != nil {
		t.Fatal(err)
	}

	// make sure the tempdir is properly removed
	defer func() {
		err := os.RemoveAll(tempdir)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// globally set append-only mode
	appendOnly = true

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
