// Package servetest provides infrastructure for running loopback
// tests of "rclone serve backend:" against the backend integration
// tests.
package servetest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/testserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var subRun = flag.String("sub-run", "", "pass this to the -run command of the backend tests")

// StartFn describes the callback which should start the server with
// the Fs passed in.
// It should return a config for the backend used to connect to the
// server and a clean up function
type StartFn func(f fs.Fs) (configmap.Simple, func())

// run runs the server then runs the unit tests for the remote against
// it.
//
// If backingRemote is non-empty (e.g. "TestS3Minio:") it is used as the
// backing Fs that the server wraps, instead of a fresh local directory.
// The matching fstest/testserver/init.d script is started automatically.
func run(t *testing.T, name string, start StartFn, useProxy bool, backingRemote, root string) {
	fremote, clean, err := makeBackingFs(backingRemote)
	require.NoError(t, err)
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
	config, cleanup := start(f)
	defer cleanup()

	// Change directory to run the tests
	cwd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir("../../../backend/" + name)
	require.NoError(t, err, "failed to cd to "+name+" backend")
	defer func() {
		// Change back to the old directory
		require.NoError(t, os.Chdir(cwd))
	}()

	// Run the backend tests with an on the fly remote
	args := []string{"test"}
	if testing.Verbose() {
		args = append(args, "-v")
	}
	if *fstest.Verbose {
		args = append(args, "-verbose")
	}
	// root lets a server that exports a named prefix (e.g. an smb share) run the
	// backend tests under that prefix rather than at the connection root.
	configName := "serve" + name + "test"
	remoteName := configName + ":" + root
	if *subRun != "" {
		args = append(args, "-run", *subRun)
	}
	args = append(args, "-remote", remoteName)
	args = append(args, "-list-retries", fmt.Sprint(*fstest.ListRetries))
	cmd := exec.Command("go", args...)

	// Configure the backend with environment variables
	cmd.Env = os.Environ()
	prefix := "RCLONE_CONFIG_" + strings.ToUpper(configName) + "_"
	for k, v := range config {
		cmd.Env = append(cmd.Env, prefix+strings.ToUpper(k)+"="+v)
	}

	// Run the test
	out, err := cmd.CombinedOutput()
	if len(out) != 0 {
		t.Logf("\n----------\n%s----------\n", string(out))
	}
	assert.NoError(t, err, "Running "+name+" integration tests")
}

// Run runs the server then runs the unit tests for the remote against
// it. The backing Fs is a fresh local directory.
func Run(t *testing.T, name string, start StartFn) {
	RunWithBackend(t, name, start, "")
}

// RunWithBackend behaves like Run but uses the supplied remote (e.g.
// "TestS3Minio:") as the backing Fs that the server wraps, instead of a
// fresh local directory. When backingRemote is empty it is equivalent to
// Run.
//
// AuthProxy is only run when backingRemote is empty: the test proxy in
// proxy_code.go hardcodes type=local, so it cannot be used with a
// non-local backing.
func RunWithBackend(t *testing.T, name string, start StartFn, backingRemote string) {
	fstest.Initialise()
	t.Run("Normal", func(t *testing.T) {
		run(t, name, start, false, backingRemote, "")
	})
	if backingRemote == "" {
		t.Run("AuthProxy", func(t *testing.T) {
			run(t, name, start, true, backingRemote, "")
		})
	}
}

// RunNoAuthProxy runs the server then the backend integration tests against it,
// but without the auth-proxy sub-test. Use it for a server that does not (yet)
// implement the auth proxy. root is the remote prefix the tests run under (e.g.
// an smb share name), or "" for the connection root.
func RunNoAuthProxy(t *testing.T, name, root string, start StartFn) {
	fstest.Initialise()
	t.Run("Normal", func(t *testing.T) {
		run(t, name, start, false, "", root)
	})
}

// makeBackingFs returns the Fs that the server should wrap, plus a
// cleanup function. When backingRemote is empty a fresh local temporary
// directory is used (current behaviour). Otherwise the matching test
// server is started and a random subdirectory of that remote is used.
func makeBackingFs(backingRemote string) (fs.Fs, func(), error) {
	if backingRemote == "" {
		fremote, _, clean, err := fstest.RandomRemote()
		return fremote, clean, err
	}

	stopServer, err := testserver.Start(backingRemote)
	if err != nil {
		return nil, func() {}, fmt.Errorf("failed to start test server for %q: %w", backingRemote, err)
	}

	subRemoteName, _, err := fstest.RandomRemoteName(backingRemote)
	if err != nil {
		stopServer()
		return nil, func() {}, err
	}

	fremote, err := fs.NewFs(context.Background(), subRemoteName)
	if err != nil {
		stopServer()
		return nil, func() {}, err
	}

	clean := func() {
		fstest.Purge(fremote)
		stopServer()
	}
	return fremote, clean, nil
}
