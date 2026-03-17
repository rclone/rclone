package sqlar_test

// Compatibility tests between rclone's sqlar backend and the sqlite3
// command-line tool's -A (archive) mode.
//
// Run with:
//
//	go test ./backend/sqlar/ -run TestSqlite3Compat -v
//
// The tests are skipped automatically when sqlite3 is not in PATH.

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/sqlar"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lookupSqlite3 returns the path to the sqlite3 binary or skips the test.
// Compatibility tests require sqlite 3.23.0 or later for sqlar support.
func lookupSqlite3(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 not found in PATH; skipping compatibility tests")
	}

	out, err := exec.Command(p, "-version").Output()
	if err != nil {
		t.Skipf("failed to query sqlite3 version: %v; skipping compatibility tests", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		t.Skip("could not parse sqlite3 version; skipping compatibility tests")
	}
	if !sqliteVersionAtLeast(fields[0], 3, 23, 0) {
		t.Skipf("sqlite3 version %s is too old; need 3.23.0 or later for compatibility tests", fields[0])
	}
	return p
}

func sqliteVersionAtLeast(version string, wantMajor, wantMinor, wantPatch int) bool {
	parts := strings.SplitN(version, ".", 4)
	if len(parts) < 3 {
		return false
	}
	got := make([]int, 3)
	for i := range 3 {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return false
		}
		got[i] = n
	}
	want := []int{wantMajor, wantMinor, wantPatch}
	for i := range 3 {
		if got[i] != want[i] {
			return got[i] > want[i]
		}
	}
	return true
}

// compatObjInfo is a minimal fs.ObjectInfo for use in compatibility tests.
type compatObjInfo struct {
	remote  string
	size    int64
	modTime time.Time
}

func (o *compatObjInfo) Fs() fs.Info                                         { return nil }
func (o *compatObjInfo) Remote() string                                      { return o.remote }
func (o *compatObjInfo) String() string                                      { return o.remote }
func (o *compatObjInfo) ModTime(_ context.Context) time.Time                 { return o.modTime }
func (o *compatObjInfo) Size() int64                                         { return o.size }
func (o *compatObjInfo) Hash(_ context.Context, _ hash.Type) (string, error) { return "", nil }
func (o *compatObjInfo) Storable() bool                                      { return true }

// openCompatFs opens a sqlar Fs at archivePath with the default compression
// level (-1, zlib default). compression_level must be set explicitly because
// configmap.Simple does not carry the option defaults declared in
// RegInfo.Options; without it the Go zero value (0) would be used, silently
// disabling compression.
func openCompatFs(t *testing.T, archivePath string) fs.Fs {
	t.Helper()
	m := configmap.Simple{"path": archivePath, "compression_level": "-1"}
	f, err := sqlar.NewFs(context.Background(), "compat-test", "", m)
	require.NoError(t, err)
	return f
}

// closeCompatFs flushes the WAL and closes the Fs.
func closeCompatFs(t *testing.T, f fs.Fs) {
	t.Helper()
	if s, ok := f.(fs.Shutdowner); ok {
		require.NoError(t, s.Shutdown(context.Background()))
	}
}

// putFile is a helper that writes content to remote in f.
func putFile(t *testing.T, f fs.Fs, remote string, content []byte) {
	t.Helper()
	info := &compatObjInfo{
		remote:  remote,
		size:    int64(len(content)),
		modTime: time.Unix(1700000000, 0),
	}
	_, err := f.Put(context.Background(), bytes.NewReader(content), info)
	require.NoError(t, err, "Put %s", remote)
}

// TestSqlite3Compat_RcloneWrite_SqliteList writes a file with rclone and
// verifies that sqlite3 -At lists it correctly.
func TestSqlite3Compat_RcloneWrite_SqliteList(t *testing.T) {
	sqlite3 := lookupSqlite3(t)

	dir := t.TempDir()
	arch := filepath.Join(dir, "test.sqlar")

	f := openCompatFs(t, arch)
	putFile(t, f, "hello.txt", []byte("hello sqlite3"))
	closeCompatFs(t, f)

	out, err := exec.Command(sqlite3, arch, "-At").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "hello.txt")
}

// TestSqlite3Compat_RcloneWrite_SqliteExtract_Uncompressed writes a file that
// is too small to compress (stored verbatim) and verifies sqlite3 -Ax
// extracts identical bytes.
func TestSqlite3Compat_RcloneWrite_SqliteExtract_Uncompressed(t *testing.T) {
	sqlite3 := lookupSqlite3(t)

	dir := t.TempDir()
	arch := filepath.Join(dir, "test.sqlar")

	// Short content that deflate won't shrink → stored as-is (sz == len(data)).
	content := []byte("tiny")
	f := openCompatFs(t, arch)
	putFile(t, f, "tiny.txt", content)
	closeCompatFs(t, f)

	extractDir := t.TempDir()
	cmd := exec.Command(sqlite3, arch, "-Ax")
	cmd.Dir = extractDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "sqlite3 -Ax: %s", out)

	got, err := os.ReadFile(filepath.Join(extractDir, "tiny.txt"))
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

// TestSqlite3Compat_RcloneWrite_SqliteExtract_Compressed writes a
// compressible file with rclone and verifies sqlite3 -Ax extracts identical
// bytes.
func TestSqlite3Compat_RcloneWrite_SqliteExtract_Compressed(t *testing.T) {
	sqlite3 := lookupSqlite3(t)

	dir := t.TempDir()
	arch := filepath.Join(dir, "test.sqlar")

	// Repetitive content that deflate will shrink → stored compressed.
	content := bytes.Repeat([]byte("compressible content "), 200)
	f := openCompatFs(t, arch)
	putFile(t, f, "data.txt", content)
	closeCompatFs(t, f)

	extractDir := t.TempDir()
	cmd := exec.Command(sqlite3, arch, "-Ax")
	cmd.Dir = extractDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "sqlite3 -Ax: %s", out)

	got, err := os.ReadFile(filepath.Join(extractDir, "data.txt"))
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

// TestSqlite3Compat_SqliteWrite_RcloneRead_Uncompressed creates an archive
// with sqlite3 -Ac containing a file too small to compress, then reads it
// with rclone and verifies the content is identical.
func TestSqlite3Compat_SqliteWrite_RcloneRead_Uncompressed(t *testing.T) {
	sqlite3 := lookupSqlite3(t)

	dir := t.TempDir()
	arch := filepath.Join(dir, "test.sqlar")

	content := []byte("tiny uncompressed")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tiny.txt"), content, 0644))

	cmd := exec.Command(sqlite3, arch, "-Ac", "tiny.txt")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "sqlite3 -Ac: %s", out)

	f := openCompatFs(t, arch)
	defer closeCompatFs(t, f)
	ctx := context.Background()

	obj, err := f.NewObject(ctx, "tiny.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), obj.Size())

	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

// TestSqlite3Compat_SqliteWrite_RcloneRead_Compressed creates an archive with
// sqlite3 -Ac containing a compressible file, then reads it with rclone and
// verifies the content is identical.
func TestSqlite3Compat_SqliteWrite_RcloneRead_Compressed(t *testing.T) {
	sqlite3 := lookupSqlite3(t)

	dir := t.TempDir()
	arch := filepath.Join(dir, "test.sqlar")

	// Repetitive content that sqlite3 will compress using zlib.
	content := bytes.Repeat([]byte("compressible content "), 200)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.txt"), content, 0644))

	cmd := exec.Command(sqlite3, arch, "-Ac", "data.txt")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "sqlite3 -Ac: %s", out)

	f := openCompatFs(t, arch)
	defer closeCompatFs(t, f)
	ctx := context.Background()

	obj, err := f.NewObject(ctx, "data.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), obj.Size())

	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

// TestSqlite3Compat_RoundTrip writes multiple files with rclone (including
// nested paths), then verifies sqlite3 -At lists all of them and sqlite3 -Ax
// extracts identical content.
func TestSqlite3Compat_RoundTrip(t *testing.T) {
	sqlite3 := lookupSqlite3(t)

	dir := t.TempDir()
	arch := filepath.Join(dir, "test.sqlar")

	files := []struct {
		name    string
		content []byte
	}{
		{"a.txt", []byte("uncompressed: short")},
		{"b.txt", bytes.Repeat([]byte("compressible "), 150)},
		{"sub/c.txt", bytes.Repeat([]byte("nested content "), 100)},
	}

	f := openCompatFs(t, arch)
	for _, file := range files {
		putFile(t, f, file.name, file.content)
	}
	closeCompatFs(t, f)

	// All file names must appear in the sqlite3 listing.
	listOut, err := exec.Command(sqlite3, arch, "-At").Output()
	require.NoError(t, err)
	listing := string(listOut)
	for _, file := range files {
		assert.True(t, strings.Contains(listing, file.name),
			"expected %q in sqlite3 listing:\n%s", file.name, listing)
	}

	// sqlite3 -Ax must extract identical content for every file.
	extractDir := t.TempDir()
	cmd := exec.Command(sqlite3, arch, "-Ax")
	cmd.Dir = extractDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "sqlite3 -Ax: %s", out)

	for _, file := range files {
		got, err := os.ReadFile(filepath.Join(extractDir, file.name))
		require.NoError(t, err, "read extracted %s", file.name)
		assert.Equal(t, file.content, got, "content mismatch for %s", file.name)
	}
}
