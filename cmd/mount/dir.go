// +build linux darwin freebsd

package mount

import (
	"os"
	"path"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/cmd/mountlib"
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
	*mountlib.Dir
	// f       fs.Fs
	// path    string
	// modTime time.Time
	// mu      sync.RWMutex // protects the following
	// read    time.Time    // time directory entry last read
	// items   map[string]*DirEntry
}

// Check interface satsified
var _ fusefs.Node = (*Dir)(nil)

// Attr updates the attributes of a directory
func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) (err error) {
	defer fs.Trace(d, "")("attr=%+v, err=%v", a, &err)
	a.Gid = gid
	a.Uid = uid
	a.Mode = os.ModeDir | dirPerms
	modTime := d.ModTime()
	a.Atime = modTime
	a.Mtime = modTime
	a.Ctime = modTime
	a.Crtime = modTime
	// FIXME include Valid so get some caching?
	// FIXME fs.Debugf(d.path, "Dir.Attr %+v", a)
	return nil
}

// Check interface satisfied
var _ fusefs.NodeSetattrer = (*Dir)(nil)

// Setattr handles attribute changes from FUSE. Currently supports ModTime only.
func (d *Dir) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) (err error) {
	defer fs.Trace(d, "stat=%+v", req)("err=%v", &err)
	if noModTime {
		return nil
	}

	if req.Valid.MtimeNow() {
		err = d.SetModTime(time.Now())
	} else if req.Valid.Mtime() {
		err = d.SetModTime(req.Mtime)
	}

	return translateError(err)
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
	defer fs.Trace(d, "name=%q", req.Name)("node=%+v, err=%v", &node, &err)
	mnode, err := d.Dir.Lookup(req.Name)
	if err != nil {
		return nil, translateError(err)
	}
	switch x := mnode.(type) {
	case *mountlib.File:
		return &File{x}, nil
	case *mountlib.Dir:
		return &Dir{x}, nil
	}
	panic("bad type")
}

// Check interface satisfied
var _ fusefs.HandleReadDirAller = (*Dir)(nil)

// ReadDirAll reads the contents of the directory
func (d *Dir) ReadDirAll(ctx context.Context) (dirents []fuse.Dirent, err error) {
	itemsRead := -1
	defer fs.Trace(d, "")("item=%d, err=%v", &itemsRead, &err)
	items, err := d.Dir.ReadDirAll()
	if err != nil {
		return nil, translateError(err)
	}
	for _, item := range items {
		var dirent fuse.Dirent
		switch x := item.Obj.(type) {
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
			return nil, errors.Errorf("unknown type %T", item)
		}
		dirents = append(dirents, dirent)
	}
	itemsRead = len(dirents)
	return dirents, nil
}

var _ fusefs.NodeCreater = (*Dir)(nil)

// Create makes a new file
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (node fusefs.Node, handle fusefs.Handle, err error) {
	defer fs.Trace(d, "name=%q", req.Name)("node=%v, handle=%v, err=%v", &node, &handle, &err)
	file, fh, err := d.Dir.Create(req.Name)
	if err != nil {
		return nil, nil, translateError(err)
	}
	return &File{file}, &WriteFileHandle{fh}, err
}

var _ fusefs.NodeMkdirer = (*Dir)(nil)

// Mkdir creates a new directory
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (node fusefs.Node, err error) {
	defer fs.Trace(d, "name=%q", req.Name)("node=%+v, err=%v", &node, &err)
	dir, err := d.Dir.Mkdir(req.Name)
	if err != nil {
		return nil, translateError(err)
	}
	return &Dir{dir}, nil
}

var _ fusefs.NodeRemover = (*Dir)(nil)

// Remove removes the entry with the given name from
// the receiver, which must be a directory.  The entry to be removed
// may correspond to a file (unlink) or to a directory (rmdir).
func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) (err error) {
	defer fs.Trace(d, "name=%q", req.Name)("err=%v", &err)
	err = d.Dir.Remove(req.Name)
	if err != nil {
		return translateError(err)
	}
	return nil
}

// Check interface satisfied
var _ fusefs.NodeRenamer = (*Dir)(nil)

// Rename the file
func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fusefs.Node) (err error) {
	defer fs.Trace(d, "oldName=%q, newName=%q, newDir=%+v", req.OldName, req.NewName, newDir)("err=%v", &err)
	destDir, ok := newDir.(*Dir)
	if !ok {
		return errors.Errorf("Unknown Dir type %T", newDir)
	}

	err = d.Dir.Rename(req.OldName, req.NewName, destDir.Dir)
	if err != nil {
		return translateError(err)
	}

	return nil
}

// Check interface satisfied
var _ fusefs.NodeFsyncer = (*Dir)(nil)

// Fsync the directory
func (d *Dir) Fsync(ctx context.Context, req *fuse.FsyncRequest) (err error) {
	defer fs.Trace(d, "")("err=%v", &err)
	err = d.Dir.Fsync()
	if err != nil {
		return translateError(err)
	}
	return nil
}
