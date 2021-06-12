package fs

import (
	"context"
	"time"
)

// Dir describes an unspecialized directory for directory/container/bucket lists

type Dir interface {
	Directory
	SetRemote(string) Dir
	SetID(string) Dir
	SetSize(int64) Dir
	SetItems(int64) Dir
}

type dir struct {
	remote  string    // name of the directory
	modTime time.Time // modification or creation time - IsZero for unknown
	size    int64     // size of directory and contents or -1 if unknown
	items   int64     // number of objects or -1 for unknown
	id      string    // optional ID
	parent  string    // optional parent directory ID
}

// NewDir creates an unspecialized *dir object
func NewDir(remote string, modTime time.Time) Dir {
	return &dir{
		remote:  remote,
		modTime: modTime,
		size:    -1,
		items:   -1,
	}
}

type lazyDir struct {
	dir
	sizeGetter  func() int64
	itemsGetter func() int64
}

// NewDirCopy creates an unspecialized copy of the Directory object passed in
func NewDirCopy(ctx context.Context, d Directory) Dir {
	dir := dir{
		remote:  d.Remote(),
		modTime: d.ModTime(ctx),
		size:    d.Size(),
		items:   d.Items(),
		id:      d.ID(),
	}
	if l, ok := d.(*lazyDir); ok {
		return &lazyDir{
			dir:         dir,
			sizeGetter:  l.sizeGetter,
			itemsGetter: l.itemsGetter,
		}
	}
	return &dir
}

func (d *lazyDir) Size() int64 {
	if d.dir.Size() == -1 {
		d.SetSize(d.sizeGetter())
	}
	return d.dir.Size()
}

func (d *lazyDir) Items() int64 {
	if d.dir.Items() == -1 {
		d.SetItems(d.itemsGetter())
	}
	return d.dir.Items()
}

func NewLazyDir(remote string, modTime time.Time, sizeGetter func() int64, itemsGetter func() int64) Dir {
	return &lazyDir{
		dir: dir{
			remote:  remote,
			modTime: modTime,
			size:    -1,
			items:   -1,
		},
		sizeGetter:  sizeGetter,
		itemsGetter: itemsGetter,
	}
}

// String returns the name
func (d *dir) String() string {
	return d.remote
}

// Remote returns the remote path
func (d *dir) Remote() string {
	return d.remote
}

// SetRemote sets the remote
func (d *dir) SetRemote(remote string) Dir {
	d.remote = remote
	return d
}

// ID gets the optional ID
func (d *dir) ID() string {
	return d.id
}

// SetID sets the optional ID
func (d *dir) SetID(id string) Dir {
	d.id = id
	return d
}

// ParentID returns the IDs of the Dir parent if known
func (d *Dir) ParentID() string {
	return d.parent
}

// SetParentID sets the optional parent ID of the Dir
func (d *Dir) SetParentID(parent string) *Dir {
	d.parent = parent
	return d
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (d *dir) ModTime(ctx context.Context) time.Time {
	if !d.modTime.IsZero() {
		return d.modTime
	}
	return time.Now()
}

// Size returns the size of the file
func (d *dir) Size() int64 {
	return d.size
}

// SetSize sets the size of the directory
func (d *dir) SetSize(size int64) Dir {
	d.size = size
	return d
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
func (d *dir) Items() int64 {
	return d.items
}

// SetItems sets the number of items in the directory
func (d *dir) SetItems(items int64) Dir {
	d.items = items
	return d
}

// Check interfaces
var (
	_ Dir       = (*dir)(nil)
	_ DirEntry  = (*dir)(nil)
	_ Directory = (*dir)(nil)
)
