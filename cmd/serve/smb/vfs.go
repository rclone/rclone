//go:build !windows && !plan9 && !(linux && (386 || arm || mips || mipsle))

// Package smb implements a server to serve a VFS remote over SMB
package smb

import (
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/macos-fuse-t/go-smb2/vfs"
	rclonefs "github.com/rclone/rclone/fs"
	rclonevfs "github.com/rclone/rclone/vfs"
)

// smbVFS bridges the macos-fuse-t/go-smb2 VFSFileSystem interface
// to the rclone VFS.
type smbVFS struct {
	vfs *rclonevfs.VFS

	mu      sync.RWMutex
	handles map[vfs.VfsHandle]*smbHandle
	nextID  atomic.Uint64
}

type smbHandle struct {
	path   string
	file   rclonevfs.Handle // open file handle (nil for dirs)
	dir    *rclonevfs.Dir   // open dir (nil for files)
	node   rclonevfs.Node   // underlying VFS node
	dirPos int              // current position in directory listing (for stateful ReadDir)
}

func newSMBVFS(vfsInst *rclonevfs.VFS) *smbVFS {
	return &smbVFS{
		vfs:     vfsInst,
		handles: make(map[vfs.VfsHandle]*smbHandle),
	}
}

// allocHandle stores a handle and returns its ID
func (s *smbVFS) allocHandle(h *smbHandle) vfs.VfsHandle {
	id := vfs.VfsHandle(s.nextID.Add(1))
	s.mu.Lock()
	s.handles[id] = h
	s.mu.Unlock()
	return id
}

// getHandle retrieves a handle by ID; handle 0 is the root
func (s *smbVFS) getHandle(id vfs.VfsHandle) *smbHandle {
	if id == 0 {
		root, err := s.vfs.Root()
		if err != nil {
			return nil
		}
		return &smbHandle{
			path: "",
			dir:  root,
			node: root,
		}
	}
	s.mu.RLock()
	h := s.handles[id]
	s.mu.RUnlock()
	return h
}

// freeHandle removes a handle from the table
func (s *smbVFS) freeHandle(id vfs.VfsHandle) {
	s.mu.Lock()
	delete(s.handles, id)
	s.mu.Unlock()
}

// cleanPath cleans an SMB path for use with the VFS
func cleanPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean("/" + p)
	if p == "/" {
		return ""
	}
	// Remove leading slash - rclone VFS uses relative paths
	return p[1:]
}

// nodeToAttrs converts a VFS node to SMB Attributes.
// Node embeds os.FileInfo, so the node itself is the FileInfo.
func nodeToAttrs(node rclonevfs.Node) *vfs.Attributes {
	a := &vfs.Attributes{}
	a.SetSizeBytes(uint64(node.Size()))
	a.SetDiskSizeBytes(uint64(node.Size()))
	a.SetLastDataModificationTime(node.ModTime())
	a.SetAccessTime(node.ModTime())
	a.SetLastStatusChangeTime(node.ModTime())
	a.SetBirthTime(node.ModTime())
	a.SetLinkCount(1)

	if node.IsDir() {
		a.SetFileType(vfs.FileTypeDirectory)
		a.SetPermissions(vfs.PermissionsRead | vfs.PermissionsWrite | vfs.PermissionsExecute)
	} else {
		a.SetFileType(vfs.FileTypeRegularFile)
		a.SetPermissions(vfs.PermissionsRead | vfs.PermissionsWrite)
	}

	inode := node.Inode()
	a.SetInodeNumber(inode)
	a.SetFileHandle(vfs.VfsNode(inode))
	a.SetChangeID(uint64(node.ModTime().UnixNano()))

	return a
}

// convertError maps common errors to syscall errors that the SMB
// library translates to NT status codes
func convertError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case err == rclonevfs.ENOENT || os.IsNotExist(err):
		return syscall.ENOENT
	case err == rclonevfs.EEXIST || os.IsExist(err):
		return syscall.EEXIST
	case err == rclonevfs.EPERM || os.IsPermission(err):
		return syscall.EACCES
	case err == rclonevfs.ENOTEMPTY:
		return syscall.ENOTEMPTY
	case err == rclonevfs.EINVAL:
		return syscall.EINVAL
	case err == rclonevfs.ENOSYS:
		return syscall.ENOSYS
	}
	return err
}

// --- VFSFileSystem interface ---

// GetAttr returns attributes for an open handle
func (s *smbVFS) GetAttr(handle vfs.VfsHandle) (*vfs.Attributes, error) {
	h := s.getHandle(handle)
	if h == nil {
		return nil, syscall.EBADF
	}
	if h.dir != nil {
		return nodeToAttrs(h.dir), nil
	}
	if h.file != nil {
		// Get the node from the file handle
		node := h.file.Node()
		if node != nil {
			return nodeToAttrs(node), nil
		}
		// Fall back to stat by path
		n, err := s.vfs.Stat(h.path)
		if err != nil {
			return nil, convertError(err)
		}
		return nodeToAttrs(n), nil
	}
	if h.node != nil {
		return nodeToAttrs(h.node), nil
	}
	return nil, syscall.EBADF
}

// SetAttr sets attributes on an open handle
func (s *smbVFS) SetAttr(handle vfs.VfsHandle, attrs *vfs.Attributes) (*vfs.Attributes, error) {
	h := s.getHandle(handle)
	if h == nil {
		return nil, syscall.EBADF
	}

	// Handle size change (truncation)
	if size, ok := attrs.GetSizeBytes(); ok {
		if h.file != nil {
			if err := h.file.Truncate(int64(size)); err != nil {
				return nil, convertError(err)
			}
		}
	}

	// Handle mtime change
	if mtime, ok := attrs.GetLastDataModificationTime(); ok {
		var node rclonevfs.Node
		if h.dir != nil {
			node = h.dir
		} else if h.node != nil {
			node = h.node
		} else if h.file != nil {
			node = h.file.Node()
		}
		if node == nil {
			n, err := s.vfs.Stat(h.path)
			if err == nil {
				node = n
			}
		}
		if node != nil {
			if err := node.SetModTime(mtime); err != nil {
				rclonefs.Debugf(nil, "SetAttr: failed to set mtime on %q: %v", h.path, err)
			}
		}
	}

	return s.GetAttr(handle)
}

// StatFS returns filesystem statistics
func (s *smbVFS) StatFS(_ vfs.VfsHandle) (*vfs.FSAttributes, error) {
	const blockSize = 4096

	total, used, free := s.vfs.Statfs()

	fsa := &vfs.FSAttributes{}
	fsa.SetBlockSize(blockSize)
	fsa.SetIOSize(blockSize)

	if total < 0 {
		total = 1 << 40 // 1 TiB default
	}
	if free < 0 {
		free = 1 << 40
	}
	_ = used

	fsa.SetBlocks(uint64(total) / blockSize)
	fsa.SetFreeBlocks(uint64(free) / blockSize)
	fsa.SetAvailableBlocks(uint64(free) / blockSize)
	fsa.SetFiles(1000000)
	fsa.SetFreeFiles(1000000)

	return fsa, nil
}

// FSync syncs an open file handle
func (s *smbVFS) FSync(handle vfs.VfsHandle) error {
	h := s.getHandle(handle)
	if h == nil {
		return syscall.EBADF
	}
	if h.file != nil {
		return convertError(h.file.Sync())
	}
	return nil
}

// Flush flushes an open file handle
func (s *smbVFS) Flush(handle vfs.VfsHandle) error {
	h := s.getHandle(handle)
	if h == nil {
		return syscall.EBADF
	}
	if h.file != nil {
		return convertError(h.file.Flush())
	}
	return nil
}

// Open opens a file and returns a handle.
// Falls back to case-insensitive matching for SMB compatibility.
func (s *smbVFS) Open(name string, flags int, mode int) (vfs.VfsHandle, error) {
	name = cleanPath(name)

	// Strip lock flags that the SMB server adds
	flags &^= (oShlock | oExlock | 0x200000)

	fh, err := s.vfs.OpenFile(name, flags, os.FileMode(mode))
	if err != nil {
		// Try case-insensitive lookup and open with the real name
		if node, err2 := s.statCaseInsensitive(name); err2 == nil {
			realPath := node.Path()
			fh, err = s.vfs.OpenFile(realPath, flags, os.FileMode(mode))
			if err == nil {
				name = realPath
			}
		}
		if err != nil {
			return 0, convertError(err)
		}
	}

	h := &smbHandle{
		path: name,
		file: fh,
		node: fh.Node(),
	}
	return s.allocHandle(h), nil
}

// Close closes an open handle
func (s *smbVFS) Close(handle vfs.VfsHandle) error {
	h := s.getHandle(handle)
	if h == nil {
		return syscall.EBADF
	}
	s.freeHandle(handle)
	if h.file != nil {
		return convertError(h.file.Close())
	}
	return nil
}

// Lookup looks up a child by name in the parent handle's directory.
// When parentHandle is 0, name is treated as a path from the root.
// Lookup is case-insensitive to match Windows SMB behavior.
func (s *smbVFS) Lookup(parentHandle vfs.VfsHandle, name string) (*vfs.Attributes, error) {
	var fullPath string
	if parentHandle == 0 {
		fullPath = cleanPath(name)
	} else {
		parent := s.getHandle(parentHandle)
		if parent == nil {
			return nil, syscall.EBADF
		}
		fullPath = cleanPath(path.Join(parent.path, name))
	}

	// Try exact match first
	node, err := s.vfs.Stat(fullPath)
	if err == nil {
		return nodeToAttrs(node), nil
	}

	// Fall back to case-insensitive lookup
	node, err = s.statCaseInsensitive(fullPath)
	if err != nil {
		return nil, convertError(err)
	}
	return nodeToAttrs(node), nil
}

// statCaseInsensitive does a case-insensitive stat by resolving each
// path component case-insensitively through directory listings.
func (s *smbVFS) statCaseInsensitive(fullPath string) (rclonevfs.Node, error) {
	if fullPath == "" {
		return s.vfs.Root()
	}

	parts := strings.Split(fullPath, "/")
	root, err := s.vfs.Root()
	if err != nil {
		return nil, err
	}
	var current rclonevfs.Node = root

	for _, part := range parts {
		dir, ok := current.(*rclonevfs.Dir)
		if !ok {
			return nil, rclonevfs.ENOENT
		}

		entries, err := dir.ReadDirAll()
		if err != nil {
			return nil, err
		}

		found := false
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), part) {
				current = entry
				found = true
				break
			}
		}
		if !found {
			return nil, rclonevfs.ENOENT
		}
	}
	return current, nil
}

// Mkdir creates a new directory
func (s *smbVFS) Mkdir(name string, _ int) (*vfs.Attributes, error) {
	name = cleanPath(name)
	dir, leaf := path.Split(name)
	dir = strings.TrimSuffix(dir, "/")

	parentNode, err := s.vfs.Stat(dir)
	if err != nil {
		return nil, convertError(err)
	}
	parentDir, ok := parentNode.(*rclonevfs.Dir)
	if !ok {
		return nil, syscall.ENOTDIR
	}
	if _, err := parentDir.Mkdir(leaf); err != nil {
		return nil, convertError(err)
	}
	node, err := s.vfs.Stat(name)
	if err != nil {
		return nil, convertError(err)
	}
	return nodeToAttrs(node), nil
}

// Read reads from an open file at the given offset
func (s *smbVFS) Read(handle vfs.VfsHandle, buf []byte, offset uint64, _ int) (int, error) {
	h := s.getHandle(handle)
	if h == nil || h.file == nil {
		return 0, syscall.EBADF
	}
	n, err := h.file.ReadAt(buf, int64(offset))
	if err == io.EOF && n > 0 {
		// Partial read with EOF - return data without error
		return n, nil
	}
	return n, convertError(err)
}

// Write writes to an open file at the given offset
func (s *smbVFS) Write(handle vfs.VfsHandle, data []byte, offset uint64, _ int) (int, error) {
	h := s.getHandle(handle)
	if h == nil || h.file == nil {
		return 0, syscall.EBADF
	}
	n, err := h.file.WriteAt(data, int64(offset))
	return n, convertError(err)
}

// OpenDir opens a directory and returns a handle.
// Falls back to case-insensitive matching for SMB compatibility.
func (s *smbVFS) OpenDir(name string) (vfs.VfsHandle, error) {
	name = cleanPath(name)
	node, err := s.vfs.Stat(name)
	if err != nil {
		// Try case-insensitive lookup
		node, err = s.statCaseInsensitive(name)
		if err != nil {
			return 0, convertError(err)
		}
	}
	dir, ok := node.(*rclonevfs.Dir)
	if !ok {
		return 0, syscall.ENOTDIR
	}
	h := &smbHandle{
		path: dir.Path(),
		dir:  dir,
		node: dir,
	}
	return s.allocHandle(h), nil
}

// ReadDir reads directory entries.
// The pos parameter: 0 = continue from current position, 1 = restart from beginning.
// The count parameter limits the number of entries returned.
// Returns io.EOF when no more entries are available.
func (s *smbVFS) ReadDir(handle vfs.VfsHandle, pos int, count int) ([]vfs.DirInfo, error) {
	h := s.getHandle(handle)
	if h == nil || h.dir == nil {
		return nil, syscall.EBADF
	}

	// pos=1 means RESTART_SCANS - restart from the beginning
	if pos == 1 {
		h.dirPos = 0
	}

	nodes, err := h.dir.ReadDirAll()
	if err != nil {
		return nil, convertError(err)
	}

	if h.dirPos >= len(nodes) {
		return nil, io.EOF
	}

	end := h.dirPos + count
	if end > len(nodes) {
		end = len(nodes)
	}
	slice := nodes[h.dirPos:end]
	h.dirPos = end

	entries := make([]vfs.DirInfo, 0, len(slice))
	for _, node := range slice {
		attrs := nodeToAttrs(node)
		entries = append(entries, vfs.DirInfo{
			Name:       node.Name(),
			Attributes: *attrs,
		})
	}
	return entries, nil
}

// Readlink reads the target of a symbolic link (not supported)
func (s *smbVFS) Readlink(_ vfs.VfsHandle) (string, error) {
	return "", syscall.ENOSYS
}

// Unlink removes a file or directory by handle
func (s *smbVFS) Unlink(handle vfs.VfsHandle) error {
	h := s.getHandle(handle)
	if h == nil {
		return syscall.EBADF
	}
	if h.path == "" {
		return syscall.EPERM
	}

	// Look up fresh to get current state
	node, err := s.vfs.Stat(h.path)
	if err != nil {
		return convertError(err)
	}
	return convertError(node.Remove())
}

// Truncate truncates a file to the given size
func (s *smbVFS) Truncate(handle vfs.VfsHandle, size uint64) error {
	h := s.getHandle(handle)
	if h == nil || h.file == nil {
		return syscall.EBADF
	}
	return convertError(h.file.Truncate(int64(size)))
}

// Rename renames/moves a file. The newPath parameter is relative to the share root.
func (s *smbVFS) Rename(handle vfs.VfsHandle, newPath string, _ int) error {
	h := s.getHandle(handle)
	if h == nil {
		return syscall.EBADF
	}
	newPath = cleanPath(newPath)
	return convertError(s.vfs.Rename(h.path, newPath))
}

// Symlink creates a symbolic link (not supported)
func (s *smbVFS) Symlink(_ vfs.VfsHandle, _ string, _ int) (*vfs.Attributes, error) {
	return nil, syscall.ENOSYS
}

// Link creates a hard link (not supported)
func (s *smbVFS) Link(_ vfs.VfsNode, _ vfs.VfsNode, _ string) (*vfs.Attributes, error) {
	return nil, syscall.ENOSYS
}

// Listxattr lists extended attributes (not supported)
func (s *smbVFS) Listxattr(_ vfs.VfsHandle) ([]string, error) {
	return nil, nil
}

// Getxattr gets an extended attribute (not supported)
func (s *smbVFS) Getxattr(_ vfs.VfsHandle, _ string, _ []byte) (int, error) {
	return 0, syscall.ENOENT
}

// Setxattr sets an extended attribute (not supported)
func (s *smbVFS) Setxattr(_ vfs.VfsHandle, _ string, _ []byte) error {
	return syscall.ENOSYS
}

// Removexattr removes an extended attribute (not supported)
func (s *smbVFS) Removexattr(_ vfs.VfsHandle, _ string) error {
	return syscall.ENOSYS
}

// oShlock and oExlock are lock flags that the SMB server may add to Open flags
const (
	oShlock = 0x10
	oExlock = 0x20
)

// Check that smbVFS implements VFSFileSystem
var _ vfs.VFSFileSystem = (*smbVFS)(nil)
