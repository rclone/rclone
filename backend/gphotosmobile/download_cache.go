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
func (dc *downloadCache) getOrStart(mediaKey string, totalSize int64) (*downloadEntry, error) {
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
	tmpFile, err := os.CreateTemp("", "gphotos_mobile_*.tmp")
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

	// Start background download
	go dc.download(entry, mediaKey)

	return entry, nil
}

// download fetches the file and writes to the entry's temp file
func (dc *downloadCache) download(entry *downloadEntry, mediaKey string) {
	downloadURL, err := dc.api.GetDownloadURL(mediaKey)
	if err != nil {
		entry.mu.Lock()
		entry.dlErr = err
		entry.done = true
		entry.mu.Unlock()
		return
	}

	body, err := dc.api.DownloadFile(downloadURL)
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
