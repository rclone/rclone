//go:build windows

package local

import (
	"os"
	"testing"

	"github.com/rclone/rclone/fs/fserrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestUpdateFatalIfNoSpaceWindows(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ERROR_DISK_FULL", windows.ERROR_DISK_FULL},
		{"openat PathError", &os.PathError{Op: "openat", Path: "test.txt", Err: windows.ERROR_DISK_FULL}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Run("off", func(t *testing.T) {
				err := updateWithReader(t, false, test.err)
				require.Error(t, err)
				assert.False(t, fserrors.IsFatalError(err))
			})
			t.Run("on", func(t *testing.T) {
				err := updateWithReader(t, true, test.err)
				require.Error(t, err)
				assert.True(t, fserrors.IsFatalError(err))
			})
		})
	}
}
