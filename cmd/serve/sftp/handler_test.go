// Test the SFTP serve handler against a real server and client.
//
// We skip tests on platforms with troublesome character mappings

//go:build !windows && !darwin && !plan9

package sftp

import (
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// startTestSSHClient starts an sftp server serving a temporary local directory
// with the given VFS options and returns an ssh client connected to it.
func startTestSSHClient(t *testing.T, vfsOpt *vfscommon.Options) *ssh.Client {
	ctx := context.Background()

	f, err := fs.NewFs(ctx, t.TempDir())
	require.NoError(t, err)

	opt := Opt
	opt.ListenAddr = testBindAddress
	opt.User = testUser
	opt.Pass = testPass

	w, err := newServer(ctx, f, &opt, vfsOpt, &proxy.Opt)
	require.NoError(t, err)
	go func() {
		_ = w.Serve()
	}()
	t.Cleanup(func() {
		assert.NoError(t, w.Shutdown())
	})

	clientConfig := &ssh.ClientConfig{
		User:            testUser,
		Auth:            []ssh.AuthMethod{ssh.Password(testPass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	conn, err := ssh.Dial("tcp", w.Addr().String(), clientConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

// startTestServer starts an sftp server as startTestSSHClient does and
// returns a connected sftp client.
func startTestServer(t *testing.T, vfsOpt *vfscommon.Options) *sftp.Client {
	client, err := sftp.NewClient(startTestSSHClient(t, vfsOpt))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

// Test that re-opening a file for write without the truncate flag preserves
// the data already written, rather than zeroing the start of the file.
//
// This reproduces a corruption seen with WinSCP "Process in Background", which
// resumes an upload on a second connection by re-opening the partial file
// without SSH_FXF_TRUNC and writing from the offset it had reached.
func TestFilewriteResumeNoTruncate(t *testing.T) {
	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeWrites
	client := startTestServer(t, &vfsOpt)

	const (
		fileName = "file.bin"
		total    = 1024 * 1024 // 1 MiB
		split    = 700 * 1024  // where the upload is "backgrounded"
	)

	// Deterministic non-zero contents so a zeroed hole is easy to spot
	contents := make([]byte, total)
	for i := range contents {
		contents[i] = byte(i%251 + 1)
	}

	// First connection: write the first part with truncate, as a fresh upload
	f, err := client.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	require.NoError(t, err)
	_, err = f.Write(contents[:split])
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Resume: re-open WITHOUT truncate and write the remainder at its offset
	f, err = client.OpenFile(fileName, os.O_WRONLY|os.O_CREATE)
	require.NoError(t, err)
	_, err = f.WriteAt(contents[split:], int64(split))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Read the file back and check it is intact - no zeroed prefix
	rd, err := client.Open(fileName)
	require.NoError(t, err)
	got, err := io.ReadAll(rd)
	require.NoError(t, err)
	require.NoError(t, rd.Close())

	require.Equal(t, total, len(got), "wrong file size")
	assert.Equal(t, contents, got, "file contents corrupted on resumed upload")
}

// Test that opening a file for write with the truncate flag does truncate any
// existing data, which is the normal overwrite case for most clients.
func TestFilewriteTruncate(t *testing.T) {
	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeWrites
	client := startTestServer(t, &vfsOpt)

	const fileName = "file.bin"

	// Write some initial long contents
	require.NoError(t, writeFile(client, fileName, strings.Repeat("A", 1024)))

	// Overwrite with shorter contents using truncate
	const newContents = "hello"
	require.NoError(t, writeFile(client, fileName, newContents))

	rd, err := client.Open(fileName)
	require.NoError(t, err)
	got, err := io.ReadAll(rd)
	require.NoError(t, err)
	require.NoError(t, rd.Close())

	assert.Equal(t, newContents, string(got))
}

// Test that a SETSTAT request with a size attribute truncates the file, rather
// than being silently ignored.
func TestSetstatTruncate(t *testing.T) {
	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeWrites
	client := startTestServer(t, &vfsOpt)

	const fileName = "file.bin"

	// Write a file and then truncate it via FSETSTAT (a size attribute) on the
	// open handle, the way a client setting the final size of an upload does.
	f, err := client.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	require.NoError(t, err)
	_, err = f.Write([]byte(strings.Repeat("A", 1024)))
	require.NoError(t, err)
	require.NoError(t, f.Truncate(10))
	require.NoError(t, f.Close())

	fi, err := client.Stat(fileName)
	require.NoError(t, err)
	assert.Equal(t, int64(10), fi.Size(), "file not truncated to requested size")

	rd, err := client.Open(fileName)
	require.NoError(t, err)
	got, err := io.ReadAll(rd)
	require.NoError(t, err)
	require.NoError(t, rd.Close())
	assert.Equal(t, strings.Repeat("A", 10), string(got))
}

// Test that the statvfs@openssh.com extension returns filesystem usage from
// the VFS rather than an unsupported status.
func TestStatVFS(t *testing.T) {
	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeWrites
	client := startTestServer(t, &vfsOpt)

	st, err := client.StatVFS("/")
	require.NoError(t, err)

	assert.Greater(t, st.TotalSpace(), uint64(0), "expected non-zero total space")
	assert.LessOrEqual(t, st.FreeSpace(), st.TotalSpace(), "free space should not exceed total")
	assert.Equal(t, uint64(255), st.Namemax)
}

// Test that a SETSTAT request setting the modification time is honoured, even
// when the time happens to be the Unix epoch
func TestSetstatMtime(t *testing.T) {
	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeOff
	client := startTestServer(t, &vfsOpt)

	const fileName = "file.bin"
	require.NoError(t, writeFile(client, fileName, "hello"))

	epoch := time.Unix(0, 0)
	require.NoError(t, client.Chtimes(fileName, epoch, epoch))

	fi, err := client.Stat(fileName)
	require.NoError(t, err)
	assert.True(t, fi.ModTime().Equal(epoch), "mtime not applied: got %v want %v", fi.ModTime(), epoch)
}

// Test that a panic in a request handler is recovered and returned as an
// error to the client rather than crashing the whole server.
func TestHandlerRecoversPanic(t *testing.T) {
	// A nil VFS makes every handler panic with a nil pointer dereference
	// inside the vfs package, standing in for a panicking backend.
	v := vfsHandler{}

	_, err := v.Fileread(&sftp.Request{Filepath: "/file"})
	assert.ErrorContains(t, err, "request handler")

	_, err = v.Filewrite(&sftp.Request{Filepath: "/file"})
	assert.ErrorContains(t, err, "request handler")

	err = v.Filecmd(&sftp.Request{Method: "Mkdir", Filepath: "/dir"})
	assert.ErrorContains(t, err, "request handler")

	_, err = v.Filelist(&sftp.Request{Method: "List", Filepath: "/"})
	assert.ErrorContains(t, err, "request handler")

	_, err = v.StatVFS(&sftp.Request{Filepath: "/"})
	assert.ErrorContains(t, err, "request handler")
}

// panickingHandle panics on ReadAt and WriteAt, standing in for a backend
// which panics during data transfer.
type panickingHandle struct {
	vfs.Handle
}

func (panickingHandle) ReadAt([]byte, int64) (int, error)  { panic("boom") }
func (panickingHandle) WriteAt([]byte, int64) (int, error) { panic("boom") }
func (panickingHandle) Close() error                       { panic("boom") }

// Test that a panic during data transfer on a handle returned from
// Fileread/Filewrite is recovered and returned as an error.
func TestRecoveringHandle(t *testing.T) {
	h := recoveringHandle{panickingHandle{}}

	_, err := h.ReadAt(make([]byte, 16), 0)
	assert.ErrorContains(t, err, "boom")

	_, err = h.WriteAt(make([]byte, 16), 0)
	assert.ErrorContains(t, err, "boom")

	assert.ErrorContains(t, h.Close(), "boom")
}

// Test that a session request with a truncated payload is rejected rather
// than panicking the out-of-band request goroutine, which would kill the
// whole server. The subsystem payload is a length-prefixed string, so an
// empty one used to be sliced out of range.
func TestShortSubsystemRequest(t *testing.T) {
	vfsOpt := vfscommon.Opt
	conn := startTestSSHClient(t, &vfsOpt)

	// Rejecting the request must not leak the goroutine waiting to find out
	// what kind of channel this is, or a client could exhaust the server by
	// opening bad channels in a loop.
	before := runtime.NumGoroutine()
	const channels = 50
	for range channels {
		channel, requests, err := conn.OpenChannel("session", nil)
		require.NoError(t, err)
		go ssh.DiscardRequests(requests)

		// Empty payload: no 4-byte length prefix at all
		_, err = channel.SendRequest("subsystem", true, []byte{})
		require.NoError(t, err)
		require.NoError(t, channel.Close())
	}

	assert.Eventually(t, func() bool {
		return runtime.NumGoroutine() < before+channels/2
	}, 10*time.Second, 50*time.Millisecond,
		"goroutines leaked: started at %d, now %d after %d rejected channels",
		before, runtime.NumGoroutine(), channels)

	// The server must still be alive and serving
	client, err := sftp.NewClient(conn)
	require.NoError(t, err, "server died after a truncated subsystem request")
	defer func() { _ = client.Close() }()
	_, err = client.Stat("/")
	assert.NoError(t, err)
}

// writeFile writes contents to fileName via the client truncating any existing
// data, the way a normal upload does.
func writeFile(client *sftp.Client, fileName, contents string) error {
	f, err := client.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(contents)); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
