package fs

import (
	"context"
	"time"
)

// Dir describes an unspecialized directory for directory/container/bucket lists
type Dir struct {
	remote  string    // name of the directory
	modTime time.Time // modification or creation time - IsZero for unknown
	size    int64     // size of directory and contents or -1 if unknown
	items   int64     // number of objects or -1 for unknown
	id      string    // optional ID
}

// NewDir creates an unspecialized Directory object
func NewDir(remote string, modTime time.Time) *Dir {
	return &Dir{
		remote:  remote,
		modTime: modTime,
		size:    -1,
		items:   -1,
	}
}

// NewDirCopy creates an unspecialized copy of the Directory object passed in
func NewDirCopy(ctx context.Context, d Directory) *Dir {
	return &Dir{
		remote:  d.Remote(),
		modTime: d.ModTime(ctx),
		size:    d.Size(),
		items:   d.Items(),
		id:      d.ID(),
	}
}

// String returns the name
func (d *Dir) String() string {
	return d.remote
}

// Remote returns the remote path
func (d *Dir) Remote() string {
	return d.remote
}

// SetRemote sets the remote
func (d *Dir) SetRemote(remote string) *Dir {
	d.remote = remote
	return d
}

// ID gets the optional ID
func (d *Dir) ID() string {
	return d.id
}

// SetID sets the optional ID
func (d *Dir) SetID(id string) *Dir {
	d.id = id
	return d
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (d *Dir) ModTime(ctx context.Context) time.Time {
	if !d.modTime.IsZero() {
		return d.modTime
	}
	return time.Now()
}

// Size returns the size of the file
func (d *Dir) Size() int64 {
	return d.size
}

// SetSize sets the size of the directory
func (d *Dir) SetSize(size int64) *Dir {
	d.size = size
	return d
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
func (d *Dir) Items() int64 {
	return d.items
}

// SetItems sets the number of items in the directory
func (d *Dir) SetItems(items int64) *Dir {
	d.items = items
	return d
}

// Check interfaces
var (
	_ DirEntry  = (*Dir)(nil)
	_ Directory = (*Dir)(nil)
)
