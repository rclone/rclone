package chunkedreader

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errStreamFailed = errors.New("stream failed")

// failingReader delivers `left` bytes of the wrapped reader, then fails.
type failingReader struct {
	rc   io.ReadCloser
	left int
}

func (fr *failingReader) Read(p []byte) (int, error) {
	if fr.left <= 0 {
		return 0, errStreamFailed
	}
	if len(p) > fr.left {
		p = p[:fr.left]
	}
	n, err := fr.rc.Read(p)
	fr.left -= n
	return n, err
}

func (fr *failingReader) Close() error { return fr.rc.Close() }

// failingObject fails the chunk starting at failOffset permanently: the
// first open delivers failAfter bytes then errors, and every re-open
// inside that chunk errors, like an object deleted mid-read.
type failingObject struct {
	*mockobject.ContentMockObject
	failOffset int64
	chunkSize  int64
	failAfter  int
	opened     atomic.Bool
}

func (o *failingObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	for _, opt := range options {
		r, ok := opt.(*fs.RangeOption)
		if !ok || r.Start < o.failOffset || r.Start >= o.failOffset+o.chunkSize {
			continue
		}
		if o.opened.Swap(true) {
			return nil, errStreamFailed
		}
		rc, err := o.ContentMockObject.Open(ctx, options...)
		if err != nil {
			return nil, err
		}
		return &failingReader{rc: rc, left: o.failAfter}, nil
	}
	return o.ContentMockObject.Open(ctx, options...)
}

// A stream that fails before delivering its chunk must return the error
// from Read rather than wait forever for more data.
func TestParallelStreamErrorDoesNotHang(t *testing.T) {
	ctx := context.Background()
	const streams = 3
	const chunkSize = multipart.BufferSize
	content := makeContent(t, 5*chunkSize)
	o := &failingObject{
		ContentMockObject: mockobject.New("test.bin").WithContent(content, mockobject.SeekModeNone),
		failOffset:        2 * chunkSize,
		chunkSize:         chunkSize,
		failAfter:         100,
	}
	cr := New(ctx, o, chunkSize, 0, streams)

	done := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(cr)
		done <- err
	}()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.ErrorIs(t, err, errStreamFailed)
	case <-time.After(10 * time.Second):
		t.Fatal("Read hung after a stream failed mid-chunk")
	}
	_ = cr.Close() // reports the stream error too; must not hang
}
