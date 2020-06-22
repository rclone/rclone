package downloaders

import (
	"context"
	"io"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/ranges"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

type testItem struct {
	mu   sync.Mutex
	t    *testing.T
	rs   ranges.Ranges
	size int64
}

// HasRange returns true if the current ranges entirely include range
func (item *testItem) HasRange(r ranges.Range) bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.rs.Present(r)
}

// FindMissing adjusts r returning a new ranges.Range which only
// contains the range which needs to be downloaded. This could be
// empty - check with IsEmpty. It also adjust this to make sure it is
// not larger than the file.
func (item *testItem) FindMissing(r ranges.Range) (outr ranges.Range) {
	item.mu.Lock()
	defer item.mu.Unlock()
	outr = item.rs.FindMissing(r)
	// Clip returned block to size of file
	outr.Clip(item.size)
	return outr
}

// WriteAtNoOverwrite writes b to the file, but will not overwrite
// already present ranges.
//
// This is used by the downloader to write bytes to the file
//
// It returns n the total bytes processed and skipped the number of
// bytes which were processed but not actually written to the file.
func (item *testItem) WriteAtNoOverwrite(b []byte, off int64) (n int, skipped int, err error) {
	item.mu.Lock()
	defer item.mu.Unlock()
	item.rs.Insert(ranges.Range{Pos: off, Size: int64(len(b))})

	// Check contents is correct
	in := readers.NewPatternReader(item.size)
	checkBuf := make([]byte, len(b))
	_, err = in.Seek(off, io.SeekStart)
	require.NoError(item.t, err)
	n, _ = in.Read(checkBuf)
	require.Equal(item.t, len(b), n)
	assert.Equal(item.t, checkBuf, b)

	return n, 0, nil
}

func TestDownloaders(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	var (
		ctx    = context.Background()
		remote = "potato.txt"
		size   = int64(50*1024*1024 - 1234)
	)

	// Write the test file
	in := ioutil.NopCloser(readers.NewPatternReader(size))
	src, err := operations.RcatSize(ctx, r.Fremote, remote, in, size, time.Now())
	require.NoError(t, err)
	assert.Equal(t, size, src.Size())

	newTest := func() (*testItem, *Downloaders) {
		item := &testItem{
			t:    t,
			size: size,
		}
		opt := vfscommon.DefaultOpt
		dls := New(item, &opt, remote, src)
		return item, dls
	}
	cancel := func(dls *Downloaders) {
		assert.NoError(t, dls.Close(nil))
	}

	t.Run("Download", func(t *testing.T) {
		item, dls := newTest()
		defer cancel(dls)

		for _, r := range []ranges.Range{
			{Pos: 100, Size: 250},
			{Pos: 500, Size: 250},
			{Pos: 25000000, Size: 250},
		} {
			err := dls.Download(r)
			require.NoError(t, err)
			assert.True(t, item.HasRange(r))
		}
	})

	t.Run("EnsureDownloader", func(t *testing.T) {
		item, dls := newTest()
		defer cancel(dls)
		r := ranges.Range{Pos: 40 * 1024 * 1024, Size: 250}
		err := dls.EnsureDownloader(r)
		require.NoError(t, err)
		// FIXME racy test
		assert.False(t, item.HasRange(r))
		time.Sleep(time.Second)
		assert.True(t, item.HasRange(r))
	})
}
