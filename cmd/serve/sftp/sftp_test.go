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
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/require"
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
	// Configure and start the server
	start := func(f fs.Fs) (configmap.Simple, func()) {
		opt := Opt
		opt.ListenAddr = testBindAddress
		opt.User = testUser
		opt.Pass = testPass

		w := newServer(context.Background(), f, &opt)
		require.NoError(t, w.serve())

		// Read the host and port we started on
		addr := w.Addr()
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
			w.Close()
			w.Wait()
		}
	}

	servetest.Run(t, "sftp", start)
}
