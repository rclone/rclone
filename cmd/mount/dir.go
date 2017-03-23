// +build linux darwin freebsd

package mount

import (
	"os"
	"path"
	"sync"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// DirEntry describes the contents of a directory entry
//
// It can be a file or a directory
//
// node may be nil, but o may not
type DirEntry struct {
	o    fs.BasicInfo
	node fusefs.Node
}

// Dir represents a directory entry
type Dir struct {
	f       fs.Fs
	path    string
	modTime time.Time
	mu      sync.RWMutex // protects the following
	read    time.Time    // time directory entry last read
	items   map[string]*DirEntry
}

func newDir(f fs.Fs, fsDir *fs.Dir) *Dir {
	return &Dir{
		f:       f,
		path:    fsDir.Name,
		modTime: fsDir.When,
	}
}

// rename should be called after the directory is renamed
//
// Reset the directory to new state, discarding all the objects and
// reading everything again
func (d *Dir) rename(newParent *Dir, fsDir *fs.Dir) {
	d.path = fsDir.Name
	d.modTime = fsDir.When
	d.items = nil
	d.read = time.Time{}
}

// addObject adds a new object or directory to the directory
//
// note that we add new objects rather than updating old ones
func (d *Dir) addObject(o fs.BasicInfo, node fusefs.Node) *DirEntry {
	item := &DirEntry{
		o:    o,
		node: node,
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
				o:    obj,
				node: nil,
			}
		case *fs.Dir:
			dir := item
			name := path.Base(dir.Remote())
			// Use old dir value if it exists
			if oldItem, ok := oldItems[name]; ok {
				if _, ok := oldItem.o.(*fs.Dir); ok {
					d.items[name] = oldItem
					continue
				}
			}
			d.items[name] = &DirEntry{
				o:    dir,
				node: nil,
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
// returns fuse.ENOENT if not found.
func (d *Dir) lookup(leaf string) (*DirEntry, error) {
	err := d.readDir()
	if err != nil {
		return nil, err
	}
	d.mu.RLock()
	item, ok := d.items[leaf]
	d.mu.RUnlock()
	if !ok {
		return nil, fuse.ENOENT
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

// Check interface satsified
var _ fusefs.Node = (*Dir)(nil)

// Attr updates the attributes of a directory
func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Gid = gid
	a.Uid = uid
	a.Mode = os.ModeDir | dirPerms
	a.Atime = d.modTime
	a.Mtime = d.modTime
	a.Ctime = d.modTime
	a.Crtime = d.modTime
	// FIXME include Valid so get some caching?
	fs.Debugf(d.path, "Dir.Attr %+v", a)
	return nil
}

// Check interface satisfied
var _ fusefs.NodeSetattrer = (*Dir)(nil)

// Setattr handles attribute changes from FUSE. Currently supports ModTime only.
func (d *Dir) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	if noModTime {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if req.Valid.MtimeNow() {
		d.modTime = time.Now()
	} else if req.Valid.Mtime() {
		d.modTime = req.Mtime
	}

	return nil
}

// lookupNode calls lookup then makes sure the node is not nil in the DirEntry
func (d *Dir) lookupNode(leaf string) (item *DirEntry, err error) {
	item, err = d.lookup(leaf)
	if err != nil {
		return nil, err
	}
	if item.node != nil {
		return item, nil
	}
	var node fusefs.Node
	switch x := item.o.(type) {
	case fs.Object:
		node, err = newFile(d, x), nil
	case *fs.Dir:
		node, err = newDir(d.f, x), nil
	default:
		err = errors.Errorf("unknown type %T", item)
	}
	if err != nil {
		return nil, err
	}
	item = d.addObject(item.o, node)
	return item, nil
}

// Check interface satisfied
var _ fusefs.NodeRequestLookuper = (*Dir)(nil)

// Lookup looks up a specific entry in the receiver.
//
// Lookup should return a Node corresponding to the entry.  If the
// name does not exist in the directory, Lookup should return ENOENT.
//
// Lookup need not to handle the names "." and "..".
func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (node fusefs.Node, err error) {
	path := path.Join(d.path, req.Name)
	fs.Debugf(path, "Dir.Lookup")
	item, err := d.lookupNode(req.Name)
	if err != nil {
		if err != fuse.ENOENT {
			fs.Errorf(path, "Dir.Lookup error: %v", err)
		}
		return nil, err
	}
	fs.Debugf(path, "Dir.Lookup OK")
	return item.node, nil
}

// Check interface satisfied
var _ fusefs.HandleReadDirAller = (*Dir)(nil)

// ReadDirAll reads the contents of the directory
func (d *Dir) ReadDirAll(ctx context.Context) (dirents []fuse.Dirent, err error) {
	fs.Debugf(d.path, "Dir.ReadDirAll")
	err = d.readDir()
	if err != nil {
		fs.Debugf(d.path, "Dir.ReadDirAll error: %v", err)
		return nil, err
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, item := range d.items {
		var dirent fuse.Dirent
		switch x := item.o.(type) {
		case fs.Object:
			dirent = fuse.Dirent{
				// Inode FIXME ???
				Type: fuse.DT_File,
				Name: path.Base(x.Remote()),
			}
		case *fs.Dir:
			dirent = fuse.Dirent{
				// Inode FIXME ???
				Type: fuse.DT_Dir,
				Name: path.Base(x.Remote()),
			}
		default:
			err = errors.Errorf("unknown type %T", item)
			fs.Errorf(d.path, "Dir.ReadDirAll error: %v", err)
			return nil, err
		}
		dirents = append(dirents, dirent)
	}
	fs.Debugf(d.path, "Dir.ReadDirAll OK with %d entries", len(dirents))
	return dirents, nil
}

var _ fusefs.NodeCreater = (*Dir)(nil)

// Create makes a new file
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fusefs.Node, fusefs.Handle, error) {
	path := path.Join(d.path, req.Name)
	fs.Debugf(path, "Dir.Create")
	src := newCreateInfo(d.f, path)
	// This gets added to the directory when the file is written
	file := newFile(d, nil)
	fh, err := newWriteFileHandle(d, file, src)
	if err != nil {
		fs.Errorf(path, "Dir.Create error: %v", err)
		return nil, nil, err
	}
	fs.Debugf(path, "Dir.Create OK")
	return file, fh, nil
}

var _ fusefs.NodeMkdirer = (*Dir)(nil)

// Mkdir creates a new directory
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fusefs.Node, error) {
	path := path.Join(d.path, req.Name)
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
	dir := newDir(d.f, fsDir)
	d.addObject(fsDir, dir)
	fs.Debugf(path, "Dir.Mkdir OK")
	return dir, nil
}

var _ fusefs.NodeRemover = (*Dir)(nil)

// Remove removes the entry with the given name from
// the receiver, which must be a directory.  The entry to be removed
// may correspond to a file (unlink) or to a directory (rmdir).
func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	path := path.Join(d.path, req.Name)
	fs.Debugf(path, "Dir.Remove")
	item, err := d.lookupNode(req.Name)
	if err != nil {
		fs.Errorf(path, "Dir.Remove error: %v", err)
		return err
	}
	switch x := item.o.(type) {
	case fs.Object:
		err = x.Remove()
		if err != nil {
			fs.Errorf(path, "Dir.Remove file error: %v", err)
			return err
		}
	case *fs.Dir:
		// Check directory is empty first
		dir := item.node.(*Dir)
		empty, err := dir.isEmpty()
		if err != nil {
			fs.Errorf(path, "Dir.Remove dir error: %v", err)
			return err
		}
		if !empty {
			// return fuse.ENOTEMPTY - doesn't exist though so use EEXIST
			fs.Errorf(path, "Dir.Remove not empty")
			return fuse.EEXIST
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
	d.delObject(req.Name)
	fs.Debugf(path, "Dir.Remove OK")
	return nil
}

// Check interface satisfied
var _ fusefs.NodeRenamer = (*Dir)(nil)

// Rename the file
func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fusefs.Node) error {
	oldPath := path.Join(d.path, req.OldName)
	destDir, ok := newDir.(*Dir)
	if !ok {
		err := errors.Errorf("Unknown Dir type %T", newDir)
		fs.Errorf(oldPath, "Dir.Rename error: %v", err)
		return err
	}
	newPath := path.Join(destDir.path, req.NewName)
	fs.Debugf(oldPath, "Dir.Rename to %q", newPath)
	oldItem, err := d.lookupNode(req.OldName)
	if err != nil {
		fs.Errorf(oldPath, "Dir.Rename error: %v", err)
		return err
	}
	var newObj fs.BasicInfo
	oldNode := oldItem.node
	switch x := oldItem.o.(type) {
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
				fs.Debugf(oldItem.o, "Updating file with %v %p", newObject, oldFile)
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
				fs.Debugf(oldItem.o, "Updating dir with %v %p", newDir, oldDir)
				oldDir.rename(destDir, newDir)
			}
		}
	default:
		err = errors.Errorf("unknown type %T", oldItem)
		fs.Errorf(d.path, "Dir.ReadDirAll error: %v", err)
		return err
	}

	// Show moved - delete from old dir and add to new
	d.delObject(req.OldName)
	destDir.addObject(newObj, oldNode)

	fs.Debugf(newPath, "Dir.Rename renamed from %q", oldPath)
	return nil
}

// Check interface satisfied
var _ fusefs.NodeFsyncer = (*Dir)(nil)

// Fsync the directory
//
// Note that we don't do anything except return OK
func (d *Dir) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}
