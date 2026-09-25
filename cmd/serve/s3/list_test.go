package s3

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rclone/gofakes3"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newListBackend serves a temporary directory populated with the given files,
// whose paths are relative to the serve root and so start with a bucket name.
func newListBackend(t *testing.T, files ...string) *s3Backend {
	fstest.Initialise()
	ctx := context.Background()
	root := t.TempDir()
	for _, file := range files {
		p := filepath.Join(root, filepath.FromSlash(file))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0777))
		require.NoError(t, os.WriteFile(p, []byte(file), 0666))
	}

	f, err := fs.NewFs(ctx, root)
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{endpoint}
	w, err := newServer(ctx, f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err)

	// Use the server's own backend so that a test which drives the server
	// and one which calls the backend directly share the same state.
	return w.backend
}

// listPrefix makes a listing prefix, with a delimiter if delimiter is set.
func listPrefix(prefix, delimiter string) *gofakes3.Prefix {
	return &gofakes3.Prefix{
		HasPrefix:    prefix != "",
		Prefix:       prefix,
		HasDelimiter: delimiter != "",
		Delimiter:    delimiter,
	}
}

// listOnce returns the keys of one page of a listing, common prefixes and
// object keys merged into the single sorted stream S3 clients see.
func listOnce(t *testing.T, b *s3Backend, prefix *gofakes3.Prefix, page gofakes3.ListBucketPage) (keys []string, list *gofakes3.ObjectList) {
	list, err := b.ListBucket(context.Background(), "bucket", prefix, page)
	require.NoError(t, err)
	for _, commonPrefix := range list.CommonPrefixes {
		keys = append(keys, commonPrefix.Prefix)
	}
	for _, object := range list.Contents {
		keys = append(keys, object.Key)
	}
	slices.Sort(keys)
	return keys, list
}

// listPages pages through a whole listing maxKeys at a time, the way an S3
// client does, and returns the keys of every page concatenated.
func listPages(t *testing.T, b *s3Backend, prefix *gofakes3.Prefix, maxKeys int) (keys []string) {
	page := gofakes3.ListBucketPage{MaxKeys: int64(maxKeys)}
	for pages := 0; ; pages++ {
		require.Less(t, pages, 1000, "listing did not terminate")
		pageKeys, list := listOnce(t, b, prefix, page)
		assert.LessOrEqual(t, len(pageKeys), maxKeys, "page has more than max-keys keys")
		keys = append(keys, pageKeys...)
		if !list.IsTruncated {
			return keys
		}
		require.NotEmpty(t, list.NextMarker, "truncated page has no next marker")
		require.Equal(t, list.NextMarker, pageKeys[len(pageKeys)-1], "next marker is not the last key of the page")
		page.Marker, page.HasMarker = list.NextMarker, true
	}
}

// TestListFlatKeyOrder checks that keys come back in the order of a flat S3
// keyspace rather than in the order a directory walk finds them: "a.txt"
// sorts before "a/b" because '.' < '/'.
func TestListFlatKeyOrder(t *testing.T) {
	b := newListBackend(t, "bucket/a/b", "bucket/a.txt", "bucket/a0")

	keys, list := listOnce(t, b, listPrefix("", ""), gofakes3.ListBucketPage{MaxKeys: 10})
	assert.Equal(t, []string{"a.txt", "a/b", "a0"}, keys)
	assert.False(t, list.IsTruncated)

	// Paging must not skip a key across the "a.txt"/"a/b" boundary
	assert.Equal(t, []string{"a.txt", "a/b", "a0"}, listPages(t, b, listPrefix("", ""), 1))
}

// TestListPaging checks that paging through a listing returns every key
// exactly once, whatever the page size.
func TestListPaging(t *testing.T) {
	var files []string
	for dir := range 5 {
		for file := range 4 {
			files = append(files, fmt.Sprintf("bucket/dir%d/file%d", dir, file))
		}
		files = append(files, fmt.Sprintf("bucket/dir%d.txt", dir))
	}
	b := newListBackend(t, files...)

	all, list := listOnce(t, b, listPrefix("", ""), gofakes3.ListBucketPage{MaxKeys: 1000})
	require.False(t, list.IsTruncated)
	require.Len(t, all, 25)
	assert.True(t, slices.IsSorted(all), "unpaged listing is not sorted: %q", all)

	for _, maxKeys := range []int{1, 2, 3, 7, 24, 25, 26} {
		t.Run(fmt.Sprintf("MaxKeys%d", maxKeys), func(t *testing.T) {
			assert.Equal(t, all, listPages(t, b, listPrefix("", ""), maxKeys))
		})
	}
}

// TestListPagingWithPrefix checks paging a listing which is filtered by a
// prefix that is not a whole path segment.
func TestListPagingWithPrefix(t *testing.T) {
	b := newListBackend(t,
		"bucket/apple/1", "bucket/apple/2", "bucket/apricot/1",
		"bucket/apt.txt", "bucket/banana/1",
	)
	prefix := listPrefix("ap", "")

	all, _ := listOnce(t, b, prefix, gofakes3.ListBucketPage{MaxKeys: 1000})
	assert.Equal(t, []string{"apple/1", "apple/2", "apricot/1", "apt.txt"}, all)

	for _, maxKeys := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("MaxKeys%d", maxKeys), func(t *testing.T) {
			assert.Equal(t, all, listPages(t, b, prefix, maxKeys))
		})
	}
}

// TestListDelimiter checks that a delimited listing pages through common
// prefixes and object keys as one merged stream.
func TestListDelimiter(t *testing.T) {
	b := newListBackend(t,
		"bucket/a/1", "bucket/b.txt", "bucket/c/1", "bucket/d.txt", "bucket/e/1",
	)
	prefix := listPrefix("", "/")

	all, _ := listOnce(t, b, prefix, gofakes3.ListBucketPage{MaxKeys: 1000})
	assert.Equal(t, []string{"a/", "b.txt", "c/", "d.txt", "e/"}, all)

	for _, maxKeys := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprintf("MaxKeys%d", maxKeys), func(t *testing.T) {
			assert.Equal(t, all, listPages(t, b, prefix, maxKeys))
		})
	}
}

// countDirs lists one page directly with a lister so that the number of
// directories the walk had to read can be checked.
func countDirs(t *testing.T, b *s3Backend, page gofakes3.ListBucketPage) (dirsRead int, list *gofakes3.ObjectList) {
	_vfs, err := b.s.getVFS(context.Background())
	require.NoError(t, err)
	list = gofakes3.NewObjectList()
	l := newLister(b, _vfs, "bucket", page, list)
	err = l.list("", "", false)
	if err != nil && !errors.Is(err, errPageFull) {
		require.NoError(t, err)
	}
	return l.dirsRead, list
}

// TestListReadsOnlyTheDirectoriesItNeeds checks that a page costs a number of
// directory reads proportional to the keys it returns, not to the size of the
// subtree, and that resuming from a marker does not re-read the subtrees
// earlier pages already returned.
func TestListReadsOnlyTheDirectoriesItNeeds(t *testing.T) {
	const dirs = 20
	var files []string
	for dir := range dirs {
		for file := range 5 {
			files = append(files, fmt.Sprintf("bucket/dir%02d/file%d", dir, file))
		}
	}
	b := newListBackend(t, files...)

	// A full listing has to read the bucket and every directory in it
	whole, list := countDirs(t, b, gofakes3.ListBucketPage{MaxKeys: 1000})
	require.False(t, list.IsTruncated)
	require.Len(t, list.Contents, dirs*5)
	assert.Equal(t, dirs+1, whole)

	// Each page of 5 reads the bucket, the directory holding the marker,
	// the directory it returns and the one holding the following key
	page := gofakes3.ListBucketPage{MaxKeys: 5}
	for pages := range dirs {
		dirsRead, list := countDirs(t, b, page)
		assert.LessOrEqual(t, dirsRead, 4, "page %d read too many directories", pages)
		assert.Less(t, dirsRead, whole, "page %d walked the whole tree", pages)
		require.Len(t, list.Contents, 5)
		assert.Equal(t, pages == dirs-1, !list.IsTruncated)
		page.Marker, page.HasMarker = list.NextMarker, true
	}
}

// TestListHidesTemporaryObjects checks that the temporary objects of
// in-progress uploads stay hidden and do not use up a page.
func TestListHidesTemporaryObjects(t *testing.T) {
	b := newListBackend(t,
		"bucket/a.txt",
		"bucket/"+putObjectPrefix+"hidden",
		"bucket/"+legacyMultipartUploadPrefix+"hidden",
		"bucket/z.txt",
	)

	keys, list := listOnce(t, b, listPrefix("", ""), gofakes3.ListBucketPage{MaxKeys: 2})
	assert.Equal(t, []string{"a.txt", "z.txt"}, keys)
	assert.False(t, list.IsTruncated)
}

// TestListSortKey checks the key a directory entry sorts under.
func TestListSortKey(t *testing.T) {
	b := newListBackend(t, "bucket/dir/file", "bucket/file")
	_vfs, err := b.s.getVFS(context.Background())
	require.NoError(t, err)

	entries, err := getDirEntries("bucket", _vfs)
	require.NoError(t, err)
	got := map[string]string{}
	for _, entry := range entries {
		got[entry.Name()] = sortKey(entry)
	}
	assert.Equal(t, map[string]string{"dir": "dir/", "file": "file"}, got)
}

// TestListSkipDir checks which subtrees a marker allows to be skipped unread.
func TestListSkipDir(t *testing.T) {
	for _, test := range []struct {
		marker string
		dir    string
		want   bool
	}{
		{marker: "", dir: "a/", want: false},        // no marker, list everything
		{marker: "a", dir: "b/", want: false},       // marker before the subtree
		{marker: "b/9", dir: "b/", want: false},     // marker inside the subtree
		{marker: "b/", dir: "b/", want: false},      // marker is the subtree
		{marker: "b0", dir: "b/", want: true},       // marker past the subtree
		{marker: "c", dir: "b/", want: true},        // marker past the subtree
		{marker: "b.txt", dir: "b/", want: false},   // '.' sorts before '/'
		{marker: "b/a", dir: "b/c/", want: false},   // marker before the subtree
		{marker: "b/c/9", dir: "b/c/", want: false}, // marker inside the subtree
		{marker: "b/d", dir: "b/c/", want: true},    // marker past the subtree
	} {
		l := &lister{marker: test.marker, hasMarker: test.marker != ""}
		assert.Equal(t, test.want, l.skipDir(test.dir), "marker %q dir %q", test.marker, test.dir)
	}
}
