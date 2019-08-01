// Serve ftp tests set up a server and run the integration tests
// for the ftp remote against it.
//
// We skip tests on platforms with troublesome character mappings

//+build !windows,!darwin,!plan9

package ftp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	ftp "github.com/goftp/server"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testHOST             = "localhost"
	testPORT             = "51780"
	testPASSIVEPORTRANGE = "30000-32000"
)

// TestFTP runs the ftp server then runs the unit tests for the
// ftp remote against it.
func TestFTP(t *testing.T) {
	opt := DefaultOpt
	opt.ListenAddr = testHOST + ":" + testPORT
	opt.PassivePorts = testPASSIVEPORTRANGE
	opt.BasicUser = "rclone"
	opt.BasicPass = "password"

	fstest.Initialise()

	fremote, _, clean, err := fstest.RandomRemote(*fstest.RemoteName, *fstest.SubDir)
	assert.NoError(t, err)
	defer clean()

	err = fremote.Mkdir(context.Background(), "")
	assert.NoError(t, err)

	// Start the server
	w, err := newServer(fremote, &opt)
	assert.NoError(t, err)

	go func() {
		err := w.serve()
		if err != ftp.ErrServerClosed {
			assert.NoError(t, err)
		}
	}()
	defer func() {
		err := w.close()
		assert.NoError(t, err)
	}()

	// Change directory to run the tests
	err = os.Chdir("../../../backend/ftp")
	assert.NoError(t, err, "failed to cd to ftp remote")

	// Run the ftp tests with an on the fly remote
	args := []string{"test"}
	if testing.Verbose() {
		args = append(args, "-v")
	}
	if *fstest.Verbose {
		args = append(args, "-verbose")
	}
	args = append(args, "-list-retries", fmt.Sprint(*fstest.ListRetries))
	args = append(args, "-remote", "ftptest:")
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(),
		"RCLONE_CONFIG_FTPTEST_TYPE=ftp",
		"RCLONE_CONFIG_FTPTEST_HOST="+testHOST,
		"RCLONE_CONFIG_FTPTEST_PORT="+testPORT,
		"RCLONE_CONFIG_FTPTEST_USER=rclone",
		"RCLONE_CONFIG_FTPTEST_PASS=0HU5Hx42YiLoNGJxppOOP3QTbr-KB_MP", // ./rclone obscure password
	)
	out, err := cmd.CombinedOutput()
	if len(out) != 0 {
		t.Logf("\n----------\n%s----------\n", string(out))
	}
	assert.NoError(t, err, "Running ftp integration tests")
}

func TestFindID(t *testing.T) {
	id, err := findID([]byte("TestFindID("))
	require.NoError(t, err)
	// id should be the argument to this function
	assert.Equal(t, fmt.Sprintf("%p", t), id)
}
