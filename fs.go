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
	// String returns a description of the FS
	String() string

	// List the Fs into a channel
	List() FsObjectsChan

	// Find the FsObject at remote.  Returns nil if can't be found
	NewFsObject(remote string) FsObject

	// Put in to the remote path with the modTime given of the given size
	//
	// May create the object even if it returns an error - if so
	// will return the object and the error, otherwise will return
	// nil and the error
	Put(in io.Reader, remote string, modTime time.Time, size int64) (FsObject, error)

	// Make the directory (container, bucket)
	Mkdir() error

	// Remove the directory (container, bucket) if empty
	Rmdir() error
}

// FIXME make f.Debugf...

// A filesystem like object which can either be a remote object or a
// local file/directory
type FsObject interface {
	// Remote returns the remote path
	Remote() string

	// Md5sum returns the md5 checksum of the file
	Md5sum() (string, error)

	// ModTime returns the modification date of the file
	ModTime() time.Time

	// SetModTime sets the metadata on the object to set the modification date
	SetModTime(time.Time)

	// Size returns the size of the file
	Size() int64

	// Open opens the file for read.  Call Close() on the returned io.ReadCloser
	Open() (io.ReadCloser, error)

	// Storable says whether this object can be stored
	Storable() bool

	// Removes this object
	Remove() error
}

// Optional interfaces
type Purger interface {
	// Purge all files in the root and the root directory
	//
	// Implement this if you have a way of deleting all the files
	// quicker than just running Remove() on the result of List()
	Purge() error
}

// A channel of FsObjects
type FsObjectsChan chan FsObject

// A slice of FsObjects
type FsObjects []FsObject

// NewFs makes a new Fs object from the path
//
// FIXME make more generic
func NewFs(path string) (Fs, error) {
	if swiftMatch.MatchString(path) {
		return NewFsSwift(path)
	}
	if s3Match.MatchString(path) {
		return NewFsS3(path)
	}
	if driveMatch.MatchString(path) {
		return NewFsDrive(path)
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
		stats.Error()
		FsLog(src, "Failed to calculate src md5: %s", err)
		return false, err
	}
	dstMd5, err := dst.Md5sum()
	if err != nil {
		stats.Error()
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
	srcModTime := src.ModTime()
	dstModTime := dst.ModTime()
	if !dstModTime.Equal(srcModTime) {
		FsDebug(src, "Modification times differ: %v, %v", srcModTime, dstModTime)
	} else {
		FsDebug(src, "Size and modification time the same")
		return true
	}

	// mtime is unreadable or different but size is the same so
	// check the MD5SUM
	same, _ := CheckMd5sums(src, dst)
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

// Copy src object to f
func Copy(f Fs, src FsObject) {
	in0, err := src.Open()
	if err != nil {
		stats.Error()
		FsLog(src, "Failed to open: %s", err)
		return
	}
	in := NewAccount(in0) // account the transfer

	dst, err := f.Put(in, src.Remote(), src.ModTime(), src.Size())
	inErr := in.Close()
	if err == nil {
		err = inErr
	}
	if err != nil {
		stats.Error()
		FsLog(src, "Failed to copy: %s", err)
		if dst != nil {
			FsDebug(dst, "Removing failed copy")
			removeErr := dst.Remove()
			if removeErr != nil {
				stats.Error()
				FsLog(dst, "Failed to remove failed copy: %s", removeErr)
			}
		}
		return
	}
	FsDebug(src, "Copied")
}
