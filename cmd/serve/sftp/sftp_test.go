// Serve sftp tests set up a server and run the integration tests
// for the sftp remote against it.
//
// We skip tests on platforms with troublesome character mappings

//go:build !windows && !darwin && !plan9

package sftp

import (
	"context"
	"strings"
	"testing"

	"github.com/pkg/sftp"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBindAddress = "localhost:0"
	testUser        = "testuser"
	testPass        = "testpass"
)

// check interfaces
var (
	_ sftp.FileReader       = vfsHandler{}
	_ sftp.FileWriter       = vfsHandler{}
	_ sftp.FileCmder        = vfsHandler{}
	_ sftp.FileLister       = vfsHandler{}
	_ sftp.StatVFSFileCmder = vfsHandler{}
)

// TestSftp runs the sftp server then runs the unit tests for the
// sftp remote against it.
func TestSftp(t *testing.T) {
	// Configure and start the server
	start := func(f fs.Fs) (configmap.Simple, func()) {
		opt := Opt
		opt.ListenAddr = testBindAddress
		opt.User = testUser
		opt.Pass = testPass

		w, err := newServer(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
		require.NoError(t, err)
		go func() {
			require.NoError(t, w.Serve())
		}()

		// Read the host and port we started on
		addr := w.Addr().String()
		colon := strings.LastIndex(addr, ":")

		// Config for the backend we'll use to connect to the server
		config := configmap.Simple{
			"type": "sftp",
			"user": testUser,
			"pass": obscure.MustObscure(testPass),
			"host": addr[:colon],
			"port": addr[colon+1:],
		}

		// return a stop function
		return config, func() {
			assert.NoError(t, w.Shutdown())
		}
	}

	servetest.Run(t, "sftp", start)
}

// TestNewServerPerServerAuthProxy checks that a per-server proxyOpt.AuthProxy
// enables proxy mode even when the process-global proxy.Opt.AuthProxy is empty,
// which is the normal case when the server is configured via serve/start.
func TestNewServerPerServerAuthProxy(t *testing.T) {
	// Ensure the global is empty so we only test the per-server option.
	assert.Equal(t, "", proxy.Opt.AuthProxy)

	opt := Opt
	opt.ListenAddr = testBindAddress

	proxyOpt := proxy.Opt
	proxyOpt.AuthProxy = "/path/to/auth/proxy"

	w, err := newServer(context.Background(), nil, &opt, &vfscommon.Opt, &proxyOpt)
	require.NoError(t, err)
	// Shutdown waits for Serve to finish, so Serve must be running first.
	go func() {
		assert.NoError(t, w.Serve())
	}()
	defer func() {
		assert.NoError(t, w.Shutdown())
	}()
	assert.True(t, w.provider.IsProxy(), "expected auth proxy to be enabled by per-server option")
	assert.Nil(t, w.provider.VFS(), "expected no fixed VFS when auth proxy is in use")
}

func TestRc(t *testing.T) {
	servetest.TestRc(t, rc.Params{
		"type":           "sftp",
		"user":           "test",
		"pass":           obscure.MustObscure("test"),
		"vfs_cache_mode": "off",
	})
}
