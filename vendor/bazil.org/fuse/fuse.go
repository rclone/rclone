// See the file LICENSE for copyright and licensing information.

// Adapted from Plan 9 from User Space's src/cmd/9pfuse/fuse.c,
// which carries this notice:
//
// The files in this directory are subject to the following license.
//
// The author of this software is Russ Cox.
//
//         Copyright (c) 2006 Russ Cox
//
// Permission to use, copy, modify, and distribute this software for any
// purpose without fee is hereby granted, provided that this entire notice
// is included in all copies of any software which is or includes a copy
// or modification of this software and in all copies of the supporting
// documentation for such software.
//
// THIS SOFTWARE IS BEING PROVIDED "AS IS", WITHOUT ANY EXPRESS OR IMPLIED
// WARRANTY.  IN PARTICULAR, THE AUTHOR MAKES NO REPRESENTATION OR WARRANTY
// OF ANY KIND CONCERNING THE MERCHANTABILITY OF THIS SOFTWARE OR ITS
// FITNESS FOR ANY PARTICULAR PURPOSE.

// Package fuse enables writing FUSE file systems on Linux, OS X, and FreeBSD.
//
// On OS X, it requires OSXFUSE (http://osxfuse.github.com/).
//
// There are two approaches to writing a FUSE file system.  The first is to speak
// the low-level message protocol, reading from a Conn using ReadRequest and
// writing using the various Respond methods.  This approach is closest to
// the actual interaction with the kernel and can be the simplest one in contexts
// such as protocol translators.
//
// Servers of synthesized file systems tend to share common
// bookkeeping abstracted away by the second approach, which is to
// call fs.Serve to serve the FUSE protocol using an implementation of
// the service methods in the interfaces FS* (file system), Node* (file
// or directory), and Handle* (opened file or directory).
// There are a daunting number of such methods that can be written,
// but few are required.
// The specific methods are described in the documentation for those interfaces.
//
// The hellofs subdirectory contains a simple illustration of the fs.Serve approach.
//
// Service Methods
//
// The required and optional methods for the FS, Node, and Handle interfaces
// have the general form
//
//	Op(ctx context.Context, req *OpRequest, resp *OpResponse) error
//
// where Op is the name of a FUSE operation. Op reads request
// parameters from req and writes results to resp. An operation whose
// only result is the error result omits the resp parameter.
//
// Multiple goroutines may call service methods simultaneously; the
// methods being called are responsible for appropriate
// synchronization.
//
// The operation must not hold on to the request or response,
// including any []byte fields such as WriteRequest.Data or
// SetxattrRequest.Xattr.
//
// Errors
//
// Operations can return errors. The FUSE interface can only
// communicate POSIX errno error numbers to file system clients, the
// message is not visible to file system clients. The returned error
// can implement ErrorNumber to control the errno returned. Without
// ErrorNumber, a generic errno (EIO) is returned.
//
// Error messages will be visible in the debug log as part of the
// response.
//
// Interrupted Operations
//
// In some file systems, some operations
// may take an undetermined amount of time.  For example, a Read waiting for
// a network message or a matching Write might wait indefinitely.  If the request
// is cancelled and no longer needed, the context will be cancelled.
// Blocking operations should select on a receive from ctx.Done() and attempt to
// abort the operation early if the receive succeeds (meaning the channel is closed).
// To indicate that the operation failed because it was aborted, return fuse.EINTR.
//
// If an operation does not block for an indefinite amount of time, supporting
// cancellation is not necessary.
//
// Authentication
//
// All requests types embed a Header, meaning that the method can
// inspect req.Pid, req.Uid, and req.Gid as necessary to implement
// permission checking. The kernel FUSE layer normally prevents other
// users from accessing the FUSE file system (to change this, see
// AllowOther, AllowRoot), but does not enforce access modes (to
// change this, see DefaultPermissions).
//
// Mount Options
//
// Behavior and metadata of the mounted file system can be changed by
// passing MountOption values to Mount.
//
package fuse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// A Conn represents a connection to a mounted FUSE file system.
type Conn struct {
	// Ready is closed when the mount is complete or has failed.
	Ready <-chan struct{}

	// MountError stores any error from the mount process. Only valid
	// after Ready is closed.
	MountError error

	// File handle for kernel communication. Only safe to access if
	// rio or wio is held.
	dev *os.File
	wio sync.RWMutex
	rio sync.RWMutex

	// Protocol version negotiated with InitRequest/InitResponse.
	proto Protocol
}

// MountpointDoesNotExistError is an error returned when the
// mountpoint does not exist.
type MountpointDoesNotExistError struct {
	Path string
}

var _ error = (*MountpointDoesNotExistError)(nil)

func (e *MountpointDoesNotExistError) Error() string {
	return fmt.Sprintf("mountpoint does not exist: %v", e.Path)
}

// Mount mounts a new FUSE connection on the named directory
// and returns a connection for reading and writing FUSE messages.
//
// After a successful return, caller must call Close to free
// resources.
//
// Even on successful return, the new mount is not guaranteed to be
// visible until after Conn.Ready is closed. See Conn.MountError for
// possible errors. Incoming requests on Conn must be served to make
// progress.
func Mount(dir string, options ...MountOption) (*Conn, error) {
	conf := mountConfig{
		options: make(map[string]string),
	}
	for _, option := range options {
		if err := option(&conf); err != nil {
			return nil, err
		}
	}

	ready := make(chan struct{}, 1)
	c := &Conn{
		Ready: ready,
	}
	f, err := mount(dir, &conf, ready, &c.MountError)
	if err != nil {
		return nil, err
	}
	c.dev = f

	if err := initMount(c, &conf); err != nil {
		c.Close()
		if err == ErrClosedWithoutInit {
			// see if we can provide a better error
			<-c.Ready
			if err := c.MountError; err != nil {
				return nil, err
			}
		}
		return nil, err
	}

	return c, nil
}

type OldVersionError struct {
	Kernel     Protocol
	LibraryMin Protocol
}

func (e *OldVersionError) Error() string {
	return fmt.Sprintf("kernel FUSE version is too old: %v < %v", e.Kernel, e.LibraryMin)
}

var (
	ErrClosedWithoutInit = errors.New("fuse connection closed without init")
)

func initMount(c *Conn, conf *mountConfig) error {
	req, err := c.ReadRequest()
	if err != nil {
		if err == io.EOF {
			return ErrClosedWithoutInit
		}
		return err
	}
	r, ok := req.(*InitRequest)
	if !ok {
		return fmt.Errorf("missing init, got: %T", req)
	}

	min := Protocol{protoVersionMinMajor, protoVersionMinMinor}
	if r.Kernel.LT(min) {
		req.RespondError(Errno(syscall.EPROTO))
		c.Close()
		return &OldVersionError{
			Kernel:     r.Kernel,
			LibraryMin: min,
		}
	}

	proto := Protocol{protoVersionMaxMajor, protoVersionMaxMinor}
	if r.Kernel.LT(proto) {
		// Kernel doesn't support the latest version we have.
		proto = r.Kernel
	}
	c.proto = proto

	s := &InitResponse{
		Library:      proto,
		MaxReadahead: conf.maxReadahead,
		MaxWrite:     maxWrite,
		Flags:        InitBigWrites | conf.initFlags,
	}
	r.Respond(s)
	return nil
}

// A Request represents a single FUSE request received from the kernel.
// Use a type switch to determine the specific kind.
// A request of unrecognized type will have concrete type *Header.
type Request interface {
	// Hdr returns the Header associated with this request.
	Hdr() *Header

	// RespondError responds to the request with the given error.
	RespondError(error)

	String() string
}

// A RequestID identifies an active FUSE request.
type RequestID uint64

func (r RequestID) String() string {
	return fmt.Sprintf("%#x", uint64(r))
}

// A NodeID is a number identifying a directory or file.
// It must be unique among IDs returned in LookupResponses
// that have not yet been forgotten by ForgetRequests.
type NodeID uint64

func (n NodeID) String() string {
	return fmt.Sprintf("%#x", uint64(n))
}

// A HandleID is a number identifying an open directory or file.
// It only needs to be unique while the directory or file is open.
type HandleID uint64

func (h HandleID) String() string {
	return fmt.Sprintf("%#x", uint64(h))
}

// The RootID identifies the root directory of a FUSE file system.
const RootID NodeID = rootID

// A Header describes the basic information sent in every request.
type Header struct {
	Conn *Conn     `json:"-"` // connection this request was received on
	ID   RequestID // unique ID for request
	Node NodeID    // file or directory the request is about
	Uid  uint32    // user ID of process making request
	Gid  uint32    // group ID of process making request
	Pid  uint32    // process ID of process making request

	// for returning to reqPool
	msg *message
}

func (h *Header) String() string {
	return fmt.Sprintf("ID=%v Node=%v Uid=%d Gid=%d Pid=%d", h.ID, h.Node, h.Uid, h.Gid, h.Pid)
}

func (h *Header) Hdr() *Header {
	return h
}

func (h *Header) noResponse() {
	putMessage(h.msg)
}

func (h *Header) respond(msg []byte) {
	out := (*outHeader)(unsafe.Pointer(&msg[0]))
	out.Unique = uint64(h.ID)
	h.Conn.respond(msg)
	putMessage(h.msg)
}

// An ErrorNumber is an error with a specific error number.
//
// Operations may return an error value that implements ErrorNumber to
// control what specific error number (errno) to return.
type ErrorNumber interface {
	// Errno returns the the error number (errno) for this error.
	Errno() Errno
}

const (
	// ENOSYS indicates that the call is not supported.
	ENOSYS = Errno(syscall.ENOSYS)

	// ESTALE is used by Serve to respond to violations of the FUSE protocol.
	ESTALE = Errno(syscall.ESTALE)

	ENOENT = Errno(syscall.ENOENT)
	EIO    = Errno(syscall.EIO)
	EPERM  = Errno(syscall.EPERM)

	// EINTR indicates request was interrupted by an InterruptRequest.
	// See also fs.Intr.
	EINTR = Errno(syscall.EINTR)

	ERANGE  = Errno(syscall.ERANGE)
	ENOTSUP = Errno(syscall.ENOTSUP)
	EEXIST  = Errno(syscall.EEXIST)
)

// DefaultErrno is the errno used when error returned does not
// implement ErrorNumber.
const DefaultErrno = EIO

var errnoNames = map[Errno]string{
	ENOSYS: "ENOSYS",
	ESTALE: "ESTALE",
	ENOENT: "ENOENT",
	EIO:    "EIO",
	EPERM:  "EPERM",
	EINTR:  "EINTR",
	EEXIST: "EEXIST",
}

// Errno implements Error and ErrorNumber using a syscall.Errno.
type Errno syscall.Errno

var _ = ErrorNumber(Errno(0))
var _ = error(Errno(0))

func (e Errno) Errno() Errno {
	return e
}

func (e Errno) String() string {
	return syscall.Errno(e).Error()
}

func (e Errno) Error() string {
	return syscall.Errno(e).Error()
}

// ErrnoName returns the short non-numeric identifier for this errno.
// For example, "EIO".
func (e Errno) ErrnoName() string {
	s := errnoNames[e]
	if s == "" {
		s = fmt.Sprint(e.Errno())
	}
	return s
}

func (e Errno) MarshalText() ([]byte, error) {
	s := e.ErrnoName()
	return []byte(s), nil
}

func (h *Header) RespondError(err error) {
	errno := DefaultErrno
	if ferr, ok := err.(ErrorNumber); ok {
		errno = ferr.Errno()
	}
	// FUSE uses negative errors!
	// TODO: File bug report against OSXFUSE: positive error causes kernel panic.
	buf := newBuffer(0)
	hOut := (*outHeader)(unsafe.Pointer(&buf[0]))
	hOut.Error = -int32(errno)
	h.respond(buf)
}

// All requests read from the kernel, without data, are shorter than
// this.
var maxRequestSize = syscall.Getpagesize()
var bufSize = maxRequestSize + maxWrite

// reqPool is a pool of messages.
//
// Lifetime of a logical message is from getMessage to putMessage.
// getMessage is called by ReadRequest. putMessage is called by
// Conn.ReadRequest, Request.Respond, or Request.RespondError.
//
// Messages in the pool are guaranteed to have conn and off zeroed,
// buf allocated and len==bufSize, and hdr set.
var reqPool = sync.Pool{
	New: allocMessage,
}

func allocMessage() interface{} {
	m := &message{buf: make([]byte, bufSize)}
	m.hdr = (*inHeader)(unsafe.Pointer(&m.buf[0]))
	return m
}

func getMessage(c *Conn) *message {
	m := reqPool.Get().(*message)
	m.conn = c
	return m
}

func putMessage(m *message) {
	m.buf = m.buf[:bufSize]
	m.conn = nil
	m.off = 0
	reqPool.Put(m)
}

// a message represents the bytes of a single FUSE message
type message struct {
	conn *Conn
	buf  []byte    // all bytes
	hdr  *inHeader // header
	off  int       // offset for reading additional fields
}

func (m *message) len() uintptr {
	return uintptr(len(m.buf) - m.off)
}

func (m *message) data() unsafe.Pointer {
	var p unsafe.Pointer
	if m.off < len(m.buf) {
		p = unsafe.Pointer(&m.buf[m.off])
	}
	return p
}

func (m *message) bytes() []byte {
	return m.buf[m.off:]
}

func (m *message) Header() Header {
	h := m.hdr
	return Header{
		Conn: m.conn,
		ID:   RequestID(h.Unique),
		Node: NodeID(h.Nodeid),
		Uid:  h.Uid,
		Gid:  h.Gid,
		Pid:  h.Pid,

		msg: m,
	}
}

// fileMode returns a Go os.FileMode from a Unix mode.
func fileMode(unixMode uint32) os.FileMode {
	mode := os.FileMode(unixMode & 0777)
	switch unixMode & syscall.S_IFMT {
	case syscall.S_IFREG:
		// nothing
	case syscall.S_IFDIR:
		mode |= os.ModeDir
	case syscall.S_IFCHR:
		mode |= os.ModeCharDevice | os.ModeDevice
	case syscall.S_IFBLK:
		mode |= os.ModeDevice
	case syscall.S_IFIFO:
		mode |= os.ModeNamedPipe
	case syscall.S_IFLNK:
		mode |= os.ModeSymlink
	case syscall.S_IFSOCK:
		mode |= os.ModeSocket
	default:
		// no idea
		mode |= os.ModeDevice
	}
	if unixMode&syscall.S_ISUID != 0 {
		mode |= os.ModeSetuid
	}
	if unixMode&syscall.S_ISGID != 0 {
		mode |= os.ModeSetgid
	}
	return mode
}

type noOpcode struct {
	Opcode uint32
}

func (m noOpcode) String() string {
	return fmt.Sprintf("No opcode %v", m.Opcode)
}

type malformedMessage struct {
}

func (malformedMessage) String() string {
	return "malformed message"
}

// Close closes the FUSE connection.
func (c *Conn) Close() error {
	c.wio.Lock()
	defer c.wio.Unlock()
	c.rio.Lock()
	defer c.rio.Unlock()
	return c.dev.Close()
}

// caller must hold wio or rio
func (c *Conn) fd() int {
	return int(c.dev.Fd())
}

func (c *Conn) Protocol() Protocol {
	return c.proto
}

// ReadRequest returns the next FUSE request from the kernel.
//
// Caller must call either Request.Respond or Request.RespondError in
// a reasonable time. Caller must not retain Request after that call.
func (c *Conn) ReadRequest() (Request, error) {
	m := getMessage(c)
loop:
	c.rio.RLock()
	n, err := syscall.Read(c.fd(), m.buf)
	c.rio.RUnlock()
	if err == syscall.EINTR {
		// OSXFUSE sends EINTR to userspace when a request interrupt
		// completed before it got sent to userspace?
		goto loop
	}
	if err != nil && err != syscall.ENODEV {
		putMessage(m)
		return nil, err
	}
	if n <= 0 {
		putMessage(m)
		return nil, io.EOF
	}
	m.buf = m.buf[:n]

	if n < inHeaderSize {
		putMessage(m)
		return nil, errors.New("fuse: message too short")
	}

	// FreeBSD FUSE sends a short length in the header
	// for FUSE_INIT even though the actual read length is correct.
	if n == inHeaderSize+initInSize && m.hdr.Opcode == opInit && m.hdr.Len < uint32(n) {
		m.hdr.Len = uint32(n)
	}

	// OSXFUSE sometimes sends the wrong m.hdr.Len in a FUSE_WRITE message.
	if m.hdr.Len < uint32(n) && m.hdr.Len >= uint32(unsafe.Sizeof(writeIn{})) && m.hdr.Opcode == opWrite {
		m.hdr.Len = uint32(n)
	}

	if m.hdr.Len != uint32(n) {
		// prepare error message before returning m to pool
		err := fmt.Errorf("fuse: read %d opcode %d but expected %d", n, m.hdr.Opcode, m.hdr.Len)
		putMessage(m)
		return nil, err
	}

	m.off = inHeaderSize

	// Convert to data structures.
	// Do not trust kernel to hand us well-formed data.
	var req Request
	switch m.hdr.Opcode {
	default:
		Debug(noOpcode{Opcode: m.hdr.Opcode})
		goto unrecognized

	case opLookup:
		buf := m.bytes()
		n := len(buf)
		if n == 0 || buf[n-1] != '\x00' {
			goto corrupt
		}
		req = &LookupRequest{
			Header: m.Header(),
			Name:   string(buf[:n-1]),
		}

	case opForget:
		in := (*forgetIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &ForgetRequest{
			Header: m.Header(),
			N:      in.Nlookup,
		}

	case opGetattr:
		switch {
		case c.proto.LT(Protocol{7, 9}):
			req = &GetattrRequest{
				Header: m.Header(),
			}

		default:
			in := (*getattrIn)(m.data())
			if m.len() < unsafe.Sizeof(*in) {
				goto corrupt
			}
			req = &GetattrRequest{
				Header: m.Header(),
				Flags:  GetattrFlags(in.GetattrFlags),
				Handle: HandleID(in.Fh),
			}
		}

	case opSetattr:
		in := (*setattrIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &SetattrRequest{
			Header:   m.Header(),
			Valid:    SetattrValid(in.Valid),
			Handle:   HandleID(in.Fh),
			Size:     in.Size,
			Atime:    time.Unix(int64(in.Atime), int64(in.AtimeNsec)),
			Mtime:    time.Unix(int64(in.Mtime), int64(in.MtimeNsec)),
			Mode:     fileMode(in.Mode),
			Uid:      in.Uid,
			Gid:      in.Gid,
			Bkuptime: in.BkupTime(),
			Chgtime:  in.Chgtime(),
			Flags:    in.Flags(),
		}

	case opReadlink:
		if len(m.bytes()) > 0 {
			goto corrupt
		}
		req = &ReadlinkRequest{
			Header: m.Header(),
		}

	case opSymlink:
		// m.bytes() is "newName\0target\0"
		names := m.bytes()
		if len(names) == 0 || names[len(names)-1] != 0 {
			goto corrupt
		}
		i := bytes.IndexByte(names, '\x00')
		if i < 0 {
			goto corrupt
		}
		newName, target := names[0:i], names[i+1:len(names)-1]
		req = &SymlinkRequest{
			Header:  m.Header(),
			NewName: string(newName),
			Target:  string(target),
		}

	case opLink:
		in := (*linkIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		newName := m.bytes()[unsafe.Sizeof(*in):]
		if len(newName) < 2 || newName[len(newName)-1] != 0 {
			goto corrupt
		}
		newName = newName[:len(newName)-1]
		req = &LinkRequest{
			Header:  m.Header(),
			OldNode: NodeID(in.Oldnodeid),
			NewName: string(newName),
		}

	case opMknod:
		size := mknodInSize(c.proto)
		if m.len() < size {
			goto corrupt
		}
		in := (*mknodIn)(m.data())
		name := m.bytes()[size:]
		if len(name) < 2 || name[len(name)-1] != '\x00' {
			goto corrupt
		}
		name = name[:len(name)-1]
		r := &MknodRequest{
			Header: m.Header(),
			Mode:   fileMode(in.Mode),
			Rdev:   in.Rdev,
			Name:   string(name),
		}
		if c.proto.GE(Protocol{7, 12}) {
			r.Umask = fileMode(in.Umask) & os.ModePerm
		}
		req = r

	case opMkdir:
		size := mkdirInSize(c.proto)
		if m.len() < size {
			goto corrupt
		}
		in := (*mkdirIn)(m.data())
		name := m.bytes()[size:]
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		r := &MkdirRequest{
			Header: m.Header(),
			Name:   string(name[:i]),
			// observed on Linux: mkdirIn.Mode & syscall.S_IFMT == 0,
			// and this causes fileMode to go into it's "no idea"
			// code branch; enforce type to directory
			Mode: fileMode((in.Mode &^ syscall.S_IFMT) | syscall.S_IFDIR),
		}
		if c.proto.GE(Protocol{7, 12}) {
			r.Umask = fileMode(in.Umask) & os.ModePerm
		}
		req = r

	case opUnlink, opRmdir:
		buf := m.bytes()
		n := len(buf)
		if n == 0 || buf[n-1] != '\x00' {
			goto corrupt
		}
		req = &RemoveRequest{
			Header: m.Header(),
			Name:   string(buf[:n-1]),
			Dir:    m.hdr.Opcode == opRmdir,
		}

	case opRename:
		in := (*renameIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		newDirNodeID := NodeID(in.Newdir)
		oldNew := m.bytes()[unsafe.Sizeof(*in):]
		// oldNew should be "old\x00new\x00"
		if len(oldNew) < 4 {
			goto corrupt
		}
		if oldNew[len(oldNew)-1] != '\x00' {
			goto corrupt
		}
		i := bytes.IndexByte(oldNew, '\x00')
		if i < 0 {
			goto corrupt
		}
		oldName, newName := string(oldNew[:i]), string(oldNew[i+1:len(oldNew)-1])
		req = &RenameRequest{
			Header:  m.Header(),
			NewDir:  newDirNodeID,
			OldName: oldName,
			NewName: newName,
		}

	case opOpendir, opOpen:
		in := (*openIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &OpenRequest{
			Header: m.Header(),
			Dir:    m.hdr.Opcode == opOpendir,
			Flags:  openFlags(in.Flags),
		}

	case opRead, opReaddir:
		in := (*readIn)(m.data())
		if m.len() < readInSize(c.proto) {
			goto corrupt
		}
		r := &ReadRequest{
			Header: m.Header(),
			Dir:    m.hdr.Opcode == opReaddir,
			Handle: HandleID(in.Fh),
			Offset: int64(in.Offset),
			Size:   int(in.Size),
		}
		if c.proto.GE(Protocol{7, 9}) {
			r.Flags = ReadFlags(in.ReadFlags)
			r.LockOwner = in.LockOwner
			r.FileFlags = openFlags(in.Flags)
		}
		req = r

	case opWrite:
		in := (*writeIn)(m.data())
		if m.len() < writeInSize(c.proto) {
			goto corrupt
		}
		r := &WriteRequest{
			Header: m.Header(),
			Handle: HandleID(in.Fh),
			Offset: int64(in.Offset),
			Flags:  WriteFlags(in.WriteFlags),
		}
		if c.proto.GE(Protocol{7, 9}) {
			r.LockOwner = in.LockOwner
			r.FileFlags = openFlags(in.Flags)
		}
		buf := m.bytes()[writeInSize(c.proto):]
		if uint32(len(buf)) < in.Size {
			goto corrupt
		}
		r.Data = buf
		req = r

	case opStatfs:
		req = &StatfsRequest{
			Header: m.Header(),
		}

	case opRelease, opReleasedir:
		in := (*releaseIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &ReleaseRequest{
			Header:       m.Header(),
			Dir:          m.hdr.Opcode == opReleasedir,
			Handle:       HandleID(in.Fh),
			Flags:        openFlags(in.Flags),
			ReleaseFlags: ReleaseFlags(in.ReleaseFlags),
			LockOwner:    in.LockOwner,
		}

	case opFsync, opFsyncdir:
		in := (*fsyncIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &FsyncRequest{
			Dir:    m.hdr.Opcode == opFsyncdir,
			Header: m.Header(),
			Handle: HandleID(in.Fh),
			Flags:  in.FsyncFlags,
		}

	case opSetxattr:
		in := (*setxattrIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		m.off += int(unsafe.Sizeof(*in))
		name := m.bytes()
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		xattr := name[i+1:]
		if uint32(len(xattr)) < in.Size {
			goto corrupt
		}
		xattr = xattr[:in.Size]
		req = &SetxattrRequest{
			Header:   m.Header(),
			Flags:    in.Flags,
			Position: in.position(),
			Name:     string(name[:i]),
			Xattr:    xattr,
		}

	case opGetxattr:
		in := (*getxattrIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		name := m.bytes()[unsafe.Sizeof(*in):]
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		req = &GetxattrRequest{
			Header:   m.Header(),
			Name:     string(name[:i]),
			Size:     in.Size,
			Position: in.position(),
		}

	case opListxattr:
		in := (*getxattrIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &ListxattrRequest{
			Header:   m.Header(),
			Size:     in.Size,
			Position: in.position(),
		}

	case opRemovexattr:
		buf := m.bytes()
		n := len(buf)
		if n == 0 || buf[n-1] != '\x00' {
			goto corrupt
		}
		req = &RemovexattrRequest{
			Header: m.Header(),
			Name:   string(buf[:n-1]),
		}

	case opFlush:
		in := (*flushIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &FlushRequest{
			Header:    m.Header(),
			Handle:    HandleID(in.Fh),
			Flags:     in.FlushFlags,
			LockOwner: in.LockOwner,
		}

	case opInit:
		in := (*initIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &InitRequest{
			Header:       m.Header(),
			Kernel:       Protocol{in.Major, in.Minor},
			MaxReadahead: in.MaxReadahead,
			Flags:        InitFlags(in.Flags),
		}

	case opGetlk:
		panic("opGetlk")
	case opSetlk:
		panic("opSetlk")
	case opSetlkw:
		panic("opSetlkw")

	case opAccess:
		in := (*accessIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &AccessRequest{
			Header: m.Header(),
			Mask:   in.Mask,
		}

	case opCreate:
		size := createInSize(c.proto)
		if m.len() < size {
			goto corrupt
		}
		in := (*createIn)(m.data())
		name := m.bytes()[size:]
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		r := &CreateRequest{
			Header: m.Header(),
			Flags:  openFlags(in.Flags),
			Mode:   fileMode(in.Mode),
			Name:   string(name[:i]),
		}
		if c.proto.GE(Protocol{7, 12}) {
			r.Umask = fileMode(in.Umask) & os.ModePerm
		}
		req = r

	case opInterrupt:
		in := (*interruptIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &InterruptRequest{
			Header: m.Header(),
			IntrID: RequestID(in.Unique),
		}

	case opBmap:
		panic("opBmap")

	case opDestroy:
		req = &DestroyRequest{
			Header: m.Header(),
		}

	// OS X
	case opSetvolname:
		panic("opSetvolname")
	case opGetxtimes:
		panic("opGetxtimes")
	case opExchange:
		in := (*exchangeIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		oldDirNodeID := NodeID(in.Olddir)
		newDirNodeID := NodeID(in.Newdir)
		oldNew := m.bytes()[unsafe.Sizeof(*in):]
		// oldNew should be "oldname\x00newname\x00"
		if len(oldNew) < 4 {
			goto corrupt
		}
		if oldNew[len(oldNew)-1] != '\x00' {
			goto corrupt
		}
		i := bytes.IndexByte(oldNew, '\x00')
		if i < 0 {
			goto corrupt
		}
		oldName, newName := string(oldNew[:i]), string(oldNew[i+1:len(oldNew)-1])
		req = &ExchangeDataRequest{
			Header:  m.Header(),
			OldDir:  oldDirNodeID,
			NewDir:  newDirNodeID,
			OldName: oldName,
			NewName: newName,
			// TODO options
		}
	}

	return req, nil

corrupt:
	Debug(malformedMessage{})
	putMessage(m)
	return nil, fmt.Errorf("fuse: malformed message")

unrecognized:
	// Unrecognized message.
	// Assume higher-level code will send a "no idea what you mean" error.
	h := m.Header()
	return &h, nil
}

type bugShortKernelWrite struct {
	Written int64
	Length  int64
	Error   string
	Stack   string
}

func (b bugShortKernelWrite) String() string {
	return fmt.Sprintf("short kernel write: written=%d/%d error=%q stack=\n%s", b.Written, b.Length, b.Error, b.Stack)
}

type bugKernelWriteError struct {
	Error string
	Stack string
}

func (b bugKernelWriteError) String() string {
	return fmt.Sprintf("kernel write error: error=%q stack=\n%s", b.Error, b.Stack)
}

// safe to call even with nil error
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (c *Conn) writeToKernel(msg []byte) error {
	out := (*outHeader)(unsafe.Pointer(&msg[0]))
	out.Len = uint32(len(msg))

	c.wio.RLock()
	defer c.wio.RUnlock()
	nn, err := syscall.Write(c.fd(), msg)
	if err == nil && nn != len(msg) {
		Debug(bugShortKernelWrite{
			Written: int64(nn),
			Length:  int64(len(msg)),
			Error:   errorString(err),
			Stack:   stack(),
		})
	}
	return err
}

func (c *Conn) respond(msg []byte) {
	if err := c.writeToKernel(msg); err != nil {
		Debug(bugKernelWriteError{
			Error: errorString(err),
			Stack: stack(),
		})
	}
}

type notCachedError struct{}

func (notCachedError) Error() string {
	return "node not cached"
}

var _ ErrorNumber = notCachedError{}

func (notCachedError) Errno() Errno {
	// Behave just like if the original syscall.ENOENT had been passed
	// straight through.
	return ENOENT
}

var (
	ErrNotCached = notCachedError{}
)

// sendInvalidate sends an invalidate notification to kernel.
//
// A returned ENOENT is translated to a friendlier error.
func (c *Conn) sendInvalidate(msg []byte) error {
	switch err := c.writeToKernel(msg); err {
	case syscall.ENOENT:
		return ErrNotCached
	default:
		return err
	}
}

// InvalidateNode invalidates the kernel cache of the attributes and a
// range of the data of a node.
//
// Giving offset 0 and size -1 means all data. To invalidate just the
// attributes, give offset 0 and size 0.
//
// Returns ErrNotCached if the kernel is not currently caching the
// node.
func (c *Conn) InvalidateNode(nodeID NodeID, off int64, size int64) error {
	buf := newBuffer(unsafe.Sizeof(notifyInvalInodeOut{}))
	h := (*outHeader)(unsafe.Pointer(&buf[0]))
	// h.Unique is 0
	h.Error = notifyCodeInvalInode
	out := (*notifyInvalInodeOut)(buf.alloc(unsafe.Sizeof(notifyInvalInodeOut{})))
	out.Ino = uint64(nodeID)
	out.Off = off
	out.Len = size
	return c.sendInvalidate(buf)
}

// InvalidateEntry invalidates the kernel cache of the directory entry
// identified by parent directory node ID and entry basename.
//
// Kernel may or may not cache directory listings. To invalidate
// those, use InvalidateNode to invalidate all of the data for a
// directory. (As of 2015-06, Linux FUSE does not cache directory
// listings.)
//
// Returns ErrNotCached if the kernel is not currently caching the
// node.
func (c *Conn) InvalidateEntry(parent NodeID, name string) error {
	const maxUint32 = ^uint32(0)
	if uint64(len(name)) > uint64(maxUint32) {
		// very unlikely, but we don't want to silently truncate
		return syscall.ENAMETOOLONG
	}
	buf := newBuffer(unsafe.Sizeof(notifyInvalEntryOut{}) + uintptr(len(name)) + 1)
	h := (*outHeader)(unsafe.Pointer(&buf[0]))
	// h.Unique is 0
	h.Error = notifyCodeInvalEntry
	out := (*notifyInvalEntryOut)(buf.alloc(unsafe.Sizeof(notifyInvalEntryOut{})))
	out.Parent = uint64(parent)
	out.Namelen = uint32(len(name))
	buf = append(buf, name...)
	buf = append(buf, '\x00')
	return c.sendInvalidate(buf)
}

// An InitRequest is the first request sent on a FUSE file system.
type InitRequest struct {
	Header `json:"-"`
	Kernel Protocol
	// Maximum readahead in bytes that the kernel plans to use.
	MaxReadahead uint32
	Flags        InitFlags
}

var _ = Request(&InitRequest{})

func (r *InitRequest) String() string {
	return fmt.Sprintf("Init [%v] %v ra=%d fl=%v", &r.Header, r.Kernel, r.MaxReadahead, r.Flags)
}

// An InitResponse is the response to an InitRequest.
type InitResponse struct {
	Library Protocol
	// Maximum readahead in bytes that the kernel can use. Ignored if
	// greater than InitRequest.MaxReadahead.
	MaxReadahead uint32
	Flags        InitFlags
	// Maximum size of a single write operation.
	// Linux enforces a minimum of 4 KiB.
	MaxWrite uint32
}

func (r *InitResponse) String() string {
	return fmt.Sprintf("Init %v ra=%d fl=%v w=%d", r.Library, r.MaxReadahead, r.Flags, r.MaxWrite)
}

// Respond replies to the request with the given response.
func (r *InitRequest) Respond(resp *InitResponse) {
	buf := newBuffer(unsafe.Sizeof(initOut{}))
	out := (*initOut)(buf.alloc(unsafe.Sizeof(initOut{})))
	out.Major = resp.Library.Major
	out.Minor = resp.Library.Minor
	out.MaxReadahead = resp.MaxReadahead
	out.Flags = uint32(resp.Flags)
	out.MaxWrite = resp.MaxWrite

	// MaxWrite larger than our receive buffer would just lead to
	// errors on large writes.
	if out.MaxWrite > maxWrite {
		out.MaxWrite = maxWrite
	}
	r.respond(buf)
}

// A StatfsRequest requests information about the mounted file system.
type StatfsRequest struct {
	Header `json:"-"`
}

var _ = Request(&StatfsRequest{})

func (r *StatfsRequest) String() string {
	return fmt.Sprintf("Statfs [%s]", &r.Header)
}

// Respond replies to the request with the given response.
func (r *StatfsRequest) Respond(resp *StatfsResponse) {
	buf := newBuffer(unsafe.Sizeof(statfsOut{}))
	out := (*statfsOut)(buf.alloc(unsafe.Sizeof(statfsOut{})))
	out.St = kstatfs{
		Blocks:  resp.Blocks,
		Bfree:   resp.Bfree,
		Bavail:  resp.Bavail,
		Files:   resp.Files,
		Ffree:   resp.Ffree,
		Bsize:   resp.Bsize,
		Namelen: resp.Namelen,
		Frsize:  resp.Frsize,
	}
	r.respond(buf)
}

// A StatfsResponse is the response to a StatfsRequest.
type StatfsResponse struct {
	Blocks  uint64 // Total data blocks in file system.
	Bfree   uint64 // Free blocks in file system.
	Bavail  uint64 // Free blocks in file system if you're not root.
	Files   uint64 // Total files in file system.
	Ffree   uint64 // Free files in file system.
	Bsize   uint32 // Block size
	Namelen uint32 // Maximum file name length?
	Frsize  uint32 // Fragment size, smallest addressable data size in the file system.
}

func (r *StatfsResponse) String() string {
	return fmt.Sprintf("Statfs blocks=%d/%d/%d files=%d/%d bsize=%d frsize=%d namelen=%d",
		r.Bavail, r.Bfree, r.Blocks,
		r.Ffree, r.Files,
		r.Bsize,
		r.Frsize,
		r.Namelen,
	)
}

// An AccessRequest asks whether the file can be accessed
// for the purpose specified by the mask.
type AccessRequest struct {
	Header `json:"-"`
	Mask   uint32
}

var _ = Request(&AccessRequest{})

func (r *AccessRequest) String() string {
	return fmt.Sprintf("Access [%s] mask=%#x", &r.Header, r.Mask)
}

// Respond replies to the request indicating that access is allowed.
// To deny access, use RespondError.
func (r *AccessRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// An Attr is the metadata for a single file or directory.
type Attr struct {
	Valid time.Duration // how long Attr can be cached

	Inode     uint64      // inode number
	Size      uint64      // size in bytes
	Blocks    uint64      // size in 512-byte units
	Atime     time.Time   // time of last access
	Mtime     time.Time   // time of last modification
	Ctime     time.Time   // time of last inode change
	Crtime    time.Time   // time of creation (OS X only)
	Mode      os.FileMode // file mode
	Nlink     uint32      // number of links (usually 1)
	Uid       uint32      // owner uid
	Gid       uint32      // group gid
	Rdev      uint32      // device numbers
	Flags     uint32      // chflags(2) flags (OS X only)
	BlockSize uint32      // preferred blocksize for filesystem I/O
}

func (a Attr) String() string {
	return fmt.Sprintf("valid=%v ino=%v size=%d mode=%v", a.Valid, a.Inode, a.Size, a.Mode)
}

func unix(t time.Time) (sec uint64, nsec uint32) {
	nano := t.UnixNano()
	sec = uint64(nano / 1e9)
	nsec = uint32(nano % 1e9)
	return
}

func (a *Attr) attr(out *attr, proto Protocol) {
	out.Ino = a.Inode
	out.Size = a.Size
	out.Blocks = a.Blocks
	out.Atime, out.AtimeNsec = unix(a.Atime)
	out.Mtime, out.MtimeNsec = unix(a.Mtime)
	out.Ctime, out.CtimeNsec = unix(a.Ctime)
	out.SetCrtime(unix(a.Crtime))
	out.Mode = uint32(a.Mode) & 0777
	switch {
	default:
		out.Mode |= syscall.S_IFREG
	case a.Mode&os.ModeDir != 0:
		out.Mode |= syscall.S_IFDIR
	case a.Mode&os.ModeDevice != 0:
		if a.Mode&os.ModeCharDevice != 0 {
			out.Mode |= syscall.S_IFCHR
		} else {
			out.Mode |= syscall.S_IFBLK
		}
	case a.Mode&os.ModeNamedPipe != 0:
		out.Mode |= syscall.S_IFIFO
	case a.Mode&os.ModeSymlink != 0:
		out.Mode |= syscall.S_IFLNK
	case a.Mode&os.ModeSocket != 0:
		out.Mode |= syscall.S_IFSOCK
	}
	if a.Mode&os.ModeSetuid != 0 {
		out.Mode |= syscall.S_ISUID
	}
	if a.Mode&os.ModeSetgid != 0 {
		out.Mode |= syscall.S_ISGID
	}
	out.Nlink = a.Nlink
	out.Uid = a.Uid
	out.Gid = a.Gid
	out.Rdev = a.Rdev
	out.SetFlags(a.Flags)
	if proto.GE(Protocol{7, 9}) {
		out.Blksize = a.BlockSize
	}

	return
}

// A GetattrRequest asks for the metadata for the file denoted by r.Node.
type GetattrRequest struct {
	Header `json:"-"`
	Flags  GetattrFlags
	Handle HandleID
}

var _ = Request(&GetattrRequest{})

func (r *GetattrRequest) String() string {
	return fmt.Sprintf("Getattr [%s] %v fl=%v", &r.Header, r.Handle, r.Flags)
}

// Respond replies to the request with the given response.
func (r *GetattrRequest) Respond(resp *GetattrResponse) {
	size := attrOutSize(r.Header.Conn.proto)
	buf := newBuffer(size)
	out := (*attrOut)(buf.alloc(size))
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.proto)
	r.respond(buf)
}

// A GetattrResponse is the response to a GetattrRequest.
type GetattrResponse struct {
	Attr Attr // file attributes
}

func (r *GetattrResponse) String() string {
	return fmt.Sprintf("Getattr %v", r.Attr)
}

// A GetxattrRequest asks for the extended attributes associated with r.Node.
type GetxattrRequest struct {
	Header `json:"-"`

	// Maximum size to return.
	Size uint32

	// Name of the attribute requested.
	Name string

	// Offset within extended attributes.
	//
	// Only valid for OS X, and then only with the resource fork
	// attribute.
	Position uint32
}

var _ = Request(&GetxattrRequest{})

func (r *GetxattrRequest) String() string {
	return fmt.Sprintf("Getxattr [%s] %q %d @%d", &r.Header, r.Name, r.Size, r.Position)
}

// Respond replies to the request with the given response.
func (r *GetxattrRequest) Respond(resp *GetxattrResponse) {
	if r.Size == 0 {
		buf := newBuffer(unsafe.Sizeof(getxattrOut{}))
		out := (*getxattrOut)(buf.alloc(unsafe.Sizeof(getxattrOut{})))
		out.Size = uint32(len(resp.Xattr))
		r.respond(buf)
	} else {
		buf := newBuffer(uintptr(len(resp.Xattr)))
		buf = append(buf, resp.Xattr...)
		r.respond(buf)
	}
}

// A GetxattrResponse is the response to a GetxattrRequest.
type GetxattrResponse struct {
	Xattr []byte
}

func (r *GetxattrResponse) String() string {
	return fmt.Sprintf("Getxattr %x", r.Xattr)
}

// A ListxattrRequest asks to list the extended attributes associated with r.Node.
type ListxattrRequest struct {
	Header   `json:"-"`
	Size     uint32 // maximum size to return
	Position uint32 // offset within attribute list
}

var _ = Request(&ListxattrRequest{})

func (r *ListxattrRequest) String() string {
	return fmt.Sprintf("Listxattr [%s] %d @%d", &r.Header, r.Size, r.Position)
}

// Respond replies to the request with the given response.
func (r *ListxattrRequest) Respond(resp *ListxattrResponse) {
	if r.Size == 0 {
		buf := newBuffer(unsafe.Sizeof(getxattrOut{}))
		out := (*getxattrOut)(buf.alloc(unsafe.Sizeof(getxattrOut{})))
		out.Size = uint32(len(resp.Xattr))
		r.respond(buf)
	} else {
		buf := newBuffer(uintptr(len(resp.Xattr)))
		buf = append(buf, resp.Xattr...)
		r.respond(buf)
	}
}

// A ListxattrResponse is the response to a ListxattrRequest.
type ListxattrResponse struct {
	Xattr []byte
}

func (r *ListxattrResponse) String() string {
	return fmt.Sprintf("Listxattr %x", r.Xattr)
}

// Append adds an extended attribute name to the response.
func (r *ListxattrResponse) Append(names ...string) {
	for _, name := range names {
		r.Xattr = append(r.Xattr, name...)
		r.Xattr = append(r.Xattr, '\x00')
	}
}

// A RemovexattrRequest asks to remove an extended attribute associated with r.Node.
type RemovexattrRequest struct {
	Header `json:"-"`
	Name   string // name of extended attribute
}

var _ = Request(&RemovexattrRequest{})

func (r *RemovexattrRequest) String() string {
	return fmt.Sprintf("Removexattr [%s] %q", &r.Header, r.Name)
}

// Respond replies to the request, indicating that the attribute was removed.
func (r *RemovexattrRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A SetxattrRequest asks to set an extended attribute associated with a file.
type SetxattrRequest struct {
	Header `json:"-"`

	// Flags can make the request fail if attribute does/not already
	// exist. Unfortunately, the constants are platform-specific and
	// not exposed by Go1.2. Look for XATTR_CREATE, XATTR_REPLACE.
	//
	// TODO improve this later
	//
	// TODO XATTR_CREATE and exist -> EEXIST
	//
	// TODO XATTR_REPLACE and not exist -> ENODATA
	Flags uint32

	// Offset within extended attributes.
	//
	// Only valid for OS X, and then only with the resource fork
	// attribute.
	Position uint32

	Name  string
	Xattr []byte
}

var _ = Request(&SetxattrRequest{})

func trunc(b []byte, max int) ([]byte, string) {
	if len(b) > max {
		return b[:max], "..."
	}
	return b, ""
}

func (r *SetxattrRequest) String() string {
	xattr, tail := trunc(r.Xattr, 16)
	return fmt.Sprintf("Setxattr [%s] %q %x%s fl=%v @%#x", &r.Header, r.Name, xattr, tail, r.Flags, r.Position)
}

// Respond replies to the request, indicating that the extended attribute was set.
func (r *SetxattrRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A LookupRequest asks to look up the given name in the directory named by r.Node.
type LookupRequest struct {
	Header `json:"-"`
	Name   string
}

var _ = Request(&LookupRequest{})

func (r *LookupRequest) String() string {
	return fmt.Sprintf("Lookup [%s] %q", &r.Header, r.Name)
}

// Respond replies to the request with the given response.
func (r *LookupRequest) Respond(resp *LookupResponse) {
	size := entryOutSize(r.Header.Conn.proto)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.proto)
	r.respond(buf)
}

// A LookupResponse is the response to a LookupRequest.
type LookupResponse struct {
	Node       NodeID
	Generation uint64
	EntryValid time.Duration
	Attr       Attr
}

func (r *LookupResponse) string() string {
	return fmt.Sprintf("%v gen=%d valid=%v attr={%v}", r.Node, r.Generation, r.EntryValid, r.Attr)
}

func (r *LookupResponse) String() string {
	return fmt.Sprintf("Lookup %s", r.string())
}

// An OpenRequest asks to open a file or directory
type OpenRequest struct {
	Header `json:"-"`
	Dir    bool // is this Opendir?
	Flags  OpenFlags
}

var _ = Request(&OpenRequest{})

func (r *OpenRequest) String() string {
	return fmt.Sprintf("Open [%s] dir=%v fl=%v", &r.Header, r.Dir, r.Flags)
}

// Respond replies to the request with the given response.
func (r *OpenRequest) Respond(resp *OpenResponse) {
	buf := newBuffer(unsafe.Sizeof(openOut{}))
	out := (*openOut)(buf.alloc(unsafe.Sizeof(openOut{})))
	out.Fh = uint64(resp.Handle)
	out.OpenFlags = uint32(resp.Flags)
	r.respond(buf)
}

// A OpenResponse is the response to a OpenRequest.
type OpenResponse struct {
	Handle HandleID
	Flags  OpenResponseFlags
}

func (r *OpenResponse) string() string {
	return fmt.Sprintf("%v fl=%v", r.Handle, r.Flags)
}

func (r *OpenResponse) String() string {
	return fmt.Sprintf("Open %s", r.string())
}

// A CreateRequest asks to create and open a file (not a directory).
type CreateRequest struct {
	Header `json:"-"`
	Name   string
	Flags  OpenFlags
	Mode   os.FileMode
	// Umask of the request. Not supported on OS X.
	Umask os.FileMode
}

var _ = Request(&CreateRequest{})

func (r *CreateRequest) String() string {
	return fmt.Sprintf("Create [%s] %q fl=%v mode=%v umask=%v", &r.Header, r.Name, r.Flags, r.Mode, r.Umask)
}

// Respond replies to the request with the given response.
func (r *CreateRequest) Respond(resp *CreateResponse) {
	eSize := entryOutSize(r.Header.Conn.proto)
	buf := newBuffer(eSize + unsafe.Sizeof(openOut{}))

	e := (*entryOut)(buf.alloc(eSize))
	e.Nodeid = uint64(resp.Node)
	e.Generation = resp.Generation
	e.EntryValid = uint64(resp.EntryValid / time.Second)
	e.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	e.AttrValid = uint64(resp.Attr.Valid / time.Second)
	e.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&e.Attr, r.Header.Conn.proto)

	o := (*openOut)(buf.alloc(unsafe.Sizeof(openOut{})))
	o.Fh = uint64(resp.Handle)
	o.OpenFlags = uint32(resp.Flags)

	r.respond(buf)
}

// A CreateResponse is the response to a CreateRequest.
// It describes the created node and opened handle.
type CreateResponse struct {
	LookupResponse
	OpenResponse
}

func (r *CreateResponse) String() string {
	return fmt.Sprintf("Create {%s} {%s}", r.LookupResponse.string(), r.OpenResponse.string())
}

// A MkdirRequest asks to create (but not open) a directory.
type MkdirRequest struct {
	Header `json:"-"`
	Name   string
	Mode   os.FileMode
	// Umask of the request. Not supported on OS X.
	Umask os.FileMode
}

var _ = Request(&MkdirRequest{})

func (r *MkdirRequest) String() string {
	return fmt.Sprintf("Mkdir [%s] %q mode=%v umask=%v", &r.Header, r.Name, r.Mode, r.Umask)
}

// Respond replies to the request with the given response.
func (r *MkdirRequest) Respond(resp *MkdirResponse) {
	size := entryOutSize(r.Header.Conn.proto)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.proto)
	r.respond(buf)
}

// A MkdirResponse is the response to a MkdirRequest.
type MkdirResponse struct {
	LookupResponse
}

func (r *MkdirResponse) String() string {
	return fmt.Sprintf("Mkdir %v", r.LookupResponse.string())
}

// A ReadRequest asks to read from an open file.
type ReadRequest struct {
	Header    `json:"-"`
	Dir       bool // is this Readdir?
	Handle    HandleID
	Offset    int64
	Size      int
	Flags     ReadFlags
	LockOwner uint64
	FileFlags OpenFlags
}

var _ = Request(&ReadRequest{})

func (r *ReadRequest) String() string {
	return fmt.Sprintf("Read [%s] %v %d @%#x dir=%v fl=%v lock=%d ffl=%v", &r.Header, r.Handle, r.Size, r.Offset, r.Dir, r.Flags, r.LockOwner, r.FileFlags)
}

// Respond replies to the request with the given response.
func (r *ReadRequest) Respond(resp *ReadResponse) {
	buf := newBuffer(uintptr(len(resp.Data)))
	buf = append(buf, resp.Data...)
	r.respond(buf)
}

// A ReadResponse is the response to a ReadRequest.
type ReadResponse struct {
	Data []byte
}

func (r *ReadResponse) String() string {
	return fmt.Sprintf("Read %d", len(r.Data))
}

type jsonReadResponse struct {
	Len uint64
}

func (r *ReadResponse) MarshalJSON() ([]byte, error) {
	j := jsonReadResponse{
		Len: uint64(len(r.Data)),
	}
	return json.Marshal(j)
}

// A ReleaseRequest asks to release (close) an open file handle.
type ReleaseRequest struct {
	Header       `json:"-"`
	Dir          bool // is this Releasedir?
	Handle       HandleID
	Flags        OpenFlags // flags from OpenRequest
	ReleaseFlags ReleaseFlags
	LockOwner    uint32
}

var _ = Request(&ReleaseRequest{})

func (r *ReleaseRequest) String() string {
	return fmt.Sprintf("Release [%s] %v fl=%v rfl=%v owner=%#x", &r.Header, r.Handle, r.Flags, r.ReleaseFlags, r.LockOwner)
}

// Respond replies to the request, indicating that the handle has been released.
func (r *ReleaseRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A DestroyRequest is sent by the kernel when unmounting the file system.
// No more requests will be received after this one, but it should still be
// responded to.
type DestroyRequest struct {
	Header `json:"-"`
}

var _ = Request(&DestroyRequest{})

func (r *DestroyRequest) String() string {
	return fmt.Sprintf("Destroy [%s]", &r.Header)
}

// Respond replies to the request.
func (r *DestroyRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A ForgetRequest is sent by the kernel when forgetting about r.Node
// as returned by r.N lookup requests.
type ForgetRequest struct {
	Header `json:"-"`
	N      uint64
}

var _ = Request(&ForgetRequest{})

func (r *ForgetRequest) String() string {
	return fmt.Sprintf("Forget [%s] %d", &r.Header, r.N)
}

// Respond replies to the request, indicating that the forgetfulness has been recorded.
func (r *ForgetRequest) Respond() {
	// Don't reply to forget messages.
	r.noResponse()
}

// A Dirent represents a single directory entry.
type Dirent struct {
	// Inode this entry names.
	Inode uint64

	// Type of the entry, for example DT_File.
	//
	// Setting this is optional. The zero value (DT_Unknown) means
	// callers will just need to do a Getattr when the type is
	// needed. Providing a type can speed up operations
	// significantly.
	Type DirentType

	// Name of the entry
	Name string
}

// Type of an entry in a directory listing.
type DirentType uint32

const (
	// These don't quite match os.FileMode; especially there's an
	// explicit unknown, instead of zero value meaning file. They
	// are also not quite syscall.DT_*; nothing says the FUSE
	// protocol follows those, and even if they were, we don't
	// want each fs to fiddle with syscall.

	// The shift by 12 is hardcoded in the FUSE userspace
	// low-level C library, so it's safe here.

	DT_Unknown DirentType = 0
	DT_Socket  DirentType = syscall.S_IFSOCK >> 12
	DT_Link    DirentType = syscall.S_IFLNK >> 12
	DT_File    DirentType = syscall.S_IFREG >> 12
	DT_Block   DirentType = syscall.S_IFBLK >> 12
	DT_Dir     DirentType = syscall.S_IFDIR >> 12
	DT_Char    DirentType = syscall.S_IFCHR >> 12
	DT_FIFO    DirentType = syscall.S_IFIFO >> 12
)

func (t DirentType) String() string {
	switch t {
	case DT_Unknown:
		return "unknown"
	case DT_Socket:
		return "socket"
	case DT_Link:
		return "link"
	case DT_File:
		return "file"
	case DT_Block:
		return "block"
	case DT_Dir:
		return "dir"
	case DT_Char:
		return "char"
	case DT_FIFO:
		return "fifo"
	}
	return "invalid"
}

// AppendDirent appends the encoded form of a directory entry to data
// and returns the resulting slice.
func AppendDirent(data []byte, dir Dirent) []byte {
	de := dirent{
		Ino:     dir.Inode,
		Namelen: uint32(len(dir.Name)),
		Type:    uint32(dir.Type),
	}
	de.Off = uint64(len(data) + direntSize + (len(dir.Name)+7)&^7)
	data = append(data, (*[direntSize]byte)(unsafe.Pointer(&de))[:]...)
	data = append(data, dir.Name...)
	n := direntSize + uintptr(len(dir.Name))
	if n%8 != 0 {
		var pad [8]byte
		data = append(data, pad[:8-n%8]...)
	}
	return data
}

// A WriteRequest asks to write to an open file.
type WriteRequest struct {
	Header
	Handle    HandleID
	Offset    int64
	Data      []byte
	Flags     WriteFlags
	LockOwner uint64
	FileFlags OpenFlags
}

var _ = Request(&WriteRequest{})

func (r *WriteRequest) String() string {
	return fmt.Sprintf("Write [%s] %v %d @%d fl=%v lock=%d ffl=%v", &r.Header, r.Handle, len(r.Data), r.Offset, r.Flags, r.LockOwner, r.FileFlags)
}

type jsonWriteRequest struct {
	Handle HandleID
	Offset int64
	Len    uint64
	Flags  WriteFlags
}

func (r *WriteRequest) MarshalJSON() ([]byte, error) {
	j := jsonWriteRequest{
		Handle: r.Handle,
		Offset: r.Offset,
		Len:    uint64(len(r.Data)),
		Flags:  r.Flags,
	}
	return json.Marshal(j)
}

// Respond replies to the request with the given response.
func (r *WriteRequest) Respond(resp *WriteResponse) {
	buf := newBuffer(unsafe.Sizeof(writeOut{}))
	out := (*writeOut)(buf.alloc(unsafe.Sizeof(writeOut{})))
	out.Size = uint32(resp.Size)
	r.respond(buf)
}

// A WriteResponse replies to a write indicating how many bytes were written.
type WriteResponse struct {
	Size int
}

func (r *WriteResponse) String() string {
	return fmt.Sprintf("Write %d", r.Size)
}

// A SetattrRequest asks to change one or more attributes associated with a file,
// as indicated by Valid.
type SetattrRequest struct {
	Header `json:"-"`
	Valid  SetattrValid
	Handle HandleID
	Size   uint64
	Atime  time.Time
	Mtime  time.Time
	Mode   os.FileMode
	Uid    uint32
	Gid    uint32

	// OS X only
	Bkuptime time.Time
	Chgtime  time.Time
	Crtime   time.Time
	Flags    uint32 // see chflags(2)
}

var _ = Request(&SetattrRequest{})

func (r *SetattrRequest) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Setattr [%s]", &r.Header)
	if r.Valid.Mode() {
		fmt.Fprintf(&buf, " mode=%v", r.Mode)
	}
	if r.Valid.Uid() {
		fmt.Fprintf(&buf, " uid=%d", r.Uid)
	}
	if r.Valid.Gid() {
		fmt.Fprintf(&buf, " gid=%d", r.Gid)
	}
	if r.Valid.Size() {
		fmt.Fprintf(&buf, " size=%d", r.Size)
	}
	if r.Valid.Atime() {
		fmt.Fprintf(&buf, " atime=%v", r.Atime)
	}
	if r.Valid.AtimeNow() {
		fmt.Fprintf(&buf, " atime=now")
	}
	if r.Valid.Mtime() {
		fmt.Fprintf(&buf, " mtime=%v", r.Mtime)
	}
	if r.Valid.MtimeNow() {
		fmt.Fprintf(&buf, " mtime=now")
	}
	if r.Valid.Handle() {
		fmt.Fprintf(&buf, " handle=%v", r.Handle)
	} else {
		fmt.Fprintf(&buf, " handle=INVALID-%v", r.Handle)
	}
	if r.Valid.LockOwner() {
		fmt.Fprintf(&buf, " lockowner")
	}
	if r.Valid.Crtime() {
		fmt.Fprintf(&buf, " crtime=%v", r.Crtime)
	}
	if r.Valid.Chgtime() {
		fmt.Fprintf(&buf, " chgtime=%v", r.Chgtime)
	}
	if r.Valid.Bkuptime() {
		fmt.Fprintf(&buf, " bkuptime=%v", r.Bkuptime)
	}
	if r.Valid.Flags() {
		fmt.Fprintf(&buf, " flags=%v", r.Flags)
	}
	return buf.String()
}

// Respond replies to the request with the given response,
// giving the updated attributes.
func (r *SetattrRequest) Respond(resp *SetattrResponse) {
	size := attrOutSize(r.Header.Conn.proto)
	buf := newBuffer(size)
	out := (*attrOut)(buf.alloc(size))
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.proto)
	r.respond(buf)
}

// A SetattrResponse is the response to a SetattrRequest.
type SetattrResponse struct {
	Attr Attr // file attributes
}

func (r *SetattrResponse) String() string {
	return fmt.Sprintf("Setattr %v", r.Attr)
}

// A FlushRequest asks for the current state of an open file to be flushed
// to storage, as when a file descriptor is being closed.  A single opened Handle
// may receive multiple FlushRequests over its lifetime.
type FlushRequest struct {
	Header    `json:"-"`
	Handle    HandleID
	Flags     uint32
	LockOwner uint64
}

var _ = Request(&FlushRequest{})

func (r *FlushRequest) String() string {
	return fmt.Sprintf("Flush [%s] %v fl=%#x lk=%#x", &r.Header, r.Handle, r.Flags, r.LockOwner)
}

// Respond replies to the request, indicating that the flush succeeded.
func (r *FlushRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A RemoveRequest asks to remove a file or directory from the
// directory r.Node.
type RemoveRequest struct {
	Header `json:"-"`
	Name   string // name of the entry to remove
	Dir    bool   // is this rmdir?
}

var _ = Request(&RemoveRequest{})

func (r *RemoveRequest) String() string {
	return fmt.Sprintf("Remove [%s] %q dir=%v", &r.Header, r.Name, r.Dir)
}

// Respond replies to the request, indicating that the file was removed.
func (r *RemoveRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A SymlinkRequest is a request to create a symlink making NewName point to Target.
type SymlinkRequest struct {
	Header          `json:"-"`
	NewName, Target string
}

var _ = Request(&SymlinkRequest{})

func (r *SymlinkRequest) String() string {
	return fmt.Sprintf("Symlink [%s] from %q to target %q", &r.Header, r.NewName, r.Target)
}

// Respond replies to the request, indicating that the symlink was created.
func (r *SymlinkRequest) Respond(resp *SymlinkResponse) {
	size := entryOutSize(r.Header.Conn.proto)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.proto)
	r.respond(buf)
}

// A SymlinkResponse is the response to a SymlinkRequest.
type SymlinkResponse struct {
	LookupResponse
}

func (r *SymlinkResponse) String() string {
	return fmt.Sprintf("Symlink %v", r.LookupResponse.string())
}

// A ReadlinkRequest is a request to read a symlink's target.
type ReadlinkRequest struct {
	Header `json:"-"`
}

var _ = Request(&ReadlinkRequest{})

func (r *ReadlinkRequest) String() string {
	return fmt.Sprintf("Readlink [%s]", &r.Header)
}

func (r *ReadlinkRequest) Respond(target string) {
	buf := newBuffer(uintptr(len(target)))
	buf = append(buf, target...)
	r.respond(buf)
}

// A LinkRequest is a request to create a hard link.
type LinkRequest struct {
	Header  `json:"-"`
	OldNode NodeID
	NewName string
}

var _ = Request(&LinkRequest{})

func (r *LinkRequest) String() string {
	return fmt.Sprintf("Link [%s] node %d to %q", &r.Header, r.OldNode, r.NewName)
}

func (r *LinkRequest) Respond(resp *LookupResponse) {
	size := entryOutSize(r.Header.Conn.proto)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.proto)
	r.respond(buf)
}

// A RenameRequest is a request to rename a file.
type RenameRequest struct {
	Header           `json:"-"`
	NewDir           NodeID
	OldName, NewName string
}

var _ = Request(&RenameRequest{})

func (r *RenameRequest) String() string {
	return fmt.Sprintf("Rename [%s] from %q to dirnode %v %q", &r.Header, r.OldName, r.NewDir, r.NewName)
}

func (r *RenameRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

type MknodRequest struct {
	Header `json:"-"`
	Name   string
	Mode   os.FileMode
	Rdev   uint32
	// Umask of the request. Not supported on OS X.
	Umask os.FileMode
}

var _ = Request(&MknodRequest{})

func (r *MknodRequest) String() string {
	return fmt.Sprintf("Mknod [%s] Name %q mode=%v umask=%v rdev=%d", &r.Header, r.Name, r.Mode, r.Umask, r.Rdev)
}

func (r *MknodRequest) Respond(resp *LookupResponse) {
	size := entryOutSize(r.Header.Conn.proto)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.proto)
	r.respond(buf)
}

type FsyncRequest struct {
	Header `json:"-"`
	Handle HandleID
	// TODO bit 1 is datasync, not well documented upstream
	Flags uint32
	Dir   bool
}

var _ = Request(&FsyncRequest{})

func (r *FsyncRequest) String() string {
	return fmt.Sprintf("Fsync [%s] Handle %v Flags %v", &r.Header, r.Handle, r.Flags)
}

func (r *FsyncRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// An InterruptRequest is a request to interrupt another pending request. The
// response to that request should return an error status of EINTR.
type InterruptRequest struct {
	Header `json:"-"`
	IntrID RequestID // ID of the request to be interrupt.
}

var _ = Request(&InterruptRequest{})

func (r *InterruptRequest) Respond() {
	// nothing to do here
	r.noResponse()
}

func (r *InterruptRequest) String() string {
	return fmt.Sprintf("Interrupt [%s] ID %v", &r.Header, r.IntrID)
}

// An ExchangeDataRequest is a request to exchange the contents of two
// files, while leaving most metadata untouched.
//
// This request comes from OS X exchangedata(2) and represents its
// specific semantics. Crucially, it is very different from Linux
// renameat(2) RENAME_EXCHANGE.
//
// https://developer.apple.com/library/mac/documentation/Darwin/Reference/ManPages/man2/exchangedata.2.html
type ExchangeDataRequest struct {
	Header           `json:"-"`
	OldDir, NewDir   NodeID
	OldName, NewName string
	// TODO options
}

var _ = Request(&ExchangeDataRequest{})

func (r *ExchangeDataRequest) String() string {
	// TODO options
	return fmt.Sprintf("ExchangeData [%s] %v %q and %v %q", &r.Header, r.OldDir, r.OldName, r.NewDir, r.NewName)
}

func (r *ExchangeDataRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}
