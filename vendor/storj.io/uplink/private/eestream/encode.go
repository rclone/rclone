// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"context"
	"io"
	"io/ioutil"
	"os"

	"github.com/vivint/infectious"
	"go.uber.org/zap"

	"storj.io/common/encryption"
	"storj.io/common/fpath"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/ranger"
	"storj.io/common/readcloser"
	"storj.io/common/storj"
	"storj.io/common/sync2"
)

// ErasureScheme represents the general format of any erasure scheme algorithm.
// If this interface can be implemented, the rest of this library will work
// with it.
type ErasureScheme interface {
	// Encode will take 'in' and call 'out' with erasure coded pieces.
	Encode(in []byte, out func(num int, data []byte)) error

	// EncodeSingle will take 'in' with the stripe and fill 'out' with the erasure share for piece 'num'.
	EncodeSingle(in, out []byte, num int) error

	// Decode will take a mapping of available erasure coded piece num -> data,
	// 'in', and append the combined data to 'out', returning it.
	Decode(out []byte, in map[int][]byte) ([]byte, error)

	// ErasureShareSize is the size of the erasure shares that come from Encode
	// and are passed to Decode.
	ErasureShareSize() int

	// StripeSize is the size the stripes that are passed to Encode and come
	// from Decode.
	StripeSize() int

	// Encode will generate this many erasure shares and therefore this many pieces.
	TotalCount() int

	// Decode requires at least this many pieces.
	RequiredCount() int
}

// RedundancyStrategy is an ErasureScheme with a repair and optimal thresholds.
type RedundancyStrategy struct {
	ErasureScheme
	repairThreshold  int
	optimalThreshold int
}

// NewRedundancyStrategy from the given ErasureScheme, repair and optimal thresholds.
//
// repairThreshold is the minimum repair threshold.
// If set to 0, it will be reset to the TotalCount of the ErasureScheme.
// optimalThreshold is the optimal threshold.
// If set to 0, it will be reset to the TotalCount of the ErasureScheme.
func NewRedundancyStrategy(es ErasureScheme, repairThreshold, optimalThreshold int) (RedundancyStrategy, error) {
	if repairThreshold == 0 {
		repairThreshold = es.TotalCount()
	}

	if optimalThreshold == 0 {
		optimalThreshold = es.TotalCount()
	}
	if repairThreshold < 0 {
		return RedundancyStrategy{}, Error.New("negative repair threshold")
	}
	if repairThreshold > 0 && repairThreshold < es.RequiredCount() {
		return RedundancyStrategy{}, Error.New("repair threshold less than required count")
	}
	if repairThreshold > es.TotalCount() {
		return RedundancyStrategy{}, Error.New("repair threshold greater than total count")
	}
	if optimalThreshold < 0 {
		return RedundancyStrategy{}, Error.New("negative optimal threshold")
	}
	if optimalThreshold > 0 && optimalThreshold < es.RequiredCount() {
		return RedundancyStrategy{}, Error.New("optimal threshold less than required count")
	}
	if optimalThreshold > es.TotalCount() {
		return RedundancyStrategy{}, Error.New("optimal threshold greater than total count")
	}
	if repairThreshold > optimalThreshold {
		return RedundancyStrategy{}, Error.New("repair threshold greater than optimal threshold")
	}
	return RedundancyStrategy{ErasureScheme: es, repairThreshold: repairThreshold, optimalThreshold: optimalThreshold}, nil
}

// NewRedundancyStrategyFromProto creates new RedundancyStrategy from the given
// RedundancyScheme protobuf.
func NewRedundancyStrategyFromProto(scheme *pb.RedundancyScheme) (RedundancyStrategy, error) {
	fc, err := infectious.NewFEC(int(scheme.GetMinReq()), int(scheme.GetTotal()))
	if err != nil {
		return RedundancyStrategy{}, Error.Wrap(err)
	}
	es := NewRSScheme(fc, int(scheme.GetErasureShareSize()))
	return NewRedundancyStrategy(es, int(scheme.GetRepairThreshold()), int(scheme.GetSuccessThreshold()))
}

// NewRedundancyStrategyFromStorj creates new RedundancyStrategy from the given
// storj.RedundancyScheme.
func NewRedundancyStrategyFromStorj(scheme storj.RedundancyScheme) (RedundancyStrategy, error) {
	fc, err := infectious.NewFEC(int(scheme.RequiredShares), int(scheme.TotalShares))
	if err != nil {
		return RedundancyStrategy{}, Error.Wrap(err)
	}
	es := NewRSScheme(fc, int(scheme.ShareSize))
	return NewRedundancyStrategy(es, int(scheme.RepairShares), int(scheme.OptimalShares))
}

// RepairThreshold is the number of available erasure pieces below which
// the data must be repaired to avoid loss.
func (rs *RedundancyStrategy) RepairThreshold() int {
	return rs.repairThreshold
}

// OptimalThreshold is the number of available erasure pieces above which
// there is no need for the data to be repaired.
func (rs *RedundancyStrategy) OptimalThreshold() int {
	return rs.optimalThreshold
}

type encodedReader struct {
	log    *zap.Logger
	ctx    context.Context
	rs     RedundancyStrategy
	pieces map[int]*encodedPiece
}

// EncodeReader takes a Reader and a RedundancyStrategy and returns a slice of
// io.ReadClosers.
func EncodeReader(ctx context.Context, log *zap.Logger, r io.Reader, rs RedundancyStrategy) (_ []io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)

	er := &encodedReader{
		log:    log,
		ctx:    ctx,
		rs:     rs,
		pieces: make(map[int]*encodedPiece, rs.TotalCount()),
	}

	var pipeReaders []sync2.PipeReader
	var pipeWriter sync2.PipeWriter

	tempDir, inmemory, _ := fpath.GetTempData(ctx)
	if inmemory {
		// TODO what default inmemory size will be enough
		pipeReaders, pipeWriter, err = sync2.NewTeeInmemory(rs.TotalCount(), memory.MiB.Int64())
	} else {
		if tempDir == "" {
			tempDir = os.TempDir()
		}
		pipeReaders, pipeWriter, err = sync2.NewTeeFile(rs.TotalCount(), tempDir)
	}
	if err != nil {
		return nil, err
	}

	readers := make([]io.ReadCloser, 0, rs.TotalCount())
	for i := 0; i < rs.TotalCount(); i++ {
		er.pieces[i] = &encodedPiece{
			er:         er,
			pipeReader: pipeReaders[i],
			num:        i,
			stripeBuf:  make([]byte, rs.StripeSize()),
			shareBuf:   make([]byte, rs.ErasureShareSize()),
		}
		readers = append(readers, er.pieces[i])
	}

	go er.fillBuffer(ctx, r, pipeWriter)

	return readers, nil
}

func (er *encodedReader) fillBuffer(ctx context.Context, r io.Reader, w sync2.PipeWriter) {
	var err error
	defer mon.Task()(&ctx)(&err)
	_, err = sync2.Copy(ctx, w, r)
	err = w.CloseWithError(err)
	if err != nil {
		er.log.Error("Error closing buffer pipe", zap.Error(err))
	}
}

type encodedPiece struct {
	er            *encodedReader
	pipeReader    sync2.PipeReader
	num           int
	currentStripe int64
	stripeBuf     []byte
	shareBuf      []byte
	available     int
	err           error
}

func (ep *encodedPiece) Read(p []byte) (n int, err error) {
	// No need to trace this function because it's very fast and called many times.
	if ep.err != nil {
		return 0, ep.err
	}

	if ep.available == 0 {
		// take the next stripe from the segment buffer
		_, err := io.ReadFull(ep.pipeReader, ep.stripeBuf)
		if err != nil {
			return 0, err
		}

		// encode the num-th erasure share
		err = ep.er.rs.EncodeSingle(ep.stripeBuf, ep.shareBuf, ep.num)
		if err != nil {
			return 0, err
		}

		ep.currentStripe++
		ep.available = ep.er.rs.ErasureShareSize()
	}

	// we have some buffer remaining for this piece. write it to the output
	off := len(ep.shareBuf) - ep.available
	n = copy(p, ep.shareBuf[off:])
	ep.available -= n

	return n, nil
}

func (ep *encodedPiece) Close() (err error) {
	ctx := ep.er.ctx
	defer mon.Task()(&ctx)(&err)
	return ep.pipeReader.Close()
}

// EncodedRanger will take an existing Ranger and provide a means to get
// multiple Ranged sub-Readers. EncodedRanger does not match the normal Ranger
// interface.
type EncodedRanger struct {
	log *zap.Logger
	rr  ranger.Ranger
	rs  RedundancyStrategy
}

// NewEncodedRanger from the given Ranger and RedundancyStrategy. See the
// comments for EncodeReader about the repair and success thresholds.
func NewEncodedRanger(log *zap.Logger, rr ranger.Ranger, rs RedundancyStrategy) (*EncodedRanger, error) {
	if rr.Size()%int64(rs.StripeSize()) != 0 {
		return nil, Error.New("invalid erasure encoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	return &EncodedRanger{
		log: log,
		rs:  rs,
		rr:  rr,
	}, nil
}

// OutputSize is like Ranger.Size but returns the Size of the erasure encoded
// pieces that come out.
func (er *EncodedRanger) OutputSize() int64 {
	blocks := er.rr.Size() / int64(er.rs.StripeSize())
	return blocks * int64(er.rs.ErasureShareSize())
}

// Range is like Ranger.Range, but returns a slice of Readers.
func (er *EncodedRanger) Range(ctx context.Context, offset, length int64) (_ []io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	// the offset and length given may not be block-aligned, so let's figure
	// out which blocks contain the request.
	firstBlock, blockCount := encryption.CalcEncompassingBlocks(
		offset, length, er.rs.ErasureShareSize())
	// okay, now let's encode the reader for the range containing the blocks
	r, err := er.rr.Range(ctx,
		firstBlock*int64(er.rs.StripeSize()),
		blockCount*int64(er.rs.StripeSize()))
	if err != nil {
		return nil, err
	}
	readers, err := EncodeReader(ctx, er.log, r, er.rs)
	if err != nil {
		return nil, err
	}
	for i, r := range readers {
		// the offset might start a few bytes in, so we potentially have to
		// discard the beginning bytes
		_, err := io.CopyN(ioutil.Discard, r,
			offset-firstBlock*int64(er.rs.ErasureShareSize()))
		if err != nil {
			return nil, Error.Wrap(err)
		}
		// the length might be shorter than a multiple of the block size, so
		// limit it
		readers[i] = readcloser.LimitReadCloser(r, length)
	}
	return readers, nil
}

// CalcPieceSize calculates what would be the piece size of the encoded data
// after erasure coding data with dataSize using the given ErasureScheme.
func CalcPieceSize(dataSize int64, scheme ErasureScheme) int64 {
	const uint32Size = 4
	stripeSize := int64(scheme.StripeSize())
	stripes := (dataSize + uint32Size + stripeSize - 1) / stripeSize

	encodedSize := stripes * int64(scheme.StripeSize())
	pieceSize := encodedSize / int64(scheme.RequiredCount())

	return pieceSize
}
