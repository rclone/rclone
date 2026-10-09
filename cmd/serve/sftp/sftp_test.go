// Serve sftp tests set up a server and run the integration tests
// for the sftp remote against it.
//
// We skip tests on platforms with troublesome character mappings

//go:build !windows && !darwin && !plan9

package sftp

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
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
	"golang.org/x/crypto/ssh"
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

// TestAuthProxyPublicKeyAsPassword checks that a public key which has
// just been used to log in through the auth proxy can't be sent as a
// password from the same address to get the same backend without the
// proxy being asked.
func TestAuthProxyPublicKeyAsPassword(t *testing.T) {
	// The proxy accepts any public key but refuses every password, so
	// a password login can only succeed by skipping the proxy
	t.Setenv("RCLONE_TEST_PROXY_KEY_ONLY", "1")
	proxyOpt := proxy.Opt
	proxyOpt.AuthProxy = "go run ../proxy/proxy_code.go"

	opt := Opt
	opt.ListenAddr = testBindAddress
	w, err := newServer(context.Background(), nil, &opt, &vfscommon.Opt, &proxyOpt)
	require.NoError(t, err)
	go func() {
		assert.NoError(t, w.Serve())
	}()
	defer func() {
		assert.NoError(t, w.Shutdown())
	}()
	addr := w.Addr().String()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)
	publicKeyString := base64.StdEncoding.EncodeToString(signer.PublicKey().Marshal())

	login := func(auth ssh.AuthMethod) error {
		client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
			User:            "victim",
			Auth:            []ssh.AuthMethod{auth},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         30 * time.Second,
		})
		if err != nil {
			return err
		}
		_ = client.Close()
		return nil
	}

	// Check the proxy refuses the public key as a password on a cold cache
	require.ErrorContains(t, login(ssh.Password(publicKeyString)), "unable to authenticate")

	// The owner of the key logs in with it
	require.NoError(t, login(ssh.PublicKeys(signer)))

	// Now the public key must still be refused as a password
	require.ErrorContains(t, login(ssh.Password(publicKeyString)), "unable to authenticate")
}

func TestRc(t *testing.T) {
	servetest.TestRc(t, rc.Params{
		"type":           "sftp",
		"user":           "test",
		"pass":           obscure.MustObscure("test"),
		"vfs_cache_mode": "off",
	})
}
