package readers

import (
	"context"
	"io"
)

// NewContextReader creates a reader, that returns any errors that ctx gives
func NewContextReader(ctx context.Context, r io.Reader) io.Reader {
	return &contextReader{
		ctx: ctx,
		r:   r,
	}
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

// Read bytes as per io.Reader interface
func (cr *contextReader) Read(p []byte) (n int, err error) {
	err = cr.ctx.Err()
	if err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}
