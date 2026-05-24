// Test the SFTP serve handler against a real server and client.
//
// We skip tests on platforms with troublesome character mappings

//go:build !windows && !darwin && !plan9

package sftp

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/pkg/sftp"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// startTestServer starts an sftp server serving a temporary local directory
// with the given VFS options and returns a connected sftp client.
func startTestServer(t *testing.T, vfsOpt *vfscommon.Options) *sftp.Client {
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

	client, err := sftp.NewClient(conn)
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
