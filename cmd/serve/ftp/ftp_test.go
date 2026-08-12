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
	"github.com/stretchr/testify/require"
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

// TestCheckPasswd checks the builtin authentication accepts only the
// configured credentials, with an empty configured password accepting any
// password.
func TestCheckPasswd(t *testing.T) {
	for _, test := range []struct {
		name    string
		optUser string
		optPass string
		user    string
		pass    string
		want    bool
	}{
		{name: "good", optUser: "user", optPass: "pass", user: "user", pass: "pass", want: true},
		{name: "bad-pass", optUser: "user", optPass: "pass", user: "user", pass: "PASS", want: false},
		{name: "bad-user", optUser: "user", optPass: "pass", user: "USER", pass: "pass", want: false},
		{name: "wrong-length-pass", optUser: "user", optPass: "pass", user: "user", pass: "pass2", want: false},
		{name: "empty-configured-pass", optUser: "user", optPass: "", user: "user", pass: "anything", want: true},
		{name: "empty-configured-pass-bad-user", optUser: "user", optPass: "", user: "USER", pass: "anything", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := &driver{}
			d.opt.User = test.optUser
			d.opt.Pass = test.optPass
			ok, err := d.CheckPasswd(nil, test.user, test.pass)
			assert.NoError(t, err)
			assert.Equal(t, test.want, ok)
		})
	}
}

// TestNewServerPerServerAuthProxy checks that a per-server proxyOpt.AuthProxy
// enables proxy mode even when the process-global proxy.Opt.AuthProxy is empty,
// which is the normal case when the server is configured via serve/start.
func TestNewServerPerServerAuthProxy(t *testing.T) {
	// Ensure the global is empty so we only test the per-server option.
	assert.Equal(t, "", proxy.Opt.AuthProxy)

	opt := Opt
	opt.ListenAddr = testHOST + ":" + testPORT
	opt.PassivePorts = testPASSIVEPORTRANGE

	proxyOpt := proxy.Opt
	proxyOpt.AuthProxy = "/path/to/auth/proxy"

	d, err := newServer(context.Background(), nil, &opt, &vfscommon.Opt, &proxyOpt)
	require.NoError(t, err)
	defer d.provider.Shutdown()
	assert.True(t, d.provider.IsProxy(), "expected auth proxy to be enabled by per-server option")
	assert.Nil(t, d.provider.VFS(), "expected no fixed VFS when auth proxy is in use")
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
