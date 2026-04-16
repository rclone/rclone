// Serve smb tests set up a server and run the integration tests
// for the smb remote against it.
//
// We skip tests on platforms with troublesome character mappings

//go:build !windows && !darwin && !plan9 && !(linux && (386 || arm || mips || mipsle))

package smb

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBindAddress = "localhost:0"
	testUser        = "testuser"
	testPass        = "testpass"
)

// startServer creates and starts an SMB server for testing.
// It returns a configmap for the SMB backend to connect and a cleanup function.
func startServer(t *testing.T, f fs.Fs) (configmap.Simple, func()) {
	opt := Opt
	opt.ListenAddr = testBindAddress
	opt.User = testUser
	opt.Pass = testPass
	opt.ShareName = "rclone"

	// Use writes cache mode so that random-access writes (OpenWriterAt) work
	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeWrites

	w, err := newServer(context.Background(), f, &opt, &vfsOpt, &proxy.Opt)
	require.NoError(t, err)
	go func() {
		require.NoError(t, w.Serve())
	}()

	// Read the host and port we started on
	addr := w.Addr().String()
	colon := strings.LastIndex(addr, ":")

	// Config for the backend we'll use to connect to the server
	config := configmap.Simple{
		"type": "smb",
		"user": testUser,
		"pass": obscure.MustObscure(testPass),
		"host": addr[:colon],
		"port": addr[colon+1:],
	}

	return config, func() {
		assert.NoError(t, w.Shutdown())
	}
}

// TestSmb runs the smb server then runs the unit tests for the
// smb remote against it.
//
// The SMB backend is bucket-based (share = bucket) and requires the
// share name in the remote path. servetest.Run passes "servesmbtest:"
// which has no share name, so we use a custom test runner that passes
// "servesmbtest:rclone" instead.
func TestSmb(t *testing.T) {
	fstest.Initialise()

	t.Run("Normal", func(t *testing.T) {
		runSMBBackendTests(t, false)
	})
	t.Run("AuthProxy", func(t *testing.T) {
		runSMBBackendTests(t, true)
	})
}

// runSMBBackendTests starts the server and runs the SMB backend integration tests
// with the share name included in the remote path.
func runSMBBackendTests(t *testing.T, useProxy bool) {
	fremote, _, clean, err := fstest.RandomRemote()
	assert.NoError(t, err)
	defer clean()

	err = fremote.Mkdir(context.Background(), "")
	assert.NoError(t, err)

	f := fremote
	if useProxy {
		// If using a proxy don't pass in the backend
		f = nil

		// the backend config will be made by the proxy
		prog, err := filepath.Abs("../servetest/proxy_code.go")
		require.NoError(t, err)
		cmd := "go run " + prog + " " + fremote.Root()

		// FIXME this is untidy setting a global variable!
		proxy.Opt.AuthProxy = cmd
		defer func() {
			proxy.Opt.AuthProxy = ""
		}()
	}
	config, cleanup := startServer(t, f)
	defer cleanup()

	// Change directory to run the tests
	cwd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir("../../../backend/smb")
	require.NoError(t, err, "failed to cd to smb backend")
	defer func() {
		require.NoError(t, os.Chdir(cwd))
	}()

	// Run the backend tests with the share name in the remote path.
	// The SMB backend expects "remoteName:shareName" format.
	args := []string{"test"}
	if testing.Verbose() {
		args = append(args, "-v")
	}
	if *fstest.Verbose {
		args = append(args, "-verbose")
	}
	remoteName := "servesmbtest:rclone"
	args = append(args, "-remote", remoteName)
	args = append(args, "-list-retries", fmt.Sprint(*fstest.ListRetries))
	cmd := exec.Command("go", args...)

	// Configure the backend with environment variables
	cmd.Env = os.Environ()
	prefix := "RCLONE_CONFIG_SERVESMBTEST_"
	for k, v := range config {
		cmd.Env = append(cmd.Env, prefix+strings.ToUpper(k)+"="+v)
	}

	// Run the test
	out, err := cmd.CombinedOutput()
	if len(out) != 0 {
		t.Logf("\n----------\n%s----------\n", string(out))
	}
	assert.NoError(t, err, "Running smb integration tests")
}

func TestRc(t *testing.T) {
	servetest.TestRc(t, rc.Params{
		"type":           "smb",
		"user":           "test",
		"pass":           obscure.MustObscure("test"),
		"vfs_cache_mode": "off",
	})
}
