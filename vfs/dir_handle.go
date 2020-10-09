package vfs

import (
	"io"
	"os"
)

// DirHandle represents an open directory
type DirHandle struct {
	baseHandle
	d   *Dir
	fis []os.FileInfo // where Readdir got to
}

// newDirHandle opens a directory for read
func newDirHandle(d *Dir) *DirHandle {
	return &DirHandle{
		d: d,
	}
}

// String converts it to printable
func (fh *DirHandle) String() string {
	if fh == nil {
		return "<nil *DirHandle>"
	}
	if fh.d == nil {
		return "<nil *DirHandle.d>"
	}
	return fh.d.String() + " (r)"
}

// Stat returns info about the current directory
func (fh *DirHandle) Stat() (fi os.FileInfo, err error) {
	return fh.d, nil
}

// Node returns the Node associated with this - satisfies Noder interface
func (fh *DirHandle) Node() Node {
	return fh.d
}

// Readdir reads the contents of the directory associated with file and returns
// a slice of up to n FileInfo values, as would be returned by Lstat, in
// directory order. Subsequent calls on the same file will yield further
// FileInfos.
//
// If n > 0, Readdir returns at most n FileInfo structures. In this case, if
// Readdir returns an empty slice, it will return a non-nil error explaining
// why. At the end of a directory, the error is io.EOF.
//
// If n <= 0, Readdir returns all the FileInfo from the directory in a single
// slice. In this case, if Readdir succeeds (reads all the way to the end of
// the directory), it returns the slice and a nil error. If it encounters an
// error before the end of the directory, Readdir returns the FileInfo read
// until that point and a non-nil error.
func (fh *DirHandle) Readdir(n int) (fis []os.FileInfo, err error) {
	if fh.fis == nil {
		nodes, err := fh.d.ReadDirAll()
		if err != nil {
			return nil, err
		}
		fh.fis = []os.FileInfo{}
		for _, node := range nodes {
			fh.fis = append(fh.fis, node)
		}
	}
	nn := len(fh.fis)
	if n > 0 {
		if nn == 0 {
			return nil, io.EOF
		}
		if nn > n {
			nn = n
		}
	}
	fis, fh.fis = fh.fis[:nn], fh.fis[nn:]
	return fis, nil
}

// Readdirnames reads and returns a slice of names from the directory f.
//
// If n > 0, Readdirnames returns at most n names. In this case, if
// Readdirnames returns an empty slice, it will return a non-nil error
// explaining why. At the end of a directory, the error is io.EOF.
//
// If n <= 0, Readdirnames returns all the names from the directory in a single
// slice. In this case, if Readdirnames succeeds (reads all the way to the end
// of the directory), it returns the slice and a nil error. If it encounters an
// error before the end of the directory, Readdirnames returns the names read
// until that point and a non-nil error.
func (fh *DirHandle) Readdirnames(n int) (names []string, err error) {
	nodes, err := fh.Readdir(n)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		names = append(names, node.Name())
	}
	return names, nil
}

// Close closes the handle
func (fh *DirHandle) Close() (err error) {
	fh.fis = nil
	return nil
}
