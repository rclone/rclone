package smb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	smb2 "github.com/cloudsoda/go-smb2"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/require"
)

// newTestServer starts an SMB server backed by a local fs at dir, listening on
// a random loopback port. cacheFull enables the VFS write cache.
func newTestServer(t *testing.T, dir string, cacheFull bool) string {
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)

	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	if cacheFull {
		vfsOpt.CacheMode = vfscommon.CacheModeFull
	}

	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	go func() { _ = s.Serve() }()
	t.Cleanup(func() { _ = s.Shutdown() })
	return s.Addr().String()
}

// dialShare connects to the server as a guest and mounts the share.
func dialShare(t *testing.T, addr string) (*smb2.Session, *smb2.Share) {
	nc, err := net.Dial("tcp", addr)
	require.NoError(t, err)

	d := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: "guest"}}
	session, err := d.DialConn(context.Background(), nc, addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Logoff() })

	share, err := session.Mount("rclone")
	require.NoError(t, err)
	t.Cleanup(func() { _ = share.Umount() })
	return session, share
}

// TestServeGuestConnect checks negotiate, guest authentication, ECHO and tree
// connect (milestone M1).
func TestServeGuestConnect(t *testing.T) {
	addr := newTestServer(t, t.TempDir(), false)
	session, share := dialShare(t, addr)
	require.NoError(t, session.Echo())
	require.NotNil(t, share)
}

// TestServeRead checks stat, directory listing and reading (milestone M2).
func TestServeRead(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0755))

	addr := newTestServer(t, dir, false)
	_, share := dialShare(t, addr)

	// Stat a file.
	fi, err := share.Stat("hello.txt")
	require.NoError(t, err)
	require.Equal(t, int64(11), fi.Size())
	require.False(t, fi.IsDir())

	// Read a file.
	data, err := share.ReadFile("hello.txt")
	require.NoError(t, err)
	require.Equal(t, "hello world", string(data))

	// List the root directory.
	entries, err := share.ReadDir(".")
	require.NoError(t, err)
	names := dirNames(entries)
	require.Contains(t, names, "hello.txt")
	require.Contains(t, names, "sub")

	// Stat a directory.
	di, err := share.Stat("sub")
	require.NoError(t, err)
	require.True(t, di.IsDir())
}

// TestServeWrite checks writing, mkdir, rename and delete (milestone M3).
func TestServeWrite(t *testing.T) {
	addr := newTestServer(t, t.TempDir(), true)
	_, share := dialShare(t, addr)

	// Write a file and read it back.
	require.NoError(t, share.WriteFile("new.txt", []byte("written data"), 0644))
	data, err := share.ReadFile("new.txt")
	require.NoError(t, err)
	require.Equal(t, "written data", string(data))

	// Make a directory.
	require.NoError(t, share.Mkdir("newdir", 0755))
	di, err := share.Stat("newdir")
	require.NoError(t, err)
	require.True(t, di.IsDir())

	// Write into the subdirectory.
	require.NoError(t, share.WriteFile("newdir/inner.txt", []byte("inner"), 0644))
	data, err = share.ReadFile("newdir/inner.txt")
	require.NoError(t, err)
	require.Equal(t, "inner", string(data))

	// Rename.
	require.NoError(t, share.Rename("new.txt", "renamed.txt"))
	_, err = share.Stat("new.txt")
	require.Error(t, err)
	renamed, err := share.ReadFile("renamed.txt")
	require.NoError(t, err)
	require.Equal(t, "written data", string(renamed))

	// Delete.
	require.NoError(t, share.Remove("renamed.txt"))
	_, err = share.Stat("renamed.txt")
	require.Error(t, err)
}

// TestServeConcurrent drives many operations at once over one connection,
// exercising the concurrent request dispatch and its locking (run under -race).
func TestServeConcurrent(t *testing.T) {
	addr := newTestServer(t, t.TempDir(), true)
	_, share := dialShare(t, addr)

	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("f%d.dat", i)
			want := []byte(fmt.Sprintf("payload-%d", i))
			if err := share.WriteFile(name, want, 0644); err != nil {
				errs <- fmt.Errorf("write %s: %w", name, err)
				return
			}
			got, err := share.ReadFile(name)
			if err != nil {
				errs <- fmt.Errorf("read %s: %w", name, err)
				return
			}
			if string(got) != string(want) {
				errs <- fmt.Errorf("%s: got %q want %q", name, got, want)
				return
			}
			if _, err := share.ReadDir("."); err != nil {
				errs <- fmt.Errorf("readdir: %w", err)
				return
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// TestServeAuth checks NTLM username/password authentication (milestone M4).
func TestServeAuth(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("top secret"), 0644))

	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	opt.User = "alice"
	opt.Pass = "s3cret"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	go func() { _ = s.Serve() }()
	t.Cleanup(func() { _ = s.Shutdown() })
	addr := s.Addr().String()

	// Correct credentials succeed and can read.
	nc, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	d := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: "alice", Password: "s3cret"}}
	session, err := d.DialConn(ctx, nc, addr)
	require.NoError(t, err)
	share, err := session.Mount("rclone")
	require.NoError(t, err)
	data, err := share.ReadFile("secret.txt")
	require.NoError(t, err)
	require.Equal(t, "top secret", string(data))
	require.NoError(t, share.Umount())
	require.NoError(t, session.Logoff())

	// Wrong password fails the session setup.
	nc2, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	d2 := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: "alice", Password: "wrong"}}
	_, err = d2.DialConn(ctx, nc2, addr)
	require.Error(t, err)
}

// startAuthServer starts a server that requires user alice/s3cret.
func startAuthServer(t *testing.T, dir string) string {
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	opt.User = "alice"
	opt.Pass = "s3cret"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	go func() { _ = s.Serve() }()
	t.Cleanup(func() { _ = s.Shutdown() })
	return s.Addr().String()
}

// readAuthShare connects requiring signing, optionally pinning a dialect, and
// returns the bytes of f.txt — exercising server message signing (milestone M4).
func readAuthShare(t *testing.T, addr string, dialect uint16) string {
	nc, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	d := &smb2.Dialer{
		Negotiator: smb2.Negotiator{RequireMessageSigning: true, SpecifiedDialect: dialect},
		Initiator:  &smb2.NTLMInitiator{User: "alice", Password: "s3cret"},
	}
	session, err := d.DialConn(context.Background(), nc, addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Logoff() })
	share, err := session.Mount("rclone")
	require.NoError(t, err)
	t.Cleanup(func() { _ = share.Umount() })
	data, err := share.ReadFile("f.txt")
	require.NoError(t, err)
	return string(data)
}

// TestServeSignedSMB3 checks AES-CMAC signing over SMB 3.0.2 (milestone M4).
func TestServeSignedSMB3(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("signed3"), 0644))
	addr := startAuthServer(t, dir)
	require.Equal(t, "signed3", readAuthShare(t, addr, 0x0302))
}

// TestServeSignedSMB2 checks HMAC-SHA256 signing over SMB 2.0.2 (milestone M4).
func TestServeSignedSMB2(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("signed2"), 0644))
	addr := startAuthServer(t, dir)
	require.Equal(t, "signed2", readAuthShare(t, addr, 0x0202))
}

// TestServeQueryDirSingleEntry checks that QUERY_DIRECTORY honours
// SMB2_RETURN_SINGLE_ENTRY (set by Windows' FindFirstFile): it must return
// exactly one entry. If it returns more, the client keeps only the first and
// resumes past the rest, silently dropping entries from the listing.
func TestServeQueryDirSingleEntry(t *testing.T) {
	dir := t.TempDir()
	const n = 5
	for i := 0; i < n; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), nil, 0644))
	}

	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	// Register a directory handle for the share root ("").
	c := newConn(s, nil)
	of := &openFile{isDir: true}
	of.fileID[0] = 1
	c.handles[of.fileID] = of

	// QUERY_DIRECTORY body: InfoClass[2], Flags[3], FileId[8:24], OutputBufferLength[28:32].
	body := make([]byte, 32)
	body[2] = 0x01 // FileDirectoryInformation
	copy(body[8:24], of.fileID[:])
	le.PutUint32(body[28:32], 1<<16)

	// With SMB2_RETURN_SINGLE_ENTRY the server must return exactly one entry.
	body[3] = 0x02
	status, _ := c.handleQueryDirectory(header{}, body)
	require.Equal(t, statusSuccess, status)
	require.Equal(t, 1, of.dirPos, "RETURN_SINGLE_ENTRY must yield exactly one entry")

	// The remaining entries come on subsequent calls; none are skipped.
	body[3] = 0x00
	status, _ = c.handleQueryDirectory(header{}, body)
	require.Equal(t, statusSuccess, status)
	require.Equal(t, n, of.dirPos, "all entries must be enumerated, none skipped")

	// Then STATUS_NO_MORE_FILES.
	status, _ = c.handleQueryDirectory(header{}, body)
	require.Equal(t, statusNoMoreFiles, status)
}

// TestServeQueryDirFileID checks that the FileId reported for a directory entry
// is derived from the path (pathFileID), not the ephemeral VFS inode, so it is
// stable across server restarts and cache evictions as Windows clients require.
func TestServeQueryDirFileID(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.tmp"), nil, 0644))

	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	c := newConn(s, nil)
	of := &openFile{isDir: true}
	of.fileID[0] = 1
	c.handles[of.fileID] = of

	// FileIdBothDirectoryInformation (0x25): FileId is at offset 96 in each entry.
	body := make([]byte, 32)
	body[2] = 0x25
	copy(body[8:24], of.fileID[:])
	le.PutUint32(body[28:32], 1<<16)
	status, resp := c.handleQueryDirectory(header{}, body)
	require.Equal(t, statusSuccess, status)

	info := resp[8:] // QUERY_DIRECTORY response: 8-byte header, then the info buffer
	require.GreaterOrEqual(t, len(info), 104)
	require.Equal(t, pathFileID("file.tmp"), le.Uint64(info[96:104]),
		"dir-entry FileId must be the stable path-derived id, not the VFS inode")

	// pathFileID is a pure function of the path: stable and path-unique.
	require.Equal(t, pathFileID("a/b"), pathFileID("a/b"))
	require.NotEqual(t, pathFileID("a/b"), pathFileID("a/c"))
}

// TestServeQueryDirPattern checks that QUERY_DIRECTORY honours the search
// pattern: a client resolves a child by name with a single-entry pattern query,
// so ignoring it made Windows path resolution follow the wrong entry.
func TestServeQueryDirPattern(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		require.NoError(t, os.Mkdir(filepath.Join(dir, name), 0755))
	}

	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	c := newConn(s, nil)
	of := &openFile{isDir: true}
	of.fileID[0] = 1
	c.handles[of.fileID] = of

	// QUERY_DIRECTORY with RESTART_SCANS|RETURN_SINGLE_ENTRY and pattern "bravo".
	pat := stringToUTF16le("bravo")
	body := make([]byte, 32+len(pat))
	body[2] = 0x25 // FileIdBothDirectoryInformation
	body[3] = 0x03 // RESTART_SCANS | RETURN_SINGLE_ENTRY
	copy(body[8:24], of.fileID[:])
	le.PutUint16(body[24:26], uint16(smb2HeaderSize+32)) // FileNameOffset
	le.PutUint16(body[26:28], uint16(len(pat)))          // FileNameLength
	le.PutUint32(body[28:32], 1<<16)                     // OutputBufferLength
	copy(body[32:], pat)

	status, resp := c.handleQueryDirectory(header{}, body)
	require.Equal(t, statusSuccess, status)
	info := resp[8:]
	nameLen := int(le.Uint32(info[60:64]))
	require.Equal(t, "bravo", utf16leToString(info[104:104+nameLen]),
		"a pattern query must return the matching entry, not the first one")

	require.True(t, matchPattern("Users", "users", true))   // case-insensitive
	require.False(t, matchPattern("Users", "users", false)) // case-sensitive: distinct
	require.True(t, matchPattern("Users", "Users", false))  // exact
	require.True(t, matchPattern("*.txt", "a.TXT", true))   // wildcard, folded
	require.False(t, matchPattern("*.txt", "a.TXT", false)) // wildcard, case-sensitive
	require.False(t, matchPattern("Users", "Default", true))
}

// treeConnectSeg builds a TREE_CONNECT request PDU (64-byte header + body) for
// \\host\<share>, for driving handleCommand directly.
func treeConnectSeg(share string) []byte {
	path := stringToUTF16le(`\\host\` + share)
	body := make([]byte, 8+len(path))
	le.PutUint16(body[0:2], 9)                 // StructureSize
	le.PutUint16(body[4:6], smb2HeaderSize+8)  // PathOffset (from header)
	le.PutUint16(body[6:8], uint16(len(path))) // PathLength
	copy(body[8:], path)
	seg := make([]byte, smb2HeaderSize+len(body))
	copy(seg[smb2HeaderSize:], body)
	return seg
}

// TestServeRequireAuth checks that with --user set, a command on a session that
// never authenticated is rejected (the auth-bypass fix), while an authenticated
// session is allowed and a guest server (no --user) is not gated.
func TestServeRequireAuth(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	seg := treeConnectSeg("rclone")
	vfsOpt := vfscommon.Opt

	// Server that requires authentication.
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	opt.User = "alice"
	opt.Pass = "s3cret"
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })
	c := newConn(s, nil)
	status := func(sessionID uint64) uint32 {
		resp := c.handleCommand(header{command: cmdTreeConnect, sessionID: sessionID}, seg, &chainCtx{})
		return le.Uint32(resp[8:12])
	}

	require.Equal(t, statusUserSessionDeleted, status(999),
		"TREE_CONNECT on an unauthenticated session must be rejected when --user is set")
	c.sessions[999] = &session{authed: true}
	require.Equal(t, statusSuccess, status(999),
		"TREE_CONNECT on an authenticated session must succeed")

	// Guest server (no --user): the auth gate is not applied.
	optG := Opt
	optG.ListenAddr = "127.0.0.1:0"
	optG.ShareName = "rclone"
	sg, err := newServer(ctx, f, &optG, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sg.Shutdown() })
	cg := newConn(sg, nil)
	resp := cg.handleCommand(header{command: cmdTreeConnect, sessionID: 42}, seg, &chainCtx{})
	require.Equal(t, statusSuccess, le.Uint32(resp[8:12]), "guest server must not gate commands")
}

// panicOnReadConn is a net.Conn whose Read panics, used to check serve() recovers.
type panicOnReadConn struct{}

func (panicOnReadConn) Read([]byte) (int, error)         { panic("simulated panic while processing a message") }
func (panicOnReadConn) Write(b []byte) (int, error)      { return len(b), nil }
func (panicOnReadConn) Close() error                     { return nil }
func (panicOnReadConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (panicOnReadConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (panicOnReadConn) SetDeadline(time.Time) error      { return nil }
func (panicOnReadConn) SetReadDeadline(time.Time) error  { return nil }
func (panicOnReadConn) SetWriteDeadline(time.Time) error { return nil }

// TestServeRecover checks that a panic while serving a connection is recovered
// (the connection is dropped) rather than crashing the whole rclone process.
func TestServeRecover(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	c := newConn(s, panicOnReadConn{})
	require.NotPanics(t, func() { c.serve() }, "a panic while serving must be recovered, not propagated")
}

// TestServeReadLengthClamp checks that a READ whose Length exceeds MaxReadSize
// is rejected rather than allocating an attacker-chosen buffer.
func TestServeReadLengthClamp(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello"), 0644))
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	handle, err := s.vfs.OpenFile("f.txt", os.O_RDONLY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })
	c := newConn(s, nil)
	of := &openFile{handle: handle, node: handle.Node()}
	of.fileID[0] = 1
	c.handles[of.fileID] = of

	readBody := func(length uint32) []byte {
		body := make([]byte, 48)
		le.PutUint32(body[4:8], length) // Length
		copy(body[16:32], of.fileID[:]) // FileId
		return body
	}

	// Over the limit (the 4 GiB DoS lives on this same path) -> rejected.
	status, _ := c.handleRead(header{}, readBody(maxIOSize+1))
	require.Equal(t, statusInvalidParameter, status, "oversized READ must be rejected, not allocated")
	// Within the limit -> still works.
	status, resp := c.handleRead(header{}, readBody(5))
	require.Equal(t, statusSuccess, status)
	require.Equal(t, "hello", string(resp[16:21]))
}

// closeErrHandle wraps a real VFS handle but fails on Close, to check that
// CLOSE surfaces the error instead of reporting success.
type closeErrHandle struct{ vfs.Handle }

func (closeErrHandle) Close() error { return errors.New("simulated upload failure") }

// TestServeCloseError checks that a failed handle Close is reported to the
// client, not swallowed (which would be silent data loss).
func TestServeCloseError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0644))
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	real, err := s.vfs.OpenFile("f.txt", os.O_RDONLY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = real.Close() })
	c := newConn(s, nil)
	of := &openFile{handle: closeErrHandle{real}, node: real.Node()}
	of.fileID[0] = 1
	c.handles[of.fileID] = of

	body := make([]byte, 24)
	copy(body[8:24], of.fileID[:]) // FileId
	status, _ := c.handleClose(header{}, body)
	require.NotEqual(t, statusSuccess, status, "CLOSE must surface a handle Close() error, not report success")
}

// TestServeShutdown checks that Shutdown returns promptly (does not hang on
// wg.Wait) while a connection is live.
func TestServeShutdown(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	go func() { _ = s.Serve() }()

	nc, err := net.Dial("tcp", s.Addr().String())
	require.NoError(t, err)
	defer func() { _ = nc.Close() }()
	time.Sleep(100 * time.Millisecond) // let the connection register

	done := make(chan struct{})
	go func() { _ = s.Shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown hung with a live connection")
	}
}

// TestServeVerifySignature checks that a signed request from an authenticated
// session is accepted only if its signature is valid.
func TestServeVerifySignature(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	c := newConn(s, nil)
	key := make([]byte, 16)                // a session signing key
	c.sessions[0] = &session{signKey: key} // the signed ECHO below is on session 0
	c.setDialect(dialect202)               // HMAC-SHA256 signing path

	// A correctly signed ECHO request.
	seg := make([]byte, smb2HeaderSize+4)
	copy(seg[0:4], smb2Magic)
	le.PutUint16(seg[4:6], smb2HeaderSize) // header StructureSize
	le.PutUint16(seg[12:14], cmdEcho)      // Command
	le.PutUint32(seg[16:20], flagsSigned)  // Flags: SIGNED
	le.PutUint16(seg[smb2HeaderSize:], 4)  // ECHO StructureSize
	signMessage(key, c.getDialect(), seg)
	h, ok := parseHeader(seg)
	require.True(t, ok)

	resp := c.handleCommand(h, seg, &chainCtx{})
	require.Equal(t, statusSuccess, le.Uint32(resp[8:12]), "a correctly signed request must be accepted")

	seg[48] ^= 0xFF // tamper the signature
	resp = c.handleCommand(h, seg, &chainCtx{})
	require.Equal(t, statusAccessDenied, le.Uint32(resp[8:12]), "a bad signature must be rejected")
}

// signedEchoSeg builds an ECHO request PDU for the given session, signed with
// the given key when signWith is non-nil (setting the SIGNED flag).
func signedEchoSeg(sessionID uint64, dialect uint16, signWith []byte) []byte {
	seg := make([]byte, smb2HeaderSize+4)
	copy(seg[0:4], smb2Magic)
	le.PutUint16(seg[4:6], smb2HeaderSize)
	le.PutUint16(seg[12:14], cmdEcho)
	le.PutUint64(seg[40:48], sessionID)   // SessionId
	le.PutUint16(seg[smb2HeaderSize:], 4) // ECHO StructureSize
	if signWith != nil {
		signMessage(signWith, dialect, seg)
	}
	return seg
}

// echoStatus dispatches an ECHO seg through handleCommand and returns its status.
func echoStatus(c *conn, seg []byte) uint32 {
	h, _ := parseHeader(seg)
	return le.Uint32(c.handleCommand(h, seg, &chainCtx{})[8:12])
}

// TestServeRequiresSigning checks that once a session is authenticated (has a
// signing key), an unsigned command is rejected -- a man-in-the-middle can't
// strip the signature and tamper with the request.
func TestServeRequiresSigning(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	c := newConn(s, nil)
	c.setDialect(dialect202)
	key := make([]byte, 16)
	c.sessions[0] = &session{signKey: key, authed: true}

	require.Equal(t, statusAccessDenied, echoStatus(c, signedEchoSeg(0, c.getDialect(), nil)),
		"an unsigned command on an authenticated session must be rejected")
	require.Equal(t, statusSuccess, echoStatus(c, signedEchoSeg(0, c.getDialect(), key)),
		"a correctly signed command on the same session must succeed")
}

// TestServeMultiSession checks that signing state is per session: two sessions
// on one connection keep independent keys, so one session's signature is never
// validated against another's key (the pre-fix per-connection key clobbered).
func TestServeMultiSession(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	c := newConn(s, nil)
	c.setDialect(dialect202)
	keyA := make([]byte, 16)
	keyB := make([]byte, 16)
	keyB[0] = 0xAA // a distinct key
	c.sessions[1] = &session{signKey: keyA, authed: true}
	c.sessions[2] = &session{signKey: keyB, authed: true}

	require.Equal(t, statusSuccess, echoStatus(c, signedEchoSeg(1, c.getDialect(), keyA)),
		"keyA signature must verify on its own session")
	require.Equal(t, statusAccessDenied, echoStatus(c, signedEchoSeg(2, c.getDialect(), keyA)),
		"session 1's key must not verify session 2's request")
	require.Equal(t, statusSuccess, echoStatus(c, signedEchoSeg(2, c.getDialect(), keyB)),
		"keyB signature must verify on session 2")
}

// TestServeNegotiateSigningRequired checks that a server with --user advertises
// SMB2_NEGOTIATE_SIGNING_REQUIRED, while a guest server only advertises signing
// as enabled (a guest session has no key to sign with).
func TestServeNegotiateSigningRequired(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	vfsOpt := vfscommon.Opt

	securityMode := func(user, pass string) uint16 {
		opt := Opt
		opt.ListenAddr = "127.0.0.1:0"
		opt.ShareName = "rclone"
		opt.User = user
		opt.Pass = pass
		s, err := newServer(ctx, f, &opt, &vfsOpt)
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Shutdown() })
		c := newConn(s, nil)
		_, resp := c.handleNegotiate(header{}, negotiateBody(dialect202))
		return le.Uint16(resp[2:4]) // SecurityMode
	}

	require.Equal(t, negotiateSigningEnabled|negotiateSigningRequired, securityMode("alice", "s3cret"),
		"an authenticated server must require signing")
	require.Equal(t, negotiateSigningEnabled, securityMode("", ""),
		"a guest server must advertise signing enabled but not required")
}

// TestServeNegotiateLargeMTU checks that LARGE_MTU (which lets clients do IO
// larger than 64 KiB) is advertised for SMB 2.1+ dialects but not for 2.0.2.
func TestServeNegotiateLargeMTU(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	caps := func(d uint16) uint32 {
		c := newConn(s, nil)
		_, resp := c.handleNegotiate(header{}, negotiateBody(d))
		return le.Uint32(resp[24:28]) // Capabilities
	}
	require.NotZero(t, caps(dialect210)&capLargeMTU, "SMB 2.1 must advertise LARGE_MTU")
	require.NotZero(t, caps(dialect302)&capLargeMTU, "SMB 3.0.2 must advertise LARGE_MTU")
	require.Zero(t, caps(dialect202)&capLargeMTU, "SMB 2.0.2 must not advertise LARGE_MTU")
}

// TestServeSignedRelatedCompound checks that a signed related compound command
// (SMB2_FLAGS_RELATED_OPERATIONS) is verified over the bytes as signed, before
// its placeholder FileId is substituted from the preceding CREATE. Verifying
// after substitution rewrites the request buffer and wrongly rejects every
// signed CREATE+op+CLOSE chain -- which is how cifs / mount_smbfs operate.
func TestServeSignedRelatedCompound(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	c := newConn(s, nil)
	c.setDialect(dialect202)
	key := make([]byte, 16)
	c.sessions[0] = &session{signKey: key}

	// A signed CLOSE with RELATED_OPERATIONS and a placeholder (0xFF) FileId.
	seg := make([]byte, smb2HeaderSize+24)
	copy(seg[0:4], smb2Magic)
	le.PutUint16(seg[4:6], smb2HeaderSize)
	le.PutUint16(seg[12:14], cmdClose)
	le.PutUint32(seg[16:20], flagsSigned|flagsRelatedOps)
	le.PutUint16(seg[smb2HeaderSize:], 24) // CLOSE StructureSize
	for i := smb2HeaderSize + 8; i < smb2HeaderSize+24; i++ {
		seg[i] = 0xFF // FileId placeholder (real id comes from the preceding CREATE)
	}
	signMessage(key, c.getDialect(), seg)
	h, ok := parseHeader(seg)
	require.True(t, ok)

	// A preceding CREATE in the chain established this real FileId.
	var realID [16]byte
	realID[0] = 0x42
	chain := &chainCtx{fileID: realID[:]}

	resp := c.handleCommand(h, seg, chain)
	require.NotEqual(t, statusAccessDenied, le.Uint32(resp[8:12]),
		"a signed related compound request must be verified before its FileId is substituted")
}

// setInfoRenameBody builds a SET_INFO FileRenameInformation request body.
func setInfoRenameBody(fileID [16]byte, target string, replace bool) []byte {
	name := stringToUTF16le(target)
	info := make([]byte, 20+len(name))
	if replace {
		info[0] = 1 // ReplaceIfExists
	}
	le.PutUint32(info[16:20], uint32(len(name))) // FileNameLength
	copy(info[20:], name)

	body := make([]byte, 32+len(info))
	body[2] = infoTypeFile                      // InfoType
	body[3] = classFileRename                   // FileInfoClass
	le.PutUint32(body[4:8], uint32(len(info)))  // BufferLength
	le.PutUint16(body[8:10], smb2HeaderSize+32) // BufferOffset (from header)
	copy(body[16:32], fileID[:])                // FileId
	copy(body[32:], info)
	return body
}

// TestServeRenameReplaceIfExists checks that a rename onto an existing target
// fails with a collision unless ReplaceIfExists is set.
func TestServeRenameReplaceIfExists(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src.txt"), []byte("s"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dst.txt"), []byte("d"), 0644))
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	node, err := s.vfs.Stat("src.txt")
	require.NoError(t, err)
	c := newConn(s, nil)
	of := &openFile{path: "src.txt", node: node}
	of.fileID[0] = 1
	c.handles[of.fileID] = of

	// Onto an existing target with ReplaceIfExists=0 -> collision.
	status, _ := c.handleSetInfo(header{}, setInfoRenameBody(of.fileID, "dst.txt", false))
	require.Equal(t, statusObjectNameCollision, status, "rename onto an existing target must collide")

	// Onto a free name -> success.
	status, _ = c.handleSetInfo(header{}, setInfoRenameBody(of.fileID, "moved.txt", false))
	require.Equal(t, statusSuccess, status)
}

// createSeg builds a CREATE request PDU that opens the named file.
func createSeg(name string) []byte {
	n := stringToUTF16le(name)
	body := make([]byte, 56+len(n))
	le.PutUint16(body[0:2], 57)                  // StructureSize
	le.PutUint32(body[36:40], dispOpen)          // CreateDisposition = OPEN
	le.PutUint16(body[44:46], smb2HeaderSize+56) // NameOffset (from header)
	le.PutUint16(body[46:48], uint16(len(n)))    // NameLength
	copy(body[56:], n)
	seg := make([]byte, smb2HeaderSize+len(body))
	copy(seg[smb2HeaderSize:], body)
	return seg
}

// TestServeIPCTreeRejectsCreate checks that opening a name on the IPC$ tree is
// rejected instead of being resolved as a VFS path (which could open a real
// file that happens to share a pipe's name, e.g. srvsvc).
func TestServeIPCTreeRejectsCreate(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "srvsvc"), []byte("data"), 0644))
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })
	c := newConn(s, nil)

	// On the disk share, opening the real file works.
	respD := c.handleCommand(header{command: cmdTreeConnect}, treeConnectSeg("rclone"), &chainCtx{})
	diskTree := le.Uint32(respD[36:40])
	respC := c.handleCommand(header{command: cmdCreate, treeID: diskTree}, createSeg("srvsvc"), &chainCtx{})
	require.Equal(t, statusSuccess, le.Uint32(respC[8:12]), "opening a real file on the disk share must work")

	// On the IPC$ tree, the same open must be rejected.
	respI := c.handleCommand(header{command: cmdTreeConnect}, treeConnectSeg("IPC$"), &chainCtx{})
	pipeTree := le.Uint32(respI[36:40])
	respP := c.handleCommand(header{command: cmdCreate, treeID: pipeTree}, createSeg("srvsvc"), &chainCtx{})
	require.Equal(t, statusObjectNameNotFound, le.Uint32(respP[8:12]), "CREATE on the IPC$ tree must be rejected")
}

// negotiateBody builds a NEGOTIATE request body offering the given dialects.
func negotiateBody(dialects ...uint16) []byte {
	body := make([]byte, 36+len(dialects)*2)
	le.PutUint16(body[0:2], 36)                    // StructureSize
	le.PutUint16(body[2:4], uint16(len(dialects))) // DialectCount
	for i, d := range dialects {
		le.PutUint16(body[36+i*2:], d)
	}
	return body
}

// TestServeNegotiateNoOverlap checks that NEGOTIATE fails when the client offers
// no dialect we support, rather than forcing an unrequested 2.0.2.
func TestServeNegotiateNoOverlap(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })
	c := newConn(s, nil)

	status, _ := c.handleNegotiate(header{}, negotiateBody(dialect311)) // 3.1.1 only (unsupported)
	require.Equal(t, statusNotSupported, status, "no supported dialect must fail NEGOTIATE")

	status, _ = c.handleNegotiate(header{}, negotiateBody(dialect202))
	require.Equal(t, statusSuccess, status)
}

// TestServeHandleCap checks that a connection can't open more than maxOpenFiles
// handles (a leaky client would otherwise exhaust file descriptors).
func TestServeHandleCap(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0644))
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })
	c := newConn(s, nil)

	for i := 0; i < maxOpenFiles; i++ {
		var id [16]byte
		le.PutUint64(id[:8], uint64(i)+1)
		c.handles[id] = &openFile{}
	}
	resp := c.handleCommand(header{command: cmdCreate}, createSeg("f.txt"), &chainCtx{})
	require.Equal(t, statusInsufficientResources, le.Uint32(resp[8:12]),
		"a CREATE past the per-connection handle cap must be refused")
}

// TestServeShutdownCancelsContext checks that Shutdown cancels the server
// context so in-flight VFS operations on a wedged backend are interrupted.
func TestServeShutdownCancelsContext(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	require.NoError(t, s.ctx.Err(), "server context must be live before Shutdown")
	require.NoError(t, s.Shutdown())
	require.Error(t, s.ctx.Err(), "Shutdown must cancel the server context")
}

// TestServeRequirePass checks that --user without --pass is rejected.
func TestServeRequirePass(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	vfsOpt := vfscommon.Opt
	mk := func(user, pass string) error {
		opt := Opt
		opt.ListenAddr = "127.0.0.1:0"
		opt.ShareName = "rclone"
		opt.User = user
		opt.Pass = pass
		s, err := newServer(ctx, f, &opt, &vfsOpt)
		if s != nil {
			t.Cleanup(func() { _ = s.Shutdown() })
		}
		return err
	}
	require.Error(t, mk("alice", ""), "--user without --pass must be rejected")
	require.NoError(t, mk("alice", "s3cret"))
	require.NoError(t, mk("", "")) // guest is fine
}

// TestServeProtocolNits covers a few edge-case status codes.
func TestServeProtocolNits(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello"), 0644))
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })
	c := newConn(s, nil)

	// Opening a regular file with FILE_DIRECTORY_FILE -> NOT_A_DIRECTORY.
	seg := createSeg("f.txt")
	le.PutUint32(seg[smb2HeaderSize+40:], optDirectoryFile) // CreateOptions
	resp := c.handleCommand(header{command: cmdCreate}, seg, &chainCtx{})
	require.Equal(t, statusNotADirectory, le.Uint32(resp[8:12]), "FILE_DIRECTORY_FILE on a file must fail")

	handle, err := s.vfs.OpenFile("f.txt", os.O_RDONLY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })
	of := &openFile{path: "f.txt", handle: handle, node: handle.Node()}
	of.fileID[0] = 1
	c.handles[of.fileID] = of

	// Zero-length READ -> SUCCESS.
	rbody := make([]byte, 48)
	copy(rbody[16:32], of.fileID[:]) // Length stays 0
	status, _ := c.handleRead(header{}, rbody)
	require.Equal(t, statusSuccess, status, "zero-length READ must succeed")

	// SET_INFO with an unknown info class -> INVALID_INFO_CLASS.
	sbody := make([]byte, 32)
	sbody[2] = infoTypeFile
	sbody[3] = 0xEE // unknown FileInfoClass
	le.PutUint16(sbody[8:10], smb2HeaderSize+32)
	copy(sbody[16:32], of.fileID[:])
	status, _ = c.handleSetInfo(header{}, sbody)
	require.Equal(t, statusInvalidInfoClass, status, "unknown SET_INFO class must be rejected")
}

// TestServeDoubleShutdown checks that a second Shutdown is a no-op, not an error.
func TestServeDoubleShutdown(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	require.NoError(t, s.Shutdown())
	require.NoError(t, s.Shutdown(), "a second Shutdown must be a no-op, not an error")
}

// TestServeQueryDirUnreadable checks that a directory whose contents can't be
// listed (locked/denied) reports as empty (STATUS_NO_MORE_FILES) rather than
// failing the request -- a generic failure makes the Windows shell copy engine
// abort a whole recursive copy.
func TestServeQueryDirUnreadable(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })
	c := newConn(s, nil)

	// A directory handle whose path can't be listed (here it doesn't exist, which
	// makes listDir error the same way a locked/denied directory does).
	of := &openFile{path: "nope-does-not-exist", isDir: true}
	of.fileID[0] = 1
	c.handles[of.fileID] = of

	body := make([]byte, 32)
	body[2] = 0x25 // FileIdBothDirectoryInformation
	copy(body[8:24], of.fileID[:])
	le.PutUint32(body[28:32], 65536) // OutputBufferLength
	status, _ := c.handleQueryDirectory(header{}, body)
	require.Equal(t, statusNoMoreFiles, status,
		"an unreadable directory must report empty, not fail the request")
}

// TestServeQueryInfoOutputBufferLength checks that QUERY_INFO never returns
// more than the client's OutputBufferLength. Windows' GetFileInformationByHandle
// reads the volume serial number with a buffer sized for the fixed part of
// FileFsVolumeInformation only; a longer reply is rejected as an invalid network
// response, which fails the caller's open.
func TestServeQueryInfoOutputBufferLength(t *testing.T) {
	dir := t.TempDir()
	const name = "a-rather-long-file-name.txt"
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0644))

	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	c := newConn(s, nil)
	node, err := s.vfs.Stat(name)
	require.NoError(t, err)
	of := &openFile{path: name, node: node}
	of.fileID[0] = 1
	c.handles[of.fileID] = of

	// QUERY_INFO body: InfoType[2], FileInfoClass[3], OutputBufferLength[4:8], FileId[24:40].
	query := func(infoType, infoClass byte, outLen uint32) (uint32, []byte) {
		body := make([]byte, 40)
		body[2], body[3] = infoType, infoClass
		le.PutUint32(body[4:8], outLen)
		copy(body[24:40], of.fileID[:])
		status, resp := c.handleQueryInfo(header{}, body)
		if status != statusSuccess && status != statusBufferOverflow {
			return status, nil
		}
		info := resp[8:] // QUERY_INFO response: 8-byte header, then the info buffer
		require.Equal(t, uint32(len(info)), le.Uint32(resp[4:8]), "OutputBufferLength must match the buffer sent")
		return status, info
	}

	// A buffer big enough for everything: the whole structure, STATUS_SUCCESS.
	status, whole := query(infoTypeFilesystem, classFsVolume, 1<<16)
	require.Equal(t, statusSuccess, status)
	labelLen := le.Uint32(whole[12:16])
	require.Equal(t, 18+int(labelLen), len(whole))

	// What Windows sends: room for the fixed part but not the whole label. The
	// reply is cut to fit and flagged, and still reports the full label length.
	status, info := query(infoTypeFilesystem, classFsVolume, 24)
	require.Equal(t, statusBufferOverflow, status)
	require.Equal(t, whole[:24], info)

	// FileAllInformation with a buffer that can't hold the whole file name.
	status, info = query(infoTypeFile, classFileAll, 104)
	require.Equal(t, statusBufferOverflow, status)
	require.Len(t, info, 104)
	require.Equal(t, uint32(2*len(name)), le.Uint32(info[96:100]), "FileNameLength must be the full length")

	// Too small for even the fixed part, or for a fixed-size structure at all.
	status, _ = query(infoTypeFilesystem, classFsVolume, 8)
	require.Equal(t, statusInfoLengthMismatch, status)
	status, _ = query(infoTypeFilesystem, classFsSize, 8)
	require.Equal(t, statusInfoLengthMismatch, status)
	status, _ = query(infoTypeFile, classFileBasic, 39)
	require.Equal(t, statusInfoLengthMismatch, status)
}

// TestServeDeleteNonEmptyDir checks that marking a directory that still has
// entries for deletion fails up front with STATUS_DIRECTORY_NOT_EMPTY. The delete
// itself happens at CLOSE, whose status Windows ignores, so accepting the request
// makes RemoveDirectory report success while nothing is removed.
func TestServeDeleteNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "d"), 0777))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "d", "f.txt"), nil, 0644))

	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })
	c := newConn(s, nil)

	// CREATE body: DesiredAccess[24:28], CreateDisposition[36:40], CreateOptions[40:44],
	// NameOffset[44:46] (from the SMB2 header), NameLength[46:48], then the name.
	create := func(options uint32) (uint32, []byte) {
		name := stringToUTF16le("d")
		body := make([]byte, 56+len(name))
		le.PutUint32(body[36:40], dispOpen)
		le.PutUint32(body[40:44], optDirectoryFile|options)
		le.PutUint16(body[44:46], smb2HeaderSize+56)
		le.PutUint16(body[46:48], uint16(len(name)))
		copy(body[56:], name)
		return c.handleCreate(header{}, body)
	}
	// SET_INFO body: InfoType[2], FileInfoClass[3], BufferLength[4:8], BufferOffset[8:10]
	// (from the SMB2 header), FileId[16:32], then FileDispositionInformation.DeletePending.
	setDeletePending := func(fileID []byte) uint32 {
		body := make([]byte, 33)
		body[2], body[3] = infoTypeFile, classFileDisposition
		le.PutUint32(body[4:8], 1)
		le.PutUint16(body[8:10], smb2HeaderSize+32)
		copy(body[16:32], fileID)
		body[32] = 1
		status, _ := c.handleSetInfo(header{}, body)
		return status
	}
	closeHandle := func(fileID []byte) uint32 {
		body := make([]byte, 24)
		copy(body[8:24], fileID)
		status, _ := c.handleClose(header{}, body)
		return status
	}

	// Opening the non-empty directory with FILE_DELETE_ON_CLOSE is refused.
	status, _ := create(optDeleteOnClose)
	require.Equal(t, statusDirectoryNotEmpty, status)

	// So is marking it for deletion on an ordinary open, and the directory survives the close.
	status, resp := create(0)
	require.Equal(t, statusSuccess, status)
	fileID := resp[64:80]
	require.Equal(t, statusDirectoryNotEmpty, setDeletePending(fileID))
	require.Equal(t, statusSuccess, closeHandle(fileID))
	require.DirExists(t, filepath.Join(dir, "d"))

	// Once it is empty the same requests delete it.
	child, err := s.vfs.Stat("d/f.txt")
	require.NoError(t, err)
	require.NoError(t, child.Remove())
	status, resp = create(0)
	require.Equal(t, statusSuccess, status)
	fileID = resp[64:80]
	require.Equal(t, statusSuccess, setDeletePending(fileID))
	require.Equal(t, statusSuccess, closeHandle(fileID))
	require.NoDirExists(t, filepath.Join(dir, "d"))
}

// TestServeClosePostQueryAttrib checks that a CLOSE carrying
// SMB2_CLOSE_FLAG_POSTQUERY_ATTRIB is answered with the file's attributes, as
// [MS-SMB2] 3.3.5.10 requires. macOS sets it on every close and logs a "Bad SMB
// 2/3 Server" kernel message for each reply that comes back without them.
func TestServeClosePostQueryAttrib(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.bin"), make([]byte, 12345), 0644))

	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })
	c := newConn(s, nil)

	// Opens file.bin and closes it again with the given CLOSE Flags[2:4], returning the response.
	openClose := func(flags uint16) []byte {
		name := stringToUTF16le("file.bin")
		body := make([]byte, 56+len(name))
		le.PutUint32(body[36:40], dispOpen)
		le.PutUint16(body[44:46], smb2HeaderSize+56)
		le.PutUint16(body[46:48], uint16(len(name)))
		copy(body[56:], name)
		status, resp := c.handleCreate(header{}, body)
		require.Equal(t, statusSuccess, status)

		body = make([]byte, 24)
		le.PutUint16(body[2:4], flags)
		copy(body[8:24], resp[64:80])
		status, resp = c.handleClose(header{}, body)
		require.Equal(t, statusSuccess, status)
		require.Len(t, resp, 60)
		return resp
	}

	// CLOSE response: Flags[2:4], four times [8:40], AllocationSize[40:48], EndofFile[48:56], FileAttributes[56:60].
	resp := openClose(0x0001)
	require.Equal(t, uint16(0x0001), le.Uint16(resp[2:4]), "the flag must be echoed when the attributes are returned")
	require.NotZero(t, le.Uint64(resp[24:32]), "LastWriteTime")
	require.Equal(t, uint64(12345), le.Uint64(resp[48:56]), "EndofFile")
	require.Equal(t, fileAttrNormal, le.Uint32(resp[56:60]))

	// Without the flag every attribute field must be zero.
	require.Equal(t, make([]byte, 58), openClose(0)[2:])
}

// TestServeConcurrentMkdir checks that when several connections create the same
// directory at once - as a multi-transfer copy into a new directory does - exactly
// one of them creates it. If two did, the second's node would displace the
// first's, taking with it any file already created there that has not reached the
// backend yet.
func TestServeConcurrentMkdir(t *testing.T) {
	ctx := context.Background()
	f, err := fs.NewFs(ctx, t.TempDir())
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	const clients = 8
	for i := 0; i < 50; i++ {
		name := stringToUTF16le(fmt.Sprintf("d%d", i))
		body := make([]byte, 56+len(name))
		le.PutUint32(body[36:40], dispCreate)
		le.PutUint32(body[40:44], optDirectoryFile)
		le.PutUint16(body[44:46], smb2HeaderSize+56)
		le.PutUint16(body[46:48], uint16(len(name)))
		copy(body[56:], name)

		var wg sync.WaitGroup
		start := make(chan struct{})
		statuses := make([]uint32, clients)
		for j := range statuses {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c := newConn(s, nil)
				<-start
				statuses[j], _ = c.handleCreate(header{}, body)
			}()
		}
		close(start)
		wg.Wait()

		created := 0
		for _, status := range statuses {
			if status == statusSuccess {
				created++
			} else {
				require.Equal(t, statusObjectNameCollision, status)
			}
		}
		require.Equal(t, 1, created, "FILE_CREATE of one directory must succeed for exactly one client")
	}
}

// TestServeNamespaceOrder checks that requests which change the namespace take
// effect in the order they arrive, though requests are served concurrently.
// Windows does not wait for the reply to a CLOSE before sending what follows, so
// deleting a file (which happens at its CLOSE) and then its directory arrives as
// two requests back to back; the directory must not be found still to hold the file.
func TestServeNamespaceOrder(t *testing.T) {
	const n = 100
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		require.NoError(t, os.Mkdir(filepath.Join(dir, fmt.Sprintf("d%d", i)), 0777))
		require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("d%d", i), "f"), nil, 0644))
	}

	ctx := context.Background()
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)
	opt := Opt
	opt.ListenAddr = "127.0.0.1:0"
	opt.ShareName = "rclone"
	vfsOpt := vfscommon.Opt
	s, err := newServer(ctx, f, &opt, &vfsOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Shutdown() })

	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	c := newConn(s, server)
	go c.serve()

	request := func(command uint16, messageID uint64, body []byte) []byte {
		msg := make([]byte, smb2HeaderSize+len(body))
		copy(msg[0:4], smb2Magic)
		le.PutUint16(msg[4:6], smb2HeaderSize)
		le.PutUint16(msg[12:14], command)
		le.PutUint64(msg[24:32], messageID)
		copy(msg[smb2HeaderSize:], body)
		return msg
	}
	closeBody := func(fileID []byte) []byte {
		body := make([]byte, 24)
		copy(body[8:24], fileID)
		return body
	}
	// Sends the requests without waiting in between and returns the responses by MessageId.
	exchange := func(requests ...[]byte) map[uint64][]byte {
		go func() {
			for _, req := range requests {
				_ = writeMessage(client, req)
			}
		}()
		responses := map[uint64][]byte{}
		for range requests {
			resp, err := readMessage(client)
			require.NoError(t, err)
			responses[le.Uint64(resp[24:32])] = resp
		}
		return responses
	}

	for i := 0; i < n; i++ {
		// The file is open and marked for deletion, as after DeleteFile's SET_INFO.
		name := fmt.Sprintf("d%d", i)
		node, err := s.vfs.Stat(name + "/f")
		require.NoError(t, err)
		of := &openFile{path: name + "/f", node: node, deleteOnClose: true}
		of.fileID = c.newFileID()
		c.addHandle(of)

		utf16 := stringToUTF16le(name)
		create := make([]byte, 56+len(utf16))
		le.PutUint32(create[36:40], dispOpen)
		le.PutUint32(create[40:44], optDirectoryFile|optDeleteOnClose)
		le.PutUint16(create[44:46], smb2HeaderSize+56)
		le.PutUint16(create[46:48], uint16(len(utf16)))
		copy(create[56:], utf16)

		responses := exchange(request(cmdClose, 1, closeBody(of.fileID[:])), request(cmdCreate, 2, create))
		require.Equal(t, statusSuccess, le.Uint32(responses[1][8:12]), "closing the file")
		resp := responses[2]
		require.Equal(t, statusSuccess, le.Uint32(resp[8:12]), "removing directory %s straight after its last file", name)

		// Closing the directory handle deletes it.
		dirID := resp[smb2HeaderSize+64 : smb2HeaderSize+80]
		require.Equal(t, statusSuccess, le.Uint32(exchange(request(cmdClose, 3, closeBody(dirID)))[3][8:12]))
		require.NoDirExists(t, filepath.Join(dir, name))
	}
}

// TestRc checks that serve smb can be started and stopped via the rc
// serve/start and serve/stop calls.
func TestRc(t *testing.T) {
	servetest.TestRc(t, rc.Params{
		"type":           "smb",
		"vfs_cache_mode": "off",
	})
}

func dirNames(entries []os.FileInfo) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	sort.Strings(names)
	return names
}
