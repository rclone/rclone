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

// TODO: let this be something else
func (d *Directory) ModTime(ctx context.Context) time.Time {
	return time.Now()
}
