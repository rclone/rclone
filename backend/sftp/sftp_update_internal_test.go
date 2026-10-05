//go:build !plan9

package sftp

// Regression test for the failed-upload handle leak: Update used to
// return the connection to the pool without closing the remote file when
// the upload failed partway through, so the server kept the (removed)
// file's handle - and its disk space - until the connection died. The
// test runs the backend against an in-process SSH/SFTP server whose
// writer fails past a quota, mimicking a full server disk, and counts
// server-side open file handles.

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	pkgsftp "github.com/pkg/sftp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type leakEntry struct {
	size    int64
	modTime time.Time
}

type leakHandle struct {
	srv     *leakServer
	path    string
	written int64
	closed  bool
}

func (h *leakHandle) WriteAt(p []byte, off int64) (int, error) {
	h.srv.mu.Lock()
	defer h.srv.mu.Unlock()
	if h.written+int64(len(p)) > h.srv.quota {
		return 0, fmt.Errorf("no space left on device")
	}
	h.written += int64(len(p))
	e := h.srv.files[h.path]
	if e == nil {
		e = &leakEntry{modTime: time.Now()}
		h.srv.files[h.path] = e
	}
	if off+int64(len(p)) > e.size {
		e.size = off + int64(len(p))
	}
	return len(p), nil
}

func (h *leakHandle) Close() error {
	h.srv.mu.Lock()
	defer h.srv.mu.Unlock()
	if !h.closed {
		h.closed = true
		h.srv.open--
	}
	return nil
}

type leakServer struct {
	mu    sync.Mutex
	files map[string]*leakEntry
	open  int
	quota int64
}

func (s *leakServer) Filewrite(r *pkgsftp.Request) (io.WriterAt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[r.Filepath] = &leakEntry{modTime: time.Now()}
	s.open++
	return &leakHandle{srv: s, path: r.Filepath}, nil
}

func (s *leakServer) Fileread(r *pkgsftp.Request) (io.ReaderAt, error) {
	return nil, fmt.Errorf("reads not supported")
}

func (s *leakServer) Filecmd(r *pkgsftp.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case "Remove":
		delete(s.files, r.Filepath)
	case "Rename":
		// target path arrives in Filepath per pkg/sftp request semantics; not exercised here
	}
	return nil
}

type leakFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi leakFileInfo) Name() string { return fi.name }
func (fi leakFileInfo) Size() int64  { return fi.size }
func (fi leakFileInfo) Mode() os.FileMode {
	if fi.isDir {
		return os.ModeDir | 0o755
	}
	return 0o644
}
func (fi leakFileInfo) ModTime() time.Time { return time.Now() }
func (fi leakFileInfo) IsDir() bool        { return fi.isDir }
func (fi leakFileInfo) Sys() interface{}   { return nil }

type leakLister struct{ files []os.FileInfo }

func (l leakLister) ListAt(out []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l.files)) {
		return 0, io.EOF
	}
	n := copy(out, l.files[offset:])
	if offset+int64(n) >= int64(len(l.files)) {
		return n, io.EOF
	}
	return n, nil
}

func (s *leakServer) Filelist(r *pkgsftp.Request) (pkgsftp.ListerAt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case "Stat", "Lstat":
		if e, ok := s.files[r.Filepath]; ok {
			return leakLister{[]os.FileInfo{leakFileInfo{name: r.Filepath, size: e.size}}}, nil
		}
		return leakLister{[]os.FileInfo{leakFileInfo{name: r.Filepath, isDir: true}}}, nil
	case "List":
		return leakLister{}, nil
	case "Readlink":
		return leakLister{[]os.FileInfo{leakFileInfo{name: r.Filepath}}}, nil
	}
	return nil, fmt.Errorf("unknown list method %q", r.Method)
}

func startLeakServer(t *testing.T, srv *leakServer) (int, ssh.Signer) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				_, chans, reqs, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					if ch.ChannelType() != "session" {
						_ = ch.Reject(ssh.UnknownChannelType, "only session")
						continue
					}
					channel, requests, err := ch.Accept()
					if err != nil {
						continue
					}
					go func() {
						for req := range requests {
							t.Logf("channel request: type=%q payloadlen=%d wantReply=%v", req.Type, len(req.Payload), req.WantReply)
							if req.Type == "subsystem" {
								_ = req.Reply(true, nil)
								server := pkgsftp.NewRequestServer(channel, pkgsftp.Handlers{
									FileGet:  srv,
									FilePut:  srv,
									FileCmd:  srv,
									FileList: srv,
								})
								_ = server.Serve()
								return
							}
							_ = req.Reply(false, nil)
						}
					}()
				}
			}(conn)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, signer
}

func TestUpdateFailedUploadClosesRemoteFile(t *testing.T) {
	srv := &leakServer{files: map[string]*leakEntry{}, quota: 1 << 20}
	port, signer := startLeakServer(t, srv)

	m := configmap.Simple{
		"host":        "127.0.0.1",
		"port":        fmt.Sprint(port),
		"user":        "u",
		"pass":        obscure.MustObscure("p"),
		"shell_type":  "none",
		"chunk_size":  "32Ki",
		"concurrency": "64",
		"host_keys":   "ssh-ed25519 " + base64.StdEncoding.EncodeToString(signer.PublicKey().Marshal()),
	}
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 1
	f, err := NewFs(ctx, "leaktest", "", m)
	require.NoError(t, err)

	// Control: a small upload under quota succeeds and leaves no open handle.
	small := bytes.Repeat([]byte("a"), 100<<10)
	src := object.NewStaticObjectInfo("small.bin", time.Now(), int64(len(small)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(small), src)
	require.NoError(t, err)
	srv.mu.Lock()
	require.Equal(t, 0, srv.open, "small upload left a handle open")
	_, smallOK := srv.files["/small.bin"]
	srv.mu.Unlock()
	require.True(t, smallOK, "small upload missing server-side")

	// Failing upload: 3 MiB against a 1 MiB quota.
	big := bytes.Repeat([]byte("b"), 3<<20)
	src = object.NewStaticObjectInfo("big.bin", time.Now(), int64(len(big)), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(big), src)
	require.Error(t, err, "upload past quota must fail")

	srv.mu.Lock()
	open := srv.open
	_, bigPresent := srv.files["/big.bin"]
	srv.mu.Unlock()
	require.False(t, bigPresent, "partial file should have been removed")
	require.Equal(t, 0, open, "failed upload leaked %d server-side file handle(s)", open)
}
