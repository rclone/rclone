//go:build linux

package mount

import (
	"context"
	"testing"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInode checks that Attr and ReadDirAll report the VFS inode numbers
func TestInode(t *testing.T) {
	ctx := context.Background()
	f, err := fs.NewFs(ctx, ":memory:"+t.Name())
	require.NoError(t, err)
	VFS := vfs.New(ctx, f, nil)
	defer VFS.Shutdown()
	require.NoError(t, VFS.Mkdir("dir", 0777))
	require.NoError(t, VFS.WriteFile("dir/file", []byte("hello"), 0666))

	fsys := NewFS(VFS, &mountlib.Opt)
	rootNode, err := fsys.Root()
	require.NoError(t, err)
	root := rootNode.(*Dir)

	checkAttr := func(node fusefs.Node, want uint64) {
		var a fuse.Attr
		require.NoError(t, node.Attr(ctx, &a))
		assert.Equal(t, want, a.Inode)
	}
	checkDirents := func(d *Dir, parentInode uint64, wantNames ...string) {
		dirents, err := d.ReadDirAll(ctx)
		require.NoError(t, err)
		var names []string
		for _, dirent := range dirents {
			names = append(names, dirent.Name)
		}
		assert.ElementsMatch(t, append([]string{".", ".."}, wantNames...), names)
		for _, dirent := range dirents {
			switch dirent.Name {
			case ".":
				assert.Equal(t, d.Inode(), dirent.Inode)
			case "..":
				assert.Equal(t, parentInode, dirent.Inode)
			default:
				node, err := d.Dir.Stat(dirent.Name)
				require.NoError(t, err)
				assert.Equal(t, node.Inode(), dirent.Inode, dirent.Name)
			}
		}
	}

	checkAttr(root, root.Inode())
	checkDirents(root, root.Inode(), "dir")

	dirNode, err := root.Lookup(ctx, &fuse.LookupRequest{Name: "dir"}, &fuse.LookupResponse{})
	require.NoError(t, err)
	dir := dirNode.(*Dir)
	checkAttr(dir, dir.Inode())
	checkDirents(dir, root.Inode(), "file")

	fileNode, err := dir.Lookup(ctx, &fuse.LookupRequest{Name: "file"}, &fuse.LookupResponse{})
	require.NoError(t, err)
	file := fileNode.(*File)
	checkAttr(file, file.Inode())
}
