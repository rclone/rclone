//go:build !plan9

package sftp

import (
	"io"
	"os"
	"syscall"
	"time"

	"github.com/pkg/sftp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// vfsHandler converts the VFS to be served by SFTP
type vfsHandler struct {
	*vfs.VFS
}

// vfsHandler returns a Handlers object with the test handlers.
func newVFSHandler(vfs *vfs.VFS) sftp.Handlers {
	v := vfsHandler{VFS: vfs}
	return sftp.Handlers{
		FileGet:  v,
		FilePut:  v,
		FileCmd:  v,
		FileList: v,
	}
}

func (v vfsHandler) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	file, err := v.OpenFile(r.Filepath, os.O_RDONLY, 0777)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (v vfsHandler) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	file, err := v.OpenFile(r.Filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (v vfsHandler) Filecmd(r *sftp.Request) error {
	switch r.Method {
	case "Setstat":
		attr := r.Attributes()
		if attr.Mtime != 0 {
			modTime := time.Unix(int64(attr.Mtime), 0)
			err := v.Chtimes(r.Filepath, modTime, modTime)
			if err != nil {
				return err
			}
		}
		return nil
	case "Rename":
		err := v.Rename(r.Filepath, r.Target)
		if err != nil {
			return err
		}
	case "Rmdir", "Remove":
		err := v.Remove(r.Filepath)
		if err != nil {
			return err
		}
	case "Mkdir":
		err := v.Mkdir(r.Filepath, 0777)
		if err != nil {
			return err
		}
	case "Symlink":
		// FIXME
		// _, err := v.fetch(r.Filepath)
		// if err != nil {
		// 	return err
		// }
		// link := newMemFile(r.Target, false)
		// link.symlink = r.Filepath
		// v.files[r.Target] = link
		return sftp.ErrSshFxOpUnsupported
	case "Link":
		return sftp.ErrSshFxOpUnsupported
	default:
		return sftp.ErrSshFxOpUnsupported
	}
	return nil
}

type listerat []os.FileInfo

// Modeled after strings.Reader's ReadAt() implementation
func (f listerat) ListAt(ls []os.FileInfo, offset int64) (int, error) {
	var n int
	if offset >= int64(len(f)) {
		return 0, io.EOF
	}
	n = copy(ls, f[offset:])
	if n < len(ls) {
		return n, io.EOF
	}
	return n, nil
}

func (v vfsHandler) Filelist(r *sftp.Request) (l sftp.ListerAt, err error) {
	var node vfs.Node
	var handle vfs.Handle
	switch r.Method {
	case "List":
		node, err = v.Stat(r.Filepath)
		if err != nil {
			return nil, err
		}
		if !node.IsDir() {
			return nil, syscall.ENOTDIR
		}
		handle, err = node.Open(os.O_RDONLY)
		if err != nil {
			return nil, err
		}
		defer fs.CheckClose(handle, &err)
		fis, err := handle.Readdir(-1)
		if err != nil {
			return nil, err
		}
		return listerat(fis), nil
	case "Stat":
		node, err = v.Stat(r.Filepath)
		if err != nil {
			return nil, err
		}
		return listerat([]os.FileInfo{node}), nil
	case "Readlink":
		// FIXME
		// if file.symlink != "" {
		// 	file, err = v.fetch(file.symlink)
		// 	if err != nil {
		// 		return nil, err
		// 	}
		// }
		// return listerat([]os.FileInfo{file}), nil
	}
	return nil, sftp.ErrSshFxOpUnsupported
}
