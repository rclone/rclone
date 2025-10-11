package vfs

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/require"
)

func readZip(t *testing.T, buf *bytes.Buffer) *zip.Reader {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	return r
}

func mustCreateZip(t *testing.T, d *Dir) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, CreateZip(context.Background(), d, &buf))
	return &buf
}

func zipReadFile(t *testing.T, zr *zip.Reader, match func(name string) bool) ([]byte, string) {
	t.Helper()
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		if match(f.Name) {
			rc, err := f.Open()
			require.NoError(t, err)
			defer func() { require.NoError(t, rc.Close()) }()
			b, err := io.ReadAll(rc)
			require.NoError(t, err)
			return b, f.Name
		}
	}
	t.Fatalf("zip entry matching predicate not found")
	return nil, ""
}

func TestZipManyFiles(t *testing.T) {
	r, vfs := newTestVFS(t)

	const N = 5
	want := make(map[string]string, N)
	items := make([]fstest.Item, 0, N)

	for i := range N {
		name := fmt.Sprintf("flat/f%03d.txt", i)
		data := strings.Repeat(fmt.Sprintf("line-%d\n", i), (i%5)+1)
		it := r.WriteObject(context.Background(), name, data, t1)
		items = append(items, it)
		want[name[strings.LastIndex(name, "/")+1:]] = data
	}
	r.CheckRemoteItems(t, items...)

	node, err := vfs.Stat("flat")
	require.NoError(t, err)
	dir := node.(*Dir)

	buf := mustCreateZip(t, dir)
	zr := readZip(t, buf)

	// count only file entries (skip dir entries with trailing "/")
	files := 0
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, "/") {
			files++
		}
	}
	require.Equal(t, N, files)

	// validate contents by base name
	for base, data := range want {
		got, _ := zipReadFile(t, zr, func(name string) bool { return name == base })
		require.Equal(t, data, string(got), "mismatch for %s", base)
	}
}

func TestZipManySubDirs(t *testing.T) {
	r, vfs := newTestVFS(t)

	r.WriteObject(context.Background(), "a/top.txt", "top", t1)
	r.WriteObject(context.Background(), "a/b/mid.txt", "mid", t1)
	r.WriteObject(context.Background(), "a/b/c/deep.txt", "deep", t1)

	node, err := vfs.Stat("a")
	require.NoError(t, err)
	dir := node.(*Dir)

	buf := mustCreateZip(t, dir)
	zr := readZip(t, buf)

	// paths may include directory prefixes; assert by suffix
	got, name := zipReadFile(t, zr, func(n string) bool { return strings.HasSuffix(n, "/top.txt") || n == "top.txt" })
	require.Equal(t, "top", string(got), "bad content for %s", name)

	got, name = zipReadFile(t, zr, func(n string) bool { return strings.HasSuffix(n, "/mid.txt") || n == "mid.txt" })
	require.Equal(t, "mid", string(got), "bad content for %s", name)

	got, name = zipReadFile(t, zr, func(n string) bool { return strings.HasSuffix(n, "/deep.txt") || n == "deep.txt" })
	require.Equal(t, "deep", string(got), "bad content for %s", name)
}

func TestZipLargeFiles(t *testing.T) {
	r, vfs := newTestVFS(t)

	if strings.HasPrefix(r.Fremote.Name(), "TestChunker") {
		t.Skip("skipping test as chunker too slow")
	}

	data := random.String(5 * 1024 * 1024)
	sum := sha256.Sum256([]byte(data))

	r.WriteObject(context.Background(), "bigdir/big.bin", data, t1)

	node, err := vfs.Stat("bigdir")
	require.NoError(t, err)
	dir := node.(*Dir)

	buf := mustCreateZip(t, dir)
	zr := readZip(t, buf)

	got, _ := zipReadFile(t, zr, func(n string) bool { return n == "big.bin" || strings.HasSuffix(n, "/big.bin") })
	require.Equal(t, sum, sha256.Sum256(got))
}

func TestZipDirsInRoot(t *testing.T) {
	r, vfs := newTestVFS(t)

	r.WriteObject(context.Background(), "dir1/a.txt", "x", t1)
	r.WriteObject(context.Background(), "dir2/b.txt", "y", t1)
	r.WriteObject(context.Background(), "dir3/c.txt", "z", t1)

	root, err := vfs.Root()
	require.NoError(t, err)

	buf := mustCreateZip(t, root)
	zr := readZip(t, buf)

	// Check each file exists (ignore exact directory-entry names)
	gx, _ := zipReadFile(t, zr, func(n string) bool { return strings.HasSuffix(n, "/a.txt") })
	require.Equal(t, "x", string(gx))

	gy, _ := zipReadFile(t, zr, func(n string) bool { return strings.HasSuffix(n, "/b.txt") })
	require.Equal(t, "y", string(gy))

	gz, _ := zipReadFile(t, zr, func(n string) bool { return strings.HasSuffix(n, "/c.txt") })
	require.Equal(t, "z", string(gz))
}
