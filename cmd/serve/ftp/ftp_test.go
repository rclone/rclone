// Serve ftp tests set up a server and run the integration tests
// for the ftp remote against it.
//
// We skip tests on platforms with troublesome character mappings

//+build !windows,!darwin,!plan9,go1.13

package ftp

import (
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	ftp "goftp.io/server/core"
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
		opt := DefaultOpt
		opt.ListenAddr = testHOST + ":" + testPORT
		opt.PassivePorts = testPASSIVEPORTRANGE
		opt.BasicUser = testUSER
		opt.BasicPass = testPASS

		w, err := newServer(f, &opt)
		assert.NoError(t, err)

		quit := make(chan struct{})
		go func() {
			err := w.serve()
			close(quit)
			if err != ftp.ErrServerClosed {
				assert.NoError(t, err)
			}
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
			err := w.close()
			assert.NoError(t, err)
			<-quit
		}
	}

	servetest.Run(t, "ftp", start)
}
