package vfscache

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/asyncreader"
	"github.com/rclone/rclone/fs/chunkedreader"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/lib/ranges"
)

// FIXME implement max downloaders

const (
	// max time a downloader can be idle before closing itself
	maxDownloaderIdleTime = 5 * time.Second
	// max number of bytes a reader should skip over before closing it
	maxSkipBytes = 1024 * 1024
	// time between background kicks of waiters to pick up errors
	backgroundKickerInterval = 5 * time.Second
)

// downloaders is a number of downloader~s and a queue of waiters
// waiting for segments to be downloaded.
type downloaders struct {
	// Write once - no locking required
	ctx    context.Context
	cancel context.CancelFunc
	item   *Item
	src    fs.Object // source object
	remote string
	fcache fs.Fs // destination Fs
	osPath string
	wg     sync.WaitGroup

	// Read write
	mu      sync.Mutex
	dls     []*downloader
	waiters []waiter
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
	dls  *downloaders   // parent structure
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

func newDownloaders(item *Item, fcache fs.Fs, remote string, src fs.Object) (dls *downloaders) {
	if src == nil {
		panic("internal error: newDownloaders called with nil src object")
	}
	ctx, cancel := context.WithCancel(context.Background())
	dls = &downloaders{
		ctx:    ctx,
		cancel: cancel,
		item:   item,
		src:    src,
		remote: remote,
		fcache: fcache,
		osPath: item.c.toOSPath(remote),
	}
	dls.wg.Add(1)
	go func() {
		defer dls.wg.Done()
		ticker := time.NewTicker(backgroundKickerInterval)
		select {
		case <-ticker.C:
			err := dls.kickWaiters()
			if err != nil {
				fs.Errorf(dls.src, "Failed to kick waiters: %v", err)
			}
		case <-ctx.Done():
			break
		}
		ticker.Stop()
	}()

	return dls
}

// Make a new downloader, starting it to download r
//
// call with lock held
func (dls *downloaders) _newDownloader(r ranges.Range) (dl *downloader, err error) {
	defer log.Trace(dls.src, "r=%v", r)("err=%v", &err)

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
		return nil, errors.Wrap(err, "failed to open downloader")
	}

	dls.dls = append(dls.dls, dl)

	dl.wg.Add(1)
	go func() {
		defer dl.wg.Done()
		err := dl.download()
		_ = dl.close(err)
		if err != nil {
			fs.Errorf(dl.dls.src, "Failed to download: %v", err)
		}
		err = dl.dls.kickWaiters()
		if err != nil {
			fs.Errorf(dl.dls.src, "Failed to kick waiters: %v", err)
		}
	}()

	return dl, nil
}

// _removeClosed() removes any downloaders which are closed.
//
// Call with the mutex held
func (dls *downloaders) _removeClosed() {
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
func (dls *downloaders) close(inErr error) (err error) {
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
	dls.wg.Wait()
	dls.dls = nil
	dls._dispatchWaiters()
	dls._closeWaiters(inErr)
	return err
}

// Ensure a downloader is running to download r
func (dls *downloaders) ensure(r ranges.Range) (err error) {
	defer log.Trace(dls.src, "r=%+v", r)("err=%v", &err)

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
func (dls *downloaders) _closeWaiters(err error) {
	for _, waiter := range dls.waiters {
		waiter.errChan <- err
	}
	dls.waiters = nil
}

// ensure a downloader is running for the range if required.  If one isn't found
// then it starts it.
//
// call with lock held
func (dls *downloaders) _ensureDownloader(r ranges.Range) (err error) {
	// FIXME this window could be a different config var?
	window := int64(fs.Config.BufferSize)

	// We may be reopening a downloader after a failure here or
	// doing a tentative prefetch so check to see that we haven't
	// read some stuff already.
	//
	// Clip r to stuff which needs downloading
	r = dls.item.findMissing(r)

	// If the range is entirely present then we only need to start a
	// dowloader if the window isn't full.
	if r.IsEmpty() {
		// Make a new range which includes the window
		rWindow := r
		if rWindow.Size < window {
			rWindow.Size = window
		}
		// Clip rWindow to stuff which needs downloading
		rWindow = dls.item.findMissing(rWindow)
		// If rWindow is empty then just return without starting a
		// downloader as there is no data within the window which needs
		// downloading.
		if rWindow.IsEmpty() {
			return nil
		}
		// Start downloading at the start of the unread window
		r.Pos = rWindow.Pos
		// But don't write anything for the moment
		r.Size = 0
	}

	var dl *downloader
	// Look through downloaders to find one in range
	// If there isn't one then start a new one
	dls._removeClosed()
	for _, dl = range dls.dls {
		start, maxOffset := dl.getRange()

		// The downloader's offset to offset+window is the gap
		// in which we would like to re-use this
		// downloader. The downloader will never reach before
		// start and maxOffset+windows is too far away - we'd
		// rather start another downloader.
		// fs.Debugf(nil, "r=%v start=%d, maxOffset=%d, found=%v", r, start, maxOffset, r.Pos >= start && r.Pos < maxOffset+window)
		if r.Pos >= start && r.Pos < maxOffset+window {
			// Found downloader which will soon have our data
			dl.setRange(r)
			return nil
		}
	}
	// Downloader not found so start a new one
	dl, err = dls._newDownloader(r)
	if err != nil {
		return errors.Wrap(err, "failed to start downloader")
	}
	return err
}

// ensure a downloader is running for offset if required.  If one
// isn't found then it starts it
func (dls *downloaders) ensureDownloader(r ranges.Range) (err error) {
	dls.mu.Lock()
	defer dls.mu.Unlock()
	return dls._ensureDownloader(r)
}

// _dispatchWaiters() sends any waiters which have completed back to
// their callers.
//
// Call with the mutex held
func (dls *downloaders) _dispatchWaiters() {
	if len(dls.waiters) == 0 {
		return
	}

	newWaiters := dls.waiters[:0]
	for _, waiter := range dls.waiters {
		if dls.item.hasRange(waiter.r) {
			waiter.errChan <- nil
		} else {
			newWaiters = append(newWaiters, waiter)
		}
	}
	dls.waiters = newWaiters
}

// Send any waiters which have completed back to their callers and make sure
// there is a downloader appropriate for each waiter
func (dls *downloaders) kickWaiters() (err error) {
	dls.mu.Lock()
	defer dls.mu.Unlock()

	dls._dispatchWaiters()

	if len(dls.waiters) == 0 {
		return nil
	}

	// Make sure each waiter has a downloader
	// This is an O(waiters*downloaders) algorithm
	// However the number of waiters and the number of downloaders
	// are both expected to be small.
	for _, waiter := range dls.waiters {
		err = dls._ensureDownloader(waiter.r)
		if err != nil {
			// Failures here will be retried by background kicker
			fs.Errorf(dls.src, "Restart download failed: %v", err)
		}
	}

	if true {

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
	defer log.Trace(dl.dls.src, "p_len=%d", len(p))("n=%d, err=%v", &n, &err)

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
	if dl.offset >= dl.maxOffset {
		var timeout = time.NewTimer(maxDownloaderIdleTime)
		dl.mu.Unlock()
		select {
		case <-dl.quit:
			dl.mu.Lock()
			timeout.Stop()
		case <-dl.kick:
			dl.mu.Lock()
			timeout.Stop()
		case <-timeout.C:
			// stop any future reading
			dl.mu.Lock()
			if !dl.stop {
				fs.Debugf(dl.dls.src, "stopping download thread as it timed out")
				dl._stop()
			}
		}
	}

	n, skipped, err := dl.dls.item.writeAtNoOverwrite(p, dl.offset)
	if skipped == n {
		dl.skipped += int64(skipped)
	} else {
		dl.skipped = 0
	}
	dl.offset += int64(n)

	// Kill this downloader if skipped too many bytes
	if !dl.stop && dl.skipped > maxSkipBytes {
		fs.Debugf(dl.dls.src, "stopping download thread as it has skipped %d bytes", dl.skipped)
		dl._stop()
	}
	return n, err
}

// open the file from offset
//
// should be called on a fresh downloader
func (dl *downloader) open(offset int64) (err error) {
	defer log.Trace(dl.dls.src, "offset=%d", offset)("err=%v", &err)
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
	// in0, err := operations.NewReOpen(dl.dls.ctx, dl.dls.src, fs.Config.LowLevelRetries, dl.dls.item.c.hashOption, rangeOption)

	in0 := chunkedreader.New(context.TODO(), dl.dls.src, int64(dl.dls.item.c.opt.ChunkSize), int64(dl.dls.item.c.opt.ChunkSizeLimit))
	_, err = in0.Seek(offset, 0)
	if err != nil {
		return errors.Wrap(err, "vfs reader: failed to open source file")
	}
	dl.in = dl.tr.Account(in0).WithBuffer() // account and buffer the transfer

	dl.offset = offset

	// FIXME set mod time
	// FIXME check checksums

	return nil
}

// close the downloader
func (dl *downloader) close(inErr error) (err error) {
	defer log.Trace(dl.dls.src, "inErr=%v", err)("err=%v", &err)
	checkErr := func(e error) {
		if e == nil || errors.Cause(err) == asyncreader.ErrorStreamAbandoned {
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
		dl.tr.Done(inErr)
		dl.tr = nil
	}
	dl._closed = true
	dl.mu.Unlock()
	return nil
}

// closed returns true if the downloader has been closed alread
func (dl *downloader) closed() bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return dl._closed
}

// stop the downloader if running
//
// Call with the mutex held
func (dl *downloader) _stop() {
	defer log.Trace(dl.dls.src, "")("")

	// exit if have already called _stop
	if dl.stop {
		return
	}
	dl.stop = true

	// Signal quit now to unblock the downloader
	close(dl.quit)

	// stop the downloader by stopping the async reader buffering
	// any more input. This causes all the stuff in the async
	// buffer (which can be many MB) to be written to the disk
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
func (dl *downloader) download() (err error) {
	defer log.Trace(dl.dls.src, "")("err=%v", &err)
	_, err = dl.in.WriteTo(dl)
	if err != nil && errors.Cause(err) != asyncreader.ErrorStreamAbandoned {
		return errors.Wrap(err, "vfs reader: failed to write to cache file")
	}
	return nil
}

// setRange makes sure the downloader is downloading the range passed in
func (dl *downloader) setRange(r ranges.Range) {
	dl.mu.Lock()
	maxOffset := r.End()
	if maxOffset > dl.maxOffset {
		dl.maxOffset = maxOffset
		// fs.Debugf(dl.dls.src, "kicking downloader with maxOffset %d", maxOffset)
		select {
		case dl.kick <- struct{}{}:
		default:
		}
	}
	dl.mu.Unlock()
}

// get the current range this downloader is working on
func (dl *downloader) getRange() (start, maxOffset int64) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return dl.start, dl.maxOffset
}
