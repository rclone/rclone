package vfs

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	entry   fs.Directory
	read    time.Time         // time directory entry last read
	items   map[string]Node   // directory entries - can be empty but not nil
	virtual map[string]vState // virtual directory entries - may be nil
	sys     atomic.Value      // user defined info to be attached here

	modTimeMu sync.Mutex // protects the following
	modTime   time.Time
}

//go:generate stringer -type=vState

// vState describes the state of the virtual directory entries
type vState byte

const (
	vOK      vState = iota // Not virtual
	vAddFile               // added file
	vAddDir                // added directory
	vDel                   // removed file or directory
)

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

// String converts it to printable
func (d *Dir) String() string {
	if d == nil {
		return "<nil *Dir>"
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.path + "/"
}

// Dumps the directory tree to the string builder with the given indent
func (d *Dir) dumpIndent(out *strings.Builder, indent string) {
	if d == nil {
		fmt.Fprintf(out, "%s<nil *Dir>\n", indent)
		return
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	fmt.Fprintf(out, "%sPath: %s\n", indent, d.path)
	fmt.Fprintf(out, "%sEntry: %v\n", indent, d.entry)
	fmt.Fprintf(out, "%sRead: %v\n", indent, d.read)
	fmt.Fprintf(out, "%s- items %d\n", indent, len(d.items))
	// Sort?
	for leaf, node := range d.items {
		switch x := node.(type) {
		case *Dir:
			fmt.Fprintf(out, "%s  %s/ - %v\n", indent, leaf, x)
			// check the parent is correct
			if x.parent != d {
				fmt.Fprintf(out, "%s  PARENT POINTER WRONG\n", indent)
			}
			x.dumpIndent(out, indent+"\t")
		case *File:
			fmt.Fprintf(out, "%s  %s - %v\n", indent, leaf, x)
		default:
			panic("bad dir entry")
		}
	}
	fmt.Fprintf(out, "%s- virtual %d\n", indent, len(d.virtual))
	for leaf, state := range d.virtual {
		fmt.Fprintf(out, "%s  %s - %v\n", indent, leaf, state)
	}
}

// Dumps a nicely formatted directory tree to a string
func (d *Dir) dump() string {
	var out strings.Builder
	d.dumpIndent(&out, "")
	return out.String()
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
	return d.sys.Load()
}

// SetSys sets the underlying data source (can be nil) - satisfies Node interface
func (d *Dir) SetSys(x interface{}) {
	d.sys.Store(x)
}

// Inode returns the inode number - satisfies Node interface
func (d *Dir) Inode() uint64 {
	return d.inode
}

// Node returns the Node associated with this - satisfies Noder interface
func (d *Dir) Node() Node {
	return d
}

// ForgetAll forgets directory entries for this directory and any children.
//
// It does not invalidate or clear the cache of the parent directory.
//
// It returns true if the directory or any of its children had virtual entries
// so could not be forgotten. Children which didn't have virtual entries and
// children with virtual entries will be forgotten even if true is returned.
func (d *Dir) ForgetAll() (hasVirtual bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fs.Debugf(d.path, "forgetting directory cache")
	for _, node := range d.items {
		if dir, ok := node.(*Dir); ok {
			if dir.ForgetAll() {
				hasVirtual = true
			}
		}
	}
	// Purge any unecessary virtual entries
	d._purgeVirtual()

	d.read = time.Time{}
	// Check if this dir has virtual entries
	if len(d.virtual) != 0 {
		hasVirtual = true
	}
	// Don't clear directory entries if there are virtual entries in this
	// directory or any children
	if !hasVirtual {
		d.items = make(map[string]Node)
	}
	return hasVirtual
}

// forgetDirPath clears the cache for itself and all subdirectories if
// they match the given path. The path is specified relative from the
// directory it is called from.
//
// It does not invalidate or clear the cache of the parent directory.
func (d *Dir) forgetDirPath(relativePath string) {
	dir := d.cachedDir(relativePath)
	if dir == nil {
		return
	}
	dir.ForgetAll()
}

// invalidateDir invalidates the directory cache for absPath relative to the root
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
// was read and the content is considered stale. age will be 0 and
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

// renameTree renames the directories under this directory
//
// path should be the desired path
func (d *Dir) renameTree(dirPath string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Make sure the path is correct for each node
	if d.path != dirPath {
		fs.Debugf(d.path, "Renaming to %q", dirPath)
		d.path = dirPath
		d.entry = fs.NewDirCopy(context.TODO(), d.entry).SetRemote(dirPath)
	}

	// Do the same to any child directories
	for leaf, node := range d.items {
		if dir, ok := node.(*Dir); ok {
			dir.renameTree(path.Join(dirPath, leaf))
		}
	}
}

// rename should be called after the directory is renamed
//
// Reset the directory to new state, discarding all the objects and
// reading everything again
func (d *Dir) rename(newParent *Dir, fsDir fs.Directory) {
	d.ForgetAll()

	d.modTimeMu.Lock()
	d.modTime = fsDir.ModTime(context.TODO())
	d.modTimeMu.Unlock()
	d.mu.Lock()
	oldPath := d.path
	d.parent = newParent
	d.entry = fsDir
	d.path = fsDir.Remote()
	newPath := d.path
	d.read = time.Time{}
	d.mu.Unlock()

	// Rename any remaining items in the tree that we couldn't forget
	d.renameTree(d.path)

	// Rename in the cache
	if d.vfs.cache != nil && d.vfs.cache.DirExists(oldPath) {
		if err := d.vfs.cache.DirRename(oldPath, newPath); err != nil {
			fs.Infof(d, "Dir.Rename failed in Cache: %v", err)
		}
	}
}

// addObject adds a new object or directory to the directory
//
// The name passed in is marked as virtual as it hasn't been read from a remote
// directory listing.
//
// note that we add new objects rather than updating old ones
func (d *Dir) addObject(node Node) {
	d.mu.Lock()
	leaf := node.Name()
	d.items[leaf] = node
	if d.virtual == nil {
		d.virtual = make(map[string]vState)
	}
	vAdd := vAddFile
	if node.IsDir() {
		vAdd = vAddDir
	}
	d.virtual[leaf] = vAdd
	fs.Debugf(d.path, "Added virtual directory entry %v: %q", vAdd, leaf)
	d.mu.Unlock()
}

// AddVirtual adds a virtual object of name and size to the directory
//
// This will be replaced with a real object when it is read back from the
// remote.
//
// This is used to add directory entries while things are uploading
func (d *Dir) AddVirtual(leaf string, size int64, isDir bool) {
	var node Node
	d.mu.RLock()
	dPath := d.path
	_, found := d.items[leaf]
	d.mu.RUnlock()
	if found {
		// Don't overwrite existing objects
		return
	}
	if isDir {
		remote := path.Join(dPath, leaf)
		entry := fs.NewDir(remote, time.Now())
		node = newDir(d.vfs, d.f, d, entry)
	} else {
		f := newFile(d, dPath, nil, leaf)
		f.setSize(size)
		node = f
	}
	d.addObject(node)

}

// delObject removes an object from the directory
//
// The name passed in is marked as virtual as the delete it hasn't been read
// from a remote directory listing.
func (d *Dir) delObject(leaf string) {
	d.mu.Lock()
	delete(d.items, leaf)
	if d.virtual == nil {
		d.virtual = make(map[string]vState)
	}
	d.virtual[leaf] = vDel
	fs.Debugf(d.path, "Added virtual directory entry %v: %q", vDel, leaf)
	d.mu.Unlock()
}

// DelVirtual removes an object from the directory listing
//
// It marks it as removed until it has confirmed the object is missing when the
// directory entries are re-read in.
//
// This is used to remove directory entries after things have been deleted or
// renamed but before we've had confirmation from the backend.
func (d *Dir) DelVirtual(leaf string) {
	d.delObject(leaf)
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

// Remove the virtual directory entry leaf
func (d *Dir) _deleteVirtual(name string) {
	virtualState, ok := d.virtual[name]
	if !ok {
		return
	}
	delete(d.virtual, name)
	if len(d.virtual) == 0 {
		d.virtual = nil
	}
	fs.Debugf(d.path, "Removed virtual directory entry %v: %q", virtualState, name)
}

// Purge virtual entries assuming the directory has just been re-read
//
// Remove all the entries except:
//
// 1) vDirAdd on remotes which can't have empty directories. These will remain
// virtual as long as the directory is empty. When the directory becomes real
// (ie files are added) the virtual directory will be removed. This means that
// directories will disappear when the last file is deleted which is probably
// OK.
//
// 2) vFileAdd that are being written or uploaded
func (d *Dir) _purgeVirtual() {
	canHaveEmptyDirectories := d.f.Features().CanHaveEmptyDirectories
	for name, virtualState := range d.virtual {
		switch virtualState {
		case vAddDir:
			if canHaveEmptyDirectories {
				// if remote can have empty directories then a
				// new dir will be read in the listing
				d._deleteVirtual(name)
				//} else {
				// leave the empty directory marker
			}
		case vAddFile:
			// Delete all virtual file adds that have finished uploading
			node, ok := d.items[name]
			if !ok {
				// if the object has disappeared somehow then remove the virtual
				d._deleteVirtual(name)
				continue
			}
			f, ok := node.(*File)
			if !ok {
				// if the object isn't a file then remove the virtual as it is wrong
				d._deleteVirtual(name)
				continue
			}
			if f.writingInProgress() {
				// if writing in progress then leave virtual
				continue
			}
			if d.vfs.Opt.CacheMode >= vfscommon.CacheModeMinimal && d.vfs.cache.InUse(f.Path()) {
				// if object in use or dirty then leave virtual
				continue
			}
			d._deleteVirtual(name)
		default:
			d._deleteVirtual(name)
		}
	}
}

// Manage the virtuals in a listing
//
// This keeps a record of the names listed in this directory so far
type manageVirtuals map[string]struct{}

// Create a new manageVirtuals and purge the d.virtuals of any entries which can
// be removed.
//
// must be called with the Dir lock held
func (d *Dir) _newManageVirtuals() manageVirtuals {
	tv := make(manageVirtuals)
	d._purgeVirtual()
	return tv
}

// This should be called for every entry added to the directory
//
// It returns true if this entry should be skipped
//
// must be called with the Dir lock held
func (mv manageVirtuals) add(d *Dir, name string) bool {
	// Keep a record of all names listed
	mv[name] = struct{}{}
	// Remove virtuals if possible
	switch d.virtual[name] {
	case vAddFile, vAddDir:
		// item was added to the dir but since it is found in a
		// listing is no longer virtual
		d._deleteVirtual(name)
	case vDel:
		// item is deleted from the dir so skip it
		return true
	case vOK:
	}
	return false
}

// This should be called after the directory entry is read to update d.items
// with virtual entries
//
// must be called with the Dir lock held
func (mv manageVirtuals) end(d *Dir) {
	// delete unused d.items
	for name := range d.items {
		if _, ok := mv[name]; !ok {
			// name was previously in the directory but wasn't found
			// in the current listing
			switch d.virtual[name] {
			case vAddFile, vAddDir:
				// virtually added so leave virtual item
			default:
				// otherwise delete it
				delete(d.items, name)
			}
		}
	}
	// delete unused d.virtual~s
	for name, virtualState := range d.virtual {
		if _, ok := mv[name]; !ok {
			// name exists as a virtual but isn't in the current
			// listing so if it is a virtual delete we can remove it
			// as it is no longer needed.
			if virtualState == vDel {
				d._deleteVirtual(name)
			}
		}
	}
}

// update d.items and if dirTree is not nil update each dir in the DirTree below this one and
// set the last read time - must be called with the lock held
func (d *Dir) _readDirFromEntries(entries fs.DirEntries, dirTree dirtree.DirTree, when time.Time) error {
	var err error
	mv := d._newManageVirtuals()
	for _, entry := range entries {
		name := path.Base(entry.Remote())
		if name == "." || name == ".." {
			continue
		}
		node := d.items[name]
		if mv.add(d, name) {
			continue
		}
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
			err = fmt.Errorf("unknown type %T", item)
			fs.Errorf(d, "readDir error: %v", err)
			return err
		}
		d.items[name] = node
	}
	mv.end(d)
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
					return nil, fmt.Errorf("duplicate filename %q detected with --vfs-case-insensitive set", leaf)
				}
				// found a case insensitive match
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
	d.modTimeMu.Lock()
	defer d.modTimeMu.Unlock()
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
	d.modTimeMu.Lock()
	d.modTime = modTime
	d.modTimeMu.Unlock()
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
	// Return existing node if one exists
	node, err := d.stat(name)
	switch err {
	case ENOENT:
		// not found, carry on
	case nil:
		// found so check what it is
		if node.IsFile() {
			return node.(*File), err
		}
		return nil, EEXIST // EISDIR would be better but we don't have that
	default:
		// a different error - report
		fs.Errorf(d, "Dir.Create stat failed: %v", err)
		return nil, err
	}
	// node doesn't exist so create it
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
	// fs.Debugf(d, "BEFORE\n%s", d.dump())
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
			err := fmt.Errorf("Fs %q can't rename file that is not a vfs.File", d.f)
			fs.Errorf(oldPath, "Dir.Rename error: %v", err)
			return err
		}
	case fs.Directory:
		features := d.f.Features()
		if features.DirMove == nil && features.Move == nil && features.Copy == nil {
			err := fmt.Errorf("Fs %q can't rename directories (no DirMove, Move or Copy)", d.f)
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
		err = fmt.Errorf("unknown type %T", oldNode)
		fs.Errorf(d.path, "Dir.Rename error: %v", err)
		return err
	}

	// Show moved - delete from old dir and add to new
	d.delObject(oldName)
	destDir.addObject(oldNode)

	// fs.Debugf(newPath, "Dir.Rename renamed from %q", oldPath)
	// fs.Debugf(d, "AFTER\n%s", d.dump())
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
