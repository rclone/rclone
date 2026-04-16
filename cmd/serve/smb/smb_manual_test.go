//go:build !windows && !darwin && !plan9

package smb

import (
	"context"
	"os"
	"testing"
	"time"

	smb2client "github.com/cloudsoda/go-smb2"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSMBManualNoAuth(t *testing.T) {
	testSMBManual(t, "", "", true)
}

func TestSMBManualAuth(t *testing.T) {
	testSMBManual(t, "testuser", "testpass", false)
}

func testSMBManual(t *testing.T, user, pass string, noAuth bool) {
	dir := t.TempDir()
	err := os.WriteFile(dir+"/hello.txt", []byte("hello world"), 0644)
	require.NoError(t, err)

	f, err := fs.NewFs(context.Background(), dir)
	require.NoError(t, err)

	opt := Options{
		ListenAddr: "localhost:0",
		User:       user,
		Pass:       pass,
		NoAuth:     noAuth,
		ShareName:  "rclone",
	}
	s, err := newServer(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err)

	go func() {
		_ = s.Serve()
	}()

	addr := s.Addr().String()
	t.Logf("Server listening on %s", addr)
	time.Sleep(200 * time.Millisecond)

	clientUser := user
	clientPass := pass
	if noAuth {
		clientUser = "guest"
		clientPass = ""
	}

	d := &smb2client.Dialer{
		Initiator: &smb2client.NTLMInitiator{
			User:     clientUser,
			Password: clientPass,
		},
	}

	session, err := d.Dial(context.Background(), addr)
	require.NoError(t, err, "SMB dial failed")

	share, err := session.Mount("rclone")
	require.NoError(t, err, "SMB mount failed")

	entries, err := share.ReadDir(".")
	require.NoError(t, err, "ReadDir failed")
	require.Len(t, entries, 1)
	assert.Equal(t, "hello.txt", entries[0].Name())

	data, err := share.ReadFile("hello.txt")
	require.NoError(t, err, "ReadFile failed")
	assert.Equal(t, "hello world", string(data))

	err = share.WriteFile("test.txt", []byte("test data"), 0644)
	require.NoError(t, err, "WriteFile failed")

	_ = share.Umount()
	_ = session.Logoff()
	assert.NoError(t, s.Shutdown())
}
