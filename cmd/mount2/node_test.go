//go:build linux || (darwin && amd64)

package mount2

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// statOf returns the os.FileInfo for a name in dir.
func statOf(t *testing.T, dir, name string) os.FileInfo {
	t.Helper()
	fi, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

// TestDirStreamSeekdir checks that Seekdir repositions the snapshot directory
// stream to an arbitrary offset. A kernel-NFS-exported mount needs this for
// readdir continuation: the stateless server opens a fresh handle and seeks to
// the last returned cookie on every batch. Offset 0 must still reset to the
// start (the rewind / re-read case) and offsets at or past the end must clamp
// to EOF.
func TestDirStreamSeekdir(t *testing.T) {
	ctx := context.Background()
	// 3 real entries plus the synthesized "." and ".." => 5 entries total,
	// occupying internal indices 0..4 (go-fuse offsets 1..5). HasNext is true
	// while i < len(nodes)+2, i.e. i < 5.
	ds := &dirStream{nodes: make([]os.FileInfo, 3)}

	for _, tc := range []struct {
		off      uint64
		wantI    int
		wantNext bool
	}{
		{0, 0, true},    // rewind to start (the re-read case)
		{2, 2, true},    // resume at the first real entry
		{4, 4, true},    // last entry
		{5, 5, false},   // exactly at end => EOF
		{99, 99, false}, // past end => clamped to EOF via HasNext
	} {
		if errno := ds.Seekdir(ctx, tc.off); errno != 0 {
			t.Fatalf("Seekdir(%d) returned errno %v, want 0", tc.off, errno)
		}
		if ds.i != tc.wantI {
			t.Errorf("Seekdir(%d): i = %d, want %d", tc.off, ds.i, tc.wantI)
		}
		if got := ds.HasNext(); got != tc.wantNext {
			t.Errorf("Seekdir(%d): HasNext() = %v, want %v", tc.off, got, tc.wantNext)
		}
	}
}

// inoFileInfo is an os.FileInfo that also carries an inode number, as a
// vfs.Node does. dirStream needs nothing else from an entry.
type inoFileInfo struct {
	os.FileInfo
	inode uint64
}

func (f *inoFileInfo) Inode() uint64 { return f.inode }

// TestDirStreamInodes checks that Readdir reports an inode number for every
// entry. Returning 0 makes go-fuse substitute FUSE_UNKNOWN_INO, which the
// kernel passes through to getdents64 as a bogus d_ino, and which defeats
// the directory scan the kernel uses to reconnect an NFS file handle.
func TestDirStreamInodes(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"file1.txt", "noinode.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	ds := &dirStream{
		dirIno:    100,
		parentIno: 200,
		nodes: []os.FileInfo{
			&inoFileInfo{FileInfo: statOf(t, dir, "file1.txt"), inode: 42},
			&inoFileInfo{FileInfo: statOf(t, dir, "subdir"), inode: 43},
			statOf(t, dir, "noinode.txt"), // plain os.FileInfo
		},
	}

	for _, want := range []struct {
		name string
		ino  uint64
	}{
		{".", 100},
		{"..", 200},
		{"file1.txt", 42},
		{"subdir", 43},
		{"noinode.txt", 0}, // no inode number: go-fuse assigns one
	} {
		if !ds.HasNext() {
			t.Fatalf("HasNext() = false before %q", want.name)
		}
		de, errno := ds.Next()
		if errno != 0 {
			t.Fatalf("Next() for %q returned errno %v", want.name, errno)
		}
		if de.Name != want.name {
			t.Fatalf("Next() = %q, want %q", de.Name, want.name)
		}
		if de.Ino != want.ino {
			t.Errorf("%q: Ino = %d, want %d", de.Name, de.Ino, want.ino)
		}
	}
}
