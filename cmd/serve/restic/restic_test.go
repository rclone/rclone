// Serve restic tests set up a server and run the integration tests
// for restic against it.

package restic

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBindAddress = "localhost:0"
	resticSource    = "../../../../../restic/restic"
)

func newOpt() Options {
	opt := DefaultOpt
	opt.HTTP.ListenAddr = []string{testBindAddress}
	return opt
}

// TestRestic runs the restic server then runs the unit tests for the
// restic remote against it.
//
// Requires the restic source code in the location indicated by resticSource.
func TestResticIntegration(t *testing.T) {
	ctx := context.Background()
	_, err := os.Stat(resticSource)
	if err != nil {
		t.Skipf("Skipping test as restic source not found: %v", err)
	}

	opt := newOpt()

	fstest.Initialise()

	fremote, _, clean, err := fstest.RandomRemote()
	assert.NoError(t, err)
	defer clean()

	err = fremote.Mkdir(context.Background(), "")
	assert.NoError(t, err)

	// Start the server
	s, err := newServer(ctx, fremote, &opt)
	require.NoError(t, err)
	testURL := s.Server.URLs()[0]
	defer func() {
		_ = s.Shutdown()
	}()

	// Change directory to run the tests
	err = os.Chdir(resticSource)
	require.NoError(t, err, "failed to cd to restic source code")

	// Run the restic tests
	runTests := func(path string) {
		args := []string{"test", "./internal/backend/rest", "-run", "TestBackendRESTExternalServer", "-count=1"}
		if testing.Verbose() {
			args = append(args, "-v")
		}
		cmd := exec.Command("go", args...)
		cmd.Env = append(os.Environ(),
			"RESTIC_TEST_REST_REPOSITORY=rest:"+testURL+path,
			"GO111MODULE=on",
		)
		out, err := cmd.CombinedOutput()
		if len(out) != 0 {
			t.Logf("\n----------\n%s----------\n", string(out))
		}
		assert.NoError(t, err, "Running restic integration tests")
	}

	// Run the tests with no path
	runTests("")
	//... and again with a path
	runTests("potato/sausage/")

}

func TestMakeRemote(t *testing.T) {
	for _, test := range []struct {
		in, want string
	}{
		{"/", ""},
		{"/data", "data"},
		{"/data/", "data"},
		{"/data/1", "data/1"},
		{"/data/12", "data/12/12"},
		{"/data/123", "data/12/123"},
		{"/data/123/", "data/12/123"},
		{"/keys", "keys"},
		{"/keys/1", "keys/1"},
		{"/keys/12", "keys/12"},
		{"/keys/123", "keys/123"},
	} {
		r := httptest.NewRequest("GET", test.in, nil)
		w := httptest.NewRecorder()
		next := http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			remote, ok := request.Context().Value(ContextRemoteKey).(string)
			assert.True(t, ok, "Failed to get remote from context")
			assert.Equal(t, test.want, remote, test.in)
		})
		got := WithRemote(next)
		got.ServeHTTP(w, r)
	}
}

type listErrorFs struct {
	fs.Fs
}

func (f *listErrorFs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	return fs.DirEntries{}, errors.New("oops")
}

func TestListErrors(t *testing.T) {
	ctx := context.Background()
	// setup rclone with a local backend in a temporary directory
	tempdir := t.TempDir()
	opt := newOpt()

	// make a new file system in the temp dir
	f := &listErrorFs{Fs: cmd.NewFsSrc([]string{tempdir})}
	s, err := newServer(ctx, f, &opt)
	require.NoError(t, err)
	router := s.Server.Router()

	req := newRequest(t, "GET", "/test/snapshots/", nil)
	checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusInternalServerError)})
}

type newObjectErrorFs struct {
	fs.Fs
	err error
}

func (f *newObjectErrorFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return nil, f.err
}

func TestServeErrors(t *testing.T) {
	ctx := context.Background()
	// setup rclone with a local backend in a temporary directory
	tempdir := t.TempDir()
	opt := newOpt()

	// make a new file system in the temp dir
	f := &newObjectErrorFs{Fs: cmd.NewFsSrc([]string{tempdir})}
	s, err := newServer(ctx, f, &opt)
	require.NoError(t, err)
	router := s.Server.Router()

	f.err = errors.New("oops")
	req := newRequest(t, "GET", "/test/config", nil)
	checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusInternalServerError)})

	f.err = fs.ErrorObjectNotFound
	checkRequest(t, router.ServeHTTP, req, []wantFunc{wantCode(http.StatusNotFound)})
}
