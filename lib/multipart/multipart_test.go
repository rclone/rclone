package multipart

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testChunkWriter struct {
	leavePartsOnError bool
	concurrency       int
	chunkSize         int64
	writeFn           func(ctx context.Context, chunk int) (int64, error)

	mu        sync.Mutex
	started   []int
	completed []int
	closed    bool
	aborted   bool
}

func (w *testChunkWriter) OpenChunkWriter(_ context.Context, _ string, _ fs.ObjectInfo, _ ...fs.OpenOption) (fs.ChunkWriterInfo, fs.ChunkWriter, error) {
	return fs.ChunkWriterInfo{
		ChunkSize:         w.chunkSize,
		Concurrency:       w.concurrency,
		LeavePartsOnError: w.leavePartsOnError,
	}, w, nil
}

func (w *testChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	w.mu.Lock()
	w.started = append(w.started, chunkNumber)
	w.mu.Unlock()

	if _, err := io.Copy(io.Discard, reader); err != nil {
		return -1, err
	}
	n, err := w.writeFn(ctx, chunkNumber)
	if err == nil {
		w.mu.Lock()
		w.completed = append(w.completed, chunkNumber)
		w.mu.Unlock()
	}
	return n, err
}

func (w *testChunkWriter) Close(_ context.Context) error {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	return nil
}

func (w *testChunkWriter) Abort(_ context.Context) error {
	w.mu.Lock()
	w.aborted = true
	w.mu.Unlock()
	return nil
}

func TestUploadMultipartLeavePartsOnError(t *testing.T) {
	payload := []byte("0123456789abcdef")

	t.Run("cancels in-flight and aborts by default", func(t *testing.T) {
		ctx := context.Background()
		terminalErr := errors.New("terminal part error")
		part1Started := make(chan struct{})
		part1Done := make(chan error, 1)

		writer := &testChunkWriter{
			leavePartsOnError: false,
			concurrency:       2,
			chunkSize:         4,
			writeFn: func(ctx context.Context, chunk int) (int64, error) {
				switch chunk {
				case 0:
					select {
					case <-part1Started:
					case <-time.After(3 * time.Second):
						return 0, errors.New("timed out waiting for part 1")
					}
					return 0, terminalErr
				case 1:
					close(part1Started)
					select {
					case <-ctx.Done():
						part1Done <- ctx.Err()
						return 0, ctx.Err()
					case <-time.After(3 * time.Second):
						part1Done <- nil
						return 4, nil
					}
				default:
					return 4, nil
				}
			},
		}

		src := mockobject.New("file.bin").WithContent(payload, mockobject.SeekModeNone)
		_, err := UploadMultipart(ctx, src, bytes.NewReader(payload), UploadMultipartOptions{Open: writer})
		require.ErrorIs(t, err, terminalErr)
		assert.ErrorIs(t, <-part1Done, context.Canceled)
		assert.True(t, writer.aborted)
		assert.False(t, writer.closed)
		for _, c := range writer.started {
			assert.Less(t, c, 2)
		}
	})

	t.Run("drains in-flight parts and skips new parts when enabled", func(t *testing.T) {
		ctx := context.Background()
		terminalErr := errors.New("terminal part error")
		part1Started := make(chan struct{})
		part0Failed := make(chan struct{})
		releasePart1 := make(chan struct{})
		part1CtxErr := make(chan error, 1)

		writer := &testChunkWriter{
			leavePartsOnError: true,
			concurrency:       2,
			chunkSize:         4,
			writeFn: func(ctx context.Context, chunk int) (int64, error) {
				switch chunk {
				case 0:
					select {
					case <-part1Started:
					case <-time.After(3 * time.Second):
						return 0, errors.New("timed out waiting for part 1")
					}
					close(part0Failed)
					return 0, terminalErr
				case 1:
					close(part1Started)
					select {
					case <-releasePart1:
						part1CtxErr <- ctx.Err()
						return 4, nil
					case <-ctx.Done():
						part1CtxErr <- ctx.Err()
						return 0, ctx.Err()
					case <-time.After(3 * time.Second):
						part1CtxErr <- errors.New("timed out waiting for release")
						return 0, errors.New("timed out waiting for release")
					}
				default:
					return 4, nil
				}
			},
		}

		go func() {
			<-part0Failed
			time.Sleep(50 * time.Millisecond)
			close(releasePart1)
		}()

		src := mockobject.New("file.bin").WithContent(payload, mockobject.SeekModeNone)
		_, err := UploadMultipart(ctx, src, bytes.NewReader(payload), UploadMultipartOptions{Open: writer})
		require.ErrorIs(t, err, terminalErr)
		require.NoError(t, <-part1CtxErr)
		assert.False(t, writer.aborted)
		assert.False(t, writer.closed)
		assert.ElementsMatch(t, []int{0, 1}, writer.started)
		assert.Equal(t, []int{1}, writer.completed)
	})

	t.Run("stops in-flight parts on caller cancel when enabled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var bothStarted sync.WaitGroup
		bothStarted.Add(2)
		partErrs := make(chan error, 2)

		writer := &testChunkWriter{
			leavePartsOnError: true,
			concurrency:       2,
			chunkSize:         4,
			writeFn: func(ctx context.Context, chunk int) (int64, error) {
				if chunk < 2 {
					bothStarted.Done()
				}
				select {
				case <-ctx.Done():
					partErrs <- ctx.Err()
					return 0, ctx.Err()
				case <-time.After(3 * time.Second):
					partErrs <- nil
					return 4, nil
				}
			},
		}

		go func() {
			bothStarted.Wait()
			cancel()
		}()

		src := mockobject.New("file.bin").WithContent(payload, mockobject.SeekModeNone)
		_, err := UploadMultipart(ctx, src, bytes.NewReader(payload), UploadMultipartOptions{Open: writer})
		require.ErrorIs(t, err, context.Canceled)
		assert.ErrorIs(t, <-partErrs, context.Canceled)
		assert.ErrorIs(t, <-partErrs, context.Canceled)
		assert.False(t, writer.aborted)
		assert.False(t, writer.closed)
	})
}
