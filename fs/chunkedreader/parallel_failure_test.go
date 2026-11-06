package chunkedreader

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/asyncreader"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/stretchr/testify/require"
)

type failingParallelObject struct {
	fs.Object
	open func(context.Context) (io.ReadCloser, error)
}

func (o failingParallelObject) Open(ctx context.Context, _ ...fs.OpenOption) (io.ReadCloser, error) {
	return o.open(ctx)
}

func parallelResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("parallel reader did not unblock")
		return nil
	}
}

func TestParallelProducerFailure(t *testing.T) {
	failure := errors.New("connection lost")
	for _, mode := range []string{"open", "short", "read"} {
		for _, action := range []string{"read", "seek"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				var source io.Reader = strings.NewReader("abc")
				if mode == "read" {
					source = io.MultiReader(source, parallelErrorReader{failure})
				}
				o := failingParallelObject{
					Object: mockobject.New("test").WithContent(make([]byte, 100), mockobject.SeekModeNone),
					open: func(context.Context) (io.ReadCloser, error) {
						if mode == "open" {
							return nil, failure
						}
						return io.NopCloser(source), nil
					},
				}
				cr := newParallel(context.Background(), o, multipart.BufferSize, 2)
				result := make(chan error, 1)
				var data []byte
				go func() {
					if action == "seek" {
						_, err := cr.Read(make([]byte, 1))
						if err == nil {
							_, err = cr.Seek(50, io.SeekStart)
						}
						result <- err
					} else {
						var err error
						data, err = io.ReadAll(cr)
						result <- err
					}
				}()
				err := parallelResult(t, result)
				if action == "read" && mode != "open" {
					require.Equal(t, []byte("abc"), data)
				}
				if mode == "short" {
					require.ErrorIs(t, err, io.ErrUnexpectedEOF)
				} else {
					require.ErrorIs(t, err, failure)
				}
				_ = cr.Close()
			})
		}
	}
}

type parallelErrorReader struct{ err error }

func (r parallelErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestParallelCancellation(t *testing.T) {
	for _, action := range []string{"cancel", "close"} {
		t.Run(action, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			o := failingParallelObject{
				Object: mockobject.New("test").WithContent(make([]byte, 100), mockobject.SeekModeNone),
				open: func(ctx context.Context) (io.ReadCloser, error) {
					close(started)
					<-ctx.Done()
					return nil, ctx.Err()
				},
			}
			cr := newParallel(ctx, o, multipart.BufferSize, 2)
			result := make(chan error, 1)
			go func() { _, err := cr.Read(make([]byte, 100)); result <- err }()
			<-started
			closed := make(chan error, 1)
			if action == "close" {
				go func() { closed <- cr.Close() }()
			} else {
				cancel()
			}
			require.ErrorIs(t, parallelResult(t, result), context.Canceled)
			if action == "close" {
				require.ErrorIs(t, parallelResult(t, closed), asyncreader.ErrorStreamAbandoned)
			} else {
				_ = cr.Close()
			}
		})
	}
}
