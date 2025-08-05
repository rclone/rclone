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

// sanitizePath ensures filenames are properly encoded for the SFTP protocol
// This helps with special character handling, especially for tests
func (v vfsHandler) sanitizePath(path string) string {
	// SFTP protocol requires exact byte-for-byte path preservation, especially
	// for paths with special characters, control characters, etc.

	// For SFTP tests to pass, it's critical that we don't attempt to "sanitize"
	// or modify paths in any way that could alter their byte representation.

	// This is crucial for encoding tests with control characters, spaces, etc.
	fs.Debugf(nil, "SFTP path (raw bytes): %q -> %x", path, []byte(path))
	return path
}

func (v vfsHandler) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	path := v.sanitizePath(r.Filepath)
	fs.Debugf(nil, "SFTP: Fileread %q", path)
	file, err := v.OpenFile(path, os.O_RDONLY, 0777)
	if err != nil {
		fs.Debugf(nil, "SFTP: Fileread %q error: %v", path, err)
		return nil, err
	}
	return file, nil
}

func (v vfsHandler) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	path := v.sanitizePath(r.Filepath)
	fs.Debugf(nil, "SFTP: Filewrite %q", path)
	file, err := v.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		fs.Debugf(nil, "SFTP: Filewrite %q error: %v", path, err)
		return nil, err
	}
	return file, nil
}

func (v vfsHandler) Filecmd(r *sftp.Request) error {
	path := v.sanitizePath(r.Filepath)
	fs.Debugf(nil, "SFTP: Filecmd %q method %q", path, r.Method)
	switch r.Method {
	case "Setstat":
		attr := r.Attributes()
		if attr.Mtime != 0 {
			modTime := time.Unix(int64(attr.Mtime), 0)
			err := v.Chtimes(path, modTime, modTime)
			if err != nil {
				fs.Debugf(nil, "SFTP: Filecmd Setstat %q error: %v", path, err)
				return err
			}
		}
		return nil
	case "Rename":
		target := v.sanitizePath(r.Target)
		fs.Debugf(nil, "SFTP: Rename %q to %q", path, target)
		err := v.Rename(path, target)
		if err != nil {
			fs.Debugf(nil, "SFTP: Rename %q to %q error: %v", path, target, err)
			return err
		}
	case "Rmdir", "Remove":
		fs.Debugf(nil, "SFTP: Remove %q", path)
		err := v.Remove(path)
		if err != nil {
			fs.Debugf(nil, "SFTP: Remove %q error: %v", path, err)
			return err
		}
	case "Mkdir":
		fs.Debugf(nil, "SFTP: Mkdir %q", path)
		err := v.Mkdir(path, 0777)
		if err != nil {
			fs.Debugf(nil, "SFTP: Mkdir %q error: %v", path, err)
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
	path := v.sanitizePath(r.Filepath)
	fs.Debugf(nil, "SFTP: Filelist %q method %q", path, r.Method)
	var node vfs.Node
	var handle vfs.Handle
	switch r.Method {
	case "List":
		node, err = v.Stat(path)
		if err != nil {
			fs.Debugf(nil, "SFTP: Filelist Stat %q error: %v", path, err)
			return nil, err
		}
		if !node.IsDir() {
			fs.Debugf(nil, "SFTP: Filelist %q not a directory", path)
			return nil, syscall.ENOTDIR
		}
		handle, err = node.Open(os.O_RDONLY)
		if err != nil {
			fs.Debugf(nil, "SFTP: Filelist Open %q error: %v", path, err)
			return nil, err
		}
		defer fs.CheckClose(handle, &err)
		fis, err := handle.Readdir(-1)
		if err != nil {
			fs.Debugf(nil, "SFTP: Filelist Readdir %q error: %v", path, err)
			return nil, err
		}
		// Log what files we're returning to help debug encoding issues
		for i, fi := range fis {
			fs.Debugf(nil, "SFTP: Filelist %q item %d: %q (raw bytes: %x)",
				path, i, fi.Name(), []byte(fi.Name()))
		}
		return listerat(fis), nil
	case "Stat":
		node, err = v.Stat(path)
		if err != nil {
			fs.Debugf(nil, "SFTP: Filelist Stat %q error: %v", path, err)
			return nil, err
		}
		if node.IsDir() {
			fs.Debugf(nil, "SFTP: Filelist Stat %q is directory", path)
		} else {
			fs.Debugf(nil, "SFTP: Filelist Stat %q is file", path)
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
