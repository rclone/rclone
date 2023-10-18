package azurefiles

import (
	"context"
	"time"
)

type Directory struct {
	common
}

func (d *Directory) Items() int64 {
	return -1
}

func (d *Directory) ID() string {
	return ""
}

func (d *Directory) Size() int64 {
	return 0
}

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
