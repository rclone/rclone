// download_cache.go manages shared temp file downloads for media files.
//
// # Why this exists
//
// Google Photos download URLs do NOT support HTTP Range requests. Every
// download must fetch the entire file from byte 0. However, rclone's VFS
// layer and chunkedreader frequently close and re-open files at different
// offsets (e.g. when seeking during video playback). Without caching, each
// seek would trigger a full re-download from the beginning.
//
// # How it works
//
//  1. When Open() is called for a media item, getOrStart() either returns
//     an existing download or starts a new one. A background goroutine
//     streams the file to a temp file on disk.
//  2. The returned cachedReader reads from the temp file. If the reader's
//     position is ahead of what's been downloaded so far, it spin-waits
//     (10ms polls) until the data arrives.
//  3. Multiple Open() calls for the same media_key share the same download
//     and temp file (reference counted via refCount).
//  4. cachedReader implements fs.RangeSeeker, so rclone can seek without
//     closing and re-opening. Seeking backwards is instant since the data
//     is already on disk.
//  5. When all readers close, a 30-second grace period allows for quick
//     re-opens before the temp file is cleaned up. Eviction is tied to the
//     entry itself, so a stale eviction never touches a newer download for
//     the same media_key.
//  6. Temp files are unlinked right after creation (except on Windows), so
//     a crashed or killed rclone never leaves them behind.
//  7. A failed download is never reused: the next Open() starts a fresh
//     download, and the failed one is closed once its readers release it.
//
// # Known limitations
//
//   - No maximum cache size: every opened file downloads fully to disk.
//   - Spin-wait polling: should use sync.Cond for efficiency.

package gphotosmobile

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/fs"
)

// downloadCacheGrace is how long an unreferenced download is kept so quick
// re-opens (e.g. chunkedreader re-creating) can reuse it.
var downloadCacheGrace = 30 * time.Second

// downloadCache manages shared temp file downloads.
// Multiple Open() calls for the same media key share a single download,
// avoiding re-downloading the entire file when rclone's VFS creates
// new chunkedreaders for seeks.
type downloadCache struct {
	mu      sync.Mutex
	entries map[string]*downloadEntry
	api     *MobileAPI
	fetch   func(ctx context.Context, entry *downloadEntry, mediaKey string) // runs a download, replaceable in tests
}

// downloadEntry represents a single file being downloaded/cached
type downloadEntry struct {
	mu        sync.Mutex // protects tmpFile, done and dlErr
	mediaKey  string
	tmpFile   *os.File
	path      string             // temp file path if it could not be unlinked while open
	cancel    context.CancelFunc // stops the background download
	written   int64              // bytes written so far (atomic read OK)
	totalSize int64              // expected total size (-1 if unknown)
	done      bool               // download complete
	dlErr     error              // download error
	refCount  int                // number of active readers, protected by downloadCache.mu
	evictGen  int                // invalidates pending evictions, protected by downloadCache.mu
	startTime time.Time
}

func newDownloadCache(api *MobileAPI) *downloadCache {
	dc := &downloadCache{
		entries: make(map[string]*downloadEntry),
		api:     api,
	}
	dc.fetch = dc.download
	return dc
}

// createTempFile creates the temp file backing a download.
//
// The file is unlinked straight away so its space is reclaimed as soon as
// it is closed, even if rclone is killed. Windows refuses to remove open
// files, so there the path is returned and removed on close instead.
func createTempFile() (*os.File, string, error) {
	tmpFile, err := os.CreateTemp("", "gphotosmobile_*.tmp")
	if err != nil {
		return nil, "", err
	}
	if os.Remove(tmpFile.Name()) == nil {
		return tmpFile, "", nil
	}
	return tmpFile, tmpFile.Name(), nil
}

// getOrStart returns an existing download entry or starts a new one
func (dc *downloadCache) getOrStart(ctx context.Context, mediaKey string, totalSize int64) (*downloadEntry, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if entry, ok := dc.entries[mediaKey]; ok {
		if !entry.failed() {
			// Reuse existing download and cancel any pending eviction
			entry.refCount++
			entry.evictGen++
			fs.Debugf(nil, "Download cache hit for %s (written=%d, refs=%d)",
				mediaKey, atomic.LoadInt64(&entry.written), entry.refCount)
			return entry, nil
		}
		// Don't hand out a failed download: detach it so it is closed
		// once its remaining readers release it, and start a new one.
		fs.Debugf(nil, "Download cache retrying failed download for %s", mediaKey)
		delete(dc.entries, mediaKey)
		if entry.refCount <= 0 {
			entry.close()
		}
	}

	// Start new download
	tmpFile, path, err := createTempFile()
	if err != nil {
		return nil, err
	}

	// Start background download with a detached context so that
	// cancellation of the first reader doesn't abort the shared download.
	// The context is cancelled when the entry is closed.
	dlCtx, cancel := context.WithCancel(context.Background())
	entry := &downloadEntry{
		mediaKey:  mediaKey,
		tmpFile:   tmpFile,
		path:      path,
		cancel:    cancel,
		totalSize: totalSize,
		refCount:  1,
		startTime: time.Now(),
	}

	dc.entries[mediaKey] = entry

	go dc.fetch(dlCtx, entry, mediaKey)

	return entry, nil
}

// download fetches the file and writes to the entry's temp file
func (dc *downloadCache) download(ctx context.Context, entry *downloadEntry, mediaKey string) {
	downloadURL, err := dc.api.GetDownloadURL(ctx, mediaKey)
	if err != nil {
		entry.finish(err)
		return
	}

	body, err := dc.api.DownloadFile(ctx, downloadURL)
	if err != nil {
		entry.finish(err)
		return
	}
	defer func() { _ = body.Close() }()

	buf := make([]byte, 256*1024)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			entry.mu.Lock()
			if entry.done {
				// Entry was closed by eviction or shutdown
				entry.mu.Unlock()
				return
			}
			_, werr := entry.tmpFile.Write(buf[:n])
			if werr != nil {
				entry.dlErr = werr
				entry.done = true
				entry.mu.Unlock()
				return
			}
			atomic.AddInt64(&entry.written, int64(n))
			entry.mu.Unlock()
		}
		if readErr != nil {
			if readErr == io.EOF {
				readErr = nil
			}
			entry.finish(readErr)
			return
		}
	}
}

// finish marks the download complete with err unless it already finished
func (e *downloadEntry) finish(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.done {
		e.dlErr = err
		e.done = true
	}
}

// failed reports whether the download finished with an error
func (e *downloadEntry) failed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.done && e.dlErr != nil
}

// close stops the download and releases the temp file
func (e *downloadEntry) close() {
	e.cancel()
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.done {
		e.dlErr = errors.New("download cache entry closed")
		e.done = true
	}
	_ = e.tmpFile.Close()
	if e.path != "" {
		_ = os.Remove(e.path)
	}
}

// shutdown closes all temp files and removes them from disk.
// Called during Fs.Shutdown and at exit to ensure no temp files are leaked.
func (dc *downloadCache) shutdown() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	for key, entry := range dc.entries {
		entry.close()
		delete(dc.entries, key)
	}
}

// release drops a reader's reference to entry and, once no readers
// remain, evicts it after a grace period to allow for quick re-opens
// (e.g. chunkedreader re-creating)
func (dc *downloadCache) release(entry *downloadEntry) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	entry.refCount--
	if entry.refCount > 0 {
		return
	}

	// A detached entry (failed and replaced, or shut down) has no reuse
	// to wait for, so close it now
	if dc.entries[entry.mediaKey] != entry {
		entry.close()
		return
	}

	entry.evictGen++
	gen := entry.evictGen
	time.AfterFunc(downloadCacheGrace, func() {
		dc.mu.Lock()
		defer dc.mu.Unlock()

		// Skip if the entry was reopened or already removed
		if entry.evictGen != gen || dc.entries[entry.mediaKey] != entry {
			return
		}
		delete(dc.entries, entry.mediaKey)
		entry.close()
		fs.Debugf(nil, "Download cache evicted %s", entry.mediaKey)
	})
}

// cachedReader reads from a shared downloadEntry
type cachedReader struct {
	entry   *downloadEntry
	readPos int64
	dc      *downloadCache
	closed  bool
}

// Read reads from the cached temp file, waiting for data if needed
func (r *cachedReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, errors.New("reader is closed")
	}

	for {
		written := atomic.LoadInt64(&r.entry.written)
		avail := written - r.readPos

		r.entry.mu.Lock()
		done := r.entry.done
		dlErr := r.entry.dlErr
		r.entry.mu.Unlock()

		if avail > 0 {
			toRead := int64(len(p))
			if toRead > avail {
				toRead = avail
			}

			n, err := r.entry.tmpFile.ReadAt(p[:toRead], r.readPos)
			if n > 0 {
				r.readPos += int64(n)
			}
			if err == io.EOF && !done {
				// Temp file EOF but download ongoing
				if n > 0 {
					return n, nil
				}
				continue
			}
			return n, err
		}

		if done {
			if dlErr != nil {
				return 0, dlErr
			}
			return 0, io.EOF
		}

		// Wait for more data
		time.Sleep(10 * time.Millisecond)
	}
}

// RangeSeek implements fs.RangeSeeker
func (r *cachedReader) RangeSeek(ctx context.Context, offset int64, whence int, length int64) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = r.readPos + offset
	case io.SeekEnd:
		if r.entry.totalSize > 0 {
			newPos = r.entry.totalSize + offset
		} else {
			newPos = atomic.LoadInt64(&r.entry.written) + offset
		}
	}

	if newPos < 0 {
		return 0, errors.New("negative seek position")
	}

	r.readPos = newPos
	return newPos, nil
}

// Close releases this reader's reference to the shared download
func (r *cachedReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	r.dc.release(r.entry)
	return nil
}
