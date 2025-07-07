package pikpak

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rclone/rclone/backend/pikpak/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/pool"
	"golang.org/x/sync/errgroup"
)

const (
	bufferSize           = 1024 * 1024     // default size of the pages used in the reader
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
		bufferPool = pool.New(bufferCacheFlushTime, bufferSize, bufferCacheSize, ci.UseMmap)
	})
	return bufferPool
}

// NewRW gets a pool.RW using the multipart pool
func NewRW() *pool.RW {
	return pool.NewRW(getPool())
}

// Upload does a multipart upload in parallel
func (w *pikpakChunkWriter) Upload(ctx context.Context) (err error) {
	// make concurrency machinery
	tokens := pacer.NewTokenDispenser(w.con)

	uploadCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer atexit.OnError(&err, func() {
		cancel()
		fs.Debugf(w.o, "multipart upload: Cancelling...")
		errCancel := w.Abort(ctx)
		if errCancel != nil {
			fs.Debugf(w.o, "multipart upload: failed to cancel: %v", errCancel)
		}
	})()

	var (
		g, gCtx   = errgroup.WithContext(uploadCtx)
		finished  = false
		off       int64
		size      = w.size
		chunkSize = w.chunkSize
	)

	// Do the accounting manually
	in, acc := accounting.UnWrapAccounting(w.in)

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
			return fmt.Errorf("multipart upload: failed to read source: %w", err)
		}

		partNum := partNum
		partOff := off
		off += n
		g.Go(func() (err error) {
			defer free()
			fs.Debugf(w.o, "multipart upload: starting chunk %d size %v offset %v/%v", partNum, fs.SizeSuffix(n), fs.SizeSuffix(partOff), fs.SizeSuffix(size))
			_, err = w.WriteChunk(gCtx, int32(partNum), rw)
			return err
		})
	}

	err = g.Wait()
	if err != nil {
		return err
	}

	err = w.Close(ctx)
	if err != nil {
		return fmt.Errorf("multipart upload: failed to finalise: %w", err)
	}

	return nil
}

var warnStreamUpload sync.Once

// state of ChunkWriter
type pikpakChunkWriter struct {
	chunkSize      int64
	size           int64
	con            int
	f              *Fs
	o              *Object
	in             io.Reader
	mu             sync.Mutex
	completedParts []types.CompletedPart
	client         *s3.Client
	mOut           *s3.CreateMultipartUploadOutput
}

func (f *Fs) newChunkWriter(ctx context.Context, remote string, size int64, p *api.ResumableParams, in io.Reader, options ...fs.OpenOption) (w *pikpakChunkWriter, err error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}

	// calculate size of parts
	chunkSize := f.opt.ChunkSize

	// size can be -1 here meaning we don't know the size of the incoming file. We use ChunkSize
	// buffers here (default 5 MiB). With a maximum number of parts (10,000) this will be a file of
	// 48 GiB which seems like a not too unreasonable limit.
	if size == -1 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				f.opt.ChunkSize, fs.SizeSuffix(int64(chunkSize)*int64(maxUploadParts)))
		})
	} else {
		chunkSize = chunksize.Calculator(o, size, maxUploadParts, chunkSize)
	}

	client, err := f.newS3Client(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload client: %w", err)
	}
	w = &pikpakChunkWriter{
		chunkSize:      int64(chunkSize),
		size:           size,
		con:            max(1, f.opt.UploadConcurrency),
		f:              f,
		o:              o,
		in:             in,
		completedParts: make([]types.CompletedPart, 0),
		client:         client,
	}

	req := &s3.CreateMultipartUploadInput{
		Bucket: &p.Bucket,
		Key:    &p.Key,
	}
	// Apply upload options
	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "":
			// ignore
		case "cache-control":
			req.CacheControl = aws.String(value)
		case "content-disposition":
			req.ContentDisposition = aws.String(value)
		case "content-encoding":
			req.ContentEncoding = aws.String(value)
		case "content-type":
			req.ContentType = aws.String(value)
		}
	}
	err = w.f.pacer.Call(func() (bool, error) {
		w.mOut, err = w.client.CreateMultipartUpload(ctx, req)
		return w.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, fmt.Errorf("create multipart upload failed: %w", err)
	}
	fs.Debugf(w.o, "multipart upload: %q initiated", *w.mOut.UploadId)
	return
}

// shouldRetry returns a boolean as to whether this err
// deserve to be retried. It returns the err as a convenience
func (w *pikpakChunkWriter) shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if fserrors.ShouldRetry(err) {
		return true, err
	}
	return false, err
}

// add a part number and etag to the completed parts
func (w *pikpakChunkWriter) addCompletedPart(part types.CompletedPart) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.completedParts = append(w.completedParts, part)
}

// WriteChunk will write chunk number with reader bytes, where chunk number >= 0
func (w *pikpakChunkWriter) WriteChunk(ctx context.Context, chunkNumber int32, reader io.ReadSeeker) (currentChunkSize int64, err error) {
	if chunkNumber < 0 {
		err := fmt.Errorf("invalid chunk number provided: %v", chunkNumber)
		return -1, err
	}

	partNumber := chunkNumber + 1
	var res *s3.UploadPartOutput
	err = w.f.pacer.Call(func() (bool, error) {
		// Discover the size by seeking to the end
		currentChunkSize, err = reader.Seek(0, io.SeekEnd)
		if err != nil {
			return false, err
		}
		// rewind the reader on retry and after reading md5
		_, err := reader.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}
		res, err = w.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     w.mOut.Bucket,
			Key:        w.mOut.Key,
			UploadId:   w.mOut.UploadId,
			PartNumber: &partNumber,
			Body:       reader,
		})
		if err != nil {
			if chunkNumber <= 8 {
				return w.shouldRetry(ctx, err)
			}
			// retry all chunks once have done the first few
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return -1, fmt.Errorf("failed to upload chunk %d with %v bytes: %w", partNumber, currentChunkSize, err)
	}

	w.addCompletedPart(types.CompletedPart{
		PartNumber: &partNumber,
		ETag:       res.ETag,
	})

	fs.Debugf(w.o, "multipart upload: wrote chunk %d with %v bytes", partNumber, currentChunkSize)
	return currentChunkSize, err
}

// Abort the multipart upload
func (w *pikpakChunkWriter) Abort(ctx context.Context) (err error) {
	// Abort the upload session
	err = w.f.pacer.Call(func() (bool, error) {
		_, err = w.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   w.mOut.Bucket,
			Key:      w.mOut.Key,
			UploadId: w.mOut.UploadId,
		})
		return w.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload %q: %w", *w.mOut.UploadId, err)
	}
	fs.Debugf(w.o, "multipart upload: %q aborted", *w.mOut.UploadId)
	return
}

// Close and finalise the multipart upload
func (w *pikpakChunkWriter) Close(ctx context.Context) (err error) {
	// sort the completed parts by part number
	sort.Slice(w.completedParts, func(i, j int) bool {
		return *w.completedParts[i].PartNumber < *w.completedParts[j].PartNumber
	})
	// Finalise the upload session
	err = w.f.pacer.Call(func() (bool, error) {
		_, err = w.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
			Bucket:   w.mOut.Bucket,
			Key:      w.mOut.Key,
			UploadId: w.mOut.UploadId,
			MultipartUpload: &types.CompletedMultipartUpload{
				Parts: w.completedParts,
			},
		})
		return w.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}
	fs.Debugf(w.o, "multipart upload: %q finished", *w.mOut.UploadId)
	return
}
