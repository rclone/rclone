//go:build linux && !mips && !mipsle && !mips64 && !mips64le && !ppc64 && !ppc64le

package file

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestXFSCommitRangeLayout(t *testing.T) {
	var args xfsCommitRange
	assert.Equal(t, uintptr(88), unsafe.Sizeof(args))
	for _, test := range []struct {
		name      string
		got, want uintptr
	}{
		{"file1_fd", unsafe.Offsetof(args.File1FD), 0},
		{"pad", unsafe.Offsetof(args.Pad), 4},
		{"file1_offset", unsafe.Offsetof(args.File1Offset), 8},
		{"file2_offset", unsafe.Offsetof(args.File2Offset), 16},
		{"length", unsafe.Offsetof(args.Length), 24},
		{"flags", unsafe.Offsetof(args.Flags), 32},
		{"file2_freshness", unsafe.Offsetof(args.File2Freshness), 40},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.got)
		})
	}
}

// Compare with the installed UAPI, not another copy of the Go constants.
func TestXFSCommitRangeUAPI(t *testing.T) {
	if os.Getenv("RCLONE_TEST_XFS_DIR") == "" {
		t.Skip("set RCLONE_TEST_XFS_DIR to test against the XFS development headers")
	}
	binary := filepath.Join(t.TempDir(), "xfs-abi")
	out, err := exec.Command("cc", "testdata/xfs_abi.c", "-o", binary).CombinedOutput()
	require.NoError(t, err, "%s", out)
	out, err = exec.Command(binary).CombinedOutput()
	require.NoError(t, err, "%s", out)
	var args xfsCommitRange
	want := fmt.Sprintf("%d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
		unsafe.Sizeof(args), unsafe.Offsetof(args.File1FD), unsafe.Offsetof(args.Pad),
		unsafe.Offsetof(args.File1Offset), unsafe.Offsetof(args.File2Offset),
		unsafe.Offsetof(args.Length), unsafe.Offsetof(args.Flags), unsafe.Offsetof(args.File2Freshness),
		xfsIOCStartCommit, xfsIOCCommitRange, xfsExchangeRangeToEOF, xfsExchangeRangeDSync, unsafe.Sizeof(xfsGeometry{}), unsafe.Offsetof(xfsGeometry{}.Flags), xfsIOCGeometry, xfsGeometryExchangeRange)
	assert.Equal(t, want, strings.TrimSpace(string(out)))
}

func TestXFSCommitUnsupportedErrors(t *testing.T) {
	for _, test := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{unix.ENOTTY, true}, {unix.EOPNOTSUPP, true}, {unix.ENOSYS, true},
		{unix.EBUSY, false}, {unix.EXDEV, false}, {unix.EBADF, false},
		{unix.EINVAL, false}, {unix.EIO, false}, {unix.ENOSPC, false},
		{unix.EDQUOT, false}, {unix.EACCES, false}, {unix.EPERM, false},
		{unix.EINTR, false}, {os.ErrNotExist, false},
	} {
		t.Run(fmt.Sprint(test.err), func(t *testing.T) {
			assert.Equal(t, test.want, isXFSCommitUnsupported(test.err))
			if test.err != nil {
				assert.Equal(t, test.want, isXFSCommitUnsupported(fmt.Errorf("ioctl: %w", test.err)))
			}
		})
	}
}

func TestXFSInvalidFiles(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "file")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	_, err = f.WriteString("unchanged")
	require.NoError(t, err)
	token, err := StartXFSCommit(nil)
	require.Nil(t, token)
	require.ErrorIs(t, err, os.ErrInvalid)
	for _, test := range []struct {
		name            string
		target, staging *os.File
		token           *XFSCommitToken
	}{
		{"NilTarget", nil, f, &XFSCommitToken{}},
		{"NilStaging", f, nil, &XFSCommitToken{}},
		{"NilToken", f, f, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.ErrorIs(t, CommitXFSRange(test.target, test.staging, test.token, false), os.ErrInvalid)
		})
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed")
	require.NoError(t, err)
	require.NoError(t, closed.Close())
	_, err = StartXFSCommit(closed)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrXFSCommitRangeUnsupported))
	assert.Error(t, CommitXFSRange(f, closed, &XFSCommitToken{}, false))
	assert.Error(t, CommitXFSRange(closed, f, &XFSCommitToken{}, false))
	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Equal(t, "unchanged", string(data))
}

func TestIsXFS(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(name, nil, 0600))
	var st unix.Statfs_t
	require.NoError(t, unix.Statfs(dir, &st))
	for _, name := range []string{dir, name} {
		got, err := IsXFS(name)
		require.NoError(t, err)
		assert.Equal(t, uint64(st.Type) == unix.XFS_SUPER_MAGIC, got)
	}
	got, err := IsXFS(filepath.Join(dir, "missing"))
	assert.False(t, got)
	assert.ErrorIs(t, err, unix.ENOENT)
}
