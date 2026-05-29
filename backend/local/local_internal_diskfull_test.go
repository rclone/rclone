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
	r := fstest.NewRun(t)
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
