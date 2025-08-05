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
	"time"

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
		// Create a context with timeout to prevent test from hanging
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

		opt := Opt
		opt.ListenAddr = testBindAddress
		opt.User = testUser
		opt.Pass = testPass

		w, err := newServer(ctx, f, &opt, &vfscommon.Opt, &proxy.Opt)
		require.NoError(t, err)

		// Channel to capture errors from the server goroutine
		serverErrors := make(chan error, 1)

		go func() {
			fs.Debugf(nil, "SFTP test server starting")
			err := w.Serve()
			if err != nil {
				fs.Debugf(nil, "SFTP test server error: %v", err)
				serverErrors <- err
			}
			close(serverErrors)
			fs.Debugf(nil, "SFTP test server stopped")
		}()

		// Read the host and port we started on
		addr := w.Addr().String()
		colon := strings.LastIndex(addr, ":")
		fs.Debugf(nil, "SFTP test server listening on %v", addr)

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
			fs.Debugf(nil, "SFTP test server shutting down")

			// Set a timeout for the shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			// Create a channel to signal when shutdown is complete
			shutdownDone := make(chan struct{})

			// Start the shutdown in a goroutine
			go func() {
				defer close(shutdownDone)

				// Cancel the server context to abort any hanging operations
				cancel()

				// Shutdown the server and check for errors
				shutdownErr := w.Shutdown()
				if shutdownErr != nil {
					fs.Errorf(nil, "SFTP test server shutdown error: %v", shutdownErr)
				}
				assert.NoError(t, shutdownErr)
			}()

			// Wait for shutdown to complete or timeout
			select {
			case <-shutdownDone:
				fs.Debugf(nil, "SFTP test server shutdown completed successfully")
			case <-shutdownCtx.Done():
				fs.Errorf(nil, "SFTP test server shutdown timed out after 30 seconds")
				t.Error("SFTP server shutdown timed out")
			}

			// Check if the server reported any errors
			select {
			case err, ok := <-serverErrors:
				if ok && err != nil {
					fs.Errorf(nil, "SFTP test server error during run: %v", err)
					t.Errorf("SFTP server error: %v", err)
				}
			default:
				// No error, continue
			}

			fs.Debugf(nil, "SFTP test server shutdown complete")
		}
	}

	servetest.Run(t, "sftp", start)
}

func TestRc(t *testing.T) {
	// Create a test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Use context in the test
	t.Run("RcTest", func(t *testing.T) {
		// Testing with a shorter timeout to fail faster if there's an issue
		testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
		defer testCancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			servetest.TestRc(t, rc.Params{
				"type":           "sftp",
				"user":           "test",
				"pass":           obscure.MustObscure("test"),
				"vfs_cache_mode": "off",
			})
		}()

		// Wait for either test completion or timeout
		select {
		case <-done:
			// Test completed normally
		case <-testCtx.Done():
			t.Errorf("TestRc timed out: %v", testCtx.Err())
		}
	})
}
