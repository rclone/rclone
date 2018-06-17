package sftp

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kr/fs"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// InternalInconsistency indicates the packets sent and the data queued to be
// written to the file don't match up. It is an unusual error and usually is
// caused by bad behavior server side or connection issues. The error is
// limited in scope to the call where it happened, the client object is still
// OK to use as long as the connection is still open.
var InternalInconsistency = errors.New("internal inconsistency")

// A ClientOption is a function which applies configuration to a Client.
type ClientOption func(*Client) error

// MaxPacketChecked sets the maximum size of the payload, measured in bytes.
// This option only accepts sizes servers should support, ie. <= 32768 bytes.
//
// If you get the error "failed to send packet header: EOF" when copying a
// large file, try lowering this number.
//
// The default packet size is 32768 bytes.
func MaxPacketChecked(size int) ClientOption {
	return func(c *Client) error {
		if size < 1 {
			return errors.Errorf("size must be greater or equal to 1")
		}
		if size > 32768 {
			return errors.Errorf("sizes larger than 32KB might not work with all servers")
		}
		c.maxPacket = size
		return nil
	}
}

// MaxPacketUnchecked sets the maximum size of the payload, measured in bytes.
// It accepts sizes larger than the 32768 bytes all servers should support.
// Only use a setting higher than 32768 if your application always connects to
// the same server or after sufficiently broad testing.
//
// If you get the error "failed to send packet header: EOF" when copying a
// large file, try lowering this number.
//
// The default packet size is 32768 bytes.
func MaxPacketUnchecked(size int) ClientOption {
	return func(c *Client) error {
		if size < 1 {
			return errors.Errorf("size must be greater or equal to 1")
		}
		c.maxPacket = size
		return nil
	}
}

// MaxPacket sets the maximum size of the payload, measured in bytes.
// This option only accepts sizes servers should support, ie. <= 32768 bytes.
// This is a synonym for MaxPacketChecked that provides backward compatibility.
//
// If you get the error "failed to send packet header: EOF" when copying a
// large file, try lowering this number.
//
// The default packet size is 32768 bytes.
func MaxPacket(size int) ClientOption {
	return MaxPacketChecked(size)
}

// MaxConcurrentRequestsPerFile sets the maximum concurrent requests allowed for a single file.
//
// The default maximum concurrent requests is 64.
func MaxConcurrentRequestsPerFile(n int) ClientOption {
	return func(c *Client) error {
		if n < 1 {
			return errors.Errorf("n must be greater or equal to 1")
		}
		c.maxConcurrentRequests = n
		return nil
	}
}

// NewClient creates a new SFTP client on conn, using zero or more option
// functions.
func NewClient(conn *ssh.Client, opts ...ClientOption) (*Client, error) {
	s, err := conn.NewSession()
	if err != nil {
		return nil, err
	}
	if err := s.RequestSubsystem("sftp"); err != nil {
		return nil, err
	}
	pw, err := s.StdinPipe()
	if err != nil {
		return nil, err
	}
	pr, err := s.StdoutPipe()
	if err != nil {
		return nil, err
	}

	return NewClientPipe(pr, pw, opts...)
}

// NewClientPipe creates a new SFTP client given a Reader and a WriteCloser.
// This can be used for connecting to an SFTP server over TCP/TLS or by using
// the system's ssh client program (e.g. via exec.Command).
func NewClientPipe(rd io.Reader, wr io.WriteCloser, opts ...ClientOption) (*Client, error) {
	sftp := &Client{
		clientConn: clientConn{
			conn: conn{
				Reader:      rd,
				WriteCloser: wr,
			},
			inflight: make(map[uint32]chan<- result),
		},
		maxPacket:             1 << 15,
		maxConcurrentRequests: 64,
	}
	if err := sftp.applyOptions(opts...); err != nil {
		wr.Close()
		return nil, err
	}
	if err := sftp.sendInit(); err != nil {
		wr.Close()
		return nil, err
	}
	if err := sftp.recvVersion(); err != nil {
		wr.Close()
		return nil, err
	}
	sftp.clientConn.wg.Add(1)
	go sftp.loop()
	return sftp, nil
}

// Client represents an SFTP session on a *ssh.ClientConn SSH connection.
// Multiple Clients can be active on a single SSH connection, and a Client
// may be called concurrently from multiple Goroutines.
//
// Client implements the github.com/kr/fs.FileSystem interface.
type Client struct {
	clientConn

	maxPacket             int // max packet size read or written.
	nextid                uint32
	maxConcurrentRequests int
}

// Create creates the named file mode 0666 (before umask), truncating it if it
// already exists. If successful, methods on the returned File can be used for
// I/O; the associated file descriptor has mode O_RDWR. If you need more
// control over the flags/mode used to open the file see client.OpenFile.
func (c *Client) Create(path string) (*File, error) {
	return c.open(path, flags(os.O_RDWR|os.O_CREATE|os.O_TRUNC))
}

const sftpProtocolVersion = 3 // http://tools.ietf.org/html/draft-ietf-secsh-filexfer-02

func (c *Client) sendInit() error {
	return c.clientConn.conn.sendPacket(sshFxInitPacket{
		Version: sftpProtocolVersion, // http://tools.ietf.org/html/draft-ietf-secsh-filexfer-02
	})
}

// returns the next value of c.nextid
func (c *Client) nextID() uint32 {
	return atomic.AddUint32(&c.nextid, 1)
}

func (c *Client) recvVersion() error {
	typ, data, err := c.recvPacket()
	if err != nil {
		return err
	}
	if typ != ssh_FXP_VERSION {
		return &unexpectedPacketErr{ssh_FXP_VERSION, typ}
	}

	version, _ := unmarshalUint32(data)
	if version != sftpProtocolVersion {
		return &unexpectedVersionErr{sftpProtocolVersion, version}
	}

	return nil
}

// Walk returns a new Walker rooted at root.
func (c *Client) Walk(root string) *fs.Walker {
	return fs.WalkFS(root, c)
}

// ReadDir reads the directory named by dirname and returns a list of
// directory entries.
func (c *Client) ReadDir(p string) ([]os.FileInfo, error) {
	handle, err := c.opendir(p)
	if err != nil {
		return nil, err
	}
	defer c.close(handle) // this has to defer earlier than the lock below
	var attrs []os.FileInfo
	var done = false
	for !done {
		id := c.nextID()
		typ, data, err1 := c.sendPacket(sshFxpReaddirPacket{
			ID:     id,
			Handle: handle,
		})
		if err1 != nil {
			err = err1
			done = true
			break
		}
		switch typ {
		case ssh_FXP_NAME:
			sid, data := unmarshalUint32(data)
			if sid != id {
				return nil, &unexpectedIDErr{id, sid}
			}
			count, data := unmarshalUint32(data)
			for i := uint32(0); i < count; i++ {
				var filename string
				filename, data = unmarshalString(data)
				_, data = unmarshalString(data) // discard longname
				var attr *FileStat
				attr, data = unmarshalAttrs(data)
				if filename == "." || filename == ".." {
					continue
				}
				attrs = append(attrs, fileInfoFromStat(attr, path.Base(filename)))
			}
		case ssh_FXP_STATUS:
			// TODO(dfc) scope warning!
			err = normaliseError(unmarshalStatus(id, data))
			done = true
		default:
			return nil, unimplementedPacketErr(typ)
		}
	}
	if err == io.EOF {
		err = nil
	}
	return attrs, err
}

func (c *Client) opendir(path string) (string, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpOpendirPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return "", err
	}
	switch typ {
	case ssh_FXP_HANDLE:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return "", &unexpectedIDErr{id, sid}
		}
		handle, _ := unmarshalString(data)
		return handle, nil
	case ssh_FXP_STATUS:
		return "", normaliseError(unmarshalStatus(id, data))
	default:
		return "", unimplementedPacketErr(typ)
	}
}

// Stat returns a FileInfo structure describing the file specified by path 'p'.
// If 'p' is a symbolic link, the returned FileInfo structure describes the referent file.
func (c *Client) Stat(p string) (os.FileInfo, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpStatPacket{
		ID:   id,
		Path: p,
	})
	if err != nil {
		return nil, err
	}
	switch typ {
	case ssh_FXP_ATTRS:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return nil, &unexpectedIDErr{id, sid}
		}
		attr, _ := unmarshalAttrs(data)
		return fileInfoFromStat(attr, path.Base(p)), nil
	case ssh_FXP_STATUS:
		return nil, normaliseError(unmarshalStatus(id, data))
	default:
		return nil, unimplementedPacketErr(typ)
	}
}

// Lstat returns a FileInfo structure describing the file specified by path 'p'.
// If 'p' is a symbolic link, the returned FileInfo structure describes the symbolic link.
func (c *Client) Lstat(p string) (os.FileInfo, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpLstatPacket{
		ID:   id,
		Path: p,
	})
	if err != nil {
		return nil, err
	}
	switch typ {
	case ssh_FXP_ATTRS:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return nil, &unexpectedIDErr{id, sid}
		}
		attr, _ := unmarshalAttrs(data)
		return fileInfoFromStat(attr, path.Base(p)), nil
	case ssh_FXP_STATUS:
		return nil, normaliseError(unmarshalStatus(id, data))
	default:
		return nil, unimplementedPacketErr(typ)
	}
}

// ReadLink reads the target of a symbolic link.
func (c *Client) ReadLink(p string) (string, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpReadlinkPacket{
		ID:   id,
		Path: p,
	})
	if err != nil {
		return "", err
	}
	switch typ {
	case ssh_FXP_NAME:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return "", &unexpectedIDErr{id, sid}
		}
		count, data := unmarshalUint32(data)
		if count != 1 {
			return "", unexpectedCount(1, count)
		}
		filename, _ := unmarshalString(data) // ignore dummy attributes
		return filename, nil
	case ssh_FXP_STATUS:
		return "", normaliseError(unmarshalStatus(id, data))
	default:
		return "", unimplementedPacketErr(typ)
	}
}

// Symlink creates a symbolic link at 'newname', pointing at target 'oldname'
func (c *Client) Symlink(oldname, newname string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpSymlinkPacket{
		ID:         id,
		Linkpath:   newname,
		Targetpath: oldname,
	})
	if err != nil {
		return err
	}
	switch typ {
	case ssh_FXP_STATUS:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// setstat is a convience wrapper to allow for changing of various parts of the file descriptor.
func (c *Client) setstat(path string, flags uint32, attrs interface{}) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpSetstatPacket{
		ID:    id,
		Path:  path,
		Flags: flags,
		Attrs: attrs,
	})
	if err != nil {
		return err
	}
	switch typ {
	case ssh_FXP_STATUS:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// Chtimes changes the access and modification times of the named file.
func (c *Client) Chtimes(path string, atime time.Time, mtime time.Time) error {
	type times struct {
		Atime uint32
		Mtime uint32
	}
	attrs := times{uint32(atime.Unix()), uint32(mtime.Unix())}
	return c.setstat(path, ssh_FILEXFER_ATTR_ACMODTIME, attrs)
}

// Chown changes the user and group owners of the named file.
func (c *Client) Chown(path string, uid, gid int) error {
	type owner struct {
		UID uint32
		GID uint32
	}
	attrs := owner{uint32(uid), uint32(gid)}
	return c.setstat(path, ssh_FILEXFER_ATTR_UIDGID, attrs)
}

// Chmod changes the permissions of the named file.
func (c *Client) Chmod(path string, mode os.FileMode) error {
	return c.setstat(path, ssh_FILEXFER_ATTR_PERMISSIONS, uint32(mode))
}

// Truncate sets the size of the named file. Although it may be safely assumed
// that if the size is less than its current size it will be truncated to fit,
// the SFTP protocol does not specify what behavior the server should do when setting
// size greater than the current size.
func (c *Client) Truncate(path string, size int64) error {
	return c.setstat(path, ssh_FILEXFER_ATTR_SIZE, uint64(size))
}

// Open opens the named file for reading. If successful, methods on the
// returned file can be used for reading; the associated file descriptor
// has mode O_RDONLY.
func (c *Client) Open(path string) (*File, error) {
	return c.open(path, flags(os.O_RDONLY))
}

// OpenFile is the generalized open call; most users will use Open or
// Create instead. It opens the named file with specified flag (O_RDONLY
// etc.). If successful, methods on the returned File can be used for I/O.
func (c *Client) OpenFile(path string, f int) (*File, error) {
	return c.open(path, flags(f))
}

func (c *Client) open(path string, pflags uint32) (*File, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpOpenPacket{
		ID:     id,
		Path:   path,
		Pflags: pflags,
	})
	if err != nil {
		return nil, err
	}
	switch typ {
	case ssh_FXP_HANDLE:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return nil, &unexpectedIDErr{id, sid}
		}
		handle, _ := unmarshalString(data)
		return &File{c: c, path: path, handle: handle}, nil
	case ssh_FXP_STATUS:
		return nil, normaliseError(unmarshalStatus(id, data))
	default:
		return nil, unimplementedPacketErr(typ)
	}
}

// close closes a handle handle previously returned in the response
// to SSH_FXP_OPEN or SSH_FXP_OPENDIR. The handle becomes invalid
// immediately after this request has been sent.
func (c *Client) close(handle string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpClosePacket{
		ID:     id,
		Handle: handle,
	})
	if err != nil {
		return err
	}
	switch typ {
	case ssh_FXP_STATUS:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

func (c *Client) fstat(handle string) (*FileStat, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpFstatPacket{
		ID:     id,
		Handle: handle,
	})
	if err != nil {
		return nil, err
	}
	switch typ {
	case ssh_FXP_ATTRS:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return nil, &unexpectedIDErr{id, sid}
		}
		attr, _ := unmarshalAttrs(data)
		return attr, nil
	case ssh_FXP_STATUS:
		return nil, normaliseError(unmarshalStatus(id, data))
	default:
		return nil, unimplementedPacketErr(typ)
	}
}

// StatVFS retrieves VFS statistics from a remote host.
//
// It implements the statvfs@openssh.com SSH_FXP_EXTENDED feature
// from http://www.opensource.apple.com/source/OpenSSH/OpenSSH-175/openssh/PROTOCOL?txt.
func (c *Client) StatVFS(path string) (*StatVFS, error) {
	// send the StatVFS packet to the server
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpStatvfsPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return nil, err
	}

	switch typ {
	// server responded with valid data
	case ssh_FXP_EXTENDED_REPLY:
		var response StatVFS
		err = binary.Read(bytes.NewReader(data), binary.BigEndian, &response)
		if err != nil {
			return nil, errors.New("can not parse reply")
		}

		return &response, nil

	// the resquest failed
	case ssh_FXP_STATUS:
		return nil, errors.New(fxp(ssh_FXP_STATUS).String())

	default:
		return nil, unimplementedPacketErr(typ)
	}
}

// Join joins any number of path elements into a single path, adding a
// separating slash if necessary. The result is Cleaned; in particular, all
// empty strings are ignored.
func (c *Client) Join(elem ...string) string { return path.Join(elem...) }

// Remove removes the specified file or directory. An error will be returned if no
// file or directory with the specified path exists, or if the specified directory
// is not empty.
func (c *Client) Remove(path string) error {
	err := c.removeFile(path)
	if err, ok := err.(*StatusError); ok {
		switch err.Code {
		// some servers, *cough* osx *cough*, return EPERM, not ENODIR.
		// serv-u returns ssh_FX_FILE_IS_A_DIRECTORY
		case ssh_FX_PERMISSION_DENIED, ssh_FX_FAILURE, ssh_FX_FILE_IS_A_DIRECTORY:
			return c.RemoveDirectory(path)
		}
	}
	return err
}

func (c *Client) removeFile(path string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpRemovePacket{
		ID:       id,
		Filename: path,
	})
	if err != nil {
		return err
	}
	switch typ {
	case ssh_FXP_STATUS:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// RemoveDirectory removes a directory path.
func (c *Client) RemoveDirectory(path string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpRmdirPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return err
	}
	switch typ {
	case ssh_FXP_STATUS:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// Rename renames a file.
func (c *Client) Rename(oldname, newname string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpRenamePacket{
		ID:      id,
		Oldpath: oldname,
		Newpath: newname,
	})
	if err != nil {
		return err
	}
	switch typ {
	case ssh_FXP_STATUS:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// PosixRename renames a file using the posix-rename@openssh.com extension
// which will replace newname if it already exists.
func (c *Client) PosixRename(oldname, newname string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpPosixRenamePacket{
		ID:      id,
		Oldpath: oldname,
		Newpath: newname,
	})
	if err != nil {
		return err
	}
	switch typ {
	case ssh_FXP_STATUS:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

func (c *Client) realpath(path string) (string, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpRealpathPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return "", err
	}
	switch typ {
	case ssh_FXP_NAME:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return "", &unexpectedIDErr{id, sid}
		}
		count, data := unmarshalUint32(data)
		if count != 1 {
			return "", unexpectedCount(1, count)
		}
		filename, _ := unmarshalString(data) // ignore attributes
		return filename, nil
	case ssh_FXP_STATUS:
		return "", normaliseError(unmarshalStatus(id, data))
	default:
		return "", unimplementedPacketErr(typ)
	}
}

// Getwd returns the current working directory of the server. Operations
// involving relative paths will be based at this location.
func (c *Client) Getwd() (string, error) {
	return c.realpath(".")
}

// Mkdir creates the specified directory. An error will be returned if a file or
// directory with the specified path already exists, or if the directory's
// parent folder does not exist (the method cannot create complete paths).
func (c *Client) Mkdir(path string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(sshFxpMkdirPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return err
	}
	switch typ {
	case ssh_FXP_STATUS:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// MkdirAll creates a directory named path, along with any necessary parents,
// and returns nil, or else returns an error.
// If path is already a directory, MkdirAll does nothing and returns nil.
// If path contains a regular file, an error is returned
func (c *Client) MkdirAll(path string) error {
	// Most of this code mimics https://golang.org/src/os/path.go?s=514:561#L13
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := c.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent
		err = c.MkdirAll(path[0 : j-1])
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = c.Mkdir(path)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := c.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}

// applyOptions applies options functions to the Client.
// If an error is encountered, option processing ceases.
func (c *Client) applyOptions(opts ...ClientOption) error {
	for _, f := range opts {
		if err := f(c); err != nil {
			return err
		}
	}
	return nil
}

// File represents a remote file.
type File struct {
	c      *Client
	path   string
	handle string
	offset uint64 // current offset within remote file
}

// Close closes the File, rendering it unusable for I/O. It returns an
// error, if any.
func (f *File) Close() error {
	return f.c.close(f.handle)
}

// Name returns the name of the file as presented to Open or Create.
func (f *File) Name() string {
	return f.path
}

// Read reads up to len(b) bytes from the File. It returns the number of bytes
// read and an error, if any. Read follows io.Reader semantics, so when Read
// encounters an error or EOF condition after successfully reading n > 0 bytes,
// it returns the number of bytes read.
//
// To maximise throughput for transferring the entire file (especially
// over high latency links) it is recommended to use WriteTo rather
// than calling Read multiple times. io.Copy will do this
// automatically.
func (f *File) Read(b []byte) (int, error) {
	// Split the read into multiple maxPacket sized concurrent reads
	// bounded by maxConcurrentRequests. This allows reads with a suitably
	// large buffer to transfer data at a much faster rate due to
	// overlapping round trip times.
	inFlight := 0
	desiredInFlight := 1
	offset := f.offset
	// maxConcurrentRequests buffer to deal with broadcastErr() floods
	// also must have a buffer of max value of (desiredInFlight - inFlight)
	ch := make(chan result, f.c.maxConcurrentRequests+1)
	type inflightRead struct {
		b      []byte
		offset uint64
	}
	reqs := map[uint32]inflightRead{}
	type offsetErr struct {
		offset uint64
		err    error
	}
	var firstErr offsetErr

	sendReq := func(b []byte, offset uint64) {
		reqID := f.c.nextID()
		f.c.dispatchRequest(ch, sshFxpReadPacket{
			ID:     reqID,
			Handle: f.handle,
			Offset: offset,
			Len:    uint32(len(b)),
		})
		inFlight++
		reqs[reqID] = inflightRead{b: b, offset: offset}
	}

	var read int
	for len(b) > 0 || inFlight > 0 {
		for inFlight < desiredInFlight && len(b) > 0 && firstErr.err == nil {
			l := min(len(b), f.c.maxPacket)
			rb := b[:l]
			sendReq(rb, offset)
			offset += uint64(l)
			b = b[l:]
		}

		if inFlight == 0 {
			break
		}
		res := <-ch
		inFlight--
		if res.err != nil {
			firstErr = offsetErr{offset: 0, err: res.err}
			continue
		}
		reqID, data := unmarshalUint32(res.data)
		req, ok := reqs[reqID]
		if !ok {
			firstErr = offsetErr{offset: 0, err: errors.Errorf("sid: %v not found", reqID)}
			continue
		}
		delete(reqs, reqID)
		switch res.typ {
		case ssh_FXP_STATUS:
			if firstErr.err == nil || req.offset < firstErr.offset {
				firstErr = offsetErr{
					offset: req.offset,
					err:    normaliseError(unmarshalStatus(reqID, res.data)),
				}
			}
		case ssh_FXP_DATA:
			l, data := unmarshalUint32(data)
			n := copy(req.b, data[:l])
			read += n
			if n < len(req.b) {
				sendReq(req.b[l:], req.offset+uint64(l))
			}
			if desiredInFlight < f.c.maxConcurrentRequests {
				desiredInFlight++
			}
		default:
			firstErr = offsetErr{offset: 0, err: unimplementedPacketErr(res.typ)}
		}
	}
	// If the error is anything other than EOF, then there
	// may be gaps in the data copied to the buffer so it's
	// best to return 0 so the caller can't make any
	// incorrect assumptions about the state of the buffer.
	if firstErr.err != nil && firstErr.err != io.EOF {
		read = 0
	}
	f.offset += uint64(read)
	return read, firstErr.err
}

// WriteTo writes the file to w. The return value is the number of bytes
// written. Any error encountered during the write is also returned.
//
// This method is preferred over calling Read multiple times to
// maximise throughput for transferring the entire file (especially
// over high latency links).
func (f *File) WriteTo(w io.Writer) (int64, error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	inFlight := 0
	desiredInFlight := 1
	offset := f.offset
	writeOffset := offset
	fileSize := uint64(fi.Size())
	// see comment on same line in Read() above
	ch := make(chan result, f.c.maxConcurrentRequests+1)
	type inflightRead struct {
		b      []byte
		offset uint64
	}
	reqs := map[uint32]inflightRead{}
	pendingWrites := map[uint64][]byte{}
	type offsetErr struct {
		offset uint64
		err    error
	}
	var firstErr offsetErr

	sendReq := func(b []byte, offset uint64) {
		reqID := f.c.nextID()
		f.c.dispatchRequest(ch, sshFxpReadPacket{
			ID:     reqID,
			Handle: f.handle,
			Offset: offset,
			Len:    uint32(len(b)),
		})
		inFlight++
		reqs[reqID] = inflightRead{b: b, offset: offset}
	}

	var copied int64
	for firstErr.err == nil || inFlight > 0 {
		if firstErr.err == nil {
			for inFlight+len(pendingWrites) < desiredInFlight {
				b := make([]byte, f.c.maxPacket)
				sendReq(b, offset)
				offset += uint64(f.c.maxPacket)
				if offset > fileSize {
					desiredInFlight = 1
				}
			}
		}

		if inFlight == 0 {
			if firstErr.err == nil && len(pendingWrites) > 0 {
				return copied, InternalInconsistency
			}
			break
		}
		res := <-ch
		inFlight--
		if res.err != nil {
			firstErr = offsetErr{offset: 0, err: res.err}
			continue
		}
		reqID, data := unmarshalUint32(res.data)
		req, ok := reqs[reqID]
		if !ok {
			firstErr = offsetErr{offset: 0, err: errors.Errorf("sid: %v not found", reqID)}
			continue
		}
		delete(reqs, reqID)
		switch res.typ {
		case ssh_FXP_STATUS:
			if firstErr.err == nil || req.offset < firstErr.offset {
				firstErr = offsetErr{offset: req.offset, err: normaliseError(unmarshalStatus(reqID, res.data))}
			}
		case ssh_FXP_DATA:
			l, data := unmarshalUint32(data)
			if req.offset == writeOffset {
				nbytes, err := w.Write(data)
				copied += int64(nbytes)
				if err != nil {
					// We will never receive another DATA with offset==writeOffset, so
					// the loop will drain inFlight and then exit.
					firstErr = offsetErr{offset: req.offset + uint64(nbytes), err: err}
					break
				}
				if nbytes < int(l) {
					firstErr = offsetErr{offset: req.offset + uint64(nbytes), err: io.ErrShortWrite}
					break
				}
				switch {
				case offset > fileSize:
					desiredInFlight = 1
				case desiredInFlight < f.c.maxConcurrentRequests:
					desiredInFlight++
				}
				writeOffset += uint64(nbytes)
				for {
					pendingData, ok := pendingWrites[writeOffset]
					if !ok {
						break
					}
					// Give go a chance to free the memory.
					delete(pendingWrites, writeOffset)
					nbytes, err := w.Write(pendingData)
					// Do not move writeOffset on error so subsequent iterations won't trigger
					// any writes.
					if err != nil {
						firstErr = offsetErr{offset: writeOffset + uint64(nbytes), err: err}
						break
					}
					if nbytes < len(pendingData) {
						firstErr = offsetErr{offset: writeOffset + uint64(nbytes), err: io.ErrShortWrite}
						break
					}
					writeOffset += uint64(nbytes)
				}
			} else {
				// Don't write the data yet because
				// this response came in out of order
				// and we need to wait for responses
				// for earlier segments of the file.
				pendingWrites[req.offset] = data
			}
		default:
			firstErr = offsetErr{offset: 0, err: unimplementedPacketErr(res.typ)}
		}
	}
	if firstErr.err != io.EOF {
		return copied, firstErr.err
	}
	return copied, nil
}

// Stat returns the FileInfo structure describing file. If there is an
// error.
func (f *File) Stat() (os.FileInfo, error) {
	fs, err := f.c.fstat(f.handle)
	if err != nil {
		return nil, err
	}
	return fileInfoFromStat(fs, path.Base(f.path)), nil
}

// Write writes len(b) bytes to the File. It returns the number of bytes
// written and an error, if any. Write returns a non-nil error when n !=
// len(b).
//
// To maximise throughput for transferring the entire file (especially
// over high latency links) it is recommended to use ReadFrom rather
// than calling Write multiple times. io.Copy will do this
// automatically.
func (f *File) Write(b []byte) (int, error) {
	// Split the write into multiple maxPacket sized concurrent writes
	// bounded by maxConcurrentRequests. This allows writes with a suitably
	// large buffer to transfer data at a much faster rate due to
	// overlapping round trip times.
	inFlight := 0
	desiredInFlight := 1
	offset := f.offset
	// see comment on same line in Read() above
	ch := make(chan result, f.c.maxConcurrentRequests+1)
	var firstErr error
	written := len(b)
	for len(b) > 0 || inFlight > 0 {
		for inFlight < desiredInFlight && len(b) > 0 && firstErr == nil {
			l := min(len(b), f.c.maxPacket)
			rb := b[:l]
			f.c.dispatchRequest(ch, sshFxpWritePacket{
				ID:     f.c.nextID(),
				Handle: f.handle,
				Offset: offset,
				Length: uint32(len(rb)),
				Data:   rb,
			})
			inFlight++
			offset += uint64(l)
			b = b[l:]
		}

		if inFlight == 0 {
			break
		}
		res := <-ch
		inFlight--
		if res.err != nil {
			firstErr = res.err
			continue
		}
		switch res.typ {
		case ssh_FXP_STATUS:
			id, _ := unmarshalUint32(res.data)
			err := normaliseError(unmarshalStatus(id, res.data))
			if err != nil && firstErr == nil {
				firstErr = err
				break
			}
			if desiredInFlight < f.c.maxConcurrentRequests {
				desiredInFlight++
			}
		default:
			firstErr = unimplementedPacketErr(res.typ)
		}
	}
	// If error is non-nil, then there may be gaps in the data written to
	// the file so it's best to return 0 so the caller can't make any
	// incorrect assumptions about the state of the file.
	if firstErr != nil {
		written = 0
	}
	f.offset += uint64(written)
	return written, firstErr
}

// ReadFrom reads data from r until EOF and writes it to the file. The return
// value is the number of bytes read. Any error except io.EOF encountered
// during the read is also returned.
//
// This method is preferred over calling Write multiple times to
// maximise throughput for transferring the entire file (especially
// over high latency links).
func (f *File) ReadFrom(r io.Reader) (int64, error) {
	inFlight := 0
	desiredInFlight := 1
	offset := f.offset
	// see comment on same line in Read() above
	ch := make(chan result, f.c.maxConcurrentRequests+1)
	var firstErr error
	read := int64(0)
	b := make([]byte, f.c.maxPacket)
	for inFlight > 0 || firstErr == nil {
		for inFlight < desiredInFlight && firstErr == nil {
			n, err := r.Read(b)
			if err != nil {
				firstErr = err
			}
			f.c.dispatchRequest(ch, sshFxpWritePacket{
				ID:     f.c.nextID(),
				Handle: f.handle,
				Offset: offset,
				Length: uint32(n),
				Data:   b[:n],
			})
			inFlight++
			offset += uint64(n)
			read += int64(n)
		}

		if inFlight == 0 {
			break
		}
		res := <-ch
		inFlight--
		if res.err != nil {
			firstErr = res.err
			continue
		}
		switch res.typ {
		case ssh_FXP_STATUS:
			id, _ := unmarshalUint32(res.data)
			err := normaliseError(unmarshalStatus(id, res.data))
			if err != nil && firstErr == nil {
				firstErr = err
				break
			}
			if desiredInFlight < f.c.maxConcurrentRequests {
				desiredInFlight++
			}
		default:
			firstErr = unimplementedPacketErr(res.typ)
		}
	}
	if firstErr == io.EOF {
		firstErr = nil
	}
	// If error is non-nil, then there may be gaps in the data written to
	// the file so it's best to return 0 so the caller can't make any
	// incorrect assumptions about the state of the file.
	if firstErr != nil {
		read = 0
	}
	f.offset += uint64(read)
	return read, firstErr
}

// Seek implements io.Seeker by setting the client offset for the next Read or
// Write. It returns the next offset read. Seeking before or after the end of
// the file is undefined. Seeking relative to the end calls Stat.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = uint64(offset)
	case io.SeekCurrent:
		f.offset = uint64(int64(f.offset) + offset)
	case io.SeekEnd:
		fi, err := f.Stat()
		if err != nil {
			return int64(f.offset), err
		}
		f.offset = uint64(fi.Size() + offset)
	default:
		return int64(f.offset), unimplementedSeekWhence(whence)
	}
	return int64(f.offset), nil
}

// Chown changes the uid/gid of the current file.
func (f *File) Chown(uid, gid int) error {
	return f.c.Chown(f.path, uid, gid)
}

// Chmod changes the permissions of the current file.
func (f *File) Chmod(mode os.FileMode) error {
	return f.c.Chmod(f.path, mode)
}

// Truncate sets the size of the current file. Although it may be safely assumed
// that if the size is less than its current size it will be truncated to fit,
// the SFTP protocol does not specify what behavior the server should do when setting
// size greater than the current size.
func (f *File) Truncate(size int64) error {
	return f.c.Truncate(f.path, size)
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

// normaliseError normalises an error into a more standard form that can be
// checked against stdlib errors like io.EOF or os.ErrNotExist.
func normaliseError(err error) error {
	switch err := err.(type) {
	case *StatusError:
		switch err.Code {
		case ssh_FX_EOF:
			return io.EOF
		case ssh_FX_NO_SUCH_FILE:
			return os.ErrNotExist
		case ssh_FX_OK:
			return nil
		default:
			return err
		}
	default:
		return err
	}
}

func unmarshalStatus(id uint32, data []byte) error {
	sid, data := unmarshalUint32(data)
	if sid != id {
		return &unexpectedIDErr{id, sid}
	}
	code, data := unmarshalUint32(data)
	msg, data, _ := unmarshalStringSafe(data)
	lang, _, _ := unmarshalStringSafe(data)
	return &StatusError{
		Code: code,
		msg:  msg,
		lang: lang,
	}
}

func marshalStatus(b []byte, err StatusError) []byte {
	b = marshalUint32(b, err.Code)
	b = marshalString(b, err.msg)
	b = marshalString(b, err.lang)
	return b
}

// flags converts the flags passed to OpenFile into ssh flags.
// Unsupported flags are ignored.
func flags(f int) uint32 {
	var out uint32
	switch f & os.O_WRONLY {
	case os.O_WRONLY:
		out |= ssh_FXF_WRITE
	case os.O_RDONLY:
		out |= ssh_FXF_READ
	}
	if f&os.O_RDWR == os.O_RDWR {
		out |= ssh_FXF_READ | ssh_FXF_WRITE
	}
	if f&os.O_APPEND == os.O_APPEND {
		out |= ssh_FXF_APPEND
	}
	if f&os.O_CREATE == os.O_CREATE {
		out |= ssh_FXF_CREAT
	}
	if f&os.O_TRUNC == os.O_TRUNC {
		out |= ssh_FXF_TRUNC
	}
	if f&os.O_EXCL == os.O_EXCL {
		out |= ssh_FXF_EXCL
	}
	return out
}
