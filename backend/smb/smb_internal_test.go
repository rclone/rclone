// Unit tests for internal SMB functions
package smb

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

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
