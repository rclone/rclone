//go:build linux && !mips && !mipsle && !mips64 && !mips64le && !ppc64 && !ppc64le

package local

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/pkg/xattr"
	filelib "github.com/rclone/rclone/lib/file"
	"golang.org/x/sys/unix"
)

// XFSCommit holds the destination open from validation through publication.
type XFSCommit struct {
	fs     *Fs
	root   *os.Root
	path   string
	target *os.File
	sample unix.Stat_t
	token  *filelib.XFSCommitToken
}

// SupportsXFSExchange reports whether the root filesystem supports content exchange.
func (f *Fs) SupportsXFSExchange() (bool, error) {
	if f.opt.TranslateSymlinks || f.opt.FollowSymlinks {
		return false, nil
	}
	return filelib.SupportsXFSExchange(f.root)
}

// StartXFSCommit samples the destination before the caller checks its ETag.
func (f *Fs) StartXFSCommit(remote string) (*XFSCommit, error) {
	if f.opt.TranslateSymlinks || f.opt.FollowSymlinks {
		return nil, filelib.ErrXFSCommitRangeUnsupported
	}
	name, err := f.localPath(remote)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(f.root, name)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(f.root)
	if err != nil {
		return nil, err
	}
	g := &XFSCommit{fs: f, root: root, path: rel}
	info, err := root.Lstat(rel)
	if err == nil && !info.Mode().IsRegular() {
		err = filelib.ErrXFSCommitRangeUnsupported
	}
	if err == nil {
		g.target, err = root.OpenFile(rel, os.O_RDWR|unix.O_NOFOLLOW, 0)
	}
	if err == nil {
		// A rename replaces one name, whereas an exchange changes every hard link.
		st := &g.sample
		err = unix.Fstat(int(g.target.Fd()), st)
		if err == nil && (st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1) {
			err = filelib.ErrXFSCommitRangeUnsupported
		}
	}
	if err == nil {
		g.token, err = filelib.StartXFSCommit(g.target)
	}
	if err != nil {
		_ = g.Close()
		return nil, err
	}
	return g, nil
}

// Publish exchanges the staged contents, returning EBUSY for a stale destination
// or ErrXFSCommitRangeUnsupported when the caller should publish by rename.
func (g *XFSCommit) Publish(stagingRemote string) error {
	name, err := g.fs.localPath(stagingRemote)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(g.fs.root, name)
	if err != nil {
		return err
	}
	// The token protects an inode, not its directory entry.
	current, err := g.root.Lstat(g.path)
	if os.IsNotExist(err) {
		return syscall.EBUSY
	}
	if err != nil {
		return err
	}
	held, err := g.target.Stat()
	if err != nil {
		return err
	}
	if !current.Mode().IsRegular() || !os.SameFile(current, held) {
		return syscall.EBUSY
	}
	staging, err := g.root.OpenFile(rel, os.O_RDWR|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { _ = staging.Close() }()
	var st unix.Stat_t
	if err := unix.Fstat(int(staging.Fd()), &st); err != nil {
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 {
		return syscall.EINVAL
	}
	var target unix.Stat_t
	if err := unix.Fstat(int(g.target.Fd()), &target); err != nil {
		return err
	}
	if target.Ctim != g.sample.Ctim {
		return syscall.EBUSY
	}
	if target.Nlink != 1 || target.Mode != st.Mode || target.Uid != st.Uid || target.Gid != st.Gid {
		return filelib.ErrXFSCommitRangeUnsupported
	}
	// A rename publishes the staging inode, including its ACL and SELinux label.
	same, err := sameXattrs(g.target, staging)
	if err != nil {
		return err
	}
	if !same {
		return filelib.ErrXFSCommitRangeUnsupported
	}
	return filelib.CommitXFSRange(g.target, staging, g.token, false)
}

func sameXattrs(a, b *os.File) (bool, error) {
	left, err := xattr.FList(a)
	if err != nil {
		return false, err
	}
	right, err := xattr.FList(b)
	if err != nil {
		return false, err
	}
	slices.Sort(left)
	slices.Sort(right)
	if !slices.Equal(left, right) {
		return false, nil
	}
	for _, name := range left {
		av, err := xattr.FGet(a, name)
		if err != nil {
			return false, err
		}
		bv, err := xattr.FGet(b, name)
		if err != nil {
			return false, err
		}
		if !bytes.Equal(av, bv) {
			return false, nil
		}
	}
	return true, nil
}

// Close releases the destination and root descriptors.
func (g *XFSCommit) Close() error {
	if g == nil {
		return nil
	}
	var err error
	if g.target != nil {
		err = g.target.Close()
		g.target = nil
	}
	if g.root != nil {
		closeErr := g.root.Close()
		g.root = nil
		if err == nil {
			err = closeErr
		}
	}
	return err
}
