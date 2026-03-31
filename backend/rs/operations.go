package rs

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
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

// BuildResult is the outcome of BuildRSShardsToWriters (content hashes and length).
type BuildResult struct {
	ContentLength int64
	Mtime         time.Time
	MD5           [16]byte
	SHA256        [32]byte
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
		bres, buildErr = BuildRSShardsToWriters(ctx, in, src, f.opt.DataShards, f.opt.ParityShards, writers, true)
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

// buildZeroLengthShards writes empty logical files (reedsolomon.Split requires at least 1 byte).
func buildZeroLengthShards(ctx context.Context, src fs.ObjectInfo, dataShards, parityShards int, writers []io.Writer, withFooter bool) (*BuildResult, error) {
	if len(writers) != dataShards+parityShards {
		return nil, fmt.Errorf("rs: writers count mismatch: got %d want %d", len(writers), dataShards+parityShards)
	}
	mtime := src.ModTime(ctx)
	for i := range writers {
		if withFooter {
			ft := NewRSFooter(0, emptyFileMD5[:], emptyFileSHA256[:], mtime, dataShards, parityShards, i, 0, crc32cChecksum(nil))
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

// BuildRSShardsToWriters reads object bytes from in, Reed-Solomon encodes into writers.
// If withFooter is true, each writer receives payload plus EC v2 footer (rclone particle format).
func BuildRSShardsToWriters(ctx context.Context, in io.Reader, src fs.ObjectInfo, dataShards, parityShards int, writers []io.Writer, withFooter bool) (*BuildResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if len(writers) != dataShards+parityShards {
		return nil, fmt.Errorf("rs: writers count mismatch: got %d want %d", len(writers), dataShards+parityShards)
	}
	payload, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("rs: read source: %w", err)
	}
	if len(payload) == 0 {
		return buildZeroLengthShards(ctx, src, dataShards, parityShards, writers, withFooter)
	}
	md5Arr := md5.Sum(payload)
	shaArr := sha256.Sum256(payload)
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("rs: create encoder: %w", err)
	}
	dataPieces, err := enc.Split(payload)
	if err != nil {
		return nil, fmt.Errorf("rs: split payload: %w", err)
	}
	shards := make([][]byte, dataShards+parityShards)
	copy(shards, dataPieces)
	for i := dataShards; i < len(shards); i++ {
		shards[i] = make([]byte, len(shards[0]))
	}
	if err := enc.Encode(shards); err != nil {
		return nil, fmt.Errorf("rs: encode shards: %w", err)
	}
	stripeSize := uint32(0)
	if len(shards) > 0 {
		stripeSize = uint32(len(shards[0]))
	}
	mtime := src.ModTime(ctx)
	for i := range shards {
		if _, err := writers[i].Write(shards[i]); err != nil {
			return nil, fmt.Errorf("rs: write shard payload %d: %w", i, err)
		}
		if withFooter {
			ft := NewRSFooter(int64(len(payload)), md5Arr[:], shaArr[:], mtime, dataShards, parityShards, i, stripeSize, crc32cChecksum(shards[i]))
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
		ContentLength: int64(len(payload)),
		Mtime:         mtime,
		MD5:           md5Arr,
		SHA256:        shaArr,
	}, nil
}
