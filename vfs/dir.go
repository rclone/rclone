package vfs

import (
	"context"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/dirtree"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Dir represents a directory entry
type Dir struct {
	vfs   *VFS   // read only
	inode uint64 // read only: inode number
	f     fs.Fs  // read only

	mu      sync.RWMutex // protects the following
	parent  *Dir         // parent, nil for root
	path    string
	modTime time.Time
	entry   fs.Directory
	read    time.Time       // time directory entry last read
	items   map[string]Node // directory entries - can be empty but not nil
	sys     interface{}     // user defined info to be attached here
}

func newDir(vfs *VFS, f fs.Fs, parent *Dir, fsDir fs.Directory) *Dir {
	return &Dir{
		vfs:     vfs,
		f:       f,
		parent:  parent,
		entry:   fsDir,
		path:    fsDir.Remote(),
		modTime: fsDir.ModTime(context.TODO()),
		inode:   newInode(),
		items:   make(map[string]Node),
	}
}

// String converts it to printablee
func (d *Dir) String() string {
	if d == nil {
		return "<nil *Dir>"
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.path + "/"
}

// IsFile returns false for Dir - satisfies Node interface
func (d *Dir) IsFile() bool {
	return false
}

// IsDir returns true for Dir - satisfies Node interface
func (d *Dir) IsDir() bool {
	return true
}

// Mode bits of the directory - satisfies Node interface
func (d *Dir) Mode() (mode os.FileMode) {
	return d.vfs.Opt.DirPerms
}

// Name (base) of the directory - satisfies Node interface
func (d *Dir) Name() (name string) {
	d.mu.RLock()
	name = path.Base(d.path)
	d.mu.RUnlock()
	if name == "." {
		name = "/"
	}
	return name
}

// Path of the directory - satisfies Node interface
func (d *Dir) Path() (name string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.path
}

// Sys returns underlying data source (can be nil) - satisfies Node interface
func (d *Dir) Sys() interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sys
}

// SetSys sets the underlying data source (can be nil) - satisfies Node interface
func (d *Dir) SetSys(x interface{}) {
	d.mu.Lock()
	d.sys = x
	d.mu.Unlock()
}

// Inode returns the inode number - satisfies Node interface
func (d *Dir) Inode() uint64 {
	return d.inode
}

// Node returns the Node assocuated with this - satisfies Noder interface
func (d *Dir) Node() Node {
	return d
}

// forgetDirPath clears the cache for itself and all subdirectories if
// they match the given path. The path is specified relative from the
// directory it is called from.
//
// It does not invalidate or clear the cache of the parent directory.
func (d *Dir) forgetDirPath(relativePath string) {
	if dir := d.cachedDir(relativePath); dir != nil {
		dir.walk(func(dir *Dir) {
			// this is called with the mutex held
			fs.Debugf(dir.path, "forgetting directory cache")
			dir.read = time.Time{}
			dir.items = make(map[string]Node)
		})
	}
}

// ForgetAll ensures the directory and all its children are purged
// from the cache.
//
// It does not invalidate or clear the cache of the parent directory.
func (d *Dir) ForgetAll() {
	d.forgetDirPath("")
}

// invalidateDir invalidates the directory cache for absPath relative to this dir
func (d *Dir) invalidateDir(absPath string) {
	node := d.vfs.root.cachedNode(absPath)
	if dir, ok := node.(*Dir); ok {
		dir.mu.Lock()
		if !dir.read.IsZero() {
			fs.Debugf(dir.path, "invalidating directory cache")
			dir.read = time.Time{}
		}
		dir.mu.Unlock()
	}
}

// changeNotify invalidates the directory cache for the relativePath
// passed in.
//
// if entryType is a directory it invalidates the parent of the directory too.
func (d *Dir) changeNotify(relativePath string, entryType fs.EntryType) {
	defer log.Trace(d.path, "relativePath=%q, type=%v", relativePath, entryType)("")
	d.mu.RLock()
	absPath := path.Join(d.path, relativePath)
	d.mu.RUnlock()
	d.invalidateDir(vfscommon.FindParent(absPath))
	if entryType == fs.EntryDirectory {
		d.invalidateDir(absPath)
	}
}

// ForgetPath clears the cache for itself and all subdirectories if
// they match the given path. The path is specified relative from the
// directory it is called from. The cache of the parent directory is
// marked as stale, but not cleared otherwise.
// It is not possible to traverse the directory tree upwards, i.e.
// you cannot clear the cache for the Dir's ancestors or siblings.
func (d *Dir) ForgetPath(relativePath string, entryType fs.EntryType) {
	defer log.Trace(d.path, "relativePath=%q, type=%v", relativePath, entryType)("")
	d.mu.RLock()
	absPath := path.Join(d.path, relativePath)
	d.mu.RUnlock()
	if absPath != "" {
		d.invalidateDir(vfscommon.FindParent(absPath))
	}
	if entryType == fs.EntryDirectory {
		d.forgetDirPath(relativePath)
	}
}

// walk runs a function on all cached directories. It will be called
// on a directory's children first.
//
// The mutex will be held for the directory when fun is called
func (d *Dir) walk(fun func(*Dir)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, node := range d.items {
		if dir, ok := node.(*Dir); ok {
			dir.walk(fun)
		}
	}

	fun(d)
}

// countActiveWriters returns the number of writers active in this
// directory and any subdirectories.
func (d *Dir) countActiveWriters() (writers int) {
	d.walk(func(d *Dir) {
		// NB d.mu is held by walk() here
		fs.Debugf(d.path, "Looking for writers")
		for leaf, item := range d.items {
			fs.Debugf(leaf, "reading active writers")
			if file, ok := item.(*File); ok {
				n := file.activeWriters()
				if n != 0 {
					fs.Debugf(file, "active writers %d", n)
				}
				writers += n
			}
		}
	})
	return writers
}

// age returns the duration since the last time the directory contents
// was read and the content is cosidered stale. age will be 0 and
// stale true if the last read time is empty.
// age must be called with d.mu held.
func (d *Dir) _age(when time.Time) (age time.Duration, stale bool) {
	if d.read.IsZero() {
		return age, true
	}
	age = when.Sub(d.read)
	stale = age > d.vfs.Opt.DirCacheTime
	return
}

// rename should be called after the directory is renamed
//
// Reset the directory to new state, discarding all the objects and
// reading everything again
func (d *Dir) rename(newParent *Dir, fsDir fs.Directory) {
	d.ForgetAll()
	d.mu.Lock()
	d.parent = newParent
	d.entry = fsDir
	d.path = fsDir.Remote()
	d.modTime = fsDir.ModTime(context.TODO())
	d.read = time.Time{}
	d.mu.Unlock()
}

// addObject adds a new object or directory to the directory
//
// note that we add new objects rather than updating old ones
func (d *Dir) addObject(node Node) {
	d.mu.Lock()
	d.items[node.Name()] = node
	d.mu.Unlock()
}

// delObject removes an object from the directory
func (d *Dir) delObject(leaf string) {
	d.mu.Lock()
	delete(d.items, leaf)
	d.mu.Unlock()
}

// read the directory and sets d.items - must be called with the lock held
func (d *Dir) _readDir() error {
	when := time.Now()
	if age, stale := d._age(when); stale {
		if age != 0 {
			fs.Debugf(d.path, "Re-reading directory (%v old)", age)
		}
	} else {
		return nil
	}
	entries, err := list.DirSorted(context.TODO(), d.f, false, d.path)
	if err == fs.ErrorDirNotFound {
		// We treat directory not found as empty because we
		// create directories on the fly
	} else if err != nil {
		return err
	}

	err = d._readDirFromEntries(entries, nil, time.Time{})
	if err != nil {
		return err
	}

	d.read = when
	return nil
}

// update d.items for each dir in the DirTree below this one and
// set the last read time - must be called with the lock held
func (d *Dir) _readDirFromDirTree(dirTree dirtree.DirTree, when time.Time) error {
	return d._readDirFromEntries(dirTree[d.path], dirTree, when)
}

// update d.items and if dirTree is not nil update each dir in the DirTree below this one and
// set the last read time - must be called with the lock held
func (d *Dir) _readDirFromEntries(entries fs.DirEntries, dirTree dirtree.DirTree, when time.Time) error {
	var err error
	// Cache the items by name
	found := make(map[string]struct{})
	for _, entry := range entries {
		name := path.Base(entry.Remote())
		if name == "." || name == ".." {
			continue
		}
		node := d.items[name]
		found[name] = struct{}{}
		switch item := entry.(type) {
		case fs.Object:
			obj := item
			// Reuse old file value if it exists
			if file, ok := node.(*File); node != nil && ok {
				file.setObjectNoUpdate(obj)
			} else {
				node = newFile(d, d.path, obj, name)
			}
		case fs.Directory:
			// Reuse old dir value if it exists
			if node == nil || !node.IsDir() {
				node = newDir(d.vfs, d.f, d, item)
			}
			if dirTree != nil {
				dir := node.(*Dir)
				dir.mu.Lock()
				err = dir._readDirFromDirTree(dirTree, when)
				if err != nil {
					dir.read = time.Time{}
				} else {
					dir.read = when
				}
				dir.mu.Unlock()
				if err != nil {
					return err
				}
			}
		default:
			err = errors.Errorf("unknown type %T", item)
			fs.Errorf(d, "readDir error: %v", err)
			return err
		}
		d.items[name] = node
	}
	// delete unused entries
	for name := range d.items {
		if _, ok := found[name]; !ok {
			delete(d.items, name)
		}
	}
	return nil
}

// readDirTree forces a refresh of the complete directory tree
func (d *Dir) readDirTree() error {
	d.mu.RLock()
	f, path := d.f, d.path
	d.mu.RUnlock()
	when := time.Now()
	fs.Debugf(path, "Reading directory tree")
	dt, err := walk.NewDirTree(context.TODO(), f, path, false, -1)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.read = time.Time{}
	err = d._readDirFromDirTree(dt, when)
	if err != nil {
		return err
	}
	fs.Debugf(d.path, "Reading directory tree done in %s", time.Since(when))
	d.read = when
	return nil
}

// readDir forces a refresh of the directory
func (d *Dir) readDir() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.read = time.Time{}
	return d._readDir()
}

// stat a single item in the directory
//
// returns ENOENT if not found.
// returns a custom error if directory on a case-insensitive file system
// contains files with names that differ only by case.
func (d *Dir) stat(leaf string) (Node, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d._readDir()
	if err != nil {
		return nil, err
	}
	item, ok := d.items[leaf]

	if !ok && d.vfs.Opt.CaseInsensitive {
		leafLower := strings.ToLower(leaf)
		for name, node := range d.items {
			if strings.ToLower(name) == leafLower {
				if ok {
					// duplicate case insensitive match is an error
					return nil, errors.Errorf("duplicate filename %q detected with --vfs-case-insensitive set", leaf)
				}
				// found a case insenstive match
				ok = true
				item = node
			}
		}
	}

	if !ok {
		return nil, ENOENT
	}
	return item, nil
}

// Check to see if a directory is empty
func (d *Dir) isEmpty() (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d._readDir()
	if err != nil {
		return false, err
	}
	return len(d.items) == 0, nil
}

// ModTime returns the modification time of the directory
func (d *Dir) ModTime() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	// fs.Debugf(d.path, "Dir.ModTime %v", d.modTime)
	return d.modTime
}

// Size of the directory
func (d *Dir) Size() int64 {
	return 0
}

// SetModTime sets the modTime for this dir
func (d *Dir) SetModTime(modTime time.Time) error {
	if d.vfs.Opt.ReadOnly {
		return EROFS
	}
	d.mu.Lock()
	d.modTime = modTime
	d.mu.Unlock()
	return nil
}

func (d *Dir) cachedDir(relativePath string) (dir *Dir) {
	dir, _ = d.cachedNode(relativePath).(*Dir)
	return
}

func (d *Dir) cachedNode(relativePath string) Node {
	segments := strings.Split(strings.Trim(relativePath, "/"), "/")
	var node Node = d
	for _, s := range segments {
		if s == "" {
			continue
		}
		if dir, ok := node.(*Dir); ok {
			dir.mu.Lock()
			node = dir.items[s]
			dir.mu.Unlock()

			if node != nil {
				continue
			}
		}
		return nil
	}

	return node
}

// Stat looks up a specific entry in the receiver.
//
// Stat should return a Node corresponding to the entry.  If the
// name does not exist in the directory, Stat should return ENOENT.
//
// Stat need not to handle the names "." and "..".
func (d *Dir) Stat(name string) (node Node, err error) {
	// fs.Debugf(path, "Dir.Stat")
	node, err = d.stat(name)
	if err != nil {
		if err != ENOENT {
			fs.Errorf(d, "Dir.Stat error: %v", err)
		}
		return nil, err
	}
	// fs.Debugf(path, "Dir.Stat OK")
	return node, nil
}

// ReadDirAll reads the contents of the directory sorted
func (d *Dir) ReadDirAll() (items Nodes, err error) {
	// fs.Debugf(d.path, "Dir.ReadDirAll")
	d.mu.Lock()
	err = d._readDir()
	if err != nil {
		fs.Debugf(d.path, "Dir.ReadDirAll error: %v", err)
		d.mu.Unlock()
		return nil, err
	}
	for _, item := range d.items {
		items = append(items, item)
	}
	d.mu.Unlock()
	sort.Sort(items)
	// fs.Debugf(d.path, "Dir.ReadDirAll OK with %d entries", len(items))
	return items, nil
}

// accessModeMask masks off the read modes from the flags
const accessModeMask = (os.O_RDONLY | os.O_WRONLY | os.O_RDWR)

// Open the directory according to the flags provided
func (d *Dir) Open(flags int) (fd Handle, err error) {
	rdwrMode := flags & accessModeMask
	if rdwrMode != os.O_RDONLY {
		fs.Errorf(d, "Can only open directories read only")
		return nil, EPERM
	}
	return newDirHandle(d), nil
}

// Create makes a new file node
func (d *Dir) Create(name string, flags int) (*File, error) {
	// fs.Debugf(path, "Dir.Create")
	if d.vfs.Opt.ReadOnly {
		return nil, EROFS
	}
	// This gets added to the directory when the file is opened for write
	return newFile(d, d.Path(), nil, name), nil
}

// Mkdir creates a new directory
func (d *Dir) Mkdir(name string) (*Dir, error) {
	if d.vfs.Opt.ReadOnly {
		return nil, EROFS
	}
	path := path.Join(d.path, name)
	node, err := d.stat(name)
	switch err {
	case ENOENT:
		// not found, carry on
	case nil:
		// found so check what it is
		if node.IsDir() {
			return node.(*Dir), err
		}
		return nil, EEXIST
	default:
		// a different error - report
		fs.Errorf(d, "Dir.Mkdir failed to read directory: %v", err)
		return nil, err
	}
	// fs.Debugf(path, "Dir.Mkdir")
	err = d.f.Mkdir(context.TODO(), path)
	if err != nil {
		fs.Errorf(d, "Dir.Mkdir failed to create directory: %v", err)
		return nil, err
	}
	fsDir := fs.NewDir(path, time.Now())
	dir := newDir(d.vfs, d.f, d, fsDir)
	d.addObject(dir)
	// fs.Debugf(path, "Dir.Mkdir OK")
	return dir, nil
}

// Remove the directory
func (d *Dir) Remove() error {
	if d.vfs.Opt.ReadOnly {
		return EROFS
	}
	// Check directory is empty first
	empty, err := d.isEmpty()
	if err != nil {
		fs.Errorf(d, "Dir.Remove dir error: %v", err)
		return err
	}
	if !empty {
		fs.Errorf(d, "Dir.Remove not empty")
		return ENOTEMPTY
	}
	// remove directory
	err = d.f.Rmdir(context.TODO(), d.path)
	if err != nil {
		fs.Errorf(d, "Dir.Remove failed to remove directory: %v", err)
		return err
	}
	// Remove the item from the parent directory listing
	if d.parent != nil {
		d.parent.delObject(d.Name())
	}
	return nil
}

// RemoveAll removes the directory and any contents recursively
func (d *Dir) RemoveAll() error {
	if d.vfs.Opt.ReadOnly {
		return EROFS
	}
	// Remove contents of the directory
	nodes, err := d.ReadDirAll()
	if err != nil {
		fs.Errorf(d, "Dir.RemoveAll failed to read directory: %v", err)
		return err
	}
	for _, node := range nodes {
		err = node.RemoveAll()
		if err != nil {
			fs.Errorf(node.Path(), "Dir.RemoveAll failed to remove: %v", err)
			return err
		}
	}
	return d.Remove()
}

// DirEntry returns the underlying fs.DirEntry
func (d *Dir) DirEntry() (entry fs.DirEntry) {
	return d.entry
}

// RemoveName removes the entry with the given name from the receiver,
// which must be a directory.  The entry to be removed may correspond
// to a file (unlink) or to a directory (rmdir).
func (d *Dir) RemoveName(name string) error {
	if d.vfs.Opt.ReadOnly {
		return EROFS
	}
	// fs.Debugf(path, "Dir.Remove")
	node, err := d.stat(name)
	if err != nil {
		fs.Errorf(d, "Dir.Remove error: %v", err)
		return err
	}
	return node.Remove()
}

// Rename the file
func (d *Dir) Rename(oldName, newName string, destDir *Dir) error {
	if d.vfs.Opt.ReadOnly {
		return EROFS
	}
	oldPath := path.Join(d.path, oldName)
	newPath := path.Join(destDir.path, newName)
	// fs.Debugf(oldPath, "Dir.Rename to %q", newPath)
	oldNode, err := d.stat(oldName)
	if err != nil {
		fs.Errorf(oldPath, "Dir.Rename error: %v", err)
		return err
	}
	switch x := oldNode.DirEntry().(type) {
	case nil:
		if oldFile, ok := oldNode.(*File); ok {
			if err = oldFile.rename(context.TODO(), destDir, newName); err != nil {
				fs.Errorf(oldPath, "Dir.Rename error: %v", err)
				return err
			}
		} else {
			fs.Errorf(oldPath, "Dir.Rename can't rename open file that is not a vfs.File")
			return EPERM
		}
	case fs.Object:
		if oldFile, ok := oldNode.(*File); ok {
			if err = oldFile.rename(context.TODO(), destDir, newName); err != nil {
				fs.Errorf(oldPath, "Dir.Rename error: %v", err)
				return err
			}
		} else {
			err := errors.Errorf("Fs %q can't rename file that is not a vfs.File", d.f)
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
	case fs.Directory:
		features := d.f.Features()
		if features.DirMove == nil && features.Move == nil && features.Copy == nil {
			err := errors.Errorf("Fs %q can't rename directories (no DirMove, Move or Copy)", d.f)
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		srcRemote := x.Remote()
		dstRemote := newPath
		err = operations.DirMove(context.TODO(), d.f, srcRemote, dstRemote)
		if err != nil {
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		newDir := fs.NewDirCopy(context.TODO(), x).SetRemote(newPath)
		// Update the node with the new details
		if oldNode != nil {
			if oldDir, ok := oldNode.(*Dir); ok {
				fs.Debugf(x, "Updating dir with %v %p", newDir, oldDir)
				oldDir.rename(destDir, newDir)
			}
		}
	default:
		err = errors.Errorf("unknown type %T", oldNode)
		fs.Errorf(d.path, "Dir.Rename error: %v", err)
		return err
	}

	// Show moved - delete from old dir and add to new
	d.delObject(oldName)
	destDir.addObject(oldNode)

	// fs.Debugf(newPath, "Dir.Rename renamed from %q", oldPath)
	return nil
}

// Sync the directory
//
// Note that we don't do anything except return OK
func (d *Dir) Sync() error {
	return nil
}

// VFS returns the instance of the VFS
func (d *Dir) VFS() *VFS {
	// No locking required
	return d.vfs
}

// Fs returns the Fs that the Dir is on
func (d *Dir) Fs() fs.Fs {
	// No locking required
	return d.f
}

// Truncate changes the size of the named file.
func (d *Dir) Truncate(size int64) error {
	return ENOSYS
}
