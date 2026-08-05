//go:build linux

package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var trashTime = fstest.Time("2011-12-25T12:59:59.123456789Z")

// TestUseTrash checks that Remove with use_trash moves the file into the
// freedesktop trash instead of deleting it permanently.
func TestUseTrash(t *testing.T) {
	ctx := context.Background()
	// point the freedesktop trash at a temporary directory
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)

	r := fstest.NewRun(t)
	f := r.Flocal.(*Fs)
	f.opt.UseTrash = true

	r.WriteFile("trashme.txt", "content", trashTime)
	obj, err := f.NewObject(ctx, "trashme.txt")
	require.NoError(t, err)
	require.NoError(t, obj.Remove(ctx))
	fstest.CheckItems(t, r.Flocal)

	// the file must be in the trash with a matching trashinfo
	trashed := filepath.Join(dataHome, "Trash", "files", "trashme.txt")
	content, err := os.ReadFile(trashed)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
	info, err := os.ReadFile(filepath.Join(dataHome, "Trash", "info", "trashme.txt.trashinfo"))
	require.NoError(t, err)
	assert.Contains(t, string(info), "[Trash Info]")
	assert.Contains(t, string(info), "Path=")

	// a second file with the same name must get a deduplicated name
	r.WriteFile("trashme.txt", "content2", trashTime)
	obj, err = f.NewObject(ctx, "trashme.txt")
	require.NoError(t, err)
	require.NoError(t, obj.Remove(ctx))
	content, err = os.ReadFile(filepath.Join(dataHome, "Trash", "files", "trashme.2.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content2", string(content))
}

// TestUseTrashPurge checks that Purge with use_trash moves the whole
// directory into the trash in one go and that Purge without the option
// reports fs.ErrorCantPurge.
func TestUseTrashPurge(t *testing.T) {
	ctx := context.Background()
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)

	r := fstest.NewRun(t)
	f := r.Flocal.(*Fs)

	assert.Equal(t, fs.ErrorCantPurge, f.Purge(ctx, "sub"))

	f.opt.UseTrash = true
	r.WriteFile("sub/one.txt", "one", trashTime)
	r.WriteFile("sub/two.txt", "two", trashTime)
	require.NoError(t, f.Purge(ctx, "sub"))
	fstest.CheckItems(t, r.Flocal)

	content, err := os.ReadFile(filepath.Join(dataHome, "Trash", "files", "sub", "one.txt"))
	require.NoError(t, err)
	assert.Equal(t, "one", string(content))
}
