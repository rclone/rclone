package azurefiles

import (
	"context"
	"time"
)

// Directory is a filesystem like directory provided by an Fs
type Directory struct {
	common
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
//
// It is unknown since getting the count of items results in a
// network request
func (d *Directory) Items() int64 {
	return -1
}

// ID returns empty string. Can be implemented as part of IDer
func (d *Directory) ID() string {
	return ""
}

// Size is returns the size of the file.
// This method is implemented because it is part of the [fs.DirEntry] interface
func (d *Directory) Size() int64 {
	return 0
}

// ModTime returns the modification time of the object
//
// TODO: check whether FileLastWriteTime is what the clients of this API want. Maybe
// FileLastWriteTime does not get changed when directory contents are updated but consumers
// of this API expect d.ModTime to do so
func (d *Directory) ModTime(ctx context.Context) time.Time {
	props, err := d.f.dirClient(d.remote).GetProperties(ctx, nil)
	if err != nil {
		return time.Now()
	}
	return *props.FileLastWriteTime
}
