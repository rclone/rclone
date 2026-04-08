package rs

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/klauspost/reedsolomon"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"golang.org/x/sync/errgroup"
)

const putOperationTimeout = 5 * time.Minute

// DefaultStripeFragmentSize is S: bytes per shard per RS stripe (fragment size).
// Erasure-coded systems commonly use 64KiB–1MiB unit sizes (e.g. Tahoe-LAFS default
// 128KiB segments; HDFS EC often 64KiB+). 256KiB balances memory (k·S per stripe),
// reconstruct work, and remote API round-trips.
const DefaultStripeFragmentSize = 256 * 1024

// NumStripesForContent returns the stripe count for a logical size, k, and fragment size S.
// For non-empty content it is ceil(ContentLength / (k*S)). Empty content yields 0.
func NumStripesForContent(contentLength int64, k, S int) int {
	if contentLength == 0 || k < 1 || S < 1 {
		return 0
	}
	capacity := int64(k) * int64(S)
	return int((contentLength + capacity - 1) / capacity)
}

func normalizeStripeFragmentSize(n int) int {
	if n <= 0 {
		return DefaultStripeFragmentSize
	}
	return n
}

// BuildResult is the outcome of BuildRSShardsToWriters (content hashes and length).
type BuildResult struct {
	ContentLength int64
	Mtime         time.Time
	MD5           [16]byte
	SHA256        [32]byte
	StripeSize    uint32
	NumStripes    uint32
}

type uploadedShard struct {
	index int
	obj   fs.Object
}

// Put writes a logical object by encoding and uploading shards (spooling path).
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if !f.opt.UseSpooling {
		return nil, errors.New("rs: use_spooling=false is not implemented yet")
	}
	return f.putSpooling(ctx, in, src, options...)
}

// PutStream is an alias for Put for the rs backend.
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

func (f *Fs) putSpooling(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	stageBase := f.opt.StagingDir
	if stageBase == "" {
		stageBase = os.TempDir()
	}
	stageDir, err := os.MkdirTemp(stageBase, "rclone-rs-*")
	if err != nil {
		return nil, fmt.Errorf("rs: create staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(stageDir) }()

	shardPaths := make([]string, len(f.backends))
	shardFiles := make([]*os.File, len(f.backends))
	for i := range shardFiles {
		shardPaths[i] = filepath.Join(stageDir, fmt.Sprintf("shard_%03d", i))
		shardFiles[i], err = os.Create(shardPaths[i])
		if err != nil {
			return nil, fmt.Errorf("rs: create shard temp file %d: %w", i, err)
		}
	}

	var preflightErr error
	var bres *BuildResult
	var buildErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		preflightErr = f.checkWriteQuorumAvailable(ctx)
	}()
	go func() {
		defer wg.Done()
		writers := make([]io.Writer, len(shardFiles))
		for i := range shardFiles {
			writers[i] = shardFiles[i]
		}
		bres, buildErr = BuildRSShardsToWriters(ctx, in, src, f.opt.DataShards, f.opt.ParityShards, f.opt.StripeFragmentSize, writers, true)
	}()
	wg.Wait()
	for _, fh := range shardFiles {
		_ = fh.Close()
	}
	if buildErr != nil {
		return nil, buildErr
	}
	if preflightErr != nil {
		return nil, preflightErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	uploadCtx, cancel := context.WithTimeout(ctx, putOperationTimeout)
	defer cancel()

	successCh := make(chan uploadedShard, len(f.backends))
	var uploaded []uploadedShard
	var uploadMu sync.Mutex

	g, gctx := errgroup.WithContext(uploadCtx)
	sem := make(chan struct{}, f.opt.MaxParallelUploads)
	for i := range f.backends {
		i := i
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			r, err := os.Open(shardPaths[i])
			if err != nil {
				return fmt.Errorf("open shard %d: %w", i, err)
			}
			defer func() { _ = r.Close() }()
			st, err := r.Stat()
			if err != nil {
				return fmt.Errorf("stat shard %d: %w", i, err)
			}
			info := object.NewStaticObjectInfo(src.Remote(), bres.Mtime, st.Size(), true, nil, nil)
			obj, err := f.backends[i].Put(gctx, r, info, options...)
			if err != nil {
				return err
			}
			successCh <- uploadedShard{index: i, obj: obj}
			return nil
		})
	}

	uploadErr := g.Wait()
	close(successCh)
	for u := range successCh {
		uploadMu.Lock()
		uploaded = append(uploaded, u)
		uploadMu.Unlock()
	}
	if uploadErr != nil {
		if f.opt.Rollback {
			_ = f.rollbackPut(uploadCtx, uploaded)
		}
		return nil, uploadErr
	}
	if len(uploaded) < f.writeQuorum() {
		if f.opt.Rollback {
			_ = f.rollbackPut(uploadCtx, uploaded)
		}
		return nil, fmt.Errorf("rs: write quorum not met: successful=%d required=%d", len(uploaded), f.writeQuorum())
	}
	return &Object{fs: f, remote: src.Remote(), footer: &Footer{
		ContentLength: bres.ContentLength,
		MD5:           bres.MD5,
		SHA256:        bres.SHA256,
		Mtime:         bres.Mtime.Unix(),
		StripeSize:    bres.StripeSize,
		NumStripes:    bres.NumStripes,
		DataShards:    uint8(f.opt.DataShards),
		ParityShards:  uint8(f.opt.ParityShards),
		Algorithm:     AlgorithmRS,
	}}, nil
}

func (f *Fs) writeQuorum() int {
	return f.opt.DataShards + 1
}

func (f *Fs) checkWriteQuorumAvailable(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	okCh := make(chan bool, len(f.backends))
	for _, b := range f.backends {
		b := b
		g.Go(func() error {
			_, err := b.List(gctx, "")
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return err
				}
				okCh <- errors.Is(err, fs.ErrorDirNotFound)
				return nil
			}
			okCh <- true
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		close(okCh)
		return err
	}
	close(okCh)
	available := 0
	for ok := range okCh {
		if ok {
			available++
		}
	}
	if available < f.writeQuorum() {
		return fmt.Errorf("rs: insufficient writable remotes for quorum: available=%d required=%d", available, f.writeQuorum())
	}
	return nil
}

// buildZeroLengthShards writes empty logical files (no payload; footer only).
func buildZeroLengthShards(ctx context.Context, src fs.ObjectInfo, dataShards, parityShards int, writers []io.Writer, withFooter bool) (*BuildResult, error) {
	if len(writers) != dataShards+parityShards {
		return nil, fmt.Errorf("rs: writers count mismatch: got %d want %d", len(writers), dataShards+parityShards)
	}
	mtime := src.ModTime(ctx)
	for i := range writers {
		if withFooter {
			ft := NewRSFooter(0, emptyFileMD5[:], emptyFileSHA256[:], mtime, dataShards, parityShards, i, 0, 0, crc32cChecksum(nil))
			fb, err := ft.MarshalBinary()
			if err != nil {
				return nil, err
			}
			if _, err := writers[i].Write(fb); err != nil {
				return nil, fmt.Errorf("rs: write shard footer %d: %w", i, err)
			}
		}
	}
	return &BuildResult{
		ContentLength: 0,
		Mtime:         mtime,
		MD5:           emptyFileMD5,
		SHA256:        emptyFileSHA256,
		StripeSize:    0,
		NumStripes:    0,
	}, nil
}

func (f *Fs) rollbackPut(ctx context.Context, uploaded []uploadedShard) error {
	var firstErr error
	for _, u := range uploaded {
		if err := u.obj.Remove(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// BuildRSShardsToWriters reads object bytes from in, Reed-Solomon encodes in stripes into writers.
// stripeFragmentSize is S (bytes per shard per stripe); if <= 0, DefaultStripeFragmentSize is used.
// If withFooter is true, each writer receives payload plus EC footer (rclone particle format).
func BuildRSShardsToWriters(ctx context.Context, in io.Reader, src fs.ObjectInfo, dataShards, parityShards int, stripeFragmentSize int, writers []io.Writer, withFooter bool) (*BuildResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if len(writers) != dataShards+parityShards {
		return nil, fmt.Errorf("rs: writers count mismatch: got %d want %d", len(writers), dataShards+parityShards)
	}
	S := normalizeStripeFragmentSize(stripeFragmentSize)
	k := dataShards
	payload, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("rs: read source: %w", err)
	}
	if len(payload) == 0 {
		return buildZeroLengthShards(ctx, src, dataShards, parityShards, writers, withFooter)
	}
	md5Arr := md5.Sum(payload)
	shaArr := sha256.Sum256(payload)
	mtime := src.ModTime(ctx)
	contentLength := int64(len(payload))

	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("rs: create encoder: %w", err)
	}

	numStripes := NumStripesForContent(contentLength, k, S)
	stripeBuf := make([]byte, k*S)
	shards := make([][]byte, k+parityShards)
	crcH := make([]hash.Hash32, len(writers))
	for i := range crcH {
		crcH[i] = crc32.New(crc32cTable)
	}

	var pos int64
	for stripe := 0; stripe < numStripes; stripe++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		logicalRemaining := contentLength - pos
		if logicalRemaining <= 0 {
			break
		}
		logicalThis := int64(k * S)
		if logicalThis > logicalRemaining {
			logicalThis = logicalRemaining
		}
		clear(stripeBuf)
		copy(stripeBuf, payload[pos:pos+logicalThis])
		pos += logicalThis

		dataPieces, err := enc.Split(stripeBuf)
		if err != nil {
			return nil, fmt.Errorf("rs: split stripe %d: %w", stripe, err)
		}
		for i := range shards {
			shards[i] = nil
		}
		copy(shards, dataPieces)
		for i := dataShards; i < len(shards); i++ {
			shards[i] = make([]byte, len(shards[0]))
		}
		if err := enc.Encode(shards); err != nil {
			return nil, fmt.Errorf("rs: encode stripe %d: %w", stripe, err)
		}
		for i := range writers {
			frag := shards[i]
			if _, err := writers[i].Write(frag); err != nil {
				return nil, fmt.Errorf("rs: write shard %d stripe %d: %w", i, stripe, err)
			}
			_, _ = crcH[i].Write(frag)
		}
	}

	if pos != contentLength {
		return nil, fmt.Errorf("rs: internal error: encoded %d of %d bytes", pos, contentLength)
	}

	stripeU32 := uint32(S)
	numStripesU32 := uint32(numStripes)
	for i := range writers {
		if withFooter {
			ft := NewRSFooter(contentLength, md5Arr[:], shaArr[:], mtime, dataShards, parityShards, i, stripeU32, numStripesU32, crcH[i].Sum32())
			fb, err := ft.MarshalBinary()
			if err != nil {
				return nil, err
			}
			if _, err := writers[i].Write(fb); err != nil {
				return nil, fmt.Errorf("rs: write shard footer %d: %w", i, err)
			}
		}
	}
	return &BuildResult{
		ContentLength: contentLength,
		Mtime:         mtime,
		MD5:           md5Arr,
		SHA256:        shaArr,
		StripeSize:    stripeU32,
		NumStripes:    numStripesU32,
	}, nil
}
