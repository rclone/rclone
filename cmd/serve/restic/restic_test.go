// Serve restic tests set up a server and run the integration tests
// for restic against it.

package restic

import (
	"context"
	"os"
	"os/exec"
	"testing"

	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/cmd/serve/httplib"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
)

const (
	testBindAddress = "localhost:0"
	resticSource    = "../../../../../restic/restic"
)

// TestRestic runs the restic server then runs the unit tests for the
// restic remote against it.
func TestRestic(t *testing.T) {
	_, err := os.Stat(resticSource)
	if err != nil {
		t.Skipf("Skipping test as restic source not found: %v", err)
	}

	opt := httplib.DefaultOpt
	opt.ListenAddr = testBindAddress

	fstest.Initialise()

	fremote, _, clean, err := fstest.RandomRemote()
	assert.NoError(t, err)
	defer clean()

	err = fremote.Mkdir(context.Background(), "")
	assert.NoError(t, err)

	// Start the server
	w := NewServer(fremote, &opt)
	assert.NoError(t, w.Serve())
	defer func() {
		w.Close()
		w.Wait()
	}()

	// Change directory to run the tests
	err = os.Chdir(resticSource)
	assert.NoError(t, err, "failed to cd to restic source code")

	// Run the restic tests
	runTests := func(path string) {
		args := []string{"test", "./internal/backend/rest", "-run", "TestBackendRESTExternalServer", "-count=1"}
		if testing.Verbose() {
			args = append(args, "-v")
		}
		cmd := exec.Command("go", args...)
		cmd.Env = append(os.Environ(),
			"RESTIC_TEST_REST_REPOSITORY=rest:"+w.Server.URL()+path,
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
		{"", ""},
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
		got := makeRemote(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}
