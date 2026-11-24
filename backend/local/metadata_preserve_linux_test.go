//go:build linux

package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestPreservePOSIXMetadata(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.PreservePOSIXProperties, ci.PreserveSymlinks = true, true
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("data"), 0600))
	require.NoError(t, os.Link(filepath.Join(root, "file"), filepath.Join(root, "hardlink")))
	require.NoError(t, os.Symlink("missing", filepath.Join(root, "link")))
	// Keep atime ahead of ctime so reads on relatime mounts cannot change the fixture.
	atime := time.Now().Add(time.Hour)
	mtime := time.Date(2020, 1, 2, 3, 4, 5, 123456789, time.UTC)
	for _, name := range []string{"file", "link"} {
		require.NoError(t, lChtimes(filepath.Join(root, name), atime, mtime))
	}
	f, err := NewFs(ctx, "local", root, configmap.Simple{})
	require.NoError(t, err)
	for _, name := range []string{"file", "hardlink", "link"} {
		t.Run(name, func(t *testing.T) {
			o, err := f.NewObject(ctx, name)
			require.NoError(t, err)
			var stat unix.Stat_t
			require.NoError(t, unix.Lstat(filepath.Join(root, name), &stat))
			meta, err := o.(fs.Metadataer).Metadata(ctx)
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprint(stat.Nlink), meta["nlink"])
			assert.Equal(t, fmt.Sprint(stat.Ino), meta["ino"])
			assert.Equal(t, fmt.Sprint(stat.Dev), meta["dev"])
			assert.Equal(t, atime.Format(time.RFC3339Nano), meta["atime"])
			assert.Equal(t, mtime.Format(time.RFC3339Nano), meta["mtime"])
			assert.Equal(t, time.Unix(stat.Ctim.Unix()).Format(time.RFC3339Nano), meta["ctime"])
			assert.NotEmpty(t, meta["statx-mask"])
			var fallback fs.Metadata
			require.NoError(t, readMetadataFromFileFstatat(o.(*Object), &fallback))
			for _, key := range []string{"mode", "uid", "gid", "nlink", "ino", "dev", "atime", "mtime", "ctime"} {
				assert.Equal(t, meta[key], fallback[key], key)
			}
			assert.NotContains(t, fallback, "statx-mask")
			assert.NotContains(t, fallback, "btime")
		})
	}

	ctx, ci = fs.AddConfig(context.Background())
	ci.PreservePOSIXProperties, ci.PreserveSymlinks = false, false
	f, err = NewFs(ctx, "local", root, configmap.Simple{})
	require.NoError(t, err)
	o, err := f.NewObject(ctx, "file")
	require.NoError(t, err)
	meta, err := o.(fs.Metadataer).Metadata(ctx)
	require.NoError(t, err)
	for _, key := range []string{"nlink", "ino", "dev", "ctime", "statx-mask", "attributes", "attributes-mask"} {
		assert.NotContains(t, meta, key)
	}
}
