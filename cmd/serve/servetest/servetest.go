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

	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
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
func run(t *testing.T, name string, start StartFn, useProxy bool) {
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
		proxyflags.Opt.AuthProxy = cmd
		defer func() {
			proxyflags.Opt.AuthProxy = ""
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
	remoteName := "serve" + name + "test:"
	if *subRun != "" {
		args = append(args, "-run", *subRun)
	}
	args = append(args, "-remote", remoteName)
	args = append(args, "-list-retries", fmt.Sprint(*fstest.ListRetries))
	cmd := exec.Command("go", args...)

	// Configure the backend with environment variables
	cmd.Env = os.Environ()
	prefix := "RCLONE_CONFIG_" + strings.ToUpper(remoteName[:len(remoteName)-1]) + "_"
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
// it.
func Run(t *testing.T, name string, start StartFn) {
	fstest.Initialise()
	t.Run("Normal", func(t *testing.T) {
		run(t, name, start, false)
	})
	t.Run("AuthProxy", func(t *testing.T) {
		run(t, name, start, true)
	})
}
