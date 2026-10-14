// Serve ftp tests set up a server and run the integration tests
// for the ftp remote against it.
//
// We skip tests on platforms with troublesome character mappings

//go:build !windows && !darwin && !plan9

package ftp

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	ftpclient "github.com/jlaffaye/ftp"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/israce"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/vfs"
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

// TestMakeDirRoot checks that "MKD /" makes the directory being
// served. The ftp backend never sends this so TestFTP doesn't cover it.
func TestMakeDirRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	f, err := fs.NewFs(context.Background(), root)
	require.NoError(t, err)

	opt := Opt
	opt.ListenAddr = testHOST + ":" + testPORT
	opt.PassivePorts = testPASSIVEPORTRANGE

	d, err := newServer(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err)
	defer d.provider.Shutdown()

	require.NoError(t, d.MakeDir(nil, "/"))
	fi, err := os.Stat(root)
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	require.NoError(t, d.MakeDir(nil, "/dir"))
	fi, err = os.Stat(filepath.Join(root, "dir"))
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
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

// TestNewServerError checks that a server initialisation failure is
// returned as an error and does not leak the VFS it created.
func TestNewServerError(t *testing.T) {
	f, err := fs.NewFs(context.Background(), t.TempDir())
	require.NoError(t, err)

	opt := Opt
	opt.ListenAddr = testHOST + ":" + testPORT
	opt.PassivePorts = "not-a-port-range"

	before := vfs.ActiveCount()
	d, err := newServer(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.Error(t, err)
	assert.Nil(t, d)
	assert.Equal(t, before, vfs.ActiveCount(), "VFS leaked after failed server creation")
}

// TestAuthProxyTransferOutlivesCache checks transfers in progress
// carry on working when the auth proxy drops their VFS from its cache,
// as it does when a transfer takes longer than the cache expiry time.
func TestAuthProxyTransferOutlivesCache(t *testing.T) {
	const addr = "127.0.0.1:" + testPORT
	root := t.TempDir()
	contents := random.String(32 * 1024 * 1024)
	require.NoError(t, os.WriteFile(filepath.Join(root, "download.bin"), []byte(contents), 0666))

	prog, err := filepath.Abs("../servetest/proxy_code.go")
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = addr
	opt.PassivePorts = testPASSIVEPORTRANGE
	proxyOpt := proxy.Opt
	proxyOpt.AuthProxy = "go run " + prog + " " + root
	d, err := newServer(context.Background(), nil, &opt, &vfscommon.Opt, &proxyOpt)
	require.NoError(t, err)
	quit := make(chan struct{})
	go func() {
		assert.NoError(t, d.Serve())
		close(quit)
	}()
	defer func() {
		assert.NoError(t, d.Shutdown())
		<-quit
	}()

	var c *ftpclient.ServerConn
	require.Eventually(t, func() bool {
		c, err = ftpclient.Dial(addr)
		return err == nil
	}, 10*time.Second, 10*time.Millisecond)
	defer func() { _ = c.Quit() }()
	require.NoError(t, c.Login(testUSER, testPASS))

	// Only the IP of the address is used, which is the same as the client's
	_, vfsKey, err := d.provider.Proxy().Call(testUSER, testPASS, false, addr)
	require.NoError(t, err)

	// expire waits for a transfer to be using the VFS, then drops
	// everything from the proxy's cache as if it had expired.
	expire := func(t *testing.T) {
		var VFS *vfs.VFS
		require.Eventually(t, func() bool {
			VFS = d.provider.Proxy().Get(vfsKey)
			return VFS != nil && VFS.Stats()["inUse"] == int32(2)
		}, 10*time.Second, 10*time.Millisecond, "transfer isn't holding the VFS")
		d.provider.Proxy().Shutdown()
		assert.Equal(t, int32(1), VFS.Stats()["inUse"], "VFS not held by the transfer alone")
	}

	t.Run("Download", func(t *testing.T) {
		resp, err := c.Retr("download.bin")
		require.NoError(t, err)
		defer func() { _ = resp.Close() }()
		start := make([]byte, 1024)
		_, err = io.ReadFull(resp, start)
		require.NoError(t, err)
		expire(t)
		rest, err := io.ReadAll(resp)
		require.NoError(t, err)
		require.NoError(t, resp.Close())
		assert.True(t, contents == string(start)+string(rest), "download corrupted")
	})

	t.Run("Upload", func(t *testing.T) {
		pr, pw := io.Pipe()
		var storErr error
		var wg sync.WaitGroup
		wg.Go(func() {
			storErr = c.Stor("upload.bin", pr)
		})
		// Finish the upload before the connection is used again
		defer func() {
			_ = pw.Close()
			wg.Wait()
		}()
		_, err := io.WriteString(pw, contents[:1024*1024])
		require.NoError(t, err)
		expire(t)
		_, err = io.WriteString(pw, contents[1024*1024:])
		require.NoError(t, err)
		require.NoError(t, pw.Close())
		wg.Wait()
		require.NoError(t, storErr)
		got, err := os.ReadFile(filepath.Join(root, "upload.bin"))
		require.NoError(t, err)
		assert.True(t, contents == string(got), "upload corrupted")
	})
}
