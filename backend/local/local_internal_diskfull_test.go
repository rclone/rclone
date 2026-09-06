//go:build !plan9

// Tests for the FatalIfNoSpace option and isDiskFullError helper.
//
// Kept in a separate file with a !plan9 build tag because syscall.ENOSPC
// is not portable to plan9, mirroring the split in
// fs/fserrors/enospc_error.go vs fs/fserrors/enospc_error_notsupported.go.

package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errReader is an io.Reader that always returns the configured error,
// used to inject a synthetic disk-full failure into Update's io.Copy.
type errReader struct{ err error }

func (r errReader) Read(p []byte) (int, error) { return 0, r.err }

// TestIsDiskFullError covers the helper used by the Update defer.
func TestIsDiskFullError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated error", errors.New("unrelated error"), false},
		{"syscall.ENOSPC direct", syscall.ENOSPC, true},
		{"syscall.ENOSPC wrapped", fmt.Errorf("io.Copy: %w", syscall.ENOSPC), true},
		{"file.ErrDiskFull direct", file.ErrDiskFull, true},
		{"file.ErrDiskFull wrapped", fmt.Errorf("preallocate: %w", file.ErrDiskFull), true},
		{"os.PathError wrapping ENOSPC", &os.PathError{Op: "write", Path: "/foo", Err: syscall.ENOSPC}, true},
		{"os.SyscallError wrapping ENOSPC", os.NewSyscallError("write", syscall.ENOSPC), true},
		{"os.PathError wrapping unrelated", &os.PathError{Op: "write", Path: "/foo", Err: syscall.EPERM}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, isDiskFullError(c.err))
		})
	}
}

// updateWithReader runs Update with an injected reader error and the given
// FatalIfNoSpace setting, returning the error from Update.
func updateWithReader(t *testing.T, fatalIfNoSpace bool, readerErr error) error {
	t.Helper()
	r := fstest.NewRunIndividual(t)
	f := r.Flocal.(*Fs)
	f.opt.FatalIfNoSpace = fatalIfNoSpace

	src := object.NewStaticObjectInfo("test.txt", time.Now(), 100, true, nil, f)
	o := &Object{
		fs:     f,
		remote: "test.txt",
		path:   filepath.Join(r.LocalName, "test.txt"),
	}
	return o.Update(context.Background(), errReader{err: readerErr}, src)
}

// TestUpdateFatalIfNoSpaceOff verifies an ENOSPC during a write is NOT
// wrapped as fatal when the option is off.
func TestUpdateFatalIfNoSpaceOff(t *testing.T) {
	err := updateWithReader(t, false, syscall.ENOSPC)
	require.Error(t, err)
	assert.False(t, fserrors.IsFatalError(err), "ENOSPC must not be fatal when FatalIfNoSpace=false")
}

// TestUpdateFatalIfNoSpaceOn verifies an ENOSPC during a write IS wrapped as
// fatal when the option is on.
func TestUpdateFatalIfNoSpaceOn(t *testing.T) {
	err := updateWithReader(t, true, syscall.ENOSPC)
	require.Error(t, err)
	assert.True(t, fserrors.IsFatalError(err), "ENOSPC must be fatal when FatalIfNoSpace=true")
}

// TestUpdateFatalIfNoSpaceOnButNotDiskFull verifies non-disk-full errors are
// NOT wrapped as fatal even when the option is on.
func TestUpdateFatalIfNoSpaceOnButNotDiskFull(t *testing.T) {
	err := updateWithReader(t, true, errors.New("unrelated network error"))
	require.Error(t, err)
	assert.False(t, fserrors.IsFatalError(err), "non-disk-full errors must not be fatal regardless of option")
}

type diskFullWriterAtCloser struct {
	writeErr error
	closeErr error
	n        int
	data     []byte
	offset   int64
	closed   bool
}

func (w *diskFullWriterAtCloser) WriteAt(p []byte, off int64) (int, error) {
	w.data = append([]byte(nil), p...)
	w.offset = off
	return w.n, w.writeErr
}

func (w *diskFullWriterAtCloser) Close() error {
	w.closed = true
	return w.closeErr
}

func testOpenWriterAtError(t *testing.T, injected error, diskFull bool) {
	t.Helper()
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%v", enabled), func(t *testing.T) {
			r := fstest.NewRunIndividual(t)
			f := r.Flocal.(*Fs)
			f.opt.FatalIfNoSpace = enabled
			f.opt.NoPreAllocate = true
			writer, err := f.OpenWriterAt(context.Background(), "test.txt", 0)
			require.NoError(t, err)
			wrapped, ok := writer.(*fatalIfNoSpaceWriterAt)
			require.True(t, ok)
			require.NoError(t, wrapped.WriterAtCloser.Close())
			wantN := 2
			if injected == nil {
				wantN = 4
			}
			underlying := &diskFullWriterAtCloser{writeErr: injected, closeErr: injected, n: wantN}
			wrapped.WriterAtCloser = underlying
			n, err := writer.WriteAt([]byte("data"), 17)
			assert.Equal(t, wantN, n)
			assert.Equal(t, []byte("data"), underlying.data)
			assert.Equal(t, int64(17), underlying.offset)
			assert.ErrorIs(t, err, injected)
			assert.Equal(t, enabled && diskFull, fserrors.IsFatalError(err))
			err = writer.Close()
			assert.True(t, underlying.closed)
			assert.ErrorIs(t, err, injected)
			assert.Equal(t, enabled && diskFull, fserrors.IsFatalError(err))
		})
	}
}

func TestOpenWriterAtFatalIfNoSpace(t *testing.T) {
	for _, test := range []struct {
		name     string
		err      error
		diskFull bool
	}{
		{"success", nil, false},
		{"unrelated", syscall.EPERM, false},
		{"ENOSPC", syscall.ENOSPC, true},
		{"ErrDiskFull", file.ErrDiskFull, true},
		{"wrapped", fmt.Errorf("write: %w", syscall.ENOSPC), true},
	} {
		t.Run(test.name, func(t *testing.T) { testOpenWriterAtError(t, test.err, test.diskFull) })
	}
}

func TestOpenWriterAtSetupError(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%v", enabled), func(t *testing.T) {
			r := fstest.NewRunIndividual(t)
			f := r.Flocal.(*Fs)
			f.opt.FatalIfNoSpace = enabled
			require.NoError(t, os.WriteFile(filepath.Join(r.LocalName, "parent"), nil, 0600))
			writer, err := f.OpenWriterAt(context.Background(), "parent/child", 0)
			require.Error(t, err)
			assert.Nil(t, writer)
			assert.False(t, fserrors.IsFatalError(err))
		})
	}
}
