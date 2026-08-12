// Test that auth-proxy credentials are bound to the FTP session and not
// shared between sessions that happen to use the same username.

//go:build !windows && !darwin && !plan9

package ftp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	ftpclient "github.com/jlaffaye/ftp"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/lib/israce"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProxyCrossSession checks that two FTP sessions authenticating with the
// same username but different auth-proxy credentials stay bound to their own
// backends. A later login must not rebind an earlier, still-open session to
// the later session's backend.
func TestProxyCrossSession(t *testing.T) {
	if israce.Enabled {
		t.Skip("Skipping under race detector as underlying library is racy")
	}

	// Two roots reached with the same username but different passwords.
	attackerRoot := t.TempDir()
	victimRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(attackerRoot, "attacker.txt"), []byte("attacker-only\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(victimRoot, "victim.txt"), []byte("victim-secret\n"), 0600))
	t.Setenv("RCLONE_TEST_ATTACKER_ROOT", attackerRoot)
	t.Setenv("RCLONE_TEST_VICTIM_ROOT", victimRoot)

	const addr = "127.0.0.1:52121"

	opt := Opt
	opt.ListenAddr = addr
	opt.PassivePorts = testPASSIVEPORTRANGE

	// The auth-proxy branch is selected from the global proxy.Opt.
	oldAuthProxy := proxy.Opt.AuthProxy
	proxy.Opt.AuthProxy = "go run proxy_crosssession_code.go"
	defer func() { proxy.Opt.AuthProxy = oldAuthProxy }()
	proxyOpt := proxy.Opt

	w, err := newServer(context.Background(), nil, &opt, &vfscommon.Opt, &proxyOpt)
	require.NoError(t, err)

	quit := make(chan struct{})
	go func() {
		assert.NoError(t, w.Serve())
		close(quit)
	}()
	defer func() {
		require.NoError(t, w.Shutdown())
		<-quit
	}()

	dial := func(pass string) *ftpclient.ServerConn {
		var c *ftpclient.ServerConn
		var err error
		for range 100 {
			c, err = ftpclient.Dial(addr, ftpclient.DialWithTimeout(5*time.Second))
			if err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.NoError(t, err)
		require.NoError(t, c.Login("shared", pass))
		return c
	}

	// The attacker logs in and establishes its authority over its own root.
	attacker := dial("attacker-token")
	defer func() { _ = attacker.Quit() }()

	names, err := attacker.NameList("/")
	require.NoError(t, err)
	assert.Contains(t, names, "attacker.txt")
	assert.NotContains(t, names, "victim.txt")

	// A second principal logs in with the same username but a different token.
	victim := dial("victim-token")
	defer func() { _ = victim.Quit() }()

	vnames, err := victim.NameList("/")
	require.NoError(t, err)
	assert.Contains(t, vnames, "victim.txt")

	// The still-open attacker session must remain bound to the attacker
	// backend rather than being rebound to the victim's.
	names, err = attacker.NameList("/")
	require.NoError(t, err)
	assert.Contains(t, names, "attacker.txt")
	assert.NotContains(t, names, "victim.txt", "attacker session was rebound to the victim backend")

	// And the attacker must not be able to read the victim's file.
	_, err = attacker.Retr("victim.txt")
	assert.Error(t, err)
}
