package operations

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/multipart"
	"golang.org/x/sync/errgroup"
)

const (
	multithreadChunkSize = 64 << 10
)

// Return a boolean as to whether we should use multi thread copy for
// this transfer
func doMultiThreadCopy(ctx context.Context, f fs.Fs, src fs.Object) bool {
	ci := fs.GetConfig(ctx)

	// Disable multi thread if...

	// ...it isn't configured
	if ci.MultiThreadStreams <= 1 {
		return false
	}
	// ...if the source doesn't support it
	if src.Fs().Features().NoMultiThreading {
		return false
	}
	// ...size of object is less than cutoff
	if src.Size() < int64(ci.MultiThreadCutoff) {
		return false
	}
	// ...destination doesn't support it
	dstFeatures := f.Features()
	if dstFeatures.OpenChunkWriter == nil && dstFeatures.OpenWriterAt == nil {
		return false
	}
	// ...if --multi-thread-streams not in use and source and
	// destination are both local
	if !ci.MultiThreadSet && dstFeatures.IsLocal && src.Fs().Features().IsLocal {
		return false
	}
	return true
}

// state for a multi-thread copy
type multiThreadCopyState struct {
	ctx         context.Context
	partSize    int64
	size        int64
	src         fs.Object
	acc         *accounting.Account
	numChunks   int
	noBuffering bool // set to read the input without buffering
}

// Copy a single chunk into place
func (mc *multiThreadCopyState) copyChunk(ctx context.Context, chunk int, writer fs.ChunkWriter) (err error) {
	defer func() {
		if err != nil {
			fs.Debugf(mc.src, "multi-thread copy: chunk %d/%d failed: %v", chunk+1, mc.numChunks, err)
		}
	}()
	start := int64(chunk) * mc.partSize
	if start >= mc.size {
		return nil
	}
	end := start + mc.partSize
	if end > mc.size {
		end = mc.size
	}
	size := end - start

	fs.Debugf(mc.src, "multi-thread copy: chunk %d/%d (%d-%d) size %v starting", chunk+1, mc.numChunks, start, end, fs.SizeSuffix(size))

	rc, err := Open(ctx, mc.src, &fs.RangeOption{Start: start, End: end - 1})
	if err != nil {
		return fmt.Errorf("multi-thread copy: failed to open source: %w", err)
	}
	defer fs.CheckClose(rc, &err)

	var rs io.ReadSeeker
	if mc.noBuffering {
		// Read directly if we are sure we aren't going to seek
		// and account with accounting
		rc.SetAccounting(mc.acc.AccountRead)
		rs = rc
	} else {
		// Read the chunk into buffered reader
		rw := multipart.NewRW()
		defer fs.CheckClose(rw, &err)
		_, err = io.CopyN(rw, rc, size)
		if err != nil {
			return fmt.Errorf("multi-thread copy: failed to read chunk: %w", err)
		}
		// Account as we go
		rw.SetAccounting(mc.acc.AccountRead)
		rs = rw
	}

	// Write the chunk
	bytesWritten, err := writer.WriteChunk(ctx, chunk, rs)
	if err != nil {
		return fmt.Errorf("multi-thread copy: failed to write chunk: %w", err)
	}

	fs.Debugf(mc.src, "multi-thread copy: chunk %d/%d (%d-%d) size %v finished", chunk+1, mc.numChunks, start, end, fs.SizeSuffix(bytesWritten))
	return nil
}

// Given a file size and a chunkSize
// it returns the number of chunks, so that chunkSize * numChunks >= size
func calculateNumChunks(size int64, chunkSize int64) int {
	numChunks := size / chunkSize
	if size%chunkSize != 0 {
		numChunks++
	}
	return int(numChunks)
}

// Copy src to (f, remote) using streams download threads. It tries to use the OpenChunkWriter feature
// and if that's not available it creates an adapter using OpenWriterAt
func multiThreadCopy(ctx context.Context, f fs.Fs, remote string, src fs.Object, concurrency int, tr *accounting.Transfer, options ...fs.OpenOption) (newDst fs.Object, err error) {
	openChunkWriter := f.Features().OpenChunkWriter
	ci := fs.GetConfig(ctx)
	noBuffering := false
	usingOpenWriterAt := false
	if openChunkWriter == nil {
		openWriterAt := f.Features().OpenWriterAt
		if openWriterAt == nil {
			return nil, errors.New("multi-thread copy: neither OpenChunkWriter nor OpenWriterAt supported")
		}
		openChunkWriter = openChunkWriterFromOpenWriterAt(openWriterAt, int64(ci.MultiThreadChunkSize), int64(ci.MultiThreadWriteBufferSize), f)
		// If we are using OpenWriterAt we don't seek the chunks so don't need to buffer
		fs.Debugf(src, "multi-thread copy: disabling buffering because destination uses OpenWriterAt")
		noBuffering = true
		usingOpenWriterAt = true
	} else if src.Fs().Features().IsLocal {
		// If the source fs is local we don't need to buffer
		fs.Debugf(src, "multi-thread copy: disabling buffering because source is local disk")
		noBuffering = true
	} else if f.Features().ChunkWriterDoesntSeek {
		// If the destination Fs promises not to seek its chunks
		// (except for retries) then we don't need buffering.
		fs.Debugf(src, "multi-thread copy: disabling buffering because destination has set ChunkWriterDoesntSeek")
		noBuffering = true
	}

	if src.Size() < 0 {
		return nil, fmt.Errorf("multi-thread copy: can't copy unknown sized file")
	}
	if src.Size() == 0 {
		return nil, fmt.Errorf("multi-thread copy: can't copy zero sized file")
	}

	info, chunkWriter, err := openChunkWriter(ctx, remote, src, options...)
	if err != nil {
		return nil, fmt.Errorf("multi-thread copy: failed to open chunk writer: %w", err)
	}

	uploadCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	uploadedOK := false
	defer atexit.OnError(&err, func() {
		cancel()
		if info.LeavePartsOnError || uploadedOK {
			return
		}
		fs.Debugf(src, "multi-thread copy: cancelling transfer on exit")
		abortErr := chunkWriter.Abort(ctx)
		if abortErr != nil {
			fs.Debugf(src, "multi-thread copy: abort failed: %v", abortErr)
		}
	})()

	if info.ChunkSize > src.Size() {
		fs.Debugf(src, "multi-thread copy: chunk size %v was bigger than source file size %v", fs.SizeSuffix(info.ChunkSize), fs.SizeSuffix(src.Size()))
		info.ChunkSize = src.Size()
	}

	// Use the backend concurrency if it is higher than --multi-thread-streams or if --multi-thread-streams wasn't set explicitly
	if !ci.MultiThreadSet || info.Concurrency > concurrency {
		fs.Debugf(src, "multi-thread copy: using backend concurrency of %d instead of --multi-thread-streams %d", info.Concurrency, concurrency)
		concurrency = info.Concurrency
	}

	numChunks := calculateNumChunks(src.Size(), info.ChunkSize)
	if concurrency > numChunks {
		fs.Debugf(src, "multi-thread copy: number of streams %d was bigger than number of chunks %d", concurrency, numChunks)
		concurrency = numChunks
	}

	if concurrency < 1 {
		concurrency = 1
	}

	g, gCtx := errgroup.WithContext(uploadCtx)
	g.SetLimit(concurrency)

	mc := &multiThreadCopyState{
		ctx:         gCtx,
		size:        src.Size(),
		src:         src,
		partSize:    info.ChunkSize,
		numChunks:   numChunks,
		noBuffering: noBuffering,
	}

	// Make accounting
	mc.acc = tr.Account(gCtx, nil)

	fs.Debugf(src, "Starting multi-thread copy with %d chunks of size %v with %v parallel streams", mc.numChunks, fs.SizeSuffix(mc.partSize), concurrency)
	for chunk := 0; chunk < mc.numChunks; chunk++ {
		// Fail fast, in case an errgroup managed function returns an error
		if gCtx.Err() != nil {
			break
		}
		chunk := chunk
		g.Go(func() error {
			return mc.copyChunk(gCtx, chunk, chunkWriter)
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}
	err = chunkWriter.Close(ctx)
	if err != nil {
		return nil, fmt.Errorf("multi-thread copy: failed to close object after copy: %w", err)
	}
	uploadedOK = true // file is definitely uploaded OK so no need to abort

	obj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("multi-thread copy: failed to find object after copy: %w", err)
	}

	// OpenWriterAt doesn't set metadata so we need to set it on completion
	if usingOpenWriterAt {
		setModTime := true
		if ci.Metadata {
			do, ok := obj.(fs.SetMetadataer)
			if ok {
				meta, err := fs.GetMetadataOptions(ctx, f, src, options)
				if err != nil {
					return nil, fmt.Errorf("multi-thread copy: failed to read metadata from source object: %w", err)
				}
				if _, foundMeta := meta["mtime"]; !foundMeta {
					meta.Set("mtime", src.ModTime(ctx).Format(time.RFC3339Nano))
				}
				err = do.SetMetadata(ctx, meta)
				if err != nil {
					return nil, fmt.Errorf("multi-thread copy: failed to set metadata: %w", err)
				}
				setModTime = false
			} else {
				fs.Errorf(obj, "multi-thread copy: can't set metadata as SetMetadata isn't implemented in: %v", f)
			}
		}
		if setModTime {
			err = obj.SetModTime(ctx, src.ModTime(ctx))
			switch err {
			case nil, fs.ErrorCantSetModTime, fs.ErrorCantSetModTimeWithoutDelete:
			default:
				return nil, fmt.Errorf("multi-thread copy: failed to set modification time: %w", err)
			}
		}
	}

	fs.Debugf(src, "Finished multi-thread copy with %d parts of size %v", mc.numChunks, fs.SizeSuffix(mc.partSize))
	return obj, nil
}

// writerAtChunkWriter converts a WriterAtCloser into a ChunkWriter
type writerAtChunkWriter struct {
	remote          string
	size            int64
	writerAt        fs.WriterAtCloser
	chunkSize       int64
	chunks          int
	writeBufferSize int64
	f               fs.Fs
	closed          bool
}

// WriteChunk writes chunkNumber from reader
func (w *writerAtChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	fs.Debugf(w.remote, "writing chunk %v", chunkNumber)

	bytesToWrite := w.chunkSize
	if chunkNumber == (w.chunks-1) && w.size%w.chunkSize != 0 {
		bytesToWrite = w.size % w.chunkSize
	}

	var writer io.Writer = io.NewOffsetWriter(w.writerAt, int64(chunkNumber)*w.chunkSize)
	if w.writeBufferSize > 0 {
		writer = bufio.NewWriterSize(writer, int(w.writeBufferSize))
	}
	n, err := io.Copy(writer, reader)
	if err != nil {
		return -1, err
	}
	if n != bytesToWrite {
		return -1, fmt.Errorf("expected to write %v bytes for chunk %v, but wrote %v bytes", bytesToWrite, chunkNumber, n)
	}
	// if we were buffering, flush to disk
	switch w := writer.(type) {
	case *bufio.Writer:
		er2 := w.Flush()
		if er2 != nil {
			return -1, fmt.Errorf("multi-thread copy: flush failed: %w", err)
		}
	}
	return n, nil
}

// Close the chunk writing
func (w *writerAtChunkWriter) Close(ctx context.Context) error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.writerAt.Close()
}

// Abort the chunk writing
func (w *writerAtChunkWriter) Abort(ctx context.Context) error {
	err := w.Close(ctx)
	if err != nil {
		fs.Errorf(w.remote, "multi-thread copy: failed to close file before aborting: %v", err)
	}
	obj, err := w.f.NewObject(ctx, w.remote)
	if err != nil {
		return fmt.Errorf("multi-thread copy: failed to find temp file when aborting chunk writer: %w", err)
	}
	return obj.Remove(ctx)
}

// openChunkWriterFromOpenWriterAt adapts an OpenWriterAtFn into an OpenChunkWriterFn using chunkSize and writeBufferSize
func openChunkWriterFromOpenWriterAt(openWriterAt fs.OpenWriterAtFn, chunkSize int64, writeBufferSize int64, f fs.Fs) fs.OpenChunkWriterFn {
	return func(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
		ci := fs.GetConfig(ctx)

		writerAt, err := openWriterAt(ctx, remote, src.Size())
		if err != nil {
			return info, nil, err
		}

		if writeBufferSize > 0 {
			fs.Debugf(src.Remote(), "multi-thread copy: write buffer set to %v", writeBufferSize)
		}

		chunkWriter := &writerAtChunkWriter{
			remote:          remote,
			size:            src.Size(),
			chunkSize:       chunkSize,
			chunks:          calculateNumChunks(src.Size(), chunkSize),
			writerAt:        writerAt,
			writeBufferSize: writeBufferSize,
			f:               f,
		}
		info = fs.ChunkWriterInfo{
			ChunkSize:   chunkSize,
			Concurrency: ci.MultiThreadStreams,
		}
		return info, chunkWriter, nil
	}
}
