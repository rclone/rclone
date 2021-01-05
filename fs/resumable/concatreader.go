package resumable

import (
	"context"
	"github.com/rclone/rclone/fs"
	"io"
)

type concatReader struct {
	objects fs.Objects
	reader  io.ReadCloser
	index   int
	ctx     context.Context
}

func NewConcatReader(ctx context.Context, objects fs.Objects) io.ReadCloser {
	return &concatReader{
		objects: objects,
		reader:  nil,
		index:   0,
		ctx:     ctx,
	}
}

func (c *concatReader) Read(p []byte) (n int, err error) {
	if c.reader == nil {
		if c.index >= len(c.objects) {
			return 0, io.EOF
		}
		c.reader, err = c.objects[c.index].Open(c.ctx)
		if err != nil {
			return
		}
	}
	n, err = c.reader.Read(p)
	if err == io.EOF {
		c.index++
		err = c.reader.Close()
		c.reader = nil
	}
	return
}

func (c *concatReader) Close() (err error) {
	if c.reader != nil {
		err = c.reader.Close()
		c.reader = nil
	}
	return
}
