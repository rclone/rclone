package mountlib

import (
	"path"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

var dirCacheTime = 60 * time.Second // FIXME needs to be settable

// DirEntry describes the contents of a directory entry
//
// It can be a file or a directory
//
// node may be nil, but o may not
type DirEntry struct {
	Obj  fs.BasicInfo
	Node Node
}

// Dir represents a directory entry
type Dir struct {
	fsys    *FS
	inode   uint64 // inode number
	f       fs.Fs
	path    string
	modTime time.Time
	mu      sync.RWMutex // protects the following
	read    time.Time    // time directory entry last read
	items   map[string]*DirEntry
}

func newDir(fsys *FS, f fs.Fs, fsDir *fs.Dir) *Dir {
	return &Dir{
		fsys:    fsys,
		f:       f,
		path:    fsDir.Name,
		modTime: fsDir.When,
		inode:   NewInode(),
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

// walk runs a function on all directories whose path matches
// the given absolute one. It will be called on a directory's
// children first. It will not apply the function to parent
// nodes, regardless of the given path.
func (d *Dir) walk(absPath string, fun func(*Dir)) {
	if d.items != nil {
		for _, entry := range d.items {
			if dir, ok := entry.Node.(*Dir); ok {
				dir.walk(absPath, fun)
			}
		}
	}

	if d.path == absPath || absPath == "" || strings.HasPrefix(d.path, absPath+"/") {
		d.mu.Lock()
		defer d.mu.Unlock()
		fun(d)
	}
}

// rename should be called after the directory is renamed
//
// Reset the directory to new state, discarding all the objects and
// reading everything again
func (d *Dir) rename(newParent *Dir, fsDir *fs.Dir) {
	d.ForgetAll()
	d.path = fsDir.Name
	d.modTime = fsDir.When
	d.read = time.Time{}
}

// addObject adds a new object or directory to the directory
//
// note that we add new objects rather than updating old ones
func (d *Dir) addObject(o fs.BasicInfo, node Node) *DirEntry {
	item := &DirEntry{
		Obj:  o,
		Node: node,
	}
	d.mu.Lock()
	d.items[path.Base(o.Remote())] = item
	d.mu.Unlock()
	return item
}

// delObject removes an object from the directory
func (d *Dir) delObject(leaf string) {
	d.mu.Lock()
	delete(d.items, leaf)
	d.mu.Unlock()
}

// read the directory
func (d *Dir) readDir() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	when := time.Now()
	if d.read.IsZero() {
		fs.Debugf(d.path, "Reading directory")
	} else {
		age := when.Sub(d.read)
		if age < dirCacheTime {
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
	d.items = make(map[string]*DirEntry, len(entries))
	for _, entry := range entries {
		switch item := entry.(type) {
		case fs.Object:
			obj := item
			name := path.Base(obj.Remote())
			d.items[name] = &DirEntry{
				Obj:  obj,
				Node: nil,
			}
		case *fs.Dir:
			dir := item
			name := path.Base(dir.Remote())
			// Use old dir value if it exists
			if oldItem, ok := oldItems[name]; ok {
				if _, ok := oldItem.Obj.(*fs.Dir); ok {
					d.items[name] = oldItem
					continue
				}
			}
			d.items[name] = &DirEntry{
				Obj:  dir,
				Node: nil,
			}
		default:
			err = errors.Errorf("unknown type %T", item)
			fs.Errorf(d.path, "readDir error: %v", err)
			return err
		}
	}
	d.read = when
	return nil
}

// lookup a single item in the directory
//
// returns ENOENT if not found.
func (d *Dir) lookup(leaf string) (*DirEntry, error) {
	err := d.readDir()
	if err != nil {
		return nil, err
	}
	d.mu.RLock()
	item, ok := d.items[leaf]
	d.mu.RUnlock()
	if !ok {
		return nil, ENOENT
	}
	return item, nil
}

// Check to see if a directory is empty
func (d *Dir) isEmpty() (bool, error) {
	err := d.readDir()
	if err != nil {
		return false, err
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.items) == 0, nil
}

// ModTime returns the modification time of the directory
func (d *Dir) ModTime() time.Time {
	fs.Debugf(d.path, "Dir.ModTime %v", d.modTime)
	return d.modTime
}

// SetModTime sets the modTime for this dir
func (d *Dir) SetModTime(modTime time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.modTime = modTime
	return nil
}

// lookupNode calls lookup then makes sure the node is not nil in the DirEntry
func (d *Dir) lookupNode(leaf string) (item *DirEntry, err error) {
	item, err = d.lookup(leaf)
	if err != nil {
		return nil, err
	}
	if item.Node != nil {
		return item, nil
	}
	var node Node
	switch x := item.Obj.(type) {
	case fs.Object:
		node, err = newFile(d, x, leaf), nil
	case *fs.Dir:
		node, err = newDir(d.fsys, d.f, x), nil
	default:
		err = errors.Errorf("unknown type %T", item)
	}
	if err != nil {
		return nil, err
	}
	item = d.addObject(item.Obj, node)
	return item, nil
}

// Lookup looks up a specific entry in the receiver.
//
// Lookup should return a Node corresponding to the entry.  If the
// name does not exist in the directory, Lookup should return ENOENT.
//
// Lookup need not to handle the names "." and "..".
func (d *Dir) Lookup(name string) (node Node, err error) {
	path := path.Join(d.path, name)
	fs.Debugf(path, "Dir.Lookup")
	item, err := d.lookupNode(name)
	if err != nil {
		if err != ENOENT {
			fs.Errorf(path, "Dir.Lookup error: %v", err)
		}
		return nil, err
	}
	fs.Debugf(path, "Dir.Lookup OK")
	return item.Node, nil
}

// ReadDirAll reads the contents of the directory
func (d *Dir) ReadDirAll() (items []*DirEntry, err error) {
	fs.Debugf(d.path, "Dir.ReadDirAll")
	err = d.readDir()
	if err != nil {
		fs.Debugf(d.path, "Dir.ReadDirAll error: %v", err)
		return nil, err
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, item := range d.items {
		items = append(items, item)
	}
	fs.Debugf(d.path, "Dir.ReadDirAll OK with %d entries", len(items))
	return items, nil
}

// Create makes a new file
func (d *Dir) Create(name string) (*File, *WriteFileHandle, error) {
	path := path.Join(d.path, name)
	fs.Debugf(path, "Dir.Create")
	src := newCreateInfo(d.f, path)
	// This gets added to the directory when the file is written
	file := newFile(d, nil, name)
	fh, err := newWriteFileHandle(d, file, src)
	if err != nil {
		fs.Errorf(path, "Dir.Create error: %v", err)
		return nil, nil, err
	}
	fs.Debugf(path, "Dir.Create OK")
	return file, fh, nil
}

// Mkdir creates a new directory
func (d *Dir) Mkdir(name string) (*Dir, error) {
	path := path.Join(d.path, name)
	fs.Debugf(path, "Dir.Mkdir")
	err := d.f.Mkdir(path)
	if err != nil {
		fs.Errorf(path, "Dir.Mkdir failed to create directory: %v", err)
		return nil, err
	}
	fsDir := &fs.Dir{
		Name: path,
		When: time.Now(),
	}
	dir := newDir(d.fsys, d.f, fsDir)
	d.addObject(fsDir, dir)
	fs.Debugf(path, "Dir.Mkdir OK")
	return dir, nil
}

// Remove removes the entry with the given name from
// the receiver, which must be a directory.  The entry to be removed
// may correspond to a file (unlink) or to a directory (rmdir).
func (d *Dir) Remove(name string) error {
	path := path.Join(d.path, name)
	fs.Debugf(path, "Dir.Remove")
	item, err := d.lookupNode(name)
	if err != nil {
		fs.Errorf(path, "Dir.Remove error: %v", err)
		return err
	}
	switch x := item.Obj.(type) {
	case fs.Object:
		err = x.Remove()
		if err != nil {
			fs.Errorf(path, "Dir.Remove file error: %v", err)
			return err
		}
	case *fs.Dir:
		// Check directory is empty first
		dir := item.Node.(*Dir)
		empty, err := dir.isEmpty()
		if err != nil {
			fs.Errorf(path, "Dir.Remove dir error: %v", err)
			return err
		}
		if !empty {
			fs.Errorf(path, "Dir.Remove not empty")
			return ENOTEMPTY
		}
		// remove directory
		err = d.f.Rmdir(path)
		if err != nil {
			fs.Errorf(path, "Dir.Remove failed to remove directory: %v", err)
			return err
		}
	default:
		fs.Errorf(path, "Dir.Remove unknown type %T", item)
		return errors.Errorf("unknown type %T", item)
	}
	// Remove the item from the directory listing
	d.delObject(name)
	fs.Debugf(path, "Dir.Remove OK")
	return nil
}

// Rename the file
func (d *Dir) Rename(oldName, newName string, destDir *Dir) error {
	oldPath := path.Join(d.path, oldName)
	newPath := path.Join(destDir.path, newName)
	fs.Debugf(oldPath, "Dir.Rename to %q", newPath)
	oldItem, err := d.lookupNode(oldName)
	if err != nil {
		fs.Errorf(oldPath, "Dir.Rename error: %v", err)
		return err
	}
	var newObj fs.BasicInfo
	oldNode := oldItem.Node
	switch x := oldItem.Obj.(type) {
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
		newObj = newObject
		// Update the node with the new details
		if oldNode != nil {
			if oldFile, ok := oldNode.(*File); ok {
				fs.Debugf(oldItem.Obj, "Updating file with %v %p", newObject, oldFile)
				oldFile.rename(destDir, newObject)
			}
		}
	case *fs.Dir:
		doDirMove := d.f.Features().DirMove
		if doDirMove == nil {
			err := errors.Errorf("Fs %q can't rename directories (no DirMove)", d.f)
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		srcRemote := x.Name
		dstRemote := newPath
		err = doDirMove(d.f, srcRemote, dstRemote)
		if err != nil {
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		newDir := new(fs.Dir)
		*newDir = *x
		newDir.Name = newPath
		newObj = newDir
		// Update the node with the new details
		if oldNode != nil {
			if oldDir, ok := oldNode.(*Dir); ok {
				fs.Debugf(oldItem.Obj, "Updating dir with %v %p", newDir, oldDir)
				oldDir.rename(destDir, newDir)
			}
		}
	default:
		err = errors.Errorf("unknown type %T", oldItem)
		fs.Errorf(d.path, "Dir.ReadDirAll error: %v", err)
		return err
	}

	// Show moved - delete from old dir and add to new
	d.delObject(oldName)
	destDir.addObject(newObj, oldNode)

	fs.Debugf(newPath, "Dir.Rename renamed from %q", oldPath)
	return nil
}

// Fsync the directory
//
// Note that we don't do anything except return OK
func (d *Dir) Fsync() error {
	return nil
}
