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

func (d *Directory) ModTime(ctx context.Context) time.Time {
	if d.properties.changeTime == nil {
		return time.Now()
	}
	return *d.properties.changeTime
}
