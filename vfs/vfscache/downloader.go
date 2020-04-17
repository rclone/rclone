package vfscache

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/asyncreader"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/lib/ranges"
	"github.com/rclone/rclone/lib/readers"
)

// downloader represents a running download for a file
type downloader struct {
	// write only
	mu     sync.Mutex
	ctx    context.Context
	item   *Item
	src    fs.Object // source object
	fcache fs.Fs     // destination Fs
	osPath string

	// per download
	out         *os.File // file we are writing to
	offset      int64    // current offset
	waiters     []waiter
	tr          *accounting.Transfer
	in          *accounting.Account // input we are reading from
	downloading bool                // whether the download thread is running
	finished    chan struct{}       // closed when download finished
}

// waiter is a range we are waiting for and a channel to signal
type waiter struct {
	r       ranges.Range
	errChan chan<- error
}

func newDownloader(item *Item, fcache fs.Fs, remote string, src fs.Object) (dl *downloader, err error) {
	defer log.Trace(src, "remote=%q", remote)("dl=%+v, err=%v", &dl, &err)

	dl = &downloader{
		ctx:    context.Background(),
		item:   item,
		src:    src,
		fcache: fcache,
		osPath: item.c.toOSPath(remote),
	}

	// make sure there is a cache file
	_, err = os.Stat(dl.osPath)
	if err == nil {
		// do nothing
	} else if os.IsNotExist(err) {
		return nil, errors.New("vfs cache: internal error: newDownloader: called before Item.Open")
		// fs.Debugf(src, "creating empty file")
		// err = item._truncateToCurrentSize()
		// if err != nil {
		// 	return nil, errors.Wrap(err, "newDownloader: failed to create empty file")
		// }
	} else {
		return nil, errors.Wrap(err, "newDownloader: failed to stat cache file")
	}

	return dl, nil
}

// close any waiters with the error passed in
//
// call with lock held
func (dl *downloader) _closeWaiters(err error) {
	for _, waiter := range dl.waiters {
		waiter.errChan <- err
	}
	dl.waiters = nil
}

// Write writes len(p) bytes from p to the underlying data stream. It
// returns the number of bytes written from p (0 <= n <= len(p)) and
// any error encountered that caused the write to stop early. Write
// must return a non-nil error if it returns n < len(p). Write must
// not modify the slice data, even temporarily.
//
// Implementations must not retain p.
func (dl *downloader) Write(p []byte) (n int, err error) {
	defer log.Trace(dl.src, "p_len=%d", len(p))("n=%d, err=%v", &n, &err)

	var (
		// Range we wish to write
		r       = ranges.Range{Pos: dl.offset, Size: int64(len(p))}
		curr    ranges.Range
		present bool
		nn      int
	)

	// Check to see what regions are already present
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.item.mu.Lock()
	defer dl.item.mu.Unlock()

	// Write the range out ignoring already written chunks
	// FIXME might stop downloading if we are ignoring chunks?
	for err == nil && !r.IsEmpty() {
		curr, r, present = dl.item.info.Rs.Find(r)
		if curr.Pos != dl.offset {
			return n, errors.New("internal error: offset of range is wrong")
		}
		if present {
			// if present want to skip this range
			fs.Debugf(dl.src, "skip chunk offset=%d size=%d", dl.offset, curr.Size)
			nn = int(curr.Size)
			_, err = dl.out.Seek(curr.Size, io.SeekCurrent)
			if err != nil {
				nn = 0
			}
		} else {
			// if range not present then we want to write it
			fs.Debugf(dl.src, "write chunk offset=%d size=%d", dl.offset, curr.Size)
			nn, err = dl.out.Write(p[:curr.Size])
			dl.item.info.Rs.Insert(ranges.Range{Pos: dl.offset, Size: int64(nn)})
		}
		dl.offset += int64(nn)
		p = p[nn:]
		n += nn
	}
	if n > 0 {
		if len(dl.waiters) > 0 {
			newWaiters := dl.waiters[:0]
			for _, waiter := range dl.waiters {
				if dl.item.info.Rs.Present(waiter.r) {
					waiter.errChan <- nil
				} else {
					newWaiters = append(newWaiters, waiter)
				}
			}
			dl.waiters = newWaiters
		}
	}
	if err != nil && err != io.EOF {
		dl._closeWaiters(err)
	}
	return n, err
}

// start the download running from offset
func (dl *downloader) start(offset int64) (err error) {
	err = dl.open(offset)
	if err != nil {
		_ = dl.close(err)
		return errors.Wrap(err, "failed to open downloader")
	}

	go func() {
		err := dl.download()
		_ = dl.close(err)
		if err != nil && errors.Cause(err) != asyncreader.ErrorStreamAbandoned {
			fs.Errorf(dl.src, "Failed to download: %v", err)
			// FIXME set an error here????
		}
	}()

	return nil
}

// open the file from offset
//
// should be called on a fresh downloader
func (dl *downloader) open(offset int64) (err error) {
	defer log.Trace(dl.src, "offset=%d", offset)("err=%v", &err)
	dl.finished = make(chan struct{})
	defer close(dl.finished)
	dl.downloading = true
	dl.tr = accounting.Stats(dl.ctx).NewTransfer(dl.src)

	size := dl.src.Size()
	if size < 0 {
		// FIXME should just completely download these
		return errors.New("can't open unknown sized file")
	}

	// FIXME hashType needs to ignore when --no-checksum is set too? Which is a VFS flag.
	var rangeOption *fs.RangeOption
	if offset > 0 {
		rangeOption = &fs.RangeOption{Start: offset, End: size - 1}
	}
	in0, err := operations.NewReOpen(dl.ctx, dl.src, fs.Config.LowLevelRetries, dl.item.c.hashOption, rangeOption)
	if err != nil {
		return errors.Wrap(err, "vfs reader: failed to open source file")
	}
	dl.in = dl.tr.Account(in0).WithBuffer() // account and buffer the transfer

	dl.out, err = file.OpenFile(dl.osPath, os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		return errors.Wrap(err, "vfs reader: failed to open cache file")
	}

	dl.offset = offset

	err = file.SetSparse(dl.out)
	if err != nil {
		fs.Debugf(dl.src, "vfs reader: failed to set as a sparse file: %v", err)
	}

	_, err = dl.out.Seek(offset, io.SeekStart)
	if err != nil {
		return errors.Wrap(err, "vfs reader: failed to seek")
	}

	// FIXME set mod time
	// FIXME check checksums

	return nil
}

var errStop = errors.New("vfs downloader: reading stopped")

// stop the downloader if running and close everything
func (dl *downloader) stop() {
	defer log.Trace(dl.src, "")("")

	dl.mu.Lock()
	if !dl.downloading || dl.in == nil {
		dl.mu.Unlock()
		return
	}

	// stop the downloader
	dl.in.StopBuffering()
	oldReader := dl.in.GetReader()
	dl.in.UpdateReader(ioutil.NopCloser(readers.ErrorReader{Err: errStop}))
	err := oldReader.Close()
	if err != nil {
		fs.Debugf(dl.src, "vfs downloader: stop close old failed: %v", err)
	}

	dl.mu.Unlock()

	// wait for downloader to finish...
	<-dl.finished
}

func (dl *downloader) close(inErr error) (err error) {
	defer log.Trace(dl.src, "inErr=%v", err)("err=%v", &err)
	dl.stop()
	dl.mu.Lock()
	if dl.in != nil {
		fs.CheckClose(dl.in, &err)
		dl.in = nil
	}
	if dl.tr != nil {
		dl.tr.Done(inErr)
		dl.tr = nil
	}
	if dl.out != nil {
		fs.CheckClose(dl.out, &err)
		dl.out = nil
	}
	dl._closeWaiters(err)
	dl.downloading = false
	dl.mu.Unlock()
	return nil
}

/*
FIXME
need gating at all the Read/Write sites
need to pass in offset somehow and start the readfile off
need to end when offset is reached
need to be able to quit on demand
Need offset to be passed to NewReOpen
*/
// fetch the (offset, size) block from the remote file
func (dl *downloader) download() (err error) {
	defer log.Trace(dl.src, "")("err=%v", &err)
	_, err = dl.in.WriteTo(dl)
	if err != nil {
		return errors.Wrap(err, "vfs reader: failed to write to cache file")
	}
	return nil
}

// ensure the range is present
func (dl *downloader) ensure(r ranges.Range) (err error) {
	defer log.Trace(dl.src, "r=%+v", r)("err=%v", &err)
	errChan := make(chan error)
	waiter := waiter{
		r:       r,
		errChan: errChan,
	}
	dl.mu.Lock()
	// FIXME racey - might have finished here
	dl.waiters = append(dl.waiters, waiter)
	dl.mu.Unlock()
	return <-errChan
}

// ensure the range is present
func (dl *downloader) running() bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return dl.downloading
}
