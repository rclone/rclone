// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"context"
	"io"

	"storj.io/common/readcloser"
)

// A Ranger is a flexible data stream type that allows for more effective
// pipelining during seeking. A Ranger can return multiple parallel Readers for
// any subranges.
type Ranger interface {
	Size() int64
	Range(ctx context.Context, offset, length int64) (io.ReadCloser, error)
}

// ByteRanger turns a byte slice into a Ranger.
type ByteRanger []byte

// Size implements Ranger.Size.
func (b ByteRanger) Size() int64 { return int64(len(b)) }

// Range implements Ranger.Range.
func (b ByteRanger) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	if offset < 0 {
		return nil, Error.New("negative offset")
	}
	if length < 0 {
		return nil, Error.New("negative length")
	}
	if offset+length > int64(len(b)) {
		return nil, Error.New("buffer runoff")
	}

	return io.NopCloser(bytes.NewReader(b[offset : offset+length])), nil
}

type concatReader struct {
	r1 Ranger
	r2 Ranger
}

func (c *concatReader) Size() int64 {
	return c.r1.Size() + c.r2.Size()
}

func (c *concatReader) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	r1Size := c.r1.Size()
	if offset+length <= r1Size {
		return c.r1.Range(ctx, offset, length)
	}
	if offset >= r1Size {
		return c.r2.Range(ctx, offset-r1Size, length)
	}
	r1Range, err := c.r1.Range(ctx, offset, r1Size-offset)
	if err != nil {
		return nil, err
	}
	return readcloser.MultiReadCloser(
		r1Range,
		readcloser.LazyReadCloser(func() (io.ReadCloser, error) {
			return c.r2.Range(ctx, 0, length-(r1Size-offset))
		})), nil
}

func concat2(r1, r2 Ranger) Ranger {
	return &concatReader{r1: r1, r2: r2}
}

// Concat concatenates Rangers.
func Concat(r ...Ranger) Ranger {
	switch len(r) {
	case 0:
		return ByteRanger(nil)
	case 1:
		return r[0]
	case 2:
		return concat2(r[0], r[1])
	default:
		mid := len(r) / 2
		return concat2(Concat(r[:mid]...), Concat(r[mid:]...))
	}
}

type subrange struct {
	r              Ranger
	offset, length int64
}

// Subrange returns a subset of a Ranger.
func Subrange(data Ranger, offset, length int64) (Ranger, error) {
	dSize := data.Size()
	if offset < 0 || offset > dSize {
		return nil, Error.New("invalid offset")
	}
	if length+offset > dSize {
		return nil, Error.New("invalid length")
	}
	return &subrange{r: data, offset: offset, length: length}, nil
}

func (s *subrange) Size() int64 {
	return s.length
}

func (s *subrange) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.r.Range(ctx, offset+s.offset, length)
}
