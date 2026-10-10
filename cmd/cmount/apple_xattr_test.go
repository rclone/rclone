//go:build cmount && darwin && cgo

package cmount

import (
	"testing"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/vfs"
	"github.com/winfsp/cgofuse/fuse"
)

func TestIgnoreAppleXattr(t *testing.T) {
	f := &FS{opt: &mountlib.Options{NoAppleXattr: true}}
	name := "com.apple.FinderInfo"
	if got := f.Setxattr("/file", name, []byte("metadata"), 0); got != 0 {
		t.Fatalf("Apple attribute write failed: %d", got)
	}
	if got, value := f.Getxattr("/file", name); got != -fuse.ENOATTR || len(value) != 0 {
		t.Fatalf("ignored attribute was retained: %d %q", got, value)
	}
	if got := f.Removexattr("/file", name); got != 0 {
		t.Fatalf("Apple attribute removal failed: %d", got)
	}
	if got := f.Listxattr("/file", func(name string) bool {
		t.Errorf("ignored attribute listed: %s", name)
		return true
	}); got != 0 {
		t.Fatalf("attribute listing failed: %d", got)
	}
	if got := f.Setxattr("/file", "user.other", []byte("metadata"), 0); got != -fuse.ENOSYS {
		t.Fatalf("unrelated attribute behavior changed: %d", got)
	}
	f.opt.NoAppleXattr = false
	if got := f.Setxattr("/file", name, []byte("metadata"), 0); got != -fuse.ENOSYS {
		t.Fatalf("default behavior changed: %d", got)
	}
}

func TestIgnoreAppleXattrMountOptions(t *testing.T) {
	options := mountOptions(&vfs.VFS{}, "remote:", "/mount", &mountlib.Options{
		NoAppleXattr:  true,
		NoAppleDouble: true,
	})
	blocksAppleDouble := false
	for _, option := range options {
		if option == "noapplexattr" {
			t.Fatal("macFUSE must not reject Apple attribute writes before FS handles them")
		}
		if option == "noappledouble" {
			blocksAppleDouble = true
		}
	}
	if !blocksAppleDouble {
		t.Fatal("AppleDouble files must remain blocked")
	}
}
