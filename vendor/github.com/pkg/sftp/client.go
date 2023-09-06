package sftp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kr/fs"
	"golang.org/x/crypto/ssh"
)

var (
	// ErrInternalInconsistency indicates the packets sent and the data queued to be
	// written to the file don't match up. It is an unusual error and usually is
	// caused by bad behavior server side or connection issues. The error is
	// limited in scope to the call where it happened, the client object is still
	// OK to use as long as the connection is still open.
	ErrInternalInconsistency = errors.New("internal inconsistency")
	// InternalInconsistency alias for ErrInternalInconsistency.
	//
	// Deprecated: please use ErrInternalInconsistency
	InternalInconsistency = ErrInternalInconsistency
)

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
			return errors.New("size must be greater or equal to 1")
		}
		if size > 32768 {
			return errors.New("sizes larger than 32KB might not work with all servers")
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
			return errors.New("size must be greater or equal to 1")
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
			return errors.New("n must be greater or equal to 1")
		}
		c.maxConcurrentRequests = n
		return nil
	}
}

// UseConcurrentWrites allows the Client to perform concurrent Writes.
//
// Using concurrency while doing writes, requires special consideration.
// A write to a later offset in a file after an error,
// could end up with a file length longer than what was successfully written.
//
// When using this option, if you receive an error during `io.Copy` or `io.WriteTo`,
// you may need to `Truncate` the target Writer to avoid “holes” in the data written.
func UseConcurrentWrites(value bool) ClientOption {
	return func(c *Client) error {
		c.useConcurrentWrites = value
		return nil
	}
}

// UseConcurrentReads allows the Client to perform concurrent Reads.
//
// Concurrent reads are generally safe to use and not using them will degrade
// performance, so this option is enabled by default.
//
// When enabled, WriteTo will use Stat/Fstat to get the file size and determines
// how many concurrent workers to use.
// Some "read once" servers will delete the file if they receive a stat call on an
// open file and then the download will fail.
// Disabling concurrent reads you will be able to download files from these servers.
// If concurrent reads are disabled, the UseFstat option is ignored.
func UseConcurrentReads(value bool) ClientOption {
	return func(c *Client) error {
		c.disableConcurrentReads = !value
		return nil
	}
}

// UseFstat sets whether to use Fstat or Stat when File.WriteTo is called
// (usually when copying files).
// Some servers limit the amount of open files and calling Stat after opening
// the file will throw an error From the server. Setting this flag will call
// Fstat instead of Stat which is suppose to be called on an open file handle.
//
// It has been found that that with IBM Sterling SFTP servers which have
// "extractability" level set to 1 which means only 1 file can be opened at
// any given time.
//
// If the server you are working with still has an issue with both Stat and
// Fstat calls you can always open a file and read it until the end.
//
// Another reason to read the file until its end and Fstat doesn't work is
// that in some servers, reading a full file will automatically delete the
// file as some of these mainframes map the file to a message in a queue.
// Once the file has been read it will get deleted.
func UseFstat(value bool) ClientOption {
	return func(c *Client) error {
		c.useFstat = value
		return nil
	}
}

// Client represents an SFTP session on a *ssh.ClientConn SSH connection.
// Multiple Clients can be active on a single SSH connection, and a Client
// may be called concurrently from multiple Goroutines.
//
// Client implements the github.com/kr/fs.FileSystem interface.
type Client struct {
	clientConn

	ext map[string]string // Extensions (name -> data).

	maxPacket             int // max packet size read or written.
	maxConcurrentRequests int
	nextid                uint32

	// write concurrency is… error prone.
	// Default behavior should be to not use it.
	useConcurrentWrites    bool
	useFstat               bool
	disableConcurrentReads bool
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
			closed:   make(chan struct{}),
		},

		ext: make(map[string]string),

		maxPacket:             1 << 15,
		maxConcurrentRequests: 64,
	}

	for _, opt := range opts {
		if err := opt(sftp); err != nil {
			wr.Close()
			return nil, err
		}
	}

	if err := sftp.sendInit(); err != nil {
		wr.Close()
		return nil, fmt.Errorf("error sending init packet to server: %w", err)
	}

	if err := sftp.recvVersion(); err != nil {
		wr.Close()
		return nil, fmt.Errorf("error receiving version packet from server: %w", err)
	}

	sftp.clientConn.wg.Add(1)
	go func() {
		defer sftp.clientConn.wg.Done()

		if err := sftp.clientConn.recv(); err != nil {
			sftp.clientConn.broadcastErr(err)
		}
	}()

	return sftp, nil
}

// Create creates the named file mode 0666 (before umask), truncating it if it
// already exists. If successful, methods on the returned File can be used for
// I/O; the associated file descriptor has mode O_RDWR. If you need more
// control over the flags/mode used to open the file see client.OpenFile.
//
// Note that some SFTP servers (eg. AWS Transfer) do not support opening files
// read/write at the same time. For those services you will need to use
// `client.OpenFile(os.O_WRONLY|os.O_CREATE|os.O_TRUNC)`.
func (c *Client) Create(path string) (*File, error) {
	return c.open(path, flags(os.O_RDWR|os.O_CREATE|os.O_TRUNC))
}

const sftpProtocolVersion = 3 // http://tools.ietf.org/html/draft-ietf-secsh-filexfer-02

func (c *Client) sendInit() error {
	return c.clientConn.conn.sendPacket(&sshFxInitPacket{
		Version: sftpProtocolVersion, // http://tools.ietf.org/html/draft-ietf-secsh-filexfer-02
	})
}

// returns the next value of c.nextid
func (c *Client) nextID() uint32 {
	return atomic.AddUint32(&c.nextid, 1)
}

func (c *Client) recvVersion() error {
	typ, data, err := c.recvPacket(0)
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("server unexpectedly closed connection: %w", io.ErrUnexpectedEOF)
		}

		return err
	}

	if typ != sshFxpVersion {
		return &unexpectedPacketErr{sshFxpVersion, typ}
	}

	version, data, err := unmarshalUint32Safe(data)
	if err != nil {
		return err
	}

	if version != sftpProtocolVersion {
		return &unexpectedVersionErr{sftpProtocolVersion, version}
	}

	for len(data) > 0 {
		var ext extensionPair
		ext, data, err = unmarshalExtensionPair(data)
		if err != nil {
			return err
		}
		c.ext[ext.Name] = ext.Data
	}

	return nil
}

// HasExtension checks whether the server supports a named extension.
//
// The first return value is the extension data reported by the server
// (typically a version number).
func (c *Client) HasExtension(name string) (string, bool) {
	data, ok := c.ext[name]
	return data, ok
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
		typ, data, err1 := c.sendPacket(nil, &sshFxpReaddirPacket{
			ID:     id,
			Handle: handle,
		})
		if err1 != nil {
			err = err1
			done = true
			break
		}
		switch typ {
		case sshFxpName:
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
		case sshFxpStatus:
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
	typ, data, err := c.sendPacket(nil, &sshFxpOpendirPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return "", err
	}
	switch typ {
	case sshFxpHandle:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return "", &unexpectedIDErr{id, sid}
		}
		handle, _ := unmarshalString(data)
		return handle, nil
	case sshFxpStatus:
		return "", normaliseError(unmarshalStatus(id, data))
	default:
		return "", unimplementedPacketErr(typ)
	}
}

// Stat returns a FileInfo structure describing the file specified by path 'p'.
// If 'p' is a symbolic link, the returned FileInfo structure describes the referent file.
func (c *Client) Stat(p string) (os.FileInfo, error) {
	fs, err := c.stat(p)
	if err != nil {
		return nil, err
	}
	return fileInfoFromStat(fs, path.Base(p)), nil
}

// Lstat returns a FileInfo structure describing the file specified by path 'p'.
// If 'p' is a symbolic link, the returned FileInfo structure describes the symbolic link.
func (c *Client) Lstat(p string) (os.FileInfo, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpLstatPacket{
		ID:   id,
		Path: p,
	})
	if err != nil {
		return nil, err
	}
	switch typ {
	case sshFxpAttrs:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return nil, &unexpectedIDErr{id, sid}
		}
		attr, _ := unmarshalAttrs(data)
		return fileInfoFromStat(attr, path.Base(p)), nil
	case sshFxpStatus:
		return nil, normaliseError(unmarshalStatus(id, data))
	default:
		return nil, unimplementedPacketErr(typ)
	}
}

// ReadLink reads the target of a symbolic link.
func (c *Client) ReadLink(p string) (string, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpReadlinkPacket{
		ID:   id,
		Path: p,
	})
	if err != nil {
		return "", err
	}
	switch typ {
	case sshFxpName:
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
	case sshFxpStatus:
		return "", normaliseError(unmarshalStatus(id, data))
	default:
		return "", unimplementedPacketErr(typ)
	}
}

// Link creates a hard link at 'newname', pointing at the same inode as 'oldname'
func (c *Client) Link(oldname, newname string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpHardlinkPacket{
		ID:      id,
		Oldpath: oldname,
		Newpath: newname,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// Symlink creates a symbolic link at 'newname', pointing at target 'oldname'
func (c *Client) Symlink(oldname, newname string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpSymlinkPacket{
		ID:         id,
		Linkpath:   newname,
		Targetpath: oldname,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

func (c *Client) setfstat(handle string, flags uint32, attrs interface{}) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpFsetstatPacket{
		ID:     id,
		Handle: handle,
		Flags:  flags,
		Attrs:  attrs,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// setstat is a convience wrapper to allow for changing of various parts of the file descriptor.
func (c *Client) setstat(path string, flags uint32, attrs interface{}) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpSetstatPacket{
		ID:    id,
		Path:  path,
		Flags: flags,
		Attrs: attrs,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
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
	return c.setstat(path, sshFileXferAttrACmodTime, attrs)
}

// Chown changes the user and group owners of the named file.
func (c *Client) Chown(path string, uid, gid int) error {
	type owner struct {
		UID uint32
		GID uint32
	}
	attrs := owner{uint32(uid), uint32(gid)}
	return c.setstat(path, sshFileXferAttrUIDGID, attrs)
}

// Chmod changes the permissions of the named file.
//
// Chmod does not apply a umask, because even retrieving the umask is not
// possible in a portable way without causing a race condition. Callers
// should mask off umask bits, if desired.
func (c *Client) Chmod(path string, mode os.FileMode) error {
	return c.setstat(path, sshFileXferAttrPermissions, toChmodPerm(mode))
}

// Truncate sets the size of the named file. Although it may be safely assumed
// that if the size is less than its current size it will be truncated to fit,
// the SFTP protocol does not specify what behavior the server should do when setting
// size greater than the current size.
func (c *Client) Truncate(path string, size int64) error {
	return c.setstat(path, sshFileXferAttrSize, uint64(size))
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
	typ, data, err := c.sendPacket(nil, &sshFxpOpenPacket{
		ID:     id,
		Path:   path,
		Pflags: pflags,
	})
	if err != nil {
		return nil, err
	}
	switch typ {
	case sshFxpHandle:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return nil, &unexpectedIDErr{id, sid}
		}
		handle, _ := unmarshalString(data)
		return &File{c: c, path: path, handle: handle}, nil
	case sshFxpStatus:
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
	typ, data, err := c.sendPacket(nil, &sshFxpClosePacket{
		ID:     id,
		Handle: handle,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

func (c *Client) stat(path string) (*FileStat, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpStatPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return nil, err
	}
	switch typ {
	case sshFxpAttrs:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return nil, &unexpectedIDErr{id, sid}
		}
		attr, _ := unmarshalAttrs(data)
		return attr, nil
	case sshFxpStatus:
		return nil, normaliseError(unmarshalStatus(id, data))
	default:
		return nil, unimplementedPacketErr(typ)
	}
}

func (c *Client) fstat(handle string) (*FileStat, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpFstatPacket{
		ID:     id,
		Handle: handle,
	})
	if err != nil {
		return nil, err
	}
	switch typ {
	case sshFxpAttrs:
		sid, data := unmarshalUint32(data)
		if sid != id {
			return nil, &unexpectedIDErr{id, sid}
		}
		attr, _ := unmarshalAttrs(data)
		return attr, nil
	case sshFxpStatus:
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
	typ, data, err := c.sendPacket(nil, &sshFxpStatvfsPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return nil, err
	}

	switch typ {
	// server responded with valid data
	case sshFxpExtendedReply:
		var response StatVFS
		err = binary.Read(bytes.NewReader(data), binary.BigEndian, &response)
		if err != nil {
			return nil, errors.New("can not parse reply")
		}

		return &response, nil

	// the resquest failed
	case sshFxpStatus:
		return nil, normaliseError(unmarshalStatus(id, data))

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
	// some servers, *cough* osx *cough*, return EPERM, not ENODIR.
	// serv-u returns ssh_FX_FILE_IS_A_DIRECTORY
	// EPERM is converted to os.ErrPermission so it is not a StatusError
	if err, ok := err.(*StatusError); ok {
		switch err.Code {
		case sshFxFailure, sshFxFileIsADirectory:
			return c.RemoveDirectory(path)
		}
	}
	if os.IsPermission(err) {
		return c.RemoveDirectory(path)
	}
	return err
}

func (c *Client) removeFile(path string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpRemovePacket{
		ID:       id,
		Filename: path,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// RemoveDirectory removes a directory path.
func (c *Client) RemoveDirectory(path string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpRmdirPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// Rename renames a file.
func (c *Client) Rename(oldname, newname string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpRenamePacket{
		ID:      id,
		Oldpath: oldname,
		Newpath: newname,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// PosixRename renames a file using the posix-rename@openssh.com extension
// which will replace newname if it already exists.
func (c *Client) PosixRename(oldname, newname string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpPosixRenamePacket{
		ID:      id,
		Oldpath: oldname,
		Newpath: newname,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return unimplementedPacketErr(typ)
	}
}

// RealPath can be used to have the server canonicalize any given path name to an absolute path.
//
// This is useful for converting path names containing ".." components,
// or relative pathnames without a leading slash into absolute paths.
func (c *Client) RealPath(path string) (string, error) {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpRealpathPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return "", err
	}
	switch typ {
	case sshFxpName:
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
	case sshFxpStatus:
		return "", normaliseError(unmarshalStatus(id, data))
	default:
		return "", unimplementedPacketErr(typ)
	}
}

// Getwd returns the current working directory of the server. Operations
// involving relative paths will be based at this location.
func (c *Client) Getwd() (string, error) {
	return c.RealPath(".")
}

// Mkdir creates the specified directory. An error will be returned if a file or
// directory with the specified path already exists, or if the directory's
// parent folder does not exist (the method cannot create complete paths).
func (c *Client) Mkdir(path string) error {
	id := c.nextID()
	typ, data, err := c.sendPacket(nil, &sshFxpMkdirPacket{
		ID:   id,
		Path: path,
	})
	if err != nil {
		return err
	}
	switch typ {
	case sshFxpStatus:
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
	for i > 0 && path[i-1] == '/' { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && path[j-1] != '/' { // Scan backward over element.
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

// File represents a remote file.
type File struct {
	c      *Client
	path   string
	handle string

	mu     sync.Mutex
	offset int64 // current offset within remote file
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
	f.mu.Lock()
	defer f.mu.Unlock()

	n, err := f.ReadAt(b, f.offset)
	f.offset += int64(n)
	return n, err
}

// readChunkAt attempts to read the whole entire length of the buffer from the file starting at the offset.
// It will continue progressively reading into the buffer until it fills the whole buffer, or an error occurs.
func (f *File) readChunkAt(ch chan result, b []byte, off int64) (n int, err error) {
	for err == nil && n < len(b) {
		id := f.c.nextID()
		typ, data, err := f.c.sendPacket(ch, &sshFxpReadPacket{
			ID:     id,
			Handle: f.handle,
			Offset: uint64(off) + uint64(n),
			Len:    uint32(len(b) - n),
		})
		if err != nil {
			return n, err
		}

		switch typ {
		case sshFxpStatus:
			return n, normaliseError(unmarshalStatus(id, data))

		case sshFxpData:
			sid, data := unmarshalUint32(data)
			if id != sid {
				return n, &unexpectedIDErr{id, sid}
			}

			l, data := unmarshalUint32(data)
			n += copy(b[n:], data[:l])

		default:
			return n, unimplementedPacketErr(typ)
		}
	}

	return
}

func (f *File) readAtSequential(b []byte, off int64) (read int, err error) {
	for read < len(b) {
		rb := b[read:]
		if len(rb) > f.c.maxPacket {
			rb = rb[:f.c.maxPacket]
		}
		n, err := f.readChunkAt(nil, rb, off+int64(read))
		if n < 0 {
			panic("sftp.File: returned negative count from readChunkAt")
		}
		if n > 0 {
			read += n
		}
		if err != nil {
			return read, err
		}
	}
	return read, nil
}

// ReadAt reads up to len(b) byte from the File at a given offset `off`. It returns
// the number of bytes read and an error, if any. ReadAt follows io.ReaderAt semantics,
// so the file offset is not altered during the read.
func (f *File) ReadAt(b []byte, off int64) (int, error) {
	if len(b) <= f.c.maxPacket {
		// This should be able to be serviced with 1/2 requests.
		// So, just do it directly.
		return f.readChunkAt(nil, b, off)
	}

	if f.c.disableConcurrentReads {
		return f.readAtSequential(b, off)
	}

	// Split the read into multiple maxPacket-sized concurrent reads bounded by maxConcurrentRequests.
	// This allows writes with a suitably large buffer to transfer data at a much faster rate
	// by overlapping round trip times.

	cancel := make(chan struct{})

	concurrency := len(b)/f.c.maxPacket + 1
	if concurrency > f.c.maxConcurrentRequests || concurrency < 1 {
		concurrency = f.c.maxConcurrentRequests
	}

	resPool := newResChanPool(concurrency)

	type work struct {
		id  uint32
		res chan result

		b   []byte
		off int64
	}
	workCh := make(chan work)

	// Slice: cut up the Read into any number of buffers of length <= f.c.maxPacket, and at appropriate offsets.
	go func() {
		defer close(workCh)

		b := b
		offset := off
		chunkSize := f.c.maxPacket

		for len(b) > 0 {
			rb := b
			if len(rb) > chunkSize {
				rb = rb[:chunkSize]
			}

			id := f.c.nextID()
			res := resPool.Get()

			f.c.dispatchRequest(res, &sshFxpReadPacket{
				ID:     id,
				Handle: f.handle,
				Offset: uint64(offset),
				Len:    uint32(chunkSize),
			})

			select {
			case workCh <- work{id, res, rb, offset}:
			case <-cancel:
				return
			}

			offset += int64(len(rb))
			b = b[len(rb):]
		}
	}()

	type rErr struct {
		off int64
		err error
	}
	errCh := make(chan rErr)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		// Map_i: each worker gets work, and then performs the Read into its buffer from its respective offset.
		go func() {
			defer wg.Done()

			for packet := range workCh {
				var n int

				s := <-packet.res
				resPool.Put(packet.res)

				err := s.err
				if err == nil {
					switch s.typ {
					case sshFxpStatus:
						err = normaliseError(unmarshalStatus(packet.id, s.data))

					case sshFxpData:
						sid, data := unmarshalUint32(s.data)
						if packet.id != sid {
							err = &unexpectedIDErr{packet.id, sid}

						} else {
							l, data := unmarshalUint32(data)
							n = copy(packet.b, data[:l])

							// For normal disk files, it is guaranteed that this will read
							// the specified number of bytes, or up to end of file.
							// This implies, if we have a short read, that means EOF.
							if n < len(packet.b) {
								err = io.EOF
							}
						}

					default:
						err = unimplementedPacketErr(s.typ)
					}
				}

				if err != nil {
					// return the offset as the start + how much we read before the error.
					errCh <- rErr{packet.off + int64(n), err}
					return
				}
			}
		}()
	}

	// Wait for long tail, before closing results.
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Reduce: collect all the results into a relevant return: the earliest offset to return an error.
	firstErr := rErr{math.MaxInt64, nil}
	for rErr := range errCh {
		if rErr.off <= firstErr.off {
			firstErr = rErr
		}

		select {
		case <-cancel:
		default:
			// stop any more work from being distributed. (Just in case.)
			close(cancel)
		}
	}

	if firstErr.err != nil {
		// firstErr.err != nil if and only if firstErr.off > our starting offset.
		return int(firstErr.off - off), firstErr.err
	}

	// As per spec for io.ReaderAt, we return nil error if and only if we read everything.
	return len(b), nil
}

// writeToSequential implements WriteTo, but works sequentially with no parallelism.
func (f *File) writeToSequential(w io.Writer) (written int64, err error) {
	b := make([]byte, f.c.maxPacket)
	ch := make(chan result, 1) // reusable channel

	for {
		n, err := f.readChunkAt(ch, b, f.offset)
		if n < 0 {
			panic("sftp.File: returned negative count from readChunkAt")
		}

		if n > 0 {
			f.offset += int64(n)

			m, err := w.Write(b[:n])
			written += int64(m)

			if err != nil {
				return written, err
			}
		}

		if err != nil {
			if err == io.EOF {
				return written, nil // return nil explicitly.
			}

			return written, err
		}
	}
}

// WriteTo writes the file to the given Writer.
// The return value is the number of bytes written.
// Any error encountered during the write is also returned.
//
// This method is preferred over calling Read multiple times
// to maximise throughput for transferring the entire file,
// especially over high latency links.
func (f *File) WriteTo(w io.Writer) (written int64, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.c.disableConcurrentReads {
		return f.writeToSequential(w)
	}

	// For concurrency, we want to guess how many concurrent workers we should use.
	var fileStat *FileStat
	if f.c.useFstat {
		fileStat, err = f.c.fstat(f.handle)
	} else {
		fileStat, err = f.c.stat(f.path)
	}
	if err != nil {
		return 0, err
	}

	fileSize := fileStat.Size
	if fileSize <= uint64(f.c.maxPacket) || !isRegular(fileStat.Mode) {
		// only regular files are guaranteed to return (full read) xor (partial read, next error)
		return f.writeToSequential(w)
	}

	concurrency64 := fileSize/uint64(f.c.maxPacket) + 1 // a bad guess, but better than no guess
	if concurrency64 > uint64(f.c.maxConcurrentRequests) || concurrency64 < 1 {
		concurrency64 = uint64(f.c.maxConcurrentRequests)
	}
	// Now that concurrency64 is saturated to an int value, we know this assignment cannot possibly overflow.
	concurrency := int(concurrency64)

	chunkSize := f.c.maxPacket
	pool := newBufPool(concurrency, chunkSize)
	resPool := newResChanPool(concurrency)

	cancel := make(chan struct{})
	var wg sync.WaitGroup
	defer func() {
		// Once the writing Reduce phase has ended, all the feed work needs to unconditionally stop.
		close(cancel)

		// We want to wait until all outstanding goroutines with an `f` or `f.c` reference have completed.
		// Just to be sure we don’t orphan any goroutines any hanging references.
		wg.Wait()
	}()

	type writeWork struct {
		b   []byte
		off int64
		err error

		next chan writeWork
	}
	writeCh := make(chan writeWork)

	type readWork struct {
		id  uint32
		res chan result
		off int64

		cur, next chan writeWork
	}
	readCh := make(chan readWork)

	// Slice: hand out chunks of work on demand, with a `cur` and `next` channel built-in for sequencing.
	go func() {
		defer close(readCh)

		off := f.offset

		cur := writeCh
		for {
			id := f.c.nextID()
			res := resPool.Get()

			next := make(chan writeWork)
			readWork := readWork{
				id:  id,
				res: res,
				off: off,

				cur:  cur,
				next: next,
			}

			f.c.dispatchRequest(res, &sshFxpReadPacket{
				ID:     id,
				Handle: f.handle,
				Offset: uint64(off),
				Len:    uint32(chunkSize),
			})

			select {
			case readCh <- readWork:
			case <-cancel:
				return
			}

			off += int64(chunkSize)
			cur = next
		}
	}()

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		// Map_i: each worker gets readWork, and does the Read into a buffer at the given offset.
		go func() {
			defer wg.Done()

			for readWork := range readCh {
				var b []byte
				var n int

				s := <-readWork.res
				resPool.Put(readWork.res)

				err := s.err
				if err == nil {
					switch s.typ {
					case sshFxpStatus:
						err = normaliseError(unmarshalStatus(readWork.id, s.data))

					case sshFxpData:
						sid, data := unmarshalUint32(s.data)
						if readWork.id != sid {
							err = &unexpectedIDErr{readWork.id, sid}

						} else {
							l, data := unmarshalUint32(data)
							b = pool.Get()[:l]
							n = copy(b, data[:l])
							b = b[:n]
						}

					default:
						err = unimplementedPacketErr(s.typ)
					}
				}

				writeWork := writeWork{
					b:   b,
					off: readWork.off,
					err: err,

					next: readWork.next,
				}

				select {
				case readWork.cur <- writeWork:
				case <-cancel:
					return
				}

				if err != nil {
					return
				}
			}
		}()
	}

	// Reduce: serialize the results from the reads into sequential writes.
	cur := writeCh
	for {
		packet, ok := <-cur
		if !ok {
			return written, errors.New("sftp.File.WriteTo: unexpectedly closed channel")
		}

		// Because writes are serialized, this will always be the last successfully read byte.
		f.offset = packet.off + int64(len(packet.b))

		if len(packet.b) > 0 {
			n, err := w.Write(packet.b)
			written += int64(n)
			if err != nil {
				return written, err
			}
		}

		if packet.err != nil {
			if packet.err == io.EOF {
				return written, nil
			}

			return written, packet.err
		}

		pool.Put(packet.b)
		cur = packet.next
	}
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
	f.mu.Lock()
	defer f.mu.Unlock()

	n, err := f.WriteAt(b, f.offset)
	f.offset += int64(n)
	return n, err
}

func (f *File) writeChunkAt(ch chan result, b []byte, off int64) (int, error) {
	typ, data, err := f.c.sendPacket(ch, &sshFxpWritePacket{
		ID:     f.c.nextID(),
		Handle: f.handle,
		Offset: uint64(off),
		Length: uint32(len(b)),
		Data:   b,
	})
	if err != nil {
		return 0, err
	}

	switch typ {
	case sshFxpStatus:
		id, _ := unmarshalUint32(data)
		err := normaliseError(unmarshalStatus(id, data))
		if err != nil {
			return 0, err
		}

	default:
		return 0, unimplementedPacketErr(typ)
	}

	return len(b), nil
}

// writeAtConcurrent implements WriterAt, but works concurrently rather than sequentially.
func (f *File) writeAtConcurrent(b []byte, off int64) (int, error) {
	// Split the write into multiple maxPacket sized concurrent writes
	// bounded by maxConcurrentRequests. This allows writes with a suitably
	// large buffer to transfer data at a much faster rate due to
	// overlapping round trip times.

	cancel := make(chan struct{})

	type work struct {
		id  uint32
		res chan result

		off int64
	}
	workCh := make(chan work)

	concurrency := len(b)/f.c.maxPacket + 1
	if concurrency > f.c.maxConcurrentRequests || concurrency < 1 {
		concurrency = f.c.maxConcurrentRequests
	}

	pool := newResChanPool(concurrency)

	// Slice: cut up the Read into any number of buffers of length <= f.c.maxPacket, and at appropriate offsets.
	go func() {
		defer close(workCh)

		var read int
		chunkSize := f.c.maxPacket

		for read < len(b) {
			wb := b[read:]
			if len(wb) > chunkSize {
				wb = wb[:chunkSize]
			}

			id := f.c.nextID()
			res := pool.Get()
			off := off + int64(read)

			f.c.dispatchRequest(res, &sshFxpWritePacket{
				ID:     id,
				Handle: f.handle,
				Offset: uint64(off),
				Length: uint32(len(wb)),
				Data:   wb,
			})

			select {
			case workCh <- work{id, res, off}:
			case <-cancel:
				return
			}

			read += len(wb)
		}
	}()

	type wErr struct {
		off int64
		err error
	}
	errCh := make(chan wErr)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		// Map_i: each worker gets work, and does the Write from each buffer to its respective offset.
		go func() {
			defer wg.Done()

			for work := range workCh {
				s := <-work.res
				pool.Put(work.res)

				err := s.err
				if err == nil {
					switch s.typ {
					case sshFxpStatus:
						err = normaliseError(unmarshalStatus(work.id, s.data))
					default:
						err = unimplementedPacketErr(s.typ)
					}
				}

				if err != nil {
					errCh <- wErr{work.off, err}
				}
			}
		}()
	}

	// Wait for long tail, before closing results.
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Reduce: collect all the results into a relevant return: the earliest offset to return an error.
	firstErr := wErr{math.MaxInt64, nil}
	for wErr := range errCh {
		if wErr.off <= firstErr.off {
			firstErr = wErr
		}

		select {
		case <-cancel:
		default:
			// stop any more work from being distributed. (Just in case.)
			close(cancel)
		}
	}

	if firstErr.err != nil {
		// firstErr.err != nil if and only if firstErr.off >= our starting offset.
		return int(firstErr.off - off), firstErr.err
	}

	return len(b), nil
}

// WriteAt writes up to len(b) byte to the File at a given offset `off`. It returns
// the number of bytes written and an error, if any. WriteAt follows io.WriterAt semantics,
// so the file offset is not altered during the write.
func (f *File) WriteAt(b []byte, off int64) (written int, err error) {
	if len(b) <= f.c.maxPacket {
		// We can do this in one write.
		return f.writeChunkAt(nil, b, off)
	}

	if f.c.useConcurrentWrites {
		return f.writeAtConcurrent(b, off)
	}

	ch := make(chan result, 1) // reusable channel

	chunkSize := f.c.maxPacket

	for written < len(b) {
		wb := b[written:]
		if len(wb) > chunkSize {
			wb = wb[:chunkSize]
		}

		n, err := f.writeChunkAt(ch, wb, off+int64(written))
		if n > 0 {
			written += n
		}

		if err != nil {
			return written, err
		}
	}

	return len(b), nil
}

// ReadFromWithConcurrency implements ReaderFrom,
// but uses the given concurrency to issue multiple requests at the same time.
//
// Giving a concurrency of less than one will default to the Client’s max concurrency.
//
// Otherwise, the given concurrency will be capped by the Client's max concurrency.
func (f *File) ReadFromWithConcurrency(r io.Reader, concurrency int) (read int64, err error) {
	// Split the write into multiple maxPacket sized concurrent writes.
	// This allows writes with a suitably large reader
	// to transfer data at a much faster rate due to overlapping round trip times.

	cancel := make(chan struct{})

	type work struct {
		id  uint32
		res chan result

		off int64
	}
	workCh := make(chan work)

	type rwErr struct {
		off int64
		err error
	}
	errCh := make(chan rwErr)

	if concurrency > f.c.maxConcurrentRequests || concurrency < 1 {
		concurrency = f.c.maxConcurrentRequests
	}

	pool := newResChanPool(concurrency)

	// Slice: cut up the Read into any number of buffers of length <= f.c.maxPacket, and at appropriate offsets.
	go func() {
		defer close(workCh)

		b := make([]byte, f.c.maxPacket)
		off := f.offset

		for {
			n, err := r.Read(b)

			if n > 0 {
				read += int64(n)

				id := f.c.nextID()
				res := pool.Get()

				f.c.dispatchRequest(res, &sshFxpWritePacket{
					ID:     id,
					Handle: f.handle,
					Offset: uint64(off),
					Length: uint32(n),
					Data:   b[:n],
				})

				select {
				case workCh <- work{id, res, off}:
				case <-cancel:
					return
				}

				off += int64(n)
			}

			if err != nil {
				if err != io.EOF {
					errCh <- rwErr{off, err}
				}
				return
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		// Map_i: each worker gets work, and does the Write from each buffer to its respective offset.
		go func() {
			defer wg.Done()

			for work := range workCh {
				s := <-work.res
				pool.Put(work.res)

				err := s.err
				if err == nil {
					switch s.typ {
					case sshFxpStatus:
						err = normaliseError(unmarshalStatus(work.id, s.data))
					default:
						err = unimplementedPacketErr(s.typ)
					}
				}

				if err != nil {
					errCh <- rwErr{work.off, err}
				}
			}
		}()
	}

	// Wait for long tail, before closing results.
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Reduce: Collect all the results into a relevant return: the earliest offset to return an error.
	firstErr := rwErr{math.MaxInt64, nil}
	for rwErr := range errCh {
		if rwErr.off <= firstErr.off {
			firstErr = rwErr
		}

		select {
		case <-cancel:
		default:
			// stop any more work from being distributed.
			close(cancel)
		}
	}

	if firstErr.err != nil {
		// firstErr.err != nil if and only if firstErr.off is a valid offset.
		//
		// firstErr.off will then be the lesser of:
		// * the offset of the first error from writing,
		// * the last successfully read offset.
		//
		// This could be less than the last successfully written offset,
		// which is the whole reason for the UseConcurrentWrites() ClientOption.
		//
		// Callers are responsible for truncating any SFTP files to a safe length.
		f.offset = firstErr.off

		// ReadFrom is defined to return the read bytes, regardless of any writer errors.
		return read, firstErr.err
	}

	f.offset += read
	return read, nil
}

// ReadFrom reads data from r until EOF and writes it to the file. The return
// value is the number of bytes read. Any error except io.EOF encountered
// during the read is also returned.
//
// This method is preferred over calling Write multiple times
// to maximise throughput for transferring the entire file,
// especially over high-latency links.
func (f *File) ReadFrom(r io.Reader) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.c.useConcurrentWrites {
		var remain int64
		switch r := r.(type) {
		case interface{ Len() int }:
			remain = int64(r.Len())

		case interface{ Size() int64 }:
			remain = r.Size()

		case *io.LimitedReader:
			remain = r.N

		case interface{ Stat() (os.FileInfo, error) }:
			info, err := r.Stat()
			if err == nil {
				remain = info.Size()
			}
		}

		if remain < 0 {
			// We can strongly assert that we want default max concurrency here.
			return f.ReadFromWithConcurrency(r, f.c.maxConcurrentRequests)
		}

		if remain > int64(f.c.maxPacket) {
			// Otherwise, only use concurrency, if it would be at least two packets.

			// This is the best reasonable guess we can make.
			concurrency64 := remain/int64(f.c.maxPacket) + 1

			// We need to cap this value to an `int` size value to avoid overflow on 32-bit machines.
			// So, we may as well pre-cap it to `f.c.maxConcurrentRequests`.
			if concurrency64 > int64(f.c.maxConcurrentRequests) {
				concurrency64 = int64(f.c.maxConcurrentRequests)
			}

			return f.ReadFromWithConcurrency(r, int(concurrency64))
		}
	}

	ch := make(chan result, 1) // reusable channel

	b := make([]byte, f.c.maxPacket)

	var read int64
	for {
		n, err := r.Read(b)
		if n < 0 {
			panic("sftp.File: reader returned negative count from Read")
		}

		if n > 0 {
			read += int64(n)

			m, err2 := f.writeChunkAt(ch, b[:n], f.offset)
			f.offset += int64(m)

			if err == nil {
				err = err2
			}
		}

		if err != nil {
			if err == io.EOF {
				return read, nil // return nil explicitly.
			}

			return read, err
		}
	}
}

// Seek implements io.Seeker by setting the client offset for the next Read or
// Write. It returns the next offset read. Seeking before or after the end of
// the file is undefined. Seeking relative to the end calls Stat.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += f.offset
	case io.SeekEnd:
		fi, err := f.Stat()
		if err != nil {
			return f.offset, err
		}
		offset += fi.Size()
	default:
		return f.offset, unimplementedSeekWhence(whence)
	}

	if offset < 0 {
		return f.offset, os.ErrInvalid
	}

	f.offset = offset
	return f.offset, nil
}

// Chown changes the uid/gid of the current file.
func (f *File) Chown(uid, gid int) error {
	return f.c.Chown(f.path, uid, gid)
}

// Chmod changes the permissions of the current file.
//
// See Client.Chmod for details.
func (f *File) Chmod(mode os.FileMode) error {
	return f.c.setfstat(f.handle, sshFileXferAttrPermissions, toChmodPerm(mode))
}

// Sync requests a flush of the contents of a File to stable storage.
//
// Sync requires the server to support the fsync@openssh.com extension.
func (f *File) Sync() error {
	id := f.c.nextID()
	typ, data, err := f.c.sendPacket(nil, &sshFxpFsyncPacket{
		ID:     id,
		Handle: f.handle,
	})

	switch {
	case err != nil:
		return err
	case typ == sshFxpStatus:
		return normaliseError(unmarshalStatus(id, data))
	default:
		return &unexpectedPacketErr{want: sshFxpStatus, got: typ}
	}
}

// Truncate sets the size of the current file. Although it may be safely assumed
// that if the size is less than its current size it will be truncated to fit,
// the SFTP protocol does not specify what behavior the server should do when setting
// size greater than the current size.
// We send a SSH_FXP_FSETSTAT here since we have a file handle
func (f *File) Truncate(size int64) error {
	return f.c.setfstat(f.handle, sshFileXferAttrSize, uint64(size))
}

// normaliseError normalises an error into a more standard form that can be
// checked against stdlib errors like io.EOF or os.ErrNotExist.
func normaliseError(err error) error {
	switch err := err.(type) {
	case *StatusError:
		switch err.Code {
		case sshFxEOF:
			return io.EOF
		case sshFxNoSuchFile:
			return os.ErrNotExist
		case sshFxPermissionDenied:
			return os.ErrPermission
		case sshFxOk:
			return nil
		default:
			return err
		}
	default:
		return err
	}
}

// flags converts the flags passed to OpenFile into ssh flags.
// Unsupported flags are ignored.
func flags(f int) uint32 {
	var out uint32
	switch f & os.O_WRONLY {
	case os.O_WRONLY:
		out |= sshFxfWrite
	case os.O_RDONLY:
		out |= sshFxfRead
	}
	if f&os.O_RDWR == os.O_RDWR {
		out |= sshFxfRead | sshFxfWrite
	}
	if f&os.O_APPEND == os.O_APPEND {
		out |= sshFxfAppend
	}
	if f&os.O_CREATE == os.O_CREATE {
		out |= sshFxfCreat
	}
	if f&os.O_TRUNC == os.O_TRUNC {
		out |= sshFxfTrunc
	}
	if f&os.O_EXCL == os.O_EXCL {
		out |= sshFxfExcl
	}
	return out
}

// toChmodPerm converts Go permission bits to POSIX permission bits.
//
// This differs from fromFileMode in that we preserve the POSIX versions of
// setuid, setgid and sticky in m, because we've historically supported those
// bits, and we mask off any non-permission bits.
func toChmodPerm(m os.FileMode) (perm uint32) {
	const mask = os.ModePerm | s_ISUID | s_ISGID | s_ISVTX
	perm = uint32(m & mask)

	if m&os.ModeSetuid != 0 {
		perm |= s_ISUID
	}
	if m&os.ModeSetgid != 0 {
		perm |= s_ISGID
	}
	if m&os.ModeSticky != 0 {
		perm |= s_ISVTX
	}

	return perm
}
