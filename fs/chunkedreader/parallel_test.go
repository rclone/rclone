package chunkedreader

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallel(t *testing.T) {
	content := makeContent(t, 1024)

	for _, mode := range mockobject.SeekModes {
		t.Run(mode.String(), testRead(content, mode, 3))
	}
}

func TestParallelErrorAfterClose(t *testing.T) {
	testErrorAfterClose(t, 3)
}

func TestParallelLarge(t *testing.T) {
	ctx := context.Background()
	const streams = 3
	const chunkSize = multipart.BufferSize
	const size = (2*streams+1)*chunkSize + 255
	content := makeContent(t, size)
	o := mockobject.New("test.bin").WithContent(content, mockobject.SeekModeNone)

	cr := New(ctx, o, chunkSize, 0, streams)

	for _, test := range []struct {
		name     string
		offset   int64
		seekMode int
	}{
		{name: "Straight", offset: 0, seekMode: -1},
		{name: "Rewind", offset: 0, seekMode: io.SeekStart},
		{name: "NearStart", offset: 1, seekMode: io.SeekStart},
		{name: "NearEnd", offset: size - 2*chunkSize - 127, seekMode: io.SeekEnd},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.seekMode >= 0 {
				var n int64
				var err error
				if test.seekMode == io.SeekEnd {
					n, err = cr.Seek(test.offset-size, test.seekMode)
				} else {
					n, err = cr.Seek(test.offset, test.seekMode)
				}
				require.NoError(t, err)
				assert.Equal(t, test.offset, n)
			}
			got, err := io.ReadAll(cr)
			require.NoError(t, err)
			require.Equal(t, len(content[test.offset:]), len(got))
			assert.Equal(t, content[test.offset:], got)
		})
	}

	require.NoError(t, cr.Close())

	t.Run("Seeky", func(t *testing.T) {
		cr := New(ctx, o, chunkSize, 0, streams)
		offset := 0
		buf := make([]byte, 1024)

		for {
			// Read and check a random read
			readSize := rand.Intn(1024)
			readBuf := buf[:readSize]
			n, err := cr.Read(readBuf)

			require.Equal(t, content[offset:offset+n], readBuf[:n])
			offset += n

			if err == io.EOF {
				assert.Equal(t, size, offset)
				break
			}
			require.NoError(t, err)

			// Now do a smaller random seek backwards
			seekSize := rand.Intn(512)
			if offset-seekSize < 0 {
				seekSize = offset
			}
			nn, err := cr.Seek(-int64(seekSize), io.SeekCurrent)
			offset -= seekSize
			require.NoError(t, err)
			assert.Equal(t, nn, int64(offset))
		}

		require.NoError(t, cr.Close())
	})

}

// errorReader reads up to failAfter bytes then returns err
type errorReader struct {
	r         io.Reader
	remaining int
	err       error
}

func (er *errorReader) Read(p []byte) (n int, err error) {
	if er.remaining <= 0 {
		return 0, er.err
	}
	if len(p) > er.remaining {
		p = p[:er.remaining]
	}
	n, err = er.r.Read(p)
	er.remaining -= n
	if err == nil && er.remaining <= 0 {
		err = er.err
	}
	return n, err
}

func (er *errorReader) Close() error {
	return nil
}

// errorMockObject mocks an fs.Object whose Open returns an errorReader
type errorMockObject struct {
	mockobject.Object
	content   []byte
	failAfter int
	err       error
}

func (o *errorMockObject) Size() int64 {
	return int64(len(o.content))
}

func (o *errorMockObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var offset int64
	for _, opt := range options {
		if ro, ok := opt.(*fs.RangeOption); ok {
			offset = ro.Start
		}
	}
	var r io.Reader = bytes.NewReader(nil)
	if offset < int64(len(o.content)) {
		r = bytes.NewReader(o.content[offset:])
	}
	remaining := o.failAfter - int(offset)
	if remaining < 0 {
		remaining = 0
	}
	return &errorReader{
		r:         r,
		remaining: remaining,
		err:       o.err,
	}, nil
}

func TestParallelReadError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("simulated remote connection failure")
	content := makeContent(t, 2*multipart.BufferSize)
	o := &errorMockObject{
		Object:    mockobject.New("error-test.bin"),
		content:   content,
		failAfter: 1024,
		err:       expectedErr,
	}

	cr := New(ctx, o, multipart.BufferSize, 0, 2)
	defer func() {
		_ = cr.Close()
	}()

	buf := make([]byte, 8192)
	var err error
	for {
		_, err = cr.Read(buf)
		if err != nil {
			break
		}
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

type openErrorMockObject struct {
	mockobject.Object
	size int64
	err  error
}

func (o *openErrorMockObject) Size() int64 {
	return o.size
}

func (o *openErrorMockObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	return nil, o.err
}

func TestParallelOpenError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("simulated 404 not found")
	o := &openErrorMockObject{
		Object: mockobject.New("notfound.bin"),
		size:   2 * multipart.BufferSize,
		err:    expectedErr,
	}

	cr := New(ctx, o, multipart.BufferSize, 0, 2)
	defer func() {
		_ = cr.Close()
	}()

	buf := make([]byte, 1024)
	_, err := cr.Read(buf)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestParallelPrematureEOF(t *testing.T) {
	ctx := context.Background()
	content := makeContent(t, 2*multipart.BufferSize)
	o := &errorMockObject{
		Object:    mockobject.New("premature-eof-test.bin"),
		content:   content,
		failAfter: 1024,
		err:       io.EOF,
	}

	cr := New(ctx, o, multipart.BufferSize, 0, 2)
	defer func() {
		_ = cr.Close()
	}()

	buf := make([]byte, 8192)
	var err error
	for {
		_, err = cr.Read(buf)
		if err != nil {
			break
		}
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestParallelSeekError(t *testing.T) {
	ctx := context.Background()
	content := makeContent(t, 2*multipart.BufferSize)
	o := &errorMockObject{
		Object:    mockobject.New("seek-error-test.bin"),
		content:   content,
		failAfter: 1024,
		err:       io.EOF,
	}

	cr := New(ctx, o, multipart.BufferSize, 0, 2)
	defer func() {
		_ = cr.Close()
	}()

	_, err := cr.Open()
	require.NoError(t, err)

	// Seek past available data into the truncated portion of the stream
	_, err = cr.Seek(2048, io.SeekStart)
	require.Error(t, err)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}
