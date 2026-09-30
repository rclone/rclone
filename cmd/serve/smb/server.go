package smb

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Server is an SMB server exposing a single share backed by a VFS.
type Server struct {
	opt        Options
	vfs        *vfs.VFS
	ctx        context.Context    // server-scoped; cancelled on Shutdown to unblock in-flight VFS ops
	cancel     context.CancelFunc // cancels ctx
	shareName  string
	serverGUID [16]byte
	listener   net.Listener
	sessionCtr atomic.Uint64
	treeCtr    atomic.Uint32

	mu      sync.Mutex
	conns   map[*conn]struct{}
	closing chan struct{}
	wg      sync.WaitGroup

	mkdirMu sync.Mutex // serialises directory creation, see mkdir
}

// newServer creates a new SMB server listening on opt.ListenAddr and serving f
// through a VFS. The listener is opened immediately so that Addr reports the
// bound address (which matters when binding to port 0).
func newServer(ctx context.Context, f fs.Fs, opt *Options, vfsOpt *vfscommon.Options) (*Server, error) {
	if opt.User != "" && opt.Pass == "" {
		return nil, errors.New("smb: a password (--pass) is required when --user is set")
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Server{
		opt:       *opt,
		vfs:       vfs.New(ctx, f, vfsOpt),
		ctx:       ctx,
		cancel:    cancel,
		shareName: opt.ShareName,
		conns:     map[*conn]struct{}{},
		closing:   make(chan struct{}),
	}
	if s.shareName == "" {
		s.shareName = "rclone"
	}
	if _, err := rand.Read(s.serverGUID[:]); err != nil {
		cancel()
		s.vfs.Shutdown()
		return nil, err
	}
	l, err := net.Listen("tcp", opt.ListenAddr)
	if err != nil {
		cancel()
		s.vfs.Shutdown()
		return nil, fmt.Errorf("smb: failed to listen on %q: %w", opt.ListenAddr, err)
	}
	s.listener = l
	return s, nil
}

// Addr returns the network address the server is listening on.
func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}

// Serve accepts and serves connections until Shutdown is called.
func (s *Server) Serve() error {
	fs.Logf(s.vfs.Fs(), "SMB server started on %s, share \\\\%s\\%s", s.listener.Addr(), hostOf(s.listener.Addr()), s.shareName)
	if s.opt.User == "" {
		fs.Logf(s.vfs.Fs(), "SMB: running with no authentication (guest access). Windows SMB clients "+
			"will reject guest sessions (a guest session is unsigned and Windows requires signing); "+
			"use --user and --pass for Windows clients. Linux and macOS clients can use guest.")
	}
	if s.vfs.Fs().Features().IsLocal && s.vfs.Opt.CacheMode != vfscommon.CacheModeOff {
		fs.Logf(s.vfs.Fs(), "SMB: serving a local backend with --vfs-cache-mode %v caches files in the "+
			"VFS cache (%s), needlessly duplicating local data; with the cache size unlimited by default a "+
			"very large file can fill that disk. Use --vfs-cache-mode off for a local drive.",
			s.vfs.Opt.CacheMode, config.GetCacheDir())
	}
	var acceptDelay time.Duration
	for {
		nc, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closing:
				return nil
			default:
			}
			// A transient accept error (e.g. fd exhaustion) must not kill the whole
			// server; back off and retry instead of returning.
			if acceptDelay == 0 {
				acceptDelay = 5 * time.Millisecond
			} else {
				acceptDelay *= 2
			}
			if acceptDelay > time.Second {
				acceptDelay = time.Second
			}
			fs.Errorf(s.vfs.Fs(), "SMB: accept error, retrying in %v: %v", acceptDelay, err)
			time.Sleep(acceptDelay)
			continue
		}
		acceptDelay = 0
		fs.Debugf(s.vfs.Fs(), "SMB: accepted connection from %s", nc.RemoteAddr())
		c := newConn(s, nc)
		s.mu.Lock()
		select {
		case <-s.closing:
			// Shutdown already ran and snapshotted s.conns; registering now would
			// leak this connection past that snapshot and hang wg.Wait on it.
			s.mu.Unlock()
			_ = nc.Close()
			return nil
		default:
		}
		s.conns[c] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.wg.Done()
			c.serve()
			s.mu.Lock()
			delete(s.conns, c)
			s.mu.Unlock()
		}()
	}
}

// Shutdown stops the server and closes all active connections.
func (s *Server) Shutdown() error {
	s.mu.Lock()
	select {
	case <-s.closing:
		s.mu.Unlock()
		return nil // already shut down
	default:
		close(s.closing)
	}
	conns := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()

	err := s.listener.Close()
	for _, c := range conns {
		_ = c.nc.Close()
	}
	s.cancel() // interrupt any in-flight VFS operation so wg.Wait can't hang on a wedged backend
	s.wg.Wait()
	s.vfs.Shutdown()
	return err
}

func (s *Server) nextSessionID() uint64 { return s.sessionCtr.Add(1) }
func (s *Server) nextTreeID() uint32    { return s.treeCtr.Add(1) }

// hostOf returns the host portion of a network address for display.
func hostOf(addr net.Addr) string {
	if host, _, err := net.SplitHostPort(addr.String()); err == nil {
		return host
	}
	return addr.String()
}

// conn is a single client TCP connection. Each incoming SMB2 message is handled
// on its own goroutine (see serve), so a slow VFS operation in one request does
// not stall the others or the keepalive ECHOes. mu guards the per-connection
// protocol state; writeMu serialises response writes; each open file carries its
// own mutex for its mutable state.
type conn struct {
	server *Server
	nc     net.Conn

	dialect atomic.Uint32 // negotiated SMB dialect (read from worker goroutines)

	writeMu sync.Mutex     // serialises response writes to nc across workers
	workers sync.WaitGroup // in-flight request workers; serve waits on these before teardown

	// mu guards the per-connection protocol state below. Handlers hold it only
	// while touching this state, releasing it around the (possibly slow) VFS calls
	// so requests are served concurrently.
	mu                 sync.Mutex
	warnedWindowsGuest bool                // a Windows-client-as-guest warning was already logged
	sessions           map[uint64]*session // SMB2 sessions on this connection, keyed by SessionId
	pipeTrees          map[uint32]struct{} // TreeIds of IPC$ shares; file opens on them are rejected
	handles            map[[16]byte]*openFile
	handleCtr          uint64
}

// session is the state of one SMB2 session on a connection. A single connection
// can host several sessions (each with its own SESSION_SETUP), so the signing
// key and in-progress challenge are kept per session, not per connection.
type session struct {
	challenge [8]byte // NTLM server challenge for the in-progress SESSION_SETUP handshake
	signKey   []byte  // message signing key, derived on successful authentication (nil for guest)
	authed    bool    // whether authentication completed (set and enforced only when opt.User != "")
}

func newConn(s *Server, nc net.Conn) *conn {
	return &conn{
		server:    s,
		nc:        nc,
		handles:   map[[16]byte]*openFile{},
		sessions:  map[uint64]*session{},
		pipeTrees: map[uint32]struct{}{},
	}
}

// getDialect and setDialect access the negotiated dialect atomically, as it is
// read from worker goroutines.
func (c *conn) getDialect() uint16  { return uint16(c.dialect.Load()) }
func (c *conn) setDialect(d uint16) { c.dialect.Store(uint32(d)) }

// authState reports whether the session authenticated and returns its signing
// key, both copied out under the lock.
func (c *conn) authState(sessionID uint64) (authed bool, signKey []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if sess := c.sessions[sessionID]; sess != nil {
		return sess.authed, sess.signKey
	}
	return false, nil
}

// isPipeTree reports whether treeID is an IPC$ (named-pipe) tree.
func (c *conn) isPipeTree(treeID uint32) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.pipeTrees[treeID]
	return ok
}

// markPipeTree records treeID as an IPC$ tree.
func (c *conn) markPipeTree(treeID uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pipeTrees[treeID] = struct{}{}
}

// maxConcurrentRequests bounds the in-flight request workers per connection so a
// client that pipelines aggressively can't spawn unbounded goroutines.
const maxConcurrentRequests = 256

// serve reads SMB2 messages and dispatches each on its own goroutine until the
// connection closes.
//
// Messages that change the namespace run one at a time, in the order they
// arrived. Clients count on that: Windows does not wait for the reply to a CLOSE
// before sending what follows, and a CLOSE is where a delete-on-close takes
// effect, so removing a file and then its directory arrives as requests back to
// back of which the second must find the file gone. Everything else - reads,
// writes, queries, keepalives - runs concurrently with them and with each other.
func (c *conn) serve() {
	sem := make(chan struct{}, maxConcurrentRequests)
	var prior chan struct{} // closed once the previous namespace-changing message is done
	defer func() {
		// A panic in the read loop must not take down the whole rclone process.
		if r := recover(); r != nil {
			fs.Errorf(c.server.vfs.Fs(), "SMB: recovered from panic serving %s: %v", c.nc.RemoteAddr(), r)
		}
		// Let in-flight request workers finish (Shutdown cancels the server context,
		// so any wedged VFS call returns) before tearing the handle table down.
		c.workers.Wait()
		c.closeAllHandles()
		_ = c.nc.Close()
	}()
	for {
		msg, err := readMessage(c.nc)
		if err != nil {
			fs.Debugf(c.server.vfs.Fs(), "SMB: connection from %s closed: %v", c.nc.RemoteAddr(), err)
			return
		}
		if len(msg) < smb2HeaderSize {
			continue
		}
		var wait, done chan struct{}
		if changesNamespace(msg) {
			wait, done = prior, make(chan struct{})
			prior = done
		}
		sem <- struct{}{} // bound concurrency; backpressures the read loop when full
		c.workers.Add(1)
		go func() {
			defer c.workers.Done()
			defer func() { <-sem }()
			if done != nil {
				defer close(done)
				if wait != nil {
					<-wait
				}
			}
			c.handleMessage(msg)
		}()
	}
}

// changesNamespace reports whether msg, a single command or a compound chain,
// holds a command that can create, delete, rename or close a file or directory.
func changesNamespace(msg []byte) bool {
	for offset := 0; offset+smb2HeaderSize <= len(msg); {
		h, ok := parseHeader(msg[offset:])
		if !ok {
			return false
		}
		switch h.command {
		case cmdCreate, cmdClose, cmdSetInfo:
			return true
		}
		next := int(h.nextCommand)
		if next < smb2HeaderSize {
			return false
		}
		offset += next
	}
	return false
}

// handleMessage dispatches one incoming message and writes its response. It runs
// on its own goroutine so a slow VFS operation doesn't block the connection's
// other in-flight requests (or its keepalive ECHOes). Responses may be written
// out of request order, which SMB2 clients handle by matching on MessageId.
func (c *conn) handleMessage(msg []byte) {
	defer func() {
		// A panic in one request must not crash the process or drop the other
		// in-flight requests; log it and abandon just this one.
		if r := recover(); r != nil {
			fs.Errorf(c.server.vfs.Fs(), "SMB: recovered from panic handling a request from %s: %v", c.nc.RemoteAddr(), r)
		}
	}()
	out, err := c.dispatch(msg)
	if err != nil {
		// A malformed message is a protocol error: drop the connection, which
		// unblocks the read loop's readMessage and triggers teardown.
		fs.Debugf(c.server.vfs.Fs(), "SMB: dispatch error: %v", err)
		_ = c.nc.Close()
		return
	}
	if out == nil {
		return
	}
	c.writeMu.Lock()
	werr := writeMessage(c.nc, out)
	c.writeMu.Unlock()
	if werr != nil {
		_ = c.nc.Close()
	}
}

// chainCtx carries state shared between the commands of a compound request: the
// FileId established by a CREATE (used by later "related" commands) and the
// session and tree ids to report.
type chainCtx struct {
	fileID    []byte
	sessionID uint64
	treeID    uint32
}

// dispatch handles an incoming SMB2 message, which may be a single command or a
// compound chain of commands ([MS-SMB2] 3.3.5.2.7), and returns the response
// message, or nil if no response should be sent.
func (c *conn) dispatch(msg []byte) ([]byte, error) {
	// The Windows redirector opens with a legacy SMB1 multi-protocol negotiate
	// (0xFF 'SMB', command 0x72 = NEGOTIATE) advertising SMB2 dialects. Reply
	// with an SMB2 negotiate so it switches to SMB2.
	if len(msg) >= 5 && msg[0] == 0xFF && string(msg[1:4]) == "SMB" && msg[4] == 0x72 {
		return c.smb1NegotiateResponse(), nil
	}
	if _, ok := parseHeader(msg); !ok {
		n := len(msg)
		if n > 16 {
			n = 16
		}
		fs.Debugf(c.server.vfs.Fs(), "SMB: unparseable message len=%d first bytes: % x", len(msg), msg[:n])
		return nil, errors.New("invalid SMB2 header")
	}
	var pdus [][]byte
	var ctx chainCtx
	offset := 0
	for offset+smb2HeaderSize <= len(msg) {
		h, ok := parseHeader(msg[offset:])
		if !ok {
			break
		}
		next := int(h.nextCommand)
		var seg []byte
		if next >= smb2HeaderSize && offset+next <= len(msg) {
			seg = msg[offset : offset+next]
		} else {
			seg = msg[offset:]
		}
		if pdu := c.handleCommand(h, seg, &ctx); pdu != nil {
			pdus = append(pdus, pdu)
		}
		if next < smb2HeaderSize {
			break
		}
		offset += next
	}
	// Sign the response(s) with the session's key (the chain shares one session).
	_, signKey := c.authState(ctx.sessionID)
	return c.assembleResponse(pdus, signKey), nil
}

// handleCommand processes one SMB2 command (one element of a possibly compound
// request) and returns its full response PDU (header + body), or nil if no
// response is required.
func (c *conn) handleCommand(h header, seg []byte, ctx *chainCtx) []byte {
	body := seg[smb2HeaderSize:]
	fs.Debugf(c.server.vfs.Fs(), "SMB >> cmd=%d msgid=%d flags=0x%x len=%d", h.command, h.messageID, h.flags, len(seg))

	credits := h.creditReqResp
	if credits < 1 {
		credits = 1
	}

	sessionID := h.sessionID
	treeID := h.treeID
	// Related compound commands inherit the session and tree of the preceding
	// command rather than carrying their own.
	if h.flags&flagsRelatedOps != 0 {
		sessionID = ctx.sessionID
		treeID = ctx.treeID
	} else {
		ctx.sessionID = sessionID
		ctx.treeID = treeID
	}

	// Enforce authentication and message signing on everything but the pre-auth
	// handshake (NEGOTIATE/SESSION_SETUP). This MUST run before substituteFileID
	// below: that rewrites the FileId in the request buffer, so verifying the
	// signature afterwards would be over mutated bytes and reject every signed
	// related compound request (which is how cifs/mount_smbfs send CREATE+X+CLOSE).
	if h.command != cmdNegotiate && h.command != cmdSessionSetup {
		authed, signKey := c.authState(sessionID)
		// With a user configured, a command needs a SessionId that finished
		// authenticating -- otherwise a client could skip SESSION_SETUP and still
		// TREE_CONNECT and open files.
		if c.server.opt.User != "" && !authed {
			fs.Debugf(c.server.vfs.Fs(), "SMB: rejected cmd=%d on unauthenticated session %d", h.command, sessionID)
			return buildResponse(h, h.command, statusUserSessionDeleted, sessionID, treeID, credits, errorResponseBody())
		}
		// An authenticated session has a signing key: require every command to be
		// signed and verify it against that session's key, so a man-in-the-middle
		// can't strip the signature or tamper with the request (we advertise
		// SIGNING_REQUIRED and sign our own responses).
		if signKey != nil && (h.flags&flagsSigned == 0 || !verifyMessage(signKey, c.getDialect(), seg)) {
			fs.Debugf(c.server.vfs.Fs(), "SMB: rejected unsigned or tampered cmd=%d on session %d", h.command, sessionID)
			return buildResponse(h, h.command, statusAccessDenied, sessionID, treeID, credits, errorResponseBody())
		}
	}

	// A related compound command reuses the FileId established by the preceding
	// CREATE; substitute it now, after the signature has been verified above.
	if h.flags&flagsRelatedOps != 0 && ctx.fileID != nil {
		substituteFileID(h.command, body, ctx.fileID)
	}

	status := statusSuccess
	var respBody []byte

	switch h.command {
	case cmdNegotiate:
		status, respBody = c.handleNegotiate(h, body)
	case cmdSessionSetup:
		status, sessionID, respBody = c.handleSessionSetup(h, body)
		ctx.sessionID = sessionID
	case cmdTreeConnect:
		status, treeID, respBody = c.handleTreeConnect(h, body)
		ctx.treeID = treeID
	case cmdTreeDisconnect:
		respBody = treeDisconnectResponseBody()
	case cmdLogoff:
		respBody = logoffResponseBody()
	case cmdEcho:
		respBody = echoResponseBody()
	case cmdCreate:
		if c.isPipeTree(treeID) {
			// We don't serve named pipes; opening one must fail rather than hit the VFS.
			status, respBody = statusObjectNameNotFound, errorResponseBody()
		} else {
			status, respBody = c.handleCreate(h, body)
			if status == statusSuccess && len(respBody) >= 80 {
				ctx.fileID = append([]byte(nil), respBody[64:80]...)
			}
		}
	case cmdClose:
		status, respBody = c.handleClose(h, body)
	case cmdRead:
		status, respBody = c.handleRead(h, body)
	case cmdWrite:
		status, respBody = c.handleWrite(h, body)
	case cmdFlush:
		status, respBody = c.handleFlush(h, body)
	case cmdQueryDirectory:
		status, respBody = c.handleQueryDirectory(h, body)
	case cmdQueryInfo:
		status, respBody = c.handleQueryInfo(h, body)
	case cmdSetInfo:
		status, respBody = c.handleSetInfo(h, body)
	case cmdIoctl:
		status, respBody = c.handleIoctl(h, body)
	case cmdLock:
		respBody = lockResponseBody()
	case cmdCancel:
		return nil // CANCEL has no response
	default:
		status = statusNotSupported
		respBody = errorResponseBody()
	}

	fs.Debugf(c.server.vfs.Fs(), "SMB << cmd=%d status=0x%08x bodylen=%d", h.command, status, len(respBody))
	return buildResponse(h, h.command, status, sessionID, treeID, credits, respBody)
}

// assembleResponse concatenates response PDUs into a single (possibly compound)
// message, setting each NextCommand offset and signing each PDU with signKey
// (nil for guest/unauthenticated sessions, which are not signed).
func (c *conn) assembleResponse(pdus [][]byte, signKey []byte) []byte {
	if len(pdus) == 0 {
		return nil
	}
	type span struct{ start, end int }
	var out []byte
	var spans []span
	for i, pdu := range pdus {
		start := len(out)
		out = append(out, pdu...)
		if i < len(pdus)-1 {
			for len(out)%8 != 0 {
				out = append(out, 0)
			}
			le.PutUint32(out[start+20:start+24], uint32(len(out)-start)) // NextCommand
		}
		spans = append(spans, span{start, len(out)})
	}
	// Sign each PDU once a signing key is established (authenticated sessions).
	if signKey != nil {
		dialect := c.getDialect()
		for _, s := range spans {
			signMessage(signKey, dialect, out[s.start:s.end])
		}
	}
	return out
}

// substituteFileID overwrites the FileId field of a related compound command's
// body with the handle from the preceding CREATE ([MS-SMB2] 3.3.5.2.7.2).
func substituteFileID(command uint16, body, fileID []byte) {
	off := -1
	switch command {
	case cmdClose, cmdFlush, cmdIoctl, cmdQueryDirectory:
		off = 8
	case cmdRead, cmdWrite, cmdSetInfo:
		off = 16
	case cmdQueryInfo:
		off = 24
	}
	if off >= 0 && off+16 <= len(body) {
		copy(body[off:off+16], fileID)
	}
}
