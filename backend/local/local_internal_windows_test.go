//go:build windows

package local

import (
	"context"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRmdirWindows tests that FILE_ATTRIBUTE_READONLY does not block Rmdir on windows.
// Microsoft docs indicate that "This attribute is not honored on directories."
// See https://learn.microsoft.com/en-us/windows/win32/fileio/file-attribute-constants#file_attribute_readonly
// and https://github.com/golang/go/issues/26295
func TestRmdirWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("windows only")
	}
	r := fstest.NewRun(t)
	defer r.Finalise()

	err := operations.Mkdir(context.Background(), r.Flocal, "testdir")
	require.NoError(t, err)

	ptr, err := syscall.UTF16PtrFromString(filepath.Join(r.Flocal.Root(), "testdir"))
	require.NoError(t, err)

	err = syscall.SetFileAttributes(ptr, uint32(syscall.FILE_ATTRIBUTE_DIRECTORY+syscall.FILE_ATTRIBUTE_READONLY))
	require.NoError(t, err)

	err = operations.Rmdir(context.Background(), r.Flocal, "testdir")
	assert.NoError(t, err)
}

// TestLocalPathUNCShareRoot checks localPath works when the root is a
// bare UNC share, where filepath.Rel would never return.
func TestLocalPathUNCShareRoot(t *testing.T) {
	for _, root := range []string{`\\server\share`, `\\?\UNC\server\share`} {
		f := &Fs{root: root}
		for _, test := range []struct {
			name string
			want string
		}{
			{name: "", want: root + `\`},
			{name: "file.txt", want: root + `\file.txt`},
			{name: "sub/file.txt", want: root + `\sub\file.txt`},
		} {
			type result struct {
				path string
				err  error
			}
			done := make(chan result, 1)
			go func() {
				path, err := f.localPath(test.name)
				done <- result{path, err}
			}()
			select {
			case res := <-done:
				require.NoError(t, res.err, root, test.name)
				assert.Equal(t, test.want, res.path, root, test.name)
			case <-time.After(10 * time.Second):
				t.Fatalf("localPath(%q) with root %q did not return", test.name, root)
			}
		}
	}
}
