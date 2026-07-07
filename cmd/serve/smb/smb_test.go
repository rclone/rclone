package smb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	smb2 "github.com/cloudsoda/go-smb2"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
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
	c.authedSessions[999] = struct{}{}
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
	c.signKey = make([]byte, 16) // a session signing key
	c.dialect = dialect202       // HMAC-SHA256 signing path

	// A correctly signed ECHO request.
	seg := make([]byte, smb2HeaderSize+4)
	copy(seg[0:4], smb2Magic)
	le.PutUint16(seg[4:6], smb2HeaderSize) // header StructureSize
	le.PutUint16(seg[12:14], cmdEcho)      // Command
	le.PutUint32(seg[16:20], flagsSigned)  // Flags: SIGNED
	le.PutUint16(seg[smb2HeaderSize:], 4)  // ECHO StructureSize
	signMessage(c.signKey, c.dialect, seg)
	h, ok := parseHeader(seg)
	require.True(t, ok)

	resp := c.handleCommand(h, seg, &chainCtx{})
	require.Equal(t, statusSuccess, le.Uint32(resp[8:12]), "a correctly signed request must be accepted")

	seg[48] ^= 0xFF // tamper the signature
	resp = c.handleCommand(h, seg, &chainCtx{})
	require.Equal(t, statusAccessDenied, le.Uint32(resp[8:12]), "a bad signature must be rejected")
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

func dirNames(entries []os.FileInfo) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	sort.Strings(names)
	return names
}
