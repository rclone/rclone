package servetest

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetEphemeralPort opens a listening port on localhost:0, closes it,
// and returns the address as "localhost:port".
func GetEphemeralPort(t *testing.T) string {
	listener, err := net.Listen("tcp", "localhost:0") // Listen on any available port
	require.NoError(t, err)
	defer func() {
		require.NoError(t, listener.Close())
	}()
	return listener.Addr().String()
}

// checkTCP attempts to establish a TCP connection to the given address,
// and closes it if successful. Returns an error if the connection fails.
func checkTCP(address string) error {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	err = conn.Close()
	if err != nil {
		return fmt.Errorf("failed to close connection to %s: %w", address, err)
	}

	return nil
}

// TestRc tests the rc interface for the servers
//
// in should contain any options necessary however this code will add
// "fs", "addr".
func TestRc(t *testing.T, in rc.Params) {
	ctx := context.Background()
	dir := t.TempDir()
	serveStart := rc.Calls.Get("serve/start")
	serveStop := rc.Calls.Get("serve/stop")
	name := in["type"].(string)
	addr := GetEphemeralPort(t)

	// Track active VFS count before starting server
	initialActive := vfs.ActiveCount()

	// Start the server
	in1 := in.Copy()
	in1["fs"] = dir
	in1["addr"] = addr
	out, err := serveStart.Fn(ctx, in1)
	require.NoError(t, err)
	id := out["id"].(string)
	assert.True(t, strings.HasPrefix(id, name+"-"))
	gotAddr := out["addr"].(string)
	assert.Equal(t, addr, gotAddr)
	vfsID, ok := out["vfsId"].(string)
	assert.True(t, ok)

	// Check we can make a TCP connection to the server
	t.Logf("Checking connection on %q", addr)
	err = checkTCP(addr)
	assert.NoError(t, err)

	// Test starting a second server reusing the same VFS ID (if server uses VFS)
	if vfsID != "" {
		addr2 := GetEphemeralPort(t)
		in2 := in.Copy()
		in2["fs"] = dir
		in2["addr"] = addr2
		in2["vfsId"] = vfsID
		out2, err := serveStart.Fn(ctx, in2)
		require.NoError(t, err)
		id2 := out2["id"].(string)
		vfsID2, ok := out2["vfsId"].(string)
		assert.True(t, ok)
		assert.Equal(t, vfsID, vfsID2)
		assert.Equal(t, initialActive+1, vfs.ActiveCount(), "ActiveCount should remain same when reusing VFS")

		// Stop the second server
		_, err = serveStop.Fn(ctx, rc.Params{"id": id2})
		require.NoError(t, err)
		assert.Equal(t, initialActive+1, vfs.ActiveCount(), "VFS should still be active for first server")

		// Check we can no longer make connections to second server
		err = checkTCP(addr2)
		assert.Error(t, err)
	}

	// Stop the first server
	_, err = serveStop.Fn(ctx, rc.Params{"id": id})
	require.NoError(t, err)

	// Check the VFS was properly released
	assert.Equal(t, initialActive, vfs.ActiveCount(), "VFS should have been shut down after all servers stop")

	// Check we can no longer make connections to the server
	err = checkTCP(addr)
	assert.Error(t, err)
}
