package vfs

import (
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// Dir represents a directory entry
type Dir struct {
	vfs     *VFS
	inode   uint64 // inode number
	f       fs.Fs
	parent  *Dir // parent, nil for root
	path    string
	modTime time.Time
	entry   fs.Directory
	mu      sync.Mutex      // protects the following
	read    time.Time       // time directory entry last read
	items   map[string]Node // NB can be nil when directory not read yet
}

func newDir(vfs *VFS, f fs.Fs, parent *Dir, fsDir fs.Directory) *Dir {
	return &Dir{
		vfs:     vfs,
		f:       f,
		parent:  parent,
		entry:   fsDir,
		path:    fsDir.Remote(),
		modTime: fsDir.ModTime(),
		inode:   newInode(),
	}
}

// String converts it to printablee
func (d *Dir) String() string {
	if d == nil {
		return "<nil *Dir>"
	}
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
	name = path.Base(d.path)
	if name == "." {
		name = "/"
	}
	return name
}

// Sys returns underlying data source (can be nil) - satisfies Node interface
func (d *Dir) Sys() interface{} {
	return nil
}

// Inode returns the inode number - satisfies Node interface
func (d *Dir) Inode() uint64 {
	return d.inode
}

// Node returns the Node assocuated with this - satisfies Noder interface
func (d *Dir) Node() Node {
	return d
}

// ForgetAll ensures the directory and all its children are purged
// from the cache.
func (d *Dir) ForgetAll() {
	d.ForgetPath("")
}

// ForgetPath clears the cache for itself and all subdirectories if
// they match the given path. The path is specified relative from the
// directory it is called from.
// It is not possible to traverse the directory tree upwards, i.e.
// you cannot clear the cache for the Dir's ancestors or siblings.
func (d *Dir) ForgetPath(relativePath string) {
	absPath := path.Join(d.path, relativePath)
	if absPath == "." {
		absPath = ""
	}

	d.walk(absPath, func(dir *Dir) {
		fs.Debugf(dir.path, "forgetting directory cache")
		dir.read = time.Time{}
		dir.items = nil
	})
}

// walk runs a function on all cached directories whose path matches
// the given absolute one. It will be called on a directory's children
// first. It will not apply the function to parent nodes, regardless
// of the given path.
func (d *Dir) walk(absPath string, fun func(*Dir)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.items != nil {
		for _, node := range d.items {
			if dir, ok := node.(*Dir); ok {
				dir.walk(absPath, fun)
			}
		}
	}

	if d.path == absPath || absPath == "" || strings.HasPrefix(d.path, absPath+"/") {
		fun(d)
	}
}

// rename should be called after the directory is renamed
//
// Reset the directory to new state, discarding all the objects and
// reading everything again
func (d *Dir) rename(newParent *Dir, fsDir fs.Directory) {
	d.ForgetAll()
	d.parent = newParent
	d.entry = fsDir
	d.path = fsDir.Remote()
	d.modTime = fsDir.ModTime()
	d.read = time.Time{}
}

// addObject adds a new object or directory to the directory
//
// note that we add new objects rather than updating old ones
func (d *Dir) addObject(node Node) {
	d.mu.Lock()
	if d.items != nil {
		d.items[node.Name()] = node
	}
	d.mu.Unlock()
}

// delObject removes an object from the directory
func (d *Dir) delObject(leaf string) {
	d.mu.Lock()
	if d.items != nil {
		delete(d.items, leaf)
	}
	d.mu.Unlock()
}

// read the directory and sets d.items - must be called with the lock held
func (d *Dir) _readDir() error {
	when := time.Now()
	if d.read.IsZero() || d.items == nil {
		// fs.Debugf(d.path, "Reading directory")
	} else {
		age := when.Sub(d.read)
		if age < d.vfs.Opt.DirCacheTime {
			return nil
		}
		fs.Debugf(d.path, "Re-reading directory (%v old)", age)
	}
	entries, err := fs.ListDirSorted(d.f, false, d.path)
	if err == fs.ErrorDirNotFound {
		// We treat directory not found as empty because we
		// create directories on the fly
	} else if err != nil {
		return err
	}
	// NB when we re-read a directory after its cache has expired
	// we drop the old files which should lead to correct
	// behaviour but may not be very efficient.

	// Keep a note of the previous contents of the directory
	oldItems := d.items

	// Cache the items by name
	d.items = make(map[string]Node, len(entries))
	for _, entry := range entries {
		switch item := entry.(type) {
		case fs.Object:
			obj := item
			name := path.Base(obj.Remote())
			d.items[name] = newFile(d, obj, name)
		case fs.Directory:
			dir := item
			name := path.Base(dir.Remote())
			// Use old dir value if it exists
			if oldItems != nil {
				if oldNode, ok := oldItems[name]; ok {
					if oldNode.IsDir() {
						d.items[name] = oldNode
						continue
					}
				}
			}
			d.items[name] = newDir(d.vfs, d.f, d, dir)
		default:
			err = errors.Errorf("unknown type %T", item)
			fs.Errorf(d, "readDir error: %v", err)
			return err
		}
	}
	d.read = when
	return nil
}

// stat a single item in the directory
//
// returns ENOENT if not found.
func (d *Dir) stat(leaf string) (Node, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d._readDir()
	if err != nil {
		return nil, err
	}
	item, ok := d.items[leaf]
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
	defer d.mu.Unlock()
	d.modTime = modTime
	return nil
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
	defer d.mu.Unlock()
	err = d._readDir()
	if err != nil {
		fs.Debugf(d.path, "Dir.ReadDirAll error: %v", err)
		return nil, err
	}
	for _, item := range d.items {
		items = append(items, item)
	}
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
func (d *Dir) Create(name string) (*File, error) {
	// fs.Debugf(path, "Dir.Create")
	if d.vfs.Opt.ReadOnly {
		return nil, EROFS
	}
	// This gets added to the directory when the file is written
	return newFile(d, nil, name), nil
}

// Mkdir creates a new directory
func (d *Dir) Mkdir(name string) (*Dir, error) {
	if d.vfs.Opt.ReadOnly {
		return nil, EROFS
	}
	path := path.Join(d.path, name)
	// fs.Debugf(path, "Dir.Mkdir")
	err := d.f.Mkdir(path)
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
	err = d.f.Rmdir(d.path)
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
			fs.Errorf(node.DirEntry(), "Dir.RemoveAll failed to remove: %v", err)
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
	case fs.Object:
		oldObject := x
		// FIXME: could Copy then Delete if Move not available
		// - though care needed if case insensitive...
		doMove := d.f.Features().Move
		if doMove == nil {
			err := errors.Errorf("Fs %q can't rename files (no Move)", d.f)
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		newObject, err := doMove(oldObject, newPath)
		if err != nil {
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		// Update the node with the new details
		if oldNode != nil {
			if oldFile, ok := oldNode.(*File); ok {
				fs.Debugf(oldNode.DirEntry(), "Updating file with %v %p", newObject, oldFile)
				oldFile.rename(destDir, newObject)
			}
		}
	case fs.Directory:
		doDirMove := d.f.Features().DirMove
		if doDirMove == nil {
			err := errors.Errorf("Fs %q can't rename directories (no DirMove)", d.f)
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		srcRemote := x.Remote()
		dstRemote := newPath
		err = doDirMove(d.f, srcRemote, dstRemote)
		if err != nil {
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		newDir := fs.NewDirCopy(x).SetRemote(newPath)
		// Update the node with the new details
		if oldNode != nil {
			if oldDir, ok := oldNode.(*Dir); ok {
				fs.Debugf(oldNode.DirEntry(), "Updating dir with %v %p", newDir, oldDir)
				oldDir.rename(destDir, newDir)
			}
		}
	default:
		err = errors.Errorf("unknown type %T", oldNode)
		fs.Errorf(d.path, "Dir.ReadDirAll error: %v", err)
		return err
	}

	// Show moved - delete from old dir and add to new
	d.delObject(oldName)
	destDir.addObject(oldNode)

	// fs.Debugf(newPath, "Dir.Rename renamed from %q", oldPath)
	return nil
}

// Fsync the directory
//
// Note that we don't do anything except return OK
func (d *Dir) Fsync() error {
	return nil
}

// VFS returns the instance of the VFS
func (d *Dir) VFS() *VFS {
	return d.vfs
}

// Truncate changes the size of the named file.
func (d *Dir) Truncate(size int64) error {
	return ENOSYS
}
