package gphotosmobile

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useTempDir points os.CreateTemp at a fresh directory and returns it
func useTempDir(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	return dir
}

// setGrace shortens the eviction grace period for the test
func setGrace(t *testing.T, d time.Duration) {
	old := downloadCacheGrace
	downloadCacheGrace = d
	t.Cleanup(func() { downloadCacheGrace = old })
}

// addTestEntry registers an entry with one reader without starting a download
func addTestEntry(t *testing.T, dc *downloadCache, mediaKey string) *downloadEntry {
	tmpFile, path, err := createTempFile()
	require.NoError(t, err)
	_, cancel := context.WithCancel(context.Background())
	entry := &downloadEntry{
		mediaKey: mediaKey,
		tmpFile:  tmpFile,
		path:     path,
		cancel:   cancel,
		refCount: 1,
	}
	dc.mu.Lock()
	dc.entries[mediaKey] = entry
	dc.mu.Unlock()
	return entry
}

func lookupEntry(dc *downloadCache, mediaKey string) *downloadEntry {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.entries[mediaKey]
}

func dirEntries(t *testing.T, dir string) []os.DirEntry {
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	return entries
}

func TestCreateTempFileUnlinked(t *testing.T) {
	dir := useTempDir(t)
	tmpFile, path, err := createTempFile()
	require.NoError(t, err)
	defer func() { _ = tmpFile.Close() }()

	if runtime.GOOS == "windows" {
		assert.NotEmpty(t, path)
		return
	}
	assert.Empty(t, path)
	assert.Empty(t, dirEntries(t, dir))

	// The open handle stays usable after unlinking
	_, err = tmpFile.Write([]byte("data"))
	require.NoError(t, err)
	buf := make([]byte, 4)
	_, err = tmpFile.ReadAt(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, "data", string(buf))
}

func TestDownloadCacheEvictsUnusedEntry(t *testing.T) {
	dir := useTempDir(t)
	setGrace(t, 50*time.Millisecond)
	dc := newDownloadCache(nil)

	entry := addTestEntry(t, dc, "key")
	dc.release(entry)
	assert.Same(t, entry, lookupEntry(dc, "key"), "entry kept during grace period")

	require.Eventually(t, func() bool { return lookupEntry(dc, "key") == nil },
		time.Second, 5*time.Millisecond)
	assert.Empty(t, dirEntries(t, dir))
	_, err := entry.tmpFile.Write([]byte("x"))
	assert.Error(t, err, "temp file closed")
}

func TestDownloadCacheReopenCancelsEviction(t *testing.T) {
	useTempDir(t)
	setGrace(t, 100*time.Millisecond)
	dc := newDownloadCache(nil)
	t.Cleanup(dc.shutdown)

	entry := addTestEntry(t, dc, "key")
	dc.release(entry)
	reopened, err := dc.getOrStart(context.Background(), "key", 0)
	require.NoError(t, err)
	require.Same(t, entry, reopened)

	time.Sleep(300 * time.Millisecond)
	assert.Same(t, entry, lookupEntry(dc, "key"))
	_, err = entry.tmpFile.Write([]byte("x"))
	assert.NoError(t, err, "temp file still open")
}

// A stale eviction for an old entry must not remove a newer entry
// registered under the same media key, or its temp file is orphaned.
func TestDownloadCacheStaleEvictionKeepsNewEntry(t *testing.T) {
	dir := useTempDir(t)
	setGrace(t, 100*time.Millisecond)
	dc := newDownloadCache(nil)

	// Release, reopen and release again so two evictions are scheduled
	old := addTestEntry(t, dc, "key")
	dc.release(old)
	_, err := dc.getOrStart(context.Background(), "key", 0)
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	dc.release(old)

	require.Eventually(t, func() bool { return lookupEntry(dc, "key") == nil },
		time.Second, 5*time.Millisecond)
	current := addTestEntry(t, dc, "key")

	time.Sleep(300 * time.Millisecond)
	assert.Same(t, current, lookupEntry(dc, "key"))
	_, err = current.tmpFile.Write([]byte("x"))
	assert.NoError(t, err, "new temp file still open")

	dc.release(current)
	require.Eventually(t, func() bool { return lookupEntry(dc, "key") == nil },
		time.Second, 5*time.Millisecond)
	assert.Empty(t, dirEntries(t, dir))
}

// stubFetch replaces the downloader and returns the number of downloads started
func stubFetch(dc *downloadCache) *atomic.Int32 {
	var started atomic.Int32
	dc.fetch = func(ctx context.Context, entry *downloadEntry, mediaKey string) {
		started.Add(1)
	}
	return &started
}

func TestDownloadCacheRetriesFailedDownload(t *testing.T) {
	dir := useTempDir(t)
	dc := newDownloadCache(nil)
	started := stubFetch(dc)
	t.Cleanup(dc.shutdown)

	failed := addTestEntry(t, dc, "key")
	failed.finish(errors.New("network error"))

	fresh, err := dc.getOrStart(context.Background(), "key", 0)
	require.NoError(t, err)
	assert.NotSame(t, failed, fresh)
	assert.Same(t, fresh, lookupEntry(dc, "key"))
	assert.Eventually(t, func() bool { return started.Load() == 1 },
		time.Second, 5*time.Millisecond, "new download started")

	// The failed entry still has a reader, so it stays open until released
	_, err = failed.tmpFile.Write([]byte("x"))
	assert.NoError(t, err)
	dc.release(failed)
	_, err = failed.tmpFile.Write([]byte("x"))
	assert.Error(t, err, "failed temp file closed on last release")
	assert.Same(t, fresh, lookupEntry(dc, "key"))

	dc.release(fresh)
	dc.shutdown()
	assert.Empty(t, dirEntries(t, dir))
}

func TestDownloadCacheRetryClosesUnreferencedFailedDownload(t *testing.T) {
	useTempDir(t)
	setGrace(t, time.Hour)
	dc := newDownloadCache(nil)
	stubFetch(dc)
	t.Cleanup(dc.shutdown)

	failed := addTestEntry(t, dc, "key")
	failed.finish(errors.New("network error"))
	dc.release(failed) // eviction pending, no readers

	_, err := dc.getOrStart(context.Background(), "key", 0)
	require.NoError(t, err)
	_, err = failed.tmpFile.Write([]byte("x"))
	assert.Error(t, err, "failed temp file closed immediately")
}

func TestDownloadCacheReusesFinishedDownload(t *testing.T) {
	useTempDir(t)
	dc := newDownloadCache(nil)
	started := stubFetch(dc)
	t.Cleanup(dc.shutdown)

	complete := addTestEntry(t, dc, "key")
	complete.finish(nil)

	reused, err := dc.getOrStart(context.Background(), "key", 0)
	require.NoError(t, err)
	assert.Same(t, complete, reused)
	assert.EqualValues(t, 0, started.Load())
}

func TestDownloadCacheShutdownRemovesFiles(t *testing.T) {
	dir := useTempDir(t)
	dc := newDownloadCache(nil)

	a := addTestEntry(t, dc, "a")
	addTestEntry(t, dc, "b")
	dc.shutdown()

	assert.Empty(t, dc.entries)
	assert.Empty(t, dirEntries(t, dir))

	// Closing a reader after shutdown must not resurrect or panic
	dc.release(a)
	assert.Empty(t, dc.entries)
}
