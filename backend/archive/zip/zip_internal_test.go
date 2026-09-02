package zip

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeZip builds a zip file with the given entry names (a name ending
// in "/" is a directory) in dir and returns its leaf name.
func writeZip(t *testing.T, dir, name string, names ...string) string {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		w, err := zw.Create(n)
		require.NoError(t, err)
		if !strings.HasSuffix(n, "/") {
			_, err = w.Write([]byte("data for " + n))
			require.NoError(t, err)
		}
	}
	require.NoError(t, zw.Close())
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0600))
	return name
}

// allRemotes returns every Object remote in the mounted archive's dirtree.
func allRemotes(t *testing.T, f fs.Fs) []string {
	zf, ok := f.(*Fs)
	require.True(t, ok)
	var remotes []string
	for _, entries := range zf.dt {
		for _, entry := range entries {
			if o, ok := entry.(*Object); ok {
				remotes = append(remotes, o.Remote())
			}
		}
	}
	// dt iteration order is nondeterministic, so sort for stable comparison.
	sort.Strings(remotes)
	return remotes
}

// A malicious zip whose entry names escape the archive's own namespace
// must not be exposed by the zip backend (Zip Slip).
func TestReadZipSlip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	name := writeZip(t, dir, "evil.zip",
		"good.txt",
		"../../etc/cron.d/evil",
		"../escape.txt",
		"sub/../../above.txt",
		`..\..\evil.exe`,
	)

	localFs, err := cache.Get(ctx, dir)
	require.NoError(t, err)

	f, err := New(ctx, localFs, name, "", "")
	require.NoError(t, err)

	remotes := allRemotes(t, f)

	// The one benign entry must survive.
	assert.Contains(t, remotes, "good.txt")

	// No escaping entry may be exposed.
	for _, remote := range remotes {
		assert.False(t, remote == ".." || strings.HasPrefix(remote, "../"),
			"escaping remote exposed: %q", remote)
	}
	assert.Equal(t, []string{"good.txt"}, remotes)
}

// Mounting with a non-empty root must only expose entries within that
// root directory, not sibling directories that merely share a name
// prefix (root "foo" must not match "foobar").
func TestReadZipRootBoundary(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	name := writeZip(t, dir, "test.zip",
		"foo/a.txt",
		"foobar/b.txt",
	)

	localFs, err := cache.Get(ctx, dir)
	require.NoError(t, err)

	f, err := New(ctx, localFs, name, "", "foo")
	require.NoError(t, err)

	remotes := allRemotes(t, f)
	assert.Equal(t, []string{"a.txt"}, remotes)
}

// A file entry whose name refers to a directory (".", "/", "./", "" or
// "sub/.") must be skipped, not turn the archive or the directory
// mounted as its root into a single file which hides every other
// entry, or become an object named for the archive itself when the
// archive is found by listing its directory.
func TestReadZipDirNamedFileEntry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	name := writeZip(t, dir, "dot.zip",
		".",
		"/",
		"./",
		"",
		"good.txt",
		"sub/",
		"sub/.",
		"sub/good.txt",
	)

	localFs, err := cache.Get(ctx, dir)
	require.NoError(t, err)

	for _, test := range []struct {
		prefix, root string
		want         []string
	}{
		{"", "", []string{"good.txt", "sub/good.txt"}},
		{"sub/dot.zip", "", []string{"sub/dot.zip/good.txt", "sub/dot.zip/sub/good.txt"}},
		{"", "sub", []string{"good.txt"}},
	} {
		t.Run(fmt.Sprintf("prefix=%q,root=%q", test.prefix, test.root), func(t *testing.T) {
			f, err := New(ctx, localFs, name, test.prefix, test.root)
			require.NoError(t, err)
			assert.Equal(t, test.want, allRemotes(t, f))
		})
	}
}

// Listings are served from a cache which must survive callers
// filtering the returned slice in place, as fs/list does.
func TestListCacheNotAliased(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	name := writeZip(t, dir, "test.zip", "a.txt", "b.txt", "c.txt")

	localFs, err := cache.Get(ctx, dir)
	require.NoError(t, err)
	f, err := New(ctx, localFs, name, "", "")
	require.NoError(t, err)

	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, entries, 3)
	// Compact in place, dropping the first entry
	copy(entries, entries[1:])

	entries, err = f.List(ctx, "")
	require.NoError(t, err)
	var remotes []string
	for _, entry := range entries {
		remotes = append(remotes, entry.Remote())
	}
	assert.Equal(t, []string{"a.txt", "b.txt", "c.txt"}, remotes)
}
