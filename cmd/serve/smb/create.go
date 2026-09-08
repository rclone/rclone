package smb

import (
	"errors"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// CreateDisposition values ([MS-SMB2] 2.2.13).
const (
	dispSupersede   uint32 = 0
	dispOpen        uint32 = 1
	dispCreate      uint32 = 2
	dispOpenIf      uint32 = 3
	dispOverwrite   uint32 = 4
	dispOverwriteIf uint32 = 5
)

// CreateOptions bit flags ([MS-SMB2] 2.2.13).
const (
	optDirectoryFile    uint32 = 0x00000001
	optNonDirectoryFile uint32 = 0x00000040
	optDeleteOnClose    uint32 = 0x00001000
)

// CreateAction values ([MS-SMB2] 2.2.14).
const (
	actionOpened      uint32 = 1
	actionCreated     uint32 = 2
	actionOverwritten uint32 = 3
)

// File attribute flags ([MS-FSCC] 2.6).
const (
	fileAttrDirectory uint32 = 0x00000010
	fileAttrNormal    uint32 = 0x00000080
)

// DesiredAccess bits we test for write intent ([MS-SMB2] 2.2.13.1).
const (
	accFileWriteData  uint32 = 0x00000002
	accFileAppendData uint32 = 0x00000004
	accGenericAll     uint32 = 0x10000000
	accGenericWrite   uint32 = 0x40000000
)

// openFile is a server-side open: an entry in a connection's handle table
// keyed by the 16-byte SMB2 FileId.
type openFile struct {
	fileID        [16]byte
	path          string     // VFS path (forward-slash, no leading slash)
	node          vfs.Node   // the file or directory node
	handle        vfs.Handle // open IO handle for files (nil for directories)
	isDir         bool
	deleteOnClose bool

	// directory enumeration state for QUERY_DIRECTORY
	dirEntries []vfs.Node
	dirPos     int
	dirLoaded  bool
}

// maxOpenFiles caps the number of open handles per connection so a client that
// never sends CLOSE can't exhaust the process's file descriptors.
const maxOpenFiles = 16384

// handleCreate handles an SMB2 CREATE request: it opens or creates a file or
// directory and adds it to the connection's handle table.
func (c *conn) handleCreate(h header, body []byte) (uint32, []byte) {
	if len(body) < 56 {
		return statusInvalidParameter, errorResponseBody()
	}
	if len(c.handles) >= maxOpenFiles {
		return statusInsufficientResources, errorResponseBody()
	}
	desiredAccess := le.Uint32(body[24:28])
	createDisposition := le.Uint32(body[36:40])
	createOptions := le.Uint32(body[40:44])
	name := ""
	if buf := bufferAt(body, le.Uint16(body[44:46]), le.Uint16(body[46:48])); buf != nil {
		name = utf16leToString(buf)
	}
	path := nameToPath(name)

	node, statErr := c.statPath(path)
	exists := statErr == nil

	// Disposition preconditions.
	switch createDisposition {
	case dispOpen, dispOverwrite:
		if !exists {
			return mapVFSError(statErr), errorResponseBody()
		}
	case dispCreate:
		if exists {
			return statusObjectNameCollision, errorResponseBody()
		}
	}

	var of *openFile
	var action uint32

	switch {
	case exists && node.IsDir():
		if createOptions&optNonDirectoryFile != 0 {
			return statusFileIsADirectory, errorResponseBody()
		}
		of = &openFile{path: path, node: node, isDir: true}
		action = actionOpened
	case !exists && createOptions&optDirectoryFile != 0:
		if err := c.server.vfs.Mkdir(path, 0777); err != nil {
			return mapVFSError(err), errorResponseBody()
		}
		newNode, err := c.statPath(path)
		if err != nil {
			return mapVFSError(err), errorResponseBody()
		}
		of = &openFile{path: path, node: newNode, isDir: true}
		action = actionCreated
	case exists && !node.IsDir() && createOptions&optDirectoryFile != 0:
		// The client asked to open a directory (FILE_DIRECTORY_FILE) but the name
		// is a regular file.
		return statusNotADirectory, errorResponseBody()
	default:
		flags := accessToFlags(desiredAccess, createDisposition)
		handle, err := c.server.vfs.OpenFile(path, flags, 0666)
		if err != nil {
			return mapVFSError(err), errorResponseBody()
		}
		of = &openFile{path: path, node: handle.Node(), handle: handle}
		switch {
		case !exists:
			action = actionCreated
		case flags&os.O_TRUNC != 0:
			action = actionOverwritten
		default:
			action = actionOpened
		}
	}

	if createOptions&optDeleteOnClose != 0 {
		of.deleteOnClose = true
	}
	of.fileID = c.newFileID()
	c.addHandle(of)
	return statusSuccess, c.buildCreateResp(action, of)
}

// buildCreateResp builds the CREATE response body ([MS-SMB2] 2.2.14).
func (c *conn) buildCreateResp(action uint32, of *openFile) []byte {
	attrs, size, mtime := nodeAttrs(of.node)
	ft := timeToFiletime(mtime)
	body := make([]byte, 89)
	le.PutUint16(body[0:2], 89) // StructureSize (fixed magic value)
	le.PutUint32(body[4:8], action)
	le.PutUint64(body[8:16], ft)  // CreationTime
	le.PutUint64(body[16:24], ft) // LastAccessTime
	le.PutUint64(body[24:32], ft) // LastWriteTime
	le.PutUint64(body[32:40], ft) // ChangeTime
	le.PutUint64(body[40:48], uint64(size))
	le.PutUint64(body[48:56], uint64(size))
	le.PutUint32(body[56:60], attrs)
	copy(body[64:80], of.fileID[:])
	return body
}

// nodeAttrs returns the SMB file attributes, size and modification time of a
// VFS node.
func nodeAttrs(node vfs.Node) (attrs uint32, size int64, mtime time.Time) {
	if node.IsDir() {
		attrs = fileAttrDirectory
	} else {
		attrs = fileAttrNormal
	}
	return attrs, node.Size(), node.ModTime()
}

// accessToFlags converts an SMB DesiredAccess and CreateDisposition into the
// os.OpenFile flags used by the VFS.
func accessToFlags(access, disposition uint32) int {
	write := access&(accGenericWrite|accGenericAll|accFileWriteData|accFileAppendData) != 0
	flags := os.O_RDONLY
	if write {
		flags = os.O_RDWR
	}
	if access&accFileAppendData != 0 && access&accFileWriteData == 0 {
		flags |= os.O_APPEND
	}
	switch disposition {
	case dispCreate:
		flags |= os.O_CREATE | os.O_EXCL
	case dispOpenIf:
		flags |= os.O_CREATE
	case dispOverwrite:
		flags |= os.O_TRUNC
	case dispOverwriteIf, dispSupersede:
		flags |= os.O_CREATE | os.O_TRUNC
	}
	return flags
}

// nameToPath converts an SMB share-relative name (backslash separated, UTF-16
// decoded) into a VFS path (forward-slash, no leading or trailing slash). It
// cleans "." and ".." components and anchors the result at the share root so a
// client cannot escape the share with "..".
func nameToPath(name string) string {
	p := strings.ReplaceAll(name, `\`, `/`)
	p = path.Clean("/" + p)
	return strings.Trim(p, "/")
}

// statPath looks up a VFS node by path, treating the empty path as the root.
func (c *conn) statPath(path string) (vfs.Node, error) {
	if path == "" {
		root, err := c.server.vfs.Root()
		return root, err
	}
	return c.server.vfs.Stat(path)
}

// listDir returns the directory entries at the given VFS path.
func (c *conn) listDir(path string) ([]vfs.Node, error) {
	node, err := c.statPath(path)
	if err != nil {
		return nil, err
	}
	dir, ok := node.(*vfs.Dir)
	if !ok {
		return nil, vfs.ENOSYS
	}
	nodes, err := dir.ReadDirAll()
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// mapVFSError maps a VFS/os error to the closest NTSTATUS code.
func mapVFSError(err error) uint32 {
	switch {
	case err == nil:
		return statusSuccess
	case errors.Is(err, os.ErrNotExist):
		return statusObjectNameNotFound
	case errors.Is(err, os.ErrExist):
		return statusObjectNameCollision
	case errors.Is(err, os.ErrPermission):
		return statusAccessDenied
	case errors.Is(err, vfs.EROFS):
		return statusMediaWriteProtected
	case errors.Is(err, vfs.ENOTEMPTY):
		return statusDirectoryNotEmpty
	case errors.Is(err, vfs.ENOSYS):
		return statusNotSupported
	case errors.Is(err, os.ErrInvalid):
		return statusInvalidParameter
	default:
		// OS-specific errors that the portable os sentinels don't catch (e.g. a
		// Windows sharing violation on a file another process holds open). Mapping
		// these to a precise status rather than the STATUS_UNSUCCESSFUL catch-all
		// lets SMB clients skip the item instead of aborting the operation.
		if s, ok := osErrorStatus(err); ok {
			return s
		}
		return statusUnsuccessful
	}
}

// --- handle table (per connection) ---

func (c *conn) newFileID() [16]byte {
	c.mu.Lock()
	c.handleCtr++
	id := c.handleCtr
	c.mu.Unlock()
	var fid [16]byte
	le.PutUint64(fid[0:8], id)  // Persistent
	le.PutUint64(fid[8:16], id) // Volatile
	return fid
}

func (c *conn) addHandle(of *openFile) {
	c.mu.Lock()
	c.handles[of.fileID] = of
	c.mu.Unlock()
}

func (c *conn) getHandle(fileID []byte) *openFile {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.handles[fileIDKey(fileID)]
}

func (c *conn) removeHandle(fileID []byte) *openFile {
	k := fileIDKey(fileID)
	c.mu.Lock()
	defer c.mu.Unlock()
	of := c.handles[k]
	delete(c.handles, k)
	return of
}

func (c *conn) closeAllHandles() {
	c.mu.Lock()
	handles := c.handles
	c.handles = map[[16]byte]*openFile{}
	c.mu.Unlock()
	for _, of := range handles {
		if of.handle != nil {
			if err := of.handle.Close(); err != nil {
				fs.Errorf(c.server.vfs.Fs(), "SMB: error closing handle on teardown: %v", err)
			}
		}
	}
}

func fileIDKey(b []byte) (k [16]byte) {
	copy(k[:], b)
	return k
}
