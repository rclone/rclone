package servetest

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/rc"
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

	// Start the server
	in["fs"] = dir
	in["addr"] = addr
	out, err := serveStart.Fn(ctx, in)
	require.NoError(t, err)
	id := out["id"].(string)
	assert.True(t, strings.HasPrefix(id, name+"-"))
	gotAddr := out["addr"].(string)
	assert.Equal(t, addr, gotAddr)

	// Check we can make a TCP connection to the server
	t.Logf("Checking connection on %q", addr)
	err = checkTCP(addr)
	assert.NoError(t, err)

	// Stop the server
	_, err = serveStop.Fn(ctx, rc.Params{"id": id})
	require.NoError(t, err)

	// Check we can make no longer make connections to the server
	err = checkTCP(addr)
	assert.Error(t, err)
}
