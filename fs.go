// File system interface

package main

import (
	"io"
	"time"
)

// A Filesystem, describes the local filesystem and the remote object store
type Fs interface {
	List() FsObjectsChan
	NewFsObject(remote string) FsObject
	Put(src FsObject)
	Mkdir() error
	Rmdir() error
}

// FIXME make f.Debugf...

// A filesystem like object which can either be a remote object or a
// local file/directory
type FsObject interface {
	Remote() string
	Debugf(string, ...interface{})
	Md5sum() (string, error)
	ModTime() (time.Time, error)
	SetModTime(time.Time)
	Size() int64
	Open() (io.ReadCloser, error)
	Storable() bool
	//	Exists() bool
	Remove() error
}

type FsObjectsChan chan FsObject

type FsObjects []FsObject

// NewFs makes a new Fs object from the path
//
// FIXME make more generic in future
func NewFs(path string) (Fs, error) {
	if swiftMatch.MatchString(path) {
		return NewFsSwift(path)
	}
	return NewFsLocal(path)
}

// checkClose is a utility function used to check the return from
// Close in a defer statement.
func checkClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil {
		*err = cerr
	}
}

// Checks to see if the src and dst objects are equal by looking at
// size, mtime and MD5SUM
//
// If the src and dst size are different then it is considered to be
// not equal.
//
// If the size is the same and the mtime is the same then it is
// considered to be equal.  This is the heuristic rsync uses when
// not using --checksum.
//
// If the size is the same and and mtime is different or unreadable
// and the MD5SUM is the same then the file is considered to be equal.
// In this case the mtime on the dst is updated.
//
// Otherwise the file is considered to be not equal including if there
// were errors reading info.
func Equal(src, dst FsObject) bool {
	if src.Size() != dst.Size() {
		src.Debugf("Sizes differ")
		return false
	}

	// Size the same so check the mtime
	srcModTime, err := src.ModTime()
	if err != nil {
		src.Debugf("Failed to read src mtime: %s", err)
	} else {
		dstModTime, err := dst.ModTime()
		if err != nil {
			dst.Debugf("Failed to read dst mtime: %s", err)
		} else if !dstModTime.Equal(srcModTime) {
			src.Debugf("Modification times differ")
		} else {
			src.Debugf("Size and modification time the same")
			return true
		}
	}

	// mtime is unreadable or different but size is the same so
	// check the MD5SUM
	srcMd5, err := src.Md5sum()
	if err != nil {
		src.Debugf("Failed to calculate src md5: %s", err)
		return false
	}
	dstMd5, err := dst.Md5sum()
	if err != nil {
		dst.Debugf("Failed to calculate dst md5: %s", err)
		return false
	}
	// fs.Debugf("Src MD5 %s", srcMd5)
	// fs.Debugf("Dst MD5 %s", obj.Hash)
	if srcMd5 != dstMd5 {
		src.Debugf("Md5sums differ")
		return false
	}

	// Size and MD5 the same but mtime different so update the
	// mtime of the dst object here
	dst.SetModTime(srcModTime)

	src.Debugf("Size and MD5SUM of src and dst objects identical")
	return true
}
