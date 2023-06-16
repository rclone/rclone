package operations

import (
	"context"
	"errors"
	"fmt"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
)

const (
	multithreadChunkSize      = 64 << 8
	multithreadChunkSizeMask  = multithreadChunkSize - 1
	multithreadReadBufferSize = 32 * 1024
)

// An offsetWriter maps writes at offset base to offset base+off in the underlying writer.
//
// Modified from the go source code. Can be replaced with
// io.OffsetWriter when we no longer need to support go1.19
type offsetWriter struct {
	w   io.WriterAt
	off int64 // the current offset
}

// newOffsetWriter returns an offsetWriter that writes to w
// starting at offset off.
func newOffsetWriter(w io.WriterAt, off int64) *offsetWriter {
	return &offsetWriter{w, off}
}

func (o *offsetWriter) Write(p []byte) (n int, err error) {
	n, err = o.w.WriteAt(p, o.off)
	o.off += int64(n)
	return
}

// Return a boolean as to whether we should use multi thread copy for
// this transfer
func doMultiThreadCopy(ctx context.Context, f fs.Fs, src fs.Object) bool {
	ci := fs.GetConfig(ctx)
	fs.Debugf("", "ci.MultiThreadStreams ", ci.MultiThreadStreams)

	// Disable multi thread if...

	// ...it isn't configured
	//if ci.MultiThreadStreams <= 1 {
	//	return false
	//}
	//// ...if the source doesn't support it
	//if src.Fs().Features().NoMultiThreading {
	//	return false
	//}
	//// ...size of object is less than cutoff
	if src.Size() < int64(ci.ChunkCutoff) {
		return false
	}
	//// ...destination doesn't support it
	//dstFeatures := f.Features()
	//if dstFeatures.OpenWriterAt == nil {
	//	return false
	//}
	//// ...if --multi-thread-streams not in use and source and
	//// destination are both local
	//if !ci.MultiThreadSet && dstFeatures.IsLocal && src.Fs().Features().IsLocal {
	//	return false
	//}
	return true
}

// state for a multi-thread copy
type multiThreadCopyState struct {
	ctx      context.Context
	partSize int64
	size     int64
	src      fs.Object
	acc      *accounting.Account
	streams  int
}

type readerAccounter struct {
	reader io.Reader
	acc    *accounting.Account
}

func (r readerAccounter) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.acc.AccountRead(n)
	return
}

// Copy a single stream into place
func (mc *multiThreadCopyState) copyStream(ctx context.Context, stream int, writer fs.ChunkWriter) (err error) {
	//ci := fs.GetConfig(ctx)
	defer func() {
		if err != nil {
			fs.Debugf(mc.src, "multi-thread copy: stream %d/%d failed: %v", stream+1, mc.streams, err)
		}
	}()
	start := int64(stream) * mc.partSize
	if start >= mc.size {
		return nil
	}
	end := start + mc.partSize
	if end > mc.size {
		end = mc.size
	}

	fs.Debugf(mc.src, "multi-thread copy: stream %d/%d (%d-%d) size %v starting", stream+1, mc.streams, start, end, fs.SizeSuffix(end-start))

	rc, err := Open(ctx, mc.src, &fs.RangeOption{Start: start, End: end - 1})
	if err != nil {
		return fmt.Errorf("multipart copy: failed to open source: %w", err)
	}
	defer fs.CheckClose(rc, &err)

	accR := readerAccounter{
		reader: rc,
		acc:    mc.acc,
	}

	err = writer.WriteChunk(stream+1, accR)
	if err != nil {
		return nil
	}

	fs.Debugf(mc.src, "multi-thread copy: stream %d/%d (%d-%d) size %v finished", stream+1, mc.streams, start, end, fs.SizeSuffix(end-start))
	return nil
}

// Calculate the chunk sizes and updated number of streams
func calculateParts(partSize int64, size int64) int {
	// calculate number of streams
	parts := int(size / partSize)
	// round streams up so partSize * streams >= size
	if (size % partSize) != 0 {
		parts++
	}
	return parts
}

// Copy src to (f, remote) using streams download threads and the OpenWriterAt feature
func multiThreadCopy(ctx context.Context, f fs.Fs, remote string, src fs.Object, streams int, tr *accounting.Transfer) (newDst fs.Object, err error) {
	ci := fs.GetConfig(ctx)

	openChunkWriter := f.Features().OpenChunkWriter
	if openChunkWriter == nil {
		return nil, errors.New("multi-part copy: OpenChunkWriter not supported")
	}

	if src.Size() < 0 {
		return nil, errors.New("multi-thread copy: can't copy unknown sized file")
	}
	if src.Size() == 0 {
		return nil, errors.New("multi-thread copy: can't copy zero sized file")
	}

	fs.Debugf(src, "name %v, %v", f.Name(), f.Root())

	g, gCtx := errgroup.WithContext(ctx)

	mc := &multiThreadCopyState{
		ctx:      gCtx,
		size:     src.Size(),
		src:      src,
		partSize: int64(ci.ChunkSize),
		streams:  calculateParts(int64(ci.ChunkSize), src.Size()),
	}

	chunkedWriter, err := openChunkWriter(ctx, remote, int64(ci.ChunkSize), src.Size())

	if err != nil {
		return nil, err
	}
	err = chunkedWriter.StartChunkWrite()
	if err != nil {
		return nil, err
	}

	// Make accounting
	mc.acc = tr.Account(ctx, nil)

	fs.Debugf(src, "Starting multi-thread copy with %d parts of size %v", mc.streams, fs.SizeSuffix(mc.partSize))
	maxWorkers := ci.ChunkConcurrency
	sem := semaphore.NewWeighted(int64(maxWorkers))
	for stream := 0; stream < mc.streams; stream++ {
		stream := stream
		fs.Debugf(src, "Acquiring semaphore...")
		if err := sem.Acquire(ctx, 1); err != nil {
			fs.Errorf(src, "Failed to acquire semaphore: %v", err)
			break
		}
		g.Go(func() (err error) {
			defer sem.Release(1)
			return mc.copyStream(gCtx, stream, chunkedWriter)
		})
	}
	err = g.Wait()
	if err != nil {
		return nil, err
	}

	err = chunkedWriter.CompleteChunkWrite()
	if err != nil {
		return nil, err
	}

	obj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("multi-thread copy: failed to find object after copy: %w", err)
	}
	fs.Debugf(src, "Completed multipart upload 2")

	//err = obj.SetModTime(ctx, src.ModTime(ctx))
	fs.Debugf(src, "Completed multipart upload 3 %v", err)

	switch err {
	case nil, fs.ErrorCantSetModTime, fs.ErrorCantSetModTimeWithoutDelete:
	default:
		fs.Errorf(src, "multi-thread copy: failed to set modification time: %v", err)
		return nil, fmt.Errorf("multi-thread copy: failed to set modification time: %w", err)
	}
	fs.Debugf(src, "Completed multipart upload 4")

	fs.Debugf(src, "Finished multi-thread copy with %d parts of size %v", mc.streams, fs.SizeSuffix(mc.partSize))
	return obj, nil
}
