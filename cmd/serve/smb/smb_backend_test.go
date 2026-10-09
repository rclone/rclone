// Serve smb tests set up a server and run the integration tests for the
// smb backend against it.
//
// We skip tests on platforms with troublesome character mappings.

//go:build !windows && !darwin

package smb

import (
	"context"
	"net"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/require"
)

// TestSMB runs the smb server then runs the integration tests for the smb
// backend against it over loopback.
func TestSMB(t *testing.T) {
	const (
		user = "rclone"
		pass = "password"
	)
	const share = "rclone"
	start := func(f fs.Fs) (configmap.Simple, func()) {
		opt := Opt
		opt.ListenAddr = "localhost:0"
		opt.ShareName = share
		opt.User = user
		opt.Pass = pass

		// The integration tests write, which needs a VFS cache.
		vfsOpt := vfscommon.Opt
		vfsOpt.CacheMode = vfscommon.CacheModeFull
		// SMB shares are conventionally case-insensitive (the smb backend
		// defaults to case_insensitive=true), so serve the VFS that way too —
		// otherwise name lookups that vary case fail against a case-sensitive
		// backing store like Linux local.
		vfsOpt.CaseInsensitive = true

		s, err := newServer(context.Background(), f, &opt, &vfsOpt)
		require.NoError(t, err)
		go func() { _ = s.Serve() }()

		host, port, err := net.SplitHostPort(s.Addr().String())
		require.NoError(t, err)

		// Config for the smb backend we'll use to connect to the server
		config := configmap.Simple{
			"type": "smb",
			"host": host,
			"port": port,
			"user": user,
			"pass": obscure.MustObscure(pass),
		}

		return config, func() {
			require.NoError(t, s.Shutdown())
		}
	}

	// Run the backend tests under our single share.
	servetest.RunNoAuthProxy(t, "smb", share, start)
}
