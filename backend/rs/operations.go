package rs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"golang.org/x/sync/errgroup"
)

const putOperationTimeout = 5 * time.Minute

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

	expectedParticle := ShardParticleFileSize(bres.ContentLength, f.opt.DataShards, f.opt.StripeFragmentSize, true)

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
			if st.Size() != expectedParticle {
				return fmt.Errorf("rs: shard %d: incorrect upload size %d != %d", i, st.Size(), expectedParticle)
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
	if err := validateUploadedShardParticleSizes(uploaded, expectedParticle); err != nil {
		if f.opt.Rollback {
			_ = f.rollbackPut(uploadCtx, uploaded)
		}
		return nil, err
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
	return f.opt.WriteQuorum
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

func (f *Fs) rollbackPut(ctx context.Context, uploaded []uploadedShard) error {
	var firstErr error
	for _, u := range uploaded {
		if err := u.obj.Remove(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// validateUploadedShardParticleSizes checks each returned object size against the expected
// particle file size when the backend reports a known size (Size() < 0 means unknown; skip).
func validateUploadedShardParticleSizes(uploaded []uploadedShard, expected int64) error {
	for _, u := range uploaded {
		sz := u.obj.Size()
		if sz < 0 {
			continue
		}
		if sz != expected {
			return fmt.Errorf("rs: shard %d: incorrect upload size %d != %d", u.index, sz, expected)
		}
	}
	return nil
}
