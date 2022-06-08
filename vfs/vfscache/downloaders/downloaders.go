package downloaders

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/asyncreader"
	"github.com/rclone/rclone/fs/chunkedreader"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/ranges"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// FIXME implement max downloaders

const (
	// max time a downloader can be idle before closing itself
	maxDownloaderIdleTime = 5 * time.Second
	// max number of bytes a reader should skip over before closing it
	maxSkipBytes = 1024 * 1024
	// time between background kicks of waiters to pick up errors
	backgroundKickerInterval = 5 * time.Second
	// maximum number of errors before declaring dead
	maxErrorCount = 10
	// If a downloader is within this range or --buffer-size
	// whichever is the larger, we will reuse the downloader
	minWindow = 1024 * 1024
)

// Item is the interface that an item to download must obey
type Item interface {
	// FindMissing adjusts r returning a new ranges.Range which only
	// contains the range which needs to be downloaded. This could be
	// empty - check with IsEmpty. It also adjust this to make sure it is
	// not larger than the file.
	FindMissing(r ranges.Range) (outr ranges.Range)

	// HasRange returns true if the current ranges entirely include range
	HasRange(r ranges.Range) bool

	// WriteAtNoOverwrite writes b to the file, but will not overwrite
	// already present ranges.
	//
	// This is used by the downloader to write bytes to the file
	//
	// It returns n the total bytes processed and skipped the number of
	// bytes which were processed but not actually written to the file.
	WriteAtNoOverwrite(b []byte, off int64) (n int, skipped int, err error)
}

// Downloaders is a number of downloader~s and a queue of waiters
// waiting for segments to be downloaded to a file.
type Downloaders struct {
	// Write once - no locking required
	ctx    context.Context
	cancel context.CancelFunc
	item   Item
	opt    *vfscommon.Options
	src    fs.Object // source object
	remote string
	wg     sync.WaitGroup

	// Read write
	mu         sync.Mutex
	dls        []*downloader
	waiters    []waiter
	errorCount int   // number of consecutive errors
	lastErr    error // last error received
}

// waiter is a range we are waiting for and a channel to signal when
// the range is found
type waiter struct {
	r       ranges.Range
	errChan chan<- error
}

// downloader represents a running download for part of a file.
type downloader struct {
	// Write once
	dls  *Downloaders   // parent structure
	quit chan struct{}  // close to quit the downloader
	wg   sync.WaitGroup // to keep track of downloader goroutine
	kick chan struct{}  // kick the downloader when needed

	// Read write
	mu        sync.Mutex
	start     int64 // start offset
	offset    int64 // current offset
	maxOffset int64 // maximum offset we are reading to
	tr        *accounting.Transfer
	in        *accounting.Account // input we are reading from
	skipped   int64               // number of bytes we have skipped sequentially
	_closed   bool                // set to true if downloader is closed
	stop      bool                // set to true if we have called _stop()
}

// New makes a downloader for item
func New(item Item, opt *vfscommon.Options, remote string, src fs.Object) (dls *Downloaders) {
	if src == nil {
		panic("internal error: newDownloaders called with nil src object")
	}
	ctx, cancel := context.WithCancel(context.Background())
	dls = &Downloaders{
		ctx:    ctx,
		cancel: cancel,
		item:   item,
		opt:    opt,
		src:    src,
		remote: remote,
	}
	dls.wg.Add(1)
	go func() {
		defer dls.wg.Done()
		ticker := time.NewTicker(backgroundKickerInterval)
		select {
		case <-ticker.C:
			err := dls.kickWaiters()
			if err != nil {
				fs.Errorf(dls.src, "vfs cache: failed to kick waiters: %v", err)
			}
		case <-ctx.Done():
			break
		}
		ticker.Stop()
	}()

	return dls
}

// Accumulate errors for this downloader
//
// It should be called with
//
//   n bytes downloaded
//   err is error from download
//
// call with lock held
func (dls *Downloaders) _countErrors(n int64, err error) {
	if err == nil && n != 0 {
		if dls.errorCount != 0 {
			fs.Infof(dls.src, "vfs cache: downloader: resetting error count to 0")
			dls.errorCount = 0
			dls.lastErr = nil
		}
		return
	}
	if err != nil {
		//if err != syscall.ENOSPC {
		dls.errorCount++
		//}
		dls.lastErr = err
		fs.Infof(dls.src, "vfs cache: downloader: error count now %d: %v", dls.errorCount, err)
	}
}

func (dls *Downloaders) countErrors(n int64, err error) {
	dls.mu.Lock()
	dls._countErrors(n, err)
	dls.mu.Unlock()
}

// Make a new downloader, starting it to download r
//
// call with lock held
func (dls *Downloaders) _newDownloader(r ranges.Range) (dl *downloader, err error) {
	// defer log.Trace(dls.src, "r=%v", r)("err=%v", &err)

	dl = &downloader{
		kick:      make(chan struct{}, 1),
		quit:      make(chan struct{}),
		dls:       dls,
		start:     r.Pos,
		offset:    r.Pos,
		maxOffset: r.End(),
	}

	err = dl.open(dl.offset)
	if err != nil {
		_ = dl.close(err)
		return nil, fmt.Errorf("failed to open downloader: %w", err)
	}

	dls.dls = append(dls.dls, dl)

	dl.wg.Add(1)
	go func() {
		defer dl.wg.Done()
		n, err := dl.download()
		_ = dl.close(err)
		dl.dls.countErrors(n, err)
		if err != nil {
			fs.Errorf(dl.dls.src, "vfs cache: failed to download: %v", err)
		}
		err = dl.dls.kickWaiters()
		if err != nil {
			fs.Errorf(dl.dls.src, "vfs cache: failed to kick waiters: %v", err)
		}
	}()

	return dl, nil
}

// _removeClosed() removes any downloaders which are closed.
//
// Call with the mutex held
func (dls *Downloaders) _removeClosed() {
	newDownloaders := dls.dls[:0]
	for _, dl := range dls.dls {
		if !dl.closed() {
			newDownloaders = append(newDownloaders, dl)
		}
	}
	dls.dls = newDownloaders
}

// Close all running downloaders and return any unfulfilled waiters
// with inErr
func (dls *Downloaders) Close(inErr error) (err error) {
	dls.mu.Lock()
	defer dls.mu.Unlock()
	dls._removeClosed()
	for _, dl := range dls.dls {
		dls.mu.Unlock()
		closeErr := dl.stopAndClose(inErr)
		dls.mu.Lock()
		if closeErr != nil && err != nil {
			err = closeErr
		}
	}
	dls.cancel()
	// dls may have entered the periodical (every 5 seconds) kickWaiters() call
	// unlock the mutex to allow it to finish so that we can get its dls.wg.Done()
	dls.mu.Unlock()
	dls.wg.Wait()
	dls.mu.Lock()
	dls.dls = nil
	dls._dispatchWaiters()
	dls._closeWaiters(inErr)
	return err
}

// Download the range passed in returning when it has been downloaded
// with an error from the downloading go routine.
func (dls *Downloaders) Download(r ranges.Range) (err error) {
	// defer log.Trace(dls.src, "r=%+v", r)("err=%v", &err)

	dls.mu.Lock()

	errChan := make(chan error)
	waiter := waiter{
		r:       r,
		errChan: errChan,
	}

	err = dls._ensureDownloader(r)
	if err != nil {
		dls.mu.Unlock()
		return err
	}

	dls.waiters = append(dls.waiters, waiter)
	dls.mu.Unlock()
	return <-errChan
}

// close any waiters with the error passed in
//
// call with lock held
func (dls *Downloaders) _closeWaiters(err error) {
	for _, waiter := range dls.waiters {
		waiter.errChan <- err
	}
	dls.waiters = nil
}

// ensure a downloader is running for the range if required.  If one isn't found
// then it starts it.
//
// call with lock held
func (dls *Downloaders) _ensureDownloader(r ranges.Range) (err error) {
	// defer log.Trace(dls.src, "r=%v", r)("err=%v", &err)

	// The window includes potentially unread data in the buffer
	window := int64(fs.GetConfig(context.TODO()).BufferSize)

	// Increase the read range by the read ahead if set
	if dls.opt.ReadAhead > 0 {
		r.Size += int64(dls.opt.ReadAhead)
	}

	// We may be reopening a downloader after a failure here or
	// doing a tentative prefetch so check to see that we haven't
	// read some stuff already.
	//
	// Clip r to stuff which needs downloading
	r = dls.item.FindMissing(r)

	// If the range is entirely present then we only need to start a
	// downloader if the window isn't full.
	startNew := true
	if r.IsEmpty() {
		// Make a new range which includes the window
		rWindow := r
		rWindow.Size += window

		// Clip rWindow to stuff which needs downloading
		rWindowClipped := dls.item.FindMissing(rWindow)

		// If rWindowClipped is empty then don't start a new downloader
		// if there isn't an existing one as there is no data within the
		// window which needs downloading. We do want to kick an
		// existing one though to stop it timing out.
		if rWindowClipped.IsEmpty() {
			// Don't start any more downloaders
			startNew = false
			// Start downloading at the start of the unread window
			// This likely has been downloaded already but it will
			// kick the downloader
			r.Pos = rWindow.End()
		} else {
			// Start downloading at the start of the unread window
			r.Pos = rWindowClipped.Pos
		}
		// But don't write anything for the moment
		r.Size = 0
	}

	// If buffer size is less than minWindow then make it that
	if window < minWindow {
		window = minWindow
	}

	var dl *downloader
	// Look through downloaders to find one in range
	// If there isn't one then start a new one
	dls._removeClosed()
	for _, dl = range dls.dls {
		start, offset := dl.getRange()

		// The downloader's offset to offset+window is the gap
		// in which we would like to re-use this
		// downloader. The downloader will never reach before
		// start and offset+windows is too far away - we'd
		// rather start another downloader.
		// fs.Debugf(nil, "r=%v start=%d, offset=%d, found=%v", r, start, offset, r.Pos >= start && r.Pos < offset+window)
		if r.Pos >= start && r.Pos < offset+window {
			// Found downloader which will soon have our data
			dl.setRange(r)
			return nil
		}
	}
	if !startNew {
		return nil
	}
	// Downloader not found so start a new one
	_, err = dls._newDownloader(r)
	if err != nil {
		dls._countErrors(0, err)
		return fmt.Errorf("failed to start downloader: %w", err)
	}
	return err
}

// EnsureDownloader makes sure a downloader is running for the range
// passed in.  If one isn't found then it starts it.
//
// It does not wait for the range to be downloaded
func (dls *Downloaders) EnsureDownloader(r ranges.Range) (err error) {
	dls.mu.Lock()
	defer dls.mu.Unlock()
	return dls._ensureDownloader(r)
}

// _dispatchWaiters() sends any waiters which have completed back to
// their callers.
//
// Call with the mutex held
func (dls *Downloaders) _dispatchWaiters() {
	if len(dls.waiters) == 0 {
		return
	}

	newWaiters := dls.waiters[:0]
	for _, waiter := range dls.waiters {
		if dls.item.HasRange(waiter.r) {
			waiter.errChan <- nil
		} else {
			newWaiters = append(newWaiters, waiter)
		}
	}
	dls.waiters = newWaiters
}

// Send any waiters which have completed back to their callers and make sure
// there is a downloader appropriate for each waiter
func (dls *Downloaders) kickWaiters() (err error) {
	dls.mu.Lock()
	defer dls.mu.Unlock()

	dls._dispatchWaiters()

	if len(dls.waiters) == 0 {
		return nil
	}

	// Make sure each waiter has a downloader
	// This is an O(waiters*Downloaders) algorithm
	// However the number of waiters and the number of downloaders
	// are both expected to be small.
	for _, waiter := range dls.waiters {
		err = dls._ensureDownloader(waiter.r)
		if err != nil {
			// Failures here will be retried by background kicker
			fs.Errorf(dls.src, "vfs cache: restart download failed: %v", err)
		}
	}
	if fserrors.IsErrNoSpace(dls.lastErr) {
		fs.Errorf(dls.src, "vfs cache: cache is out of space %d/%d: last error: %v", dls.errorCount, maxErrorCount, dls.lastErr)
		dls._closeWaiters(dls.lastErr)
		return dls.lastErr
	}

	if dls.errorCount > maxErrorCount {
		fs.Errorf(dls.src, "vfs cache: too many errors %d/%d: last error: %v", dls.errorCount, maxErrorCount, dls.lastErr)
		dls._closeWaiters(dls.lastErr)
		return dls.lastErr
	}

	return nil
}

// Write writes len(p) bytes from p to the underlying data stream. It
// returns the number of bytes written from p (0 <= n <= len(p)) and
// any error encountered that caused the write to stop early. Write
// must return a non-nil error if it returns n < len(p). Write must
// not modify the slice data, even temporarily.
//
// Implementations must not retain p.
func (dl *downloader) Write(p []byte) (n int, err error) {
	// defer log.Trace(dl.dls.src, "p_len=%d", len(p))("n=%d, err=%v", &n, &err)

	// Kick the waiters on exit if some characters received
	defer func() {
		if n <= 0 {
			return
		}
		if waitErr := dl.dls.kickWaiters(); waitErr != nil {
			fs.Errorf(dl.dls.src, "vfs cache: download write: failed to kick waiters: %v", waitErr)
			if err == nil {
				err = waitErr
			}
		}
	}()

	dl.mu.Lock()
	defer dl.mu.Unlock()

	// Wait here if we have reached maxOffset until
	// - we are quitting
	// - we get kicked
	// - timeout happens
loop:
	for dl.offset >= dl.maxOffset {
		var timeout = time.NewTimer(maxDownloaderIdleTime)
		dl.mu.Unlock()
		select {
		case <-dl.quit:
			dl.mu.Lock()
			timeout.Stop()
			break loop
		case <-dl.kick:
			dl.mu.Lock()
			timeout.Stop()
		case <-timeout.C:
			// stop any future reading
			dl.mu.Lock()
			if !dl.stop {
				fs.Debugf(dl.dls.src, "vfs cache: stopping download thread as it timed out")
				dl._stop()
			}
			break loop
		}
	}

	n, skipped, err := dl.dls.item.WriteAtNoOverwrite(p, dl.offset)
	if skipped == n {
		dl.skipped += int64(skipped)
	} else {
		dl.skipped = 0
	}
	dl.offset += int64(n)

	// Kill this downloader if skipped too many bytes
	if !dl.stop && dl.skipped > maxSkipBytes {
		fs.Debugf(dl.dls.src, "vfs cache: stopping download thread as it has skipped %d bytes", dl.skipped)
		dl._stop()
	}

	// If running without a async buffer then stop now as
	// StopBuffering has no effect if the Account wasn't buffered
	// so we need to stop manually now rather than wait for the
	// AsyncReader to stop.
	if dl.stop && !dl.in.HasBuffer() {
		err = asyncreader.ErrorStreamAbandoned
	}
	return n, err
}

// open the file from offset
//
// should be called on a fresh downloader
func (dl *downloader) open(offset int64) (err error) {
	// defer log.Trace(dl.dls.src, "offset=%d", offset)("err=%v", &err)
	dl.tr = accounting.Stats(dl.dls.ctx).NewTransfer(dl.dls.src)

	size := dl.dls.src.Size()
	if size < 0 {
		// FIXME should just completely download these
		return errors.New("can't open unknown sized file")
	}

	// FIXME hashType needs to ignore when --no-checksum is set too? Which is a VFS flag.
	// var rangeOption *fs.RangeOption
	// if offset > 0 {
	// 	rangeOption = &fs.RangeOption{Start: offset, End: size - 1}
	// }
	// in0, err := operations.NewReOpen(dl.dls.ctx, dl.dls.src, ci.LowLevelRetries, dl.dls.item.c.hashOption, rangeOption)

	in0 := chunkedreader.New(context.TODO(), dl.dls.src, int64(dl.dls.opt.ChunkSize), int64(dl.dls.opt.ChunkSizeLimit))
	_, err = in0.Seek(offset, 0)
	if err != nil {
		return fmt.Errorf("vfs reader: failed to open source file: %w", err)
	}
	dl.in = dl.tr.Account(dl.dls.ctx, in0).WithBuffer() // account and buffer the transfer

	dl.offset = offset

	// FIXME set mod time
	// FIXME check checksums

	return nil
}

// close the downloader
func (dl *downloader) close(inErr error) (err error) {
	// defer log.Trace(dl.dls.src, "inErr=%v", err)("err=%v", &err)
	checkErr := func(e error) {
		if e == nil || errors.Is(err, asyncreader.ErrorStreamAbandoned) {
			return
		}
		err = e
	}
	dl.mu.Lock()
	if dl.in != nil {
		checkErr(dl.in.Close())
		dl.in = nil
	}
	if dl.tr != nil {
		dl.tr.Done(dl.dls.ctx, inErr)
		dl.tr = nil
	}
	dl._closed = true
	dl.mu.Unlock()
	return nil
}

// closed returns true if the downloader has been closed already
func (dl *downloader) closed() bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return dl._closed
}

// stop the downloader if running
//
// Call with the mutex held
func (dl *downloader) _stop() {
	// defer log.Trace(dl.dls.src, "")("")

	// exit if have already called _stop
	if dl.stop {
		return
	}
	dl.stop = true

	// Signal quit now to unblock the downloader
	close(dl.quit)

	// stop the downloader by stopping the async reader buffering
	// any more input. This causes all the stuff in the async
	// buffer (which can be many MiB) to be written to the disk
	// before exiting.
	if dl.in != nil {
		dl.in.StopBuffering()
	}
}

// stop the downloader if running then close it with the error passed in
func (dl *downloader) stopAndClose(inErr error) (err error) {
	// Stop the downloader by closing its input
	dl.mu.Lock()
	dl._stop()
	dl.mu.Unlock()
	// wait for downloader to finish...
	// do this without mutex as asyncreader
	// calls back into Write() which needs the lock
	dl.wg.Wait()
	return dl.close(inErr)
}

// Start downloading to the local file starting at offset until maxOffset.
func (dl *downloader) download() (n int64, err error) {
	// defer log.Trace(dl.dls.src, "")("err=%v", &err)
	n, err = dl.in.WriteTo(dl)
	if err != nil && !errors.Is(err, asyncreader.ErrorStreamAbandoned) {
		return n, fmt.Errorf("vfs reader: failed to write to cache file: %w", err)
	}

	return n, nil
}

// setRange makes sure the downloader is downloading the range passed in
func (dl *downloader) setRange(r ranges.Range) {
	// defer log.Trace(dl.dls.src, "r=%v", r)("")
	dl.mu.Lock()
	maxOffset := r.End()
	if maxOffset > dl.maxOffset {
		dl.maxOffset = maxOffset
	}
	dl.mu.Unlock()
	// fs.Debugf(dl.dls.src, "kicking downloader with maxOffset %d", maxOffset)
	select {
	case dl.kick <- struct{}{}:
	default:
	}
}

// get the current range this downloader is working on
func (dl *downloader) getRange() (start, offset int64) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return dl.start, dl.offset
}
