//go:build !plan9

package sftp

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/pkg/sftp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// recoverPanic turns a panic into an error assigned through err.
func recoverPanic(err *error) {
	if r := recover(); r != nil {
		fs.Errorf("sftp", "panic in request handler: %v\n%s", r, debug.Stack())
		*err = fmt.Errorf("request handler: %v", r)
	}
}

// recoveringHandle wraps a vfs.Handle so panics in ReadAt and WriteAt,
// which pkg/sftp calls from its packet worker goroutines, are returned
// as errors.
type recoveringHandle struct {
	vfs.Handle
}

func (h recoveringHandle) ReadAt(b []byte, off int64) (n int, err error) {
	defer recoverPanic(&err)
	return h.Handle.ReadAt(b, off)
}

func (h recoveringHandle) WriteAt(b []byte, off int64) (n int, err error) {
	defer recoverPanic(&err)
	return h.Handle.WriteAt(b, off)
}

func (h recoveringHandle) Close() (err error) {
	defer recoverPanic(&err)
	return h.Handle.Close()
}

// vfsHandler converts the VFS to be served by SFTP
type vfsHandler struct {
	*vfs.VFS
}

// newVFSHandler returns a Handlers object with the test handlers.
func newVFSHandler(vfs *vfs.VFS) sftp.Handlers {
	v := vfsHandler{VFS: vfs}
	return sftp.Handlers{
		FileGet:  v,
		FilePut:  v,
		FileCmd:  v,
		FileList: v,
	}
}

func (v vfsHandler) Fileread(r *sftp.Request) (ra io.ReaderAt, err error) {
	defer recoverPanic(&err)
	file, err := v.OpenFile(r.Filepath, os.O_RDONLY, 0777)
	if err != nil {
		return nil, err
	}
	return recoveringHandle{file}, nil
}

func (v vfsHandler) Filewrite(r *sftp.Request) (wa io.WriterAt, err error) {
	defer recoverPanic(&err)
	// Respect the flags requested in the SFTP OPEN packet
	p := r.Pflags()
	flags := os.O_WRONLY
	if p.Append {
		flags |= os.O_APPEND
	}
	if p.Creat {
		flags |= os.O_CREATE
	}
	if p.Trunc {
		flags |= os.O_TRUNC
	}
	if p.Excl {
		flags |= os.O_EXCL
	}
	file, err := v.OpenFile(r.Filepath, flags, 0777)
	if err != nil {
		return nil, err
	}
	return recoveringHandle{file}, nil
}

func (v vfsHandler) Filecmd(r *sftp.Request) (err error) {
	defer recoverPanic(&err)
	switch r.Method {
	case "Setstat":
		attr := r.Attributes()
		flags := r.AttrFlags()
		// A size attribute is a request to truncate the file
		if flags.Size {
			node, err := v.Stat(r.Filepath)
			if err != nil {
				return err
			}
			if err := node.Truncate(int64(attr.Size)); err != nil {
				return err
			}
		}
		if flags.Acmodtime {
			atime := time.Unix(int64(attr.Atime), 0)
			mtime := time.Unix(int64(attr.Mtime), 0)
			err := v.Chtimes(r.Filepath, atime, mtime)
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

// StatVFS implements the statvfs@openssh.com extension, returning filesystem
// usage information from the VFS. It satisfies sftp.StatVFSFileCmder.
func (v vfsHandler) StatVFS(r *sftp.Request) (st *sftp.StatVFS, err error) {
	defer recoverPanic(&err)
	const blockSize = 4096
	total, _, free := v.Statfs()
	blocks := uint64(total) / blockSize
	bfree := uint64(free) / blockSize
	return &sftp.StatVFS{
		Bsize:   blockSize,
		Frsize:  blockSize,
		Blocks:  blocks,
		Bfree:   bfree,
		Bavail:  bfree,
		Files:   1e9, // total file inodes - made up as the VFS has no concept of these
		Ffree:   1e9, // free file inodes
		Favail:  1e9, // free file inodes for non-root
		Namemax: 255, // maximum filename length
	}, nil
}

type listerat []os.FileInfo

// Modeled after strings.Reader's ReadAt() implementation
func (f listerat) ListAt(ls []os.FileInfo, offset int64) (n int, err error) {
	defer recoverPanic(&err)
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
	defer recoverPanic(&err)
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
