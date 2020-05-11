// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"io/ioutil"

	"storj.io/common/ranger"
	"storj.io/common/readcloser"
)

// makePadding calculates how many bytes of padding are needed to fill
// an encryption block then creates a slice of zero bytes that size.
// The last byte of the padding slice contains the count of the total padding bytes added.
func makePadding(dataLen int64, blockSize int) []byte {
	amount := dataLen + uint32Size
	r := amount % int64(blockSize)
	padding := uint32Size
	if r > 0 {
		padding += blockSize - int(r)
	}
	paddingBytes := bytes.Repeat([]byte{0}, padding)
	binary.BigEndian.PutUint32(paddingBytes[padding-uint32Size:], uint32(padding))
	return paddingBytes
}

// Pad takes a Ranger and returns another Ranger that is a multiple of
// blockSize in length. The return value padding is a convenience to report how
// much padding was added.
func Pad(data ranger.Ranger, blockSize int) (
	rr ranger.Ranger, padding int) {
	paddingBytes := makePadding(data.Size(), blockSize)
	return ranger.Concat(data, ranger.ByteRanger(paddingBytes)), len(paddingBytes)
}

// Unpad takes a previously padded Ranger data source and returns an unpadded
// ranger, given the amount of padding. This is preferable to UnpadSlow if you
// can swing it.
func Unpad(data ranger.Ranger, padding int) (ranger.Ranger, error) {
	return ranger.Subrange(data, 0, data.Size()-int64(padding))
}

// UnpadSlow is like Unpad, but does not require the amount of padding.
// UnpadSlow will have to do extra work to make up for this missing information.
func UnpadSlow(ctx context.Context, data ranger.Ranger) (_ ranger.Ranger, err error) {
	r, err := data.Range(ctx, data.Size()-uint32Size, uint32Size)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	var p [uint32Size]byte
	_, err = io.ReadFull(r, p[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return Unpad(data, int(binary.BigEndian.Uint32(p[:])))
}

// PadReader is like Pad but works on a basic Reader instead of a Ranger.
func PadReader(data io.ReadCloser, blockSize int) io.ReadCloser {
	cr := newCountingReader(data)
	return readcloser.MultiReadCloser(cr,
		readcloser.LazyReadCloser(func() (io.ReadCloser, error) {
			return ioutil.NopCloser(bytes.NewReader(makePadding(cr.N, blockSize))), nil
		}))
}

type countingReader struct {
	R io.ReadCloser
	N int64
}

func newCountingReader(r io.ReadCloser) *countingReader {
	return &countingReader{R: r}
}

func (cr *countingReader) Read(p []byte) (n int, err error) {
	n, err = cr.R.Read(p)
	cr.N += int64(n)
	return n, err
}

func (cr *countingReader) Close() error {
	return cr.R.Close()
}
