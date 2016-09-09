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
	f     fs.Fs
	path  string
	mu    sync.RWMutex // protects the following
	read  bool
	items map[string]*DirEntry
}

func newDir(f fs.Fs, path string) *Dir {
	return &Dir{
		f:    f,
		path: path,
	}
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
	if d.read {
		return nil
	}
	objs, dirs, err := fs.NewLister().SetLevel(1).Start(d.f, d.path).GetAll()
	if err == fs.ErrorDirNotFound {
		// We treat directory not found as empty because we
		// create directories on the fly
	} else if err != nil {
		return err
	}
	// Cache the items by name
	d.items = make(map[string]*DirEntry, len(objs)+len(dirs))
	for _, obj := range objs {
		name := path.Base(obj.Remote())
		d.items[name] = &DirEntry{
			o:    obj,
			node: nil,
		}
	}
	for _, dir := range dirs {
		name := path.Base(dir.Remote())
		d.items[name] = &DirEntry{
			o:    dir,
			node: nil,
		}
	}
	d.read = true
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

// Attr updates the attribes of a directory
func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	fs.Debug(d.path, "Dir.Attr")
	a.Gid = gid
	a.Uid = uid
	a.Mode = os.ModeDir | dirPerms
	// FIXME include Valid so get some caching? Also mtime
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
		node, err = newDir(d.f, x.Remote()), nil
	default:
		err = errors.Errorf("unknown type %T", item)
	}
	if err != nil {
		return nil, err
	}
	item = d.addObject(item.o, node)
	return item, err
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
	fs.Debug(path, "Dir.Lookup")
	item, err := d.lookupNode(req.Name)
	if err != nil {
		if err != fuse.ENOENT {
			fs.ErrorLog(path, "Dir.Lookup error: %v", err)
		}
		return nil, err
	}
	fs.Debug(path, "Dir.Lookup OK")
	return item.node, nil
}

// Check interface satisfied
var _ fusefs.HandleReadDirAller = (*Dir)(nil)

// ReadDirAll reads the contents of the directory
func (d *Dir) ReadDirAll(ctx context.Context) (dirents []fuse.Dirent, err error) {
	fs.Debug(d.path, "Dir.ReadDirAll")
	err = d.readDir()
	if err != nil {
		fs.Debug(d.path, "Dir.ReadDirAll error: %v", err)
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
			fs.ErrorLog(d.path, "Dir.ReadDirAll error: %v", err)
			return nil, err
		}
		dirents = append(dirents, dirent)
	}
	fs.Debug(d.path, "Dir.ReadDirAll OK with %d entries", len(dirents))
	return dirents, nil
}

var _ fusefs.NodeCreater = (*Dir)(nil)

// Create makes a new file
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fusefs.Node, fusefs.Handle, error) {
	path := path.Join(d.path, req.Name)
	fs.Debug(path, "Dir.Create")
	src := newCreateInfo(d.f, path)
	// This gets added to the directory when the file is written
	file := newFile(d, nil)
	fh, err := newWriteFileHandle(d, file, src)
	if err != nil {
		fs.ErrorLog(path, "Dir.Create error: %v", err)
		return nil, nil, err
	}
	fs.Debug(path, "Dir.Create OK")
	return file, fh, nil
}

var _ fusefs.NodeMkdirer = (*Dir)(nil)

// Mkdir creates a new directory
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fusefs.Node, error) {
	// We just pretend to have created the directory - rclone will
	// actually create the directory if we write files into it
	path := path.Join(d.path, req.Name)
	fs.Debug(path, "Dir.Mkdir")
	fsDir := &fs.Dir{
		Name: path,
		When: time.Now(),
	}
	dir := newDir(d.f, path)
	d.addObject(fsDir, dir)
	fs.Debug(path, "Dir.Mkdir OK")
	return dir, nil
}

var _ fusefs.NodeRemover = (*Dir)(nil)

// Remove removes the entry with the given name from
// the receiver, which must be a directory.  The entry to be removed
// may correspond to a file (unlink) or to a directory (rmdir).
func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	path := path.Join(d.path, req.Name)
	fs.Debug(path, "Dir.Remove")
	item, err := d.lookupNode(req.Name)
	if err != nil {
		fs.ErrorLog(path, "Dir.Remove error: %v", err)
		return err
	}
	switch x := item.o.(type) {
	case fs.Object:
		err = x.Remove()
		if err != nil {
			fs.ErrorLog(path, "Dir.Remove file error: %v", err)
			return err
		}
	case *fs.Dir:
		// Do nothing for deleting directory - rclone can't
		// currently remote a random directory
		//
		// Check directory is empty first though
		dir := item.node.(*Dir)
		empty, err := dir.isEmpty()
		if err != nil {
			fs.ErrorLog(path, "Dir.Remove dir error: %v", err)
			return err
		}
		if !empty {
			// return fuse.ENOTEMPTY - doesn't exist though so use EEXIST
			fs.ErrorLog(path, "Dir.Remove not empty")
			return fuse.EEXIST
		}
	default:
		fs.ErrorLog(path, "Dir.Remove unknown type %T", item)
		return errors.Errorf("unknown type %T", item)
	}
	// Remove the item from the directory listing
	d.delObject(req.Name)
	fs.Debug(path, "Dir.Remove OK")
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
		fs.ErrorLog(oldPath, "Dir.Rename error: %v", err)
		return err
	}
	newPath := path.Join(destDir.path, req.NewName)
	fs.Debug(oldPath, "Dir.Rename to %q", newPath)
	oldItem, err := d.lookupNode(req.OldName)
	if err != nil {
		fs.ErrorLog(oldPath, "Dir.Rename error: %v", err)
		return err
	}
	var newObj fs.BasicInfo
	switch x := oldItem.o.(type) {
	case fs.Object:
		oldObject := x
		do, ok := d.f.(fs.Mover)
		if !ok {
			err := errors.Errorf("Fs %q can't Move files", d.f)
			fs.ErrorLog(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		newObject, err := do.Move(oldObject, newPath)
		if err != nil {
			fs.ErrorLog(oldPath, "Dir.Rename error: %v", err)
			return err
		}
		newObj = newObject
	case *fs.Dir:
		oldDir := oldItem.node.(*Dir)
		empty, err := oldDir.isEmpty()
		if err != nil {
			fs.ErrorLog(oldPath, "Dir.Rename dir error: %v", err)
			return err
		}
		if !empty {
			// return fuse.ENOTEMPTY - doesn't exist though so use EEXIST
			fs.ErrorLog(oldPath, "Dir.Rename can't rename non empty directory")
			return fuse.EEXIST
		}
		newObj = &fs.Dir{
			Name: newPath,
			When: time.Now(),
		}
	default:
		err = errors.Errorf("unknown type %T", oldItem)
		fs.ErrorLog(d.path, "Dir.ReadDirAll error: %v", err)
		return err
	}

	// Show moved - delete from old dir and add to new
	d.delObject(req.OldName)
	destDir.addObject(newObj, nil)

	// FIXME need to flush the dir also

	// FIXME use DirMover to move a directory?
	// or maybe use MoveDir which can move anything
	// fallback to Copy/Delete if no Move?
	// if dir is empty then can move it

	fs.ErrorLog(newPath, "Dir.Rename renamed from %q", oldPath)
	return nil
}
