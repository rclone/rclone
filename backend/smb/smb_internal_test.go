// Unit tests for internal SMB functions
package smb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDialClosesConnectionOnSetupError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, listener.Close()) }()

	type acceptResult struct {
		conn net.Conn
		err  error
	}
	accepted := make(chan acceptResult, 1)
	go func() {
		conn, err := listener.Accept()
		accepted <- acceptResult{conn: conn, err: err}
	}()

	f := &Fs{opt: Options{Pass: "invalid"}}
	_, err = f.dial(context.Background(), "tcp", listener.Addr().String())
	require.Error(t, err)

	var result acceptResult
	select {
	case result = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for server to accept connection")
	}
	require.NoError(t, result.err)
	defer func() { require.NoError(t, result.conn.Close()) }()
	require.NoError(t, result.conn.SetReadDeadline(time.Now().Add(time.Second)))

	buffer := make([]byte, 1)
	n, err := result.conn.Read(buffer)
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF)
}

// TestUploadConnectionReuse checks an upload leaves only one connection in the
// pool, ie the connection it used is available again for the SetModTime which
// follows it rather than a second one being dialled.
//
// This needs a real SMB server so it is skipped if one isn't configured.
func TestUploadConnectionReuse(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()
	remoteName := *fstest.RemoteName
	if remoteName == "" {
		remoteName = "TestSMB:rclone"
	}
	remote, err := fs.NewFs(ctx, remoteName)
	if errors.Is(err, fs.ErrorNotFoundInConfigFile) {
		t.Skipf("skipping as %q is not configured", remoteName)
	}
	require.NoError(t, err)
	f, ok := remote.(*Fs)
	if !ok {
		t.Skipf("skipping as %q is not an SMB remote", remoteName)
	}

	defer func() { require.NoError(t, f.Shutdown(ctx)) }()

	// Empty the pool so the connections counted below are only the upload's
	require.NoError(t, f.drainPool(ctx))

	const contents = "connection reuse test"
	remotePath := fmt.Sprintf("rclone-test-connection-reuse-%d.txt", time.Now().UnixNano())
	src := object.NewStaticObjectInfo(remotePath, time.Now(), int64(len(contents)), true, nil, nil)
	o, err := f.Put(ctx, strings.NewReader(contents), src)
	require.NoError(t, err)
	defer func() { require.NoError(t, o.Remove(ctx)) }()

	f.poolMu.Lock()
	pooled := len(f.pool)
	f.poolMu.Unlock()
	assert.Equal(t, 1, pooled, "upload should leave exactly one connection in the pool")
}

// TestOptionsWorkstationConfig verifies the workstation config key is backwards compatible
func TestOptionsWorkstationConfig(t *testing.T) {
	tests := []struct {
		name        string
		workstation string
	}{
		{"named workstation", "MYWORKSTATION"},
		{"empty workstation sends no name", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := configmap.Simple{
				"host":        "example.com",
				"workstation": tt.workstation,
			}
			opt := new(Options)
			if err := configstruct.Set(m, opt); err != nil {
				t.Fatalf("configstruct.Set failed: %v", err)
			}
			if opt.Workstation != tt.workstation {
				t.Errorf("opt.Workstation = %q, want %q", opt.Workstation, tt.workstation)
			}
		})
	}
}

// TestIsPathDir tests the isPathDir function logic
func TestIsPathDir(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		// Empty path should be considered a directory
		{"", true},

		// Paths with trailing slash should be directories
		{"/", true},
		{"share/", true},
		{"share/dir/", true},
		{"share/dir/subdir/", true},

		// Paths without trailing slash should not be directories
		{"share", false},
		{"share/dir", false},
		{"share/dir/file", false},
		{"share/dir/subdir/file", false},

		// Edge cases
		{"share//", true},
		{"share///", true},
		{"share/dir//", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isPathDir(tt.path)
			if result != tt.expected {
				t.Errorf("isPathDir(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestNewFsUser(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		user string
		want string
	}{
		{user: "", want: currentUser},
		{user: currentUser, want: currentUser},
		{user: "someone", want: "someone"},
	} {
		m := configmap.Simple{"host": "localhost", "user": test.user}
		f, err := NewFs(ctx, "TestSMB", "", m)
		require.NoError(t, err)
		assert.Equal(t, test.want, f.(*Fs).opt.User, "user=%q", test.user)
	}

	// No user in the config at all
	f, err := NewFs(ctx, "TestSMB", "", configmap.Simple{"host": "localhost"})
	require.NoError(t, err)
	assert.Equal(t, currentUser, f.(*Fs).opt.User)
}

// The default is blank so that a user name which happens to match the
// current user is still written to the config file.
func TestUserDefault(t *testing.T) {
	ri, err := fs.Find("smb")
	require.NoError(t, err)
	assert.Equal(t, "", ri.Options.Get("user").Default)
}
