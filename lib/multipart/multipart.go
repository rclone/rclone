// Package multipart implements generic multipart uploading.
package multipart

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/pool"
	"golang.org/x/sync/errgroup"
)

const (
	BufferSize           = 1024 * 1024     // BufferSize is the default size of the pages used in the reader
	bufferCacheSize      = 64              // max number of buffers to keep in cache
	bufferCacheFlushTime = 5 * time.Second // flush the cached buffers after this long
)

// bufferPool is a global pool of buffers
var (
	bufferPool     *pool.Pool
	bufferPoolOnce sync.Once
)

// get a buffer pool
func getPool() *pool.Pool {
	bufferPoolOnce.Do(func() {
		ci := fs.GetConfig(context.Background())
		// Initialise the buffer pool when used
		bufferPool = pool.New(bufferCacheFlushTime, BufferSize, bufferCacheSize, ci.UseMmap)
	})
	return bufferPool
}

// NewRW gets a pool.RW using the multipart pool
func NewRW() *pool.RW {
	return pool.NewRW(getPool())
}

// UploadMultipartOptions options for the generic multipart upload
type UploadMultipartOptions struct {
	Open        fs.OpenChunkWriter // thing to call OpenChunkWriter on
	OpenOptions []fs.OpenOption    // options for OpenChunkWriter
}

// UploadMultipart does a generic multipart upload from src using f as OpenChunkWriter.
//
// in is read seqentially and chunks from it are uploaded in parallel.
//
// It returns the chunkWriter used in case the caller needs to extract any private info from it.
func UploadMultipart(ctx context.Context, src fs.ObjectInfo, in io.Reader, opt UploadMultipartOptions) (chunkWriterOut fs.ChunkWriter, err error) {
	info, chunkWriter, err := opt.Open.OpenChunkWriter(ctx, src.Remote(), src, opt.OpenOptions...)
	if err != nil {
		return nil, fmt.Errorf("multipart upload failed to initialise: %w", err)
	}

	// make concurrency machinery
	concurrency := info.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	tokens := pacer.NewTokenDispenser(concurrency)

	uploadCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer atexit.OnError(&err, func() {
		cancel()
		if info.LeavePartsOnError {
			return
		}
		fs.Debugf(src, "Cancelling multipart upload")
		errCancel := chunkWriter.Abort(ctx)
		if errCancel != nil {
			fs.Debugf(src, "Failed to cancel multipart upload: %v", errCancel)
		}
	})()

	var (
		g, gCtx   = errgroup.WithContext(uploadCtx)
		finished  = false
		off       int64
		size      = src.Size()
		chunkSize = info.ChunkSize
	)

	// Do the accounting manually
	in, acc := accounting.UnWrapAccounting(in)

	for partNum := int64(0); !finished; partNum++ {
		// Get a block of memory from the pool and token which limits concurrency.
		tokens.Get()
		rw := NewRW()
		if acc != nil {
			rw.SetAccounting(acc.AccountRead)
		}

		free := func() {
			// return the memory and token
			_ = rw.Close() // Can't return an error
			tokens.Put()
		}

		// Fail fast, in case an errgroup managed function returns an error
		// gCtx is cancelled. There is no point in uploading all the other parts.
		if gCtx.Err() != nil {
			free()
			break
		}

		// Read the chunk
		var n int64
		n, err = io.CopyN(rw, in, chunkSize)
		if err == io.EOF {
			if n == 0 && partNum != 0 { // end if no data and if not first chunk
				free()
				break
			}
			finished = true
		} else if err != nil {
			free()
			return nil, fmt.Errorf("multipart upload: failed to read source: %w", err)
		}

		partNum := partNum
		partOff := off
		off += n
		g.Go(func() (err error) {
			defer free()
			fs.Debugf(src, "multipart upload: starting chunk %d size %v offset %v/%v", partNum, fs.SizeSuffix(n), fs.SizeSuffix(partOff), fs.SizeSuffix(size))
			_, err = chunkWriter.WriteChunk(gCtx, int(partNum), rw)
			return err
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}

	err = chunkWriter.Close(ctx)
	if err != nil {
		return nil, fmt.Errorf("multipart upload: failed to finalise: %w", err)
	}

	return chunkWriter, nil
}
