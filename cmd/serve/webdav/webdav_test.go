// Serve webdav tests set up a server and run the integration tests
// for the webdav remote against it.
//
// We skip tests on platforms with troublesome character mappings

//+build !windows,!darwin,go1.9

package webdav

import (
	"os"
	"os/exec"
	"testing"

	_ "github.com/ncw/rclone/backend/local"
	"github.com/ncw/rclone/cmd/serve/httplib"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/webdav"
)

const (
	testBindAddress = "localhost:51778"
	testURL         = "http://" + testBindAddress + "/"
)

// check interfaces
var (
	_ os.FileInfo         = FileInfo{nil}
	_ webdav.ETager       = FileInfo{nil}
	_ webdav.ContentTyper = FileInfo{nil}
)

// TestWebDav runs the webdav server then runs the unit tests for the
// webdav remote against it.
func TestWebDav(t *testing.T) {
	opt := httplib.DefaultOpt
	opt.ListenAddr = testBindAddress

	fstest.Initialise()

	fremote, _, clean, err := fstest.RandomRemote(*fstest.RemoteName, *fstest.SubDir)
	assert.NoError(t, err)
	defer clean()

	err = fremote.Mkdir("")
	assert.NoError(t, err)

	// Start the server
	w := newWebDAV(fremote, &opt)
	assert.NoError(t, w.serve())
	defer func() {
		w.Close()
		w.Wait()
	}()

	// Change directory to run the tests
	err = os.Chdir("../../../backend/webdav")
	assert.NoError(t, err, "failed to cd to webdav remote")

	// Run the webdav tests with an on the fly remote
	args := []string{"test"}
	if testing.Verbose() {
		args = append(args, "-v")
	}
	if *fstest.Verbose {
		args = append(args, "-verbose")
	}
	args = append(args, "-remote", "webdavtest:")
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(),
		"RCLONE_CONFIG_WEBDAVTEST_TYPE=webdav",
		"RCLONE_CONFIG_WEBDAVTEST_URL="+testURL,
		"RCLONE_CONFIG_WEBDAVTEST_VENDOR=other",
	)
	out, err := cmd.CombinedOutput()
	if len(out) != 0 {
		t.Logf("\n----------\n%s----------\n", string(out))
	}
	assert.NoError(t, err, "Running webdav integration tests")
}
