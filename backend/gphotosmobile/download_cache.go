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
//     re-opens before the temp file is cleaned up.
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

// downloadCache manages shared temp file downloads.
// Multiple Open() calls for the same media key share a single download,
// avoiding re-downloading the entire file when rclone's VFS creates
// new chunkedreaders for seeks.
type downloadCache struct {
	mu      sync.Mutex
	entries map[string]*downloadEntry
	api     *MobileAPI
}

// downloadEntry represents a single file being downloaded/cached
type downloadEntry struct {
	mu        sync.Mutex
	mediaKey  string
	tmpFile   *os.File
	path      string
	written   int64 // bytes written so far (atomic read OK)
	totalSize int64 // expected total size (-1 if unknown)
	done      bool  // download complete
	dlErr     error // download error
	refCount  int32 // number of active readers (atomic)
	startTime time.Time
}

func newDownloadCache(api *MobileAPI) *downloadCache {
	return &downloadCache{
		entries: make(map[string]*downloadEntry),
		api:     api,
	}
}

// getOrStart returns an existing download entry or starts a new one
func (dc *downloadCache) getOrStart(ctx context.Context, mediaKey string, totalSize int64) (*downloadEntry, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if entry, ok := dc.entries[mediaKey]; ok {
		// Reuse existing download
		atomic.AddInt32(&entry.refCount, 1)
		fs.Debugf(nil, "Download cache hit for %s (written=%d, done=%v, refs=%d)",
			mediaKey, atomic.LoadInt64(&entry.written), entry.done, atomic.LoadInt32(&entry.refCount))
		return entry, nil
	}

	// Start new download
	tmpFile, err := os.CreateTemp("", "gphotosmobile_*.tmp")
	if err != nil {
		return nil, err
	}

	entry := &downloadEntry{
		mediaKey:  mediaKey,
		tmpFile:   tmpFile,
		path:      tmpFile.Name(),
		totalSize: totalSize,
		refCount:  1,
		startTime: time.Now(),
	}

	dc.entries[mediaKey] = entry

	// Start background download with a detached context so that
	// cancellation of the first reader doesn't abort the shared download.
	go dc.download(context.Background(), entry, mediaKey)

	return entry, nil
}

// download fetches the file and writes to the entry's temp file
func (dc *downloadCache) download(ctx context.Context, entry *downloadEntry, mediaKey string) {
	downloadURL, err := dc.api.GetDownloadURL(ctx, mediaKey)
	if err != nil {
		entry.mu.Lock()
		entry.dlErr = err
		entry.done = true
		entry.mu.Unlock()
		return
	}

	body, err := dc.api.DownloadFile(ctx, downloadURL)
	if err != nil {
		entry.mu.Lock()
		entry.dlErr = err
		entry.done = true
		entry.mu.Unlock()
		return
	}
	defer func() { _ = body.Close() }()

	buf := make([]byte, 256*1024)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			entry.mu.Lock()
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
			entry.mu.Lock()
			if readErr != io.EOF {
				entry.dlErr = readErr
			}
			entry.done = true
			entry.mu.Unlock()
			return
		}
	}
}

// shutdown closes all temp files and removes them from disk.
// Called during Fs.Shutdown to ensure no temp files are leaked.
func (dc *downloadCache) shutdown() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	for key, entry := range dc.entries {
		_ = entry.tmpFile.Close()
		_ = os.Remove(entry.path)
		delete(dc.entries, key)
	}
}

// release decrements ref count and cleans up if no readers remain
func (dc *downloadCache) release(mediaKey string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	entry, ok := dc.entries[mediaKey]
	if !ok {
		return
	}

	refs := atomic.AddInt32(&entry.refCount, -1)
	if refs <= 0 {
		// No more readers - schedule cleanup after a delay
		// to allow for quick re-opens (e.g. chunkedreader re-creating)
		go func() {
			time.Sleep(30 * time.Second)
			dc.mu.Lock()
			defer dc.mu.Unlock()

			// Re-check: someone may have opened it again
			if atomic.LoadInt32(&entry.refCount) <= 0 {
				_ = entry.tmpFile.Close()
				_ = os.Remove(entry.path)
				delete(dc.entries, mediaKey)
				fs.Debugf(nil, "Download cache evicted %s", mediaKey)
			}
		}()
	}
}

// cachedReader reads from a shared downloadEntry
type cachedReader struct {
	entry    *downloadEntry
	readPos  int64
	mediaKey string
	dc       *downloadCache
	closed   bool
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
	r.dc.release(r.mediaKey)
	return nil
}
