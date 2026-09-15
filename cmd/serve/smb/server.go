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

// conn is a single client TCP connection. SMB2 messages on a connection are
// processed one at a time by serve, so the handle table is only accessed from
// that goroutine (the mutex guards against the Shutdown path).
type conn struct {
	server *Server
	nc     net.Conn

	dialect            uint16              // negotiated SMB dialect
	authChallenge      [8]byte             // NTLM server challenge for the in-progress session setup
	signKey            []byte              // message signing key (nil for guest/unsigned sessions)
	warnedWindowsGuest bool                // a Windows-client-as-guest warning was already logged
	authedSessions     map[uint64]struct{} // SessionIds that completed authentication (only enforced when opt.User != "")
	pipeTrees          map[uint32]struct{} // TreeIds of IPC$ (named-pipe) shares; file opens on them are rejected

	mu        sync.Mutex
	handles   map[[16]byte]*openFile
	handleCtr uint64
}

func newConn(s *Server, nc net.Conn) *conn {
	return &conn{
		server:         s,
		nc:             nc,
		handles:        map[[16]byte]*openFile{},
		authedSessions: map[uint64]struct{}{},
		pipeTrees:      map[uint32]struct{}{},
	}
}

// serve reads and dispatches SMB2 messages until the connection closes.
func (c *conn) serve() {
	defer func() {
		// A panic in one request handler must not take down the whole rclone
		// process; log it and drop just this connection.
		if r := recover(); r != nil {
			fs.Errorf(c.server.vfs.Fs(), "SMB: recovered from panic serving %s: %v", c.nc.RemoteAddr(), r)
		}
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
		out, err := c.dispatch(msg)
		if err != nil {
			fs.Debugf(c.server.vfs.Fs(), "SMB: dispatch error: %v", err)
			return
		}
		if out != nil {
			if err := writeMessage(c.nc, out); err != nil {
				return
			}
		}
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
	return c.assembleResponse(pdus), nil
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
	// Related compound commands inherit the session, tree and file handle of the
	// preceding command rather than carrying their own.
	if h.flags&flagsRelatedOps != 0 {
		sessionID = ctx.sessionID
		treeID = ctx.treeID
		if ctx.fileID != nil {
			substituteFileID(h.command, body, ctx.fileID)
		}
	} else {
		ctx.sessionID = sessionID
		ctx.treeID = treeID
	}

	// Enforce authentication: with a user configured, every command other than
	// the pre-auth handshake (NEGOTIATE/SESSION_SETUP) requires a SessionId that
	// finished authenticating. Without this a client could skip SESSION_SETUP and
	// still TREE_CONNECT and open files.
	if c.server.opt.User != "" && h.command != cmdNegotiate && h.command != cmdSessionSetup {
		if _, ok := c.authedSessions[sessionID]; !ok {
			fs.Debugf(c.server.vfs.Fs(), "SMB: rejected cmd=%d on unauthenticated session %d", h.command, sessionID)
			return buildResponse(h, h.command, statusUserSessionDeleted, sessionID, treeID, credits, errorResponseBody())
		}
	}

	// Verify the signature of a signed request from an authenticated session so a
	// man-in-the-middle can't tamper with it (we already sign our responses).
	if c.signKey != nil && h.flags&flagsSigned != 0 && !verifyMessage(c.signKey, c.dialect, seg) {
		fs.Debugf(c.server.vfs.Fs(), "SMB: rejected cmd=%d with a bad signature on session %d", h.command, sessionID)
		return buildResponse(h, h.command, statusAccessDenied, sessionID, treeID, credits, errorResponseBody())
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
		if _, isPipe := c.pipeTrees[treeID]; isPipe {
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
// message, setting each NextCommand offset and signing each PDU.
func (c *conn) assembleResponse(pdus [][]byte) []byte {
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
	if c.signKey != nil {
		for _, s := range spans {
			signMessage(c.signKey, c.dialect, out[s.start:s.end])
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
