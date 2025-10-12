// Serve ftp tests set up a server and run the integration tests
// for the ftp remote against it.
//
// We skip tests on platforms with troublesome character mappings

//go:build !windows && !darwin && !plan9

package ftp

import (
	"context"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/israce"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
)

const (
	testHOST             = "localhost"
	testPORT             = "51780"
	testPASSIVEPORTRANGE = "30000-32000"
	testUSER             = "rclone"
	testPASS             = "password"
)

// TestFTP runs the ftp server then runs the unit tests for the
// ftp remote against it.
func TestFTP(t *testing.T) {
	// Configure and start the server
	start := func(f fs.Fs) (configmap.Simple, func()) {
		opt := Opt
		opt.ListenAddr = testHOST + ":" + testPORT
		opt.PassivePorts = testPASSIVEPORTRANGE
		opt.User = testUSER
		opt.Pass = testPASS

		w, err := newServer(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
		assert.NoError(t, err)

		quit := make(chan struct{})
		go func() {
			assert.NoError(t, w.Serve())
			close(quit)
		}()

		// Config for the backend we'll use to connect to the server
		config := configmap.Simple{
			"type": "ftp",
			"host": testHOST,
			"port": testPORT,
			"user": testUSER,
			"pass": obscure.MustObscure(testPASS),
		}

		return config, func() {
			err := w.Shutdown()
			assert.NoError(t, err)
			<-quit
		}
	}

	servetest.Run(t, "ftp", start)
}

func TestRc(t *testing.T) {
	if israce.Enabled {
		t.Skip("Skipping under race detector as underlying library is racy")
	}
	servetest.TestRc(t, rc.Params{
		"type":           "ftp",
		"vfs_cache_mode": "off",
	})
}
