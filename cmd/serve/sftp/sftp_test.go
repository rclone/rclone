// Serve sftp tests set up a server and run the integration tests
// for the sftp remote against it.
//
// We skip tests on platforms with troublesome character mappings

//+build !windows,!darwin,!plan9

package sftp

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/pkg/sftp"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
)

const (
	testBindAddress = "localhost:0"
	testUser        = "testuser"
	testPass        = "testpass"
)

// check interfaces
var (
	_ sftp.FileReader = vfsHandler{}
	_ sftp.FileWriter = vfsHandler{}
	_ sftp.FileCmder  = vfsHandler{}
	_ sftp.FileLister = vfsHandler{}
)

// TestSftp runs the sftp server then runs the unit tests for the
// sftp remote against it.
func TestSftp(t *testing.T) {
	fstest.Initialise()

	fremote, _, clean, err := fstest.RandomRemote(*fstest.RemoteName, *fstest.SubDir)
	assert.NoError(t, err)
	defer clean()

	err = fremote.Mkdir(context.Background(), "")
	assert.NoError(t, err)

	opt := DefaultOpt
	opt.ListenAddr = testBindAddress
	opt.User = testUser
	opt.Pass = testPass

	// Start the server
	w := newServer(fremote, &opt)
	assert.NoError(t, w.serve())
	defer func() {
		w.Close()
		w.Wait()
	}()

	// Change directory to run the tests
	err = os.Chdir("../../../backend/sftp")
	assert.NoError(t, err, "failed to cd to sftp backend")

	// Run the sftp tests with an on the fly remote
	args := []string{"test"}
	if testing.Verbose() {
		args = append(args, "-v")
	}
	if *fstest.Verbose {
		args = append(args, "-verbose")
	}
	args = append(args, "-remote", "sftptest:")
	cmd := exec.Command("go", args...)
	addr := w.Addr()
	colon := strings.LastIndex(addr, ":")
	if colon < 0 {
		panic("need a : in the address: " + addr)
	}
	host, port := addr[:colon], addr[colon+1:]
	cmd.Env = append(os.Environ(),
		"RCLONE_CONFIG_SFTPTEST_TYPE=sftp",
		"RCLONE_CONFIG_SFTPTEST_HOST="+host,
		"RCLONE_CONFIG_SFTPTEST_PORT="+port,
		"RCLONE_CONFIG_SFTPTEST_USER="+testUser,
		"RCLONE_CONFIG_SFTPTEST_PASS="+obscure.MustObscure(testPass),
	)
	out, err := cmd.CombinedOutput()
	if len(out) != 0 {
		t.Logf("\n----------\n%s----------\n", string(out))
	}
	assert.NoError(t, err, "Running sftp integration tests")
}
