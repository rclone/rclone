//go:build cmount && ((linux && cgo) || (darwin && cgo) || (freebsd && cgo) || (openbsd && cgo) || windows) && (!race || !windows)

package cmount

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/winfsp/cgofuse/fuse"
)

// TestMkdirExclusive verifies that creating an already existing name at the
// filesystem boundary fails with EEXIST, while the idempotent VFS contract
// used by rclone's internal callers is preserved.
func TestMkdirExclusive(t *testing.T) {
	root := t.TempDir()
	f, err := fs.NewFs(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	opt := vfscommon.Opt
	opt.PollInterval = 0
	v := vfs.New(context.Background(), f, &opt)
	t.Cleanup(v.Shutdown)
	mount := NewFS(v, &mountlib.Options{})
	if got := mount.Mkdir("/dir", 0777); got != 0 {
		t.Fatalf("create: %d", got)
	}
	if got := mount.Mkdir("/dir", 0777); got != -fuse.EEXIST {
		t.Fatalf("duplicate: %d", got)
	}
	if runtime.GOOS == "windows" {
		// The local backend on Windows is case-insensitive.
		if got := mount.Mkdir("/DIR", 0777); got != -fuse.EEXIST {
			t.Fatalf("case-insensitive duplicate: %d", got)
		}
	}
	// Preserve the internal VFS contract used by rclone operations.
	parent, err := v.Root()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parent.Mkdir("dir"); err != nil {
		t.Fatalf("VFS idempotence changed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("unchanged"), 0644); err != nil {
		t.Fatal(err)
	}
	parent.ForgetAll()
	if got := mount.Mkdir("/file", 0777); got != -fuse.EEXIST {
		t.Fatalf("file collision: %d", got)
	}
	b, err := os.ReadFile(filepath.Join(root, "file"))
	if err != nil || string(b) != "unchanged" {
		t.Fatalf("existing file changed: %q %v", b, err)
	}
	if got := mount.Mkdir("/missing/child", 0777); got != -fuse.ENOENT {
		t.Fatalf("missing parent: %d", got)
	}
	const n = 16
	start := make(chan struct{})
	results := make(chan int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- mount.Mkdir("/race", 0777) }()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for got := range results {
		if got == 0 {
			successes++
		} else if got != -fuse.EEXIST {
			t.Fatalf("concurrent mkdir: %d", got)
		}
	}
	if successes != 1 {
		t.Fatalf("expected one winner, got %d", successes)
	}
}
