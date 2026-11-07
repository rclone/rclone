//go:build !openbsd && !plan9

package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/xattr"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestXattrThroughPlantedSymlinkBlocked checks user metadata (xattrs)
// can't be written outside the root through a planted symlink with
// --links, while in-tree writes still work.
func TestXattrThroughPlantedSymlinkBlocked(t *testing.T) {
	if !xattrSupported {
		t.Skip("xattrs not supported on this OS")
	}
	skipIfNoSymlinks(t)
	ctx := context.Background()

	r := fstest.NewRun(t)
	f := r.Flocal.(*Fs)
	linksMode(f)

	metadata := fs.Metadata{
		"pwned": "attacker-controlled-value",
	}

	// Control: an in-tree directory gets the xattr as normal.
	_, err := f.MkdirMetadata(ctx, "sub", metadata)
	require.NoError(t, err)
	if f.xattrSupported.Load() == 0 {
		t.Skip("xattrs not supported on this filesystem")
	}
	v, err := xattr.LGet(filepath.Join(f.root, "sub"), "user.pwned")
	require.NoError(t, err)
	assert.Equal(t, "attacker-controlled-value", string(v))

	// A directory outside the destination, with a pre-existing child,
	// whose xattrs the attacker targets.
	evil := t.TempDir()
	evilDir := filepath.Join(evil, "secret.d")
	require.NoError(t, os.Mkdir(evilDir, 0700))
	evilSub := filepath.Join(evilDir, "sub")
	require.NoError(t, os.Mkdir(evilSub, 0700))

	// The planted symlink dst/pwn -> outside, faithfully reproduced by --links.
	require.NoError(t, putLink(ctx, f, "pwn", evilDir))

	// The source now presents "pwn/sub" as a directory with attacker-chosen
	// user metadata. Applying it must not reach through the planted symlink.
	_, err = f.MkdirMetadata(ctx, "pwn/sub", metadata)
	require.Error(t, err, "applying metadata through a planted symlink should be refused")

	_, err = xattr.LGet(evilSub, "user.pwned")
	require.Error(t, err, "xattr escaped through planted symlink to %q", evilSub)
}

// TestXattrWriteOnlyFile checks user metadata can still be set on a
// write-only file with --links, where the confined open can't read it.
func TestXattrWriteOnlyFile(t *testing.T) {
	if !xattrSupported {
		t.Skip("xattrs not supported on this OS")
	}
	skipIfNoSymlinks(t)
	ctx := context.Background()

	r := fstest.NewRun(t)
	f := r.Flocal.(*Fs)
	linksMode(f)

	require.NoError(t, putFile(ctx, f, "wo.txt", "write only"))
	path := filepath.Join(f.root, "wo.txt")
	require.NoError(t, os.Chmod(path, 0200))

	obj, err := f.NewObject(ctx, "wo.txt")
	require.NoError(t, err)
	o := obj.(*Object)
	err = o.setXattr(fs.Metadata{"potato": "chips"})
	require.NoError(t, err)
	if f.xattrSupported.Load() == 0 {
		t.Skip("xattrs not supported on this filesystem")
	}
	// Reading xattrs needs read permission
	require.NoError(t, os.Chmod(path, 0600))
	v, err := xattr.LGet(path, "user.potato")
	require.NoError(t, err)
	assert.Equal(t, "chips", string(v))
}
