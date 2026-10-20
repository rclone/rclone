package restic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	ctx := context.Background()
	configfile.Install()
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
	tempdir := t.TempDir()

	// set append-only mode
	opt := newOpt()
	opt.AppendOnly = true

	// make a new file system in the temp dir
	f := cmd.NewFsSrc([]string{tempdir})
	s, err := newServer(ctx, f, &opt)
	require.NoError(t, err)
	router := s.server.Router()

	// create the repo
	checkRequest(t, router.ServeHTTP,
		newRequest(t, "POST", "/?create=true", nil),
		[]wantFunc{wantCode(http.StatusOK)})

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			for i, seq := range test.seq {
				t.Logf("request %v: %v %v", i, seq.req.Method, seq.req.URL.Path)
				checkRequest(t, router.ServeHTTP, seq.req, seq.want)
			}
		})
	}
}

func TestAppendOnlyFailsClosedOnLookupError(t *testing.T) {
	ctx := context.Background()
	configfile.Install()

	underlying := cmd.NewFsSrc([]string{t.TempDir()})
	f := &newObjectErrorFs{
		Fs:  underlying,
		err: errors.New("lookup failed"),
	}
	opt := newOpt()
	opt.AppendOnly = true
	s, err := newServer(ctx, f, &opt)
	require.NoError(t, err)

	req := newRequest(t, "POST", "/config", strings.NewReader("replacement"))
	checkRequest(t, s.server.Router().ServeHTTP, req, []wantFunc{wantCode(http.StatusInternalServerError)})

	_, err = underlying.NewObject(ctx, "config")
	assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
}

func TestAppendOnlyConcurrentUploads(t *testing.T) {
	configfile.Install()
	opt := newOpt()
	opt.AppendOnly = true
	f := cmd.NewFsSrc([]string{t.TempDir()})
	s, err := newServer(context.Background(), f, &opt)
	require.NoError(t, err)
	handler := s.server.Router().ServeHTTP

	const uploads = 10
	start := make(chan struct{})
	results := make(chan int, uploads)
	for range uploads {
		go func() {
			req := newRequest(t, "POST", "/config", strings.NewReader("config data"))
			recorder := httptest.NewRecorder()
			<-start
			handler(recorder, req)
			results <- recorder.Code
		}()
	}
	close(start)

	succeeded := 0
	for range uploads {
		if code := <-results; code == http.StatusOK {
			succeeded++
		} else {
			assert.Equal(t, http.StatusForbidden, code)
		}
	}
	assert.Equal(t, 1, succeeded)
	checkRequest(t, handler, newRequest(t, "GET", "/config", nil),
		[]wantFunc{wantCode(http.StatusOK), wantBody("config data")})
}
