// File system interface

package main

import (
	"fmt"
	"io"
	"log"
	"time"
)

// A Filesystem, describes the local filesystem and the remote object store
type Fs interface {
	String() string
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

// Write debuging output for this FsObject
func FsDebug(fs FsObject, text string, args ...interface{}) {
	if *verbose {
		out := fmt.Sprintf(text, args...)
		log.Printf("%s: %s", fs.Remote(), out)
	}
}

// Write log output for this FsObject
func FsLog(fs FsObject, text string, args ...interface{}) {
	if !*quiet {
		out := fmt.Sprintf(text, args...)
		log.Printf("%s: %s", fs.Remote(), out)
	}
}

// checkClose is a utility function used to check the return from
// Close in a defer statement.
func checkClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil {
		*err = cerr
	}
}

// Check the two files to see if the MD5sums are the same
//
// May return an error which will already have been logged
//
// If an error is returned it will return false
func CheckMd5sums(src, dst FsObject) (bool, error) {
	srcMd5, err := src.Md5sum()
	if err != nil {
		FsLog(src, "Failed to calculate src md5: %s", err)
		return false, err
	}
	dstMd5, err := dst.Md5sum()
	if err != nil {
		FsLog(dst, "Failed to calculate dst md5: %s", err)
		return false, err
	}
	// FsDebug("Src MD5 %s", srcMd5)
	// FsDebug("Dst MD5 %s", obj.Hash)
	return srcMd5 == dstMd5, nil
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
		FsDebug(src, "Sizes differ")
		return false
	}

	// Size the same so check the mtime
	srcModTime, err := src.ModTime()
	if err != nil {
		FsDebug(src, "Failed to read src mtime: %s", err)
	} else {
		dstModTime, err := dst.ModTime()
		if err != nil {
			FsDebug(dst, "Failed to read dst mtime: %s", err)
		} else if !dstModTime.Equal(srcModTime) {
			FsDebug(src, "Modification times differ")
		} else {
			FsDebug(src, "Size and modification time the same")
			return true
		}
	}

	// mtime is unreadable or different but size is the same so
	// check the MD5SUM
	same, err := CheckMd5sums(src, dst)
	if !same {
		FsDebug(src, "Md5sums differ")
		return false
	}

	// Size and MD5 the same but mtime different so update the
	// mtime of the dst object here
	dst.SetModTime(srcModTime)

	FsDebug(src, "Size and MD5SUM of src and dst objects identical")
	return true
}
