//go:build !windows && !plan9

package smb

import (
	"sync"
	"syscall"

	"github.com/macos-fuse-t/go-smb2/vfs"
	"github.com/rclone/rclone/cmd/serve/proxy"
	rclonefs "github.com/rclone/rclone/fs"
)

// proxySMBVFS wraps smbVFS with lazy initialization via the auth proxy.
//
// NTLM authentication doesn't provide plaintext passwords, so we
// can't do per-user auth proxy dispatch the way HTTP/SFTP can. Instead
// we allow guest access at the NTLM level and call the proxy once
// (with the configured user/pass) to obtain a VFS. All SMB sessions
// share that VFS.
type proxySMBVFS struct {
	proxy *proxy.Proxy
	user  string
	pass  string

	once sync.Once
	vfs  *smbVFS
	err  error
}

func newProxySMBVFS(p *proxy.Proxy, user, pass string) *proxySMBVFS {
	return &proxySMBVFS{
		proxy: p,
		user:  user,
		pass:  pass,
	}
}

// init lazily creates the underlying smbVFS by calling the proxy
func (p *proxySMBVFS) init() (*smbVFS, error) {
	p.once.Do(func() {
		user := p.user
		if user == "" {
			user = "anonymous"
		}
		pass := p.pass
		if pass == "" {
			pass = "anonymous"
		}
		VFS, _, err := p.proxy.Call(user, pass, false)
		if err != nil {
			rclonefs.Errorf(nil, "serve smb: auth proxy call failed: %v", err)
			p.err = err
			return
		}
		p.vfs = newSMBVFS(VFS)
	})
	return p.vfs, p.err
}

// All VFSFileSystem methods delegate to the lazily-initialized smbVFS.

func (p *proxySMBVFS) GetAttr(h vfs.VfsHandle) (*vfs.Attributes, error) {
	s, err := p.init()
	if err != nil {
		return nil, syscall.EIO
	}
	return s.GetAttr(h)
}

func (p *proxySMBVFS) SetAttr(h vfs.VfsHandle, a *vfs.Attributes) (*vfs.Attributes, error) {
	s, err := p.init()
	if err != nil {
		return nil, syscall.EIO
	}
	return s.SetAttr(h, a)
}

func (p *proxySMBVFS) StatFS(h vfs.VfsHandle) (*vfs.FSAttributes, error) {
	s, err := p.init()
	if err != nil {
		return nil, syscall.EIO
	}
	return s.StatFS(h)
}

func (p *proxySMBVFS) FSync(h vfs.VfsHandle) error {
	s, err := p.init()
	if err != nil {
		return syscall.EIO
	}
	return s.FSync(h)
}

func (p *proxySMBVFS) Flush(h vfs.VfsHandle) error {
	s, err := p.init()
	if err != nil {
		return syscall.EIO
	}
	return s.Flush(h)
}

func (p *proxySMBVFS) Open(name string, flags int, mode int) (vfs.VfsHandle, error) {
	s, err := p.init()
	if err != nil {
		return 0, syscall.EIO
	}
	return s.Open(name, flags, mode)
}

func (p *proxySMBVFS) Close(h vfs.VfsHandle) error {
	s, err := p.init()
	if err != nil {
		return syscall.EIO
	}
	return s.Close(h)
}

func (p *proxySMBVFS) Lookup(h vfs.VfsHandle, name string) (*vfs.Attributes, error) {
	s, err := p.init()
	if err != nil {
		return nil, syscall.EIO
	}
	return s.Lookup(h, name)
}

func (p *proxySMBVFS) Mkdir(name string, mode int) (*vfs.Attributes, error) {
	s, err := p.init()
	if err != nil {
		return nil, syscall.EIO
	}
	return s.Mkdir(name, mode)
}

func (p *proxySMBVFS) Read(h vfs.VfsHandle, buf []byte, offset uint64, flags int) (int, error) {
	s, err := p.init()
	if err != nil {
		return 0, syscall.EIO
	}
	return s.Read(h, buf, offset, flags)
}

func (p *proxySMBVFS) Write(h vfs.VfsHandle, data []byte, offset uint64, flags int) (int, error) {
	s, err := p.init()
	if err != nil {
		return 0, syscall.EIO
	}
	return s.Write(h, data, offset, flags)
}

func (p *proxySMBVFS) OpenDir(name string) (vfs.VfsHandle, error) {
	s, err := p.init()
	if err != nil {
		return 0, syscall.EIO
	}
	return s.OpenDir(name)
}

func (p *proxySMBVFS) ReadDir(h vfs.VfsHandle, pos int, count int) ([]vfs.DirInfo, error) {
	s, err := p.init()
	if err != nil {
		return nil, syscall.EIO
	}
	return s.ReadDir(h, pos, count)
}

func (p *proxySMBVFS) Readlink(h vfs.VfsHandle) (string, error) {
	s, err := p.init()
	if err != nil {
		return "", syscall.EIO
	}
	return s.Readlink(h)
}

func (p *proxySMBVFS) Unlink(h vfs.VfsHandle) error {
	s, err := p.init()
	if err != nil {
		return syscall.EIO
	}
	return s.Unlink(h)
}

func (p *proxySMBVFS) Truncate(h vfs.VfsHandle, size uint64) error {
	s, err := p.init()
	if err != nil {
		return syscall.EIO
	}
	return s.Truncate(h, size)
}

func (p *proxySMBVFS) Rename(h vfs.VfsHandle, newPath string, flags int) error {
	s, err := p.init()
	if err != nil {
		return syscall.EIO
	}
	return s.Rename(h, newPath, flags)
}

func (p *proxySMBVFS) Symlink(h vfs.VfsHandle, target string, flags int) (*vfs.Attributes, error) {
	return nil, syscall.ENOSYS
}

func (p *proxySMBVFS) Link(from vfs.VfsNode, to vfs.VfsNode, name string) (*vfs.Attributes, error) {
	return nil, syscall.ENOSYS
}

func (p *proxySMBVFS) Listxattr(h vfs.VfsHandle) ([]string, error) {
	return nil, nil
}

func (p *proxySMBVFS) Getxattr(h vfs.VfsHandle, name string, buf []byte) (int, error) {
	return 0, syscall.ENOENT
}

func (p *proxySMBVFS) Setxattr(h vfs.VfsHandle, name string, data []byte) error {
	return syscall.ENOSYS
}

func (p *proxySMBVFS) Removexattr(h vfs.VfsHandle, name string) error {
	return syscall.ENOSYS
}

// Check that proxySMBVFS implements VFSFileSystem
var _ vfs.VFSFileSystem = (*proxySMBVFS)(nil)
