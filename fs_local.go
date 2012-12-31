// Local filesystem interface
package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"
)

// FsLocal represents a local filesystem rooted at root
type FsLocal struct {
	root string
}

// FsObjectLocal represents a local filesystem object
type FsObjectLocal struct {
	remote string      // The remote path
	path   string      // The local path
	info   os.FileInfo // Interface for file info
}

// ------------------------------------------------------------

// NewFsLocal contstructs an FsLocal from the path
func NewFsLocal(root string) (*FsLocal, error) {
	root = path.Clean(root)
	f := &FsLocal{root: root}
	return f, nil
}

// String converts this FsLocal to a string
func (f *FsLocal) String() string {
	return fmt.Sprintf("Local file system at %s", f.root)
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsLocal) NewFsObjectWithInfo(remote string, info os.FileInfo) FsObject {
	path := filepath.Join(f.root, remote)
	fs := &FsObjectLocal{remote: remote, path: path}
	if info != nil {
		fs.info = info
	} else {
		err := fs.lstat()
		if err != nil {
			log.Printf("Failed to stat %s: %s", path, err)
			return nil
		}
	}
	return fs
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsLocal) NewFsObject(remote string) FsObject {
	return f.NewFsObjectWithInfo(remote, nil)
}

// List the path returning a channel of FsObjects
//
// Ignores everything which isn't Storable, eg links etc
func (f *FsLocal) List() FsObjectsChan {
	out := make(FsObjectsChan, *checkers)
	go func() {
		err := filepath.Walk(f.root, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Failed to open directory: %s: %s", path, err)
			} else {
				remote, err := filepath.Rel(f.root, path)
				if err != nil {
					log.Printf("Failed to get relative path %s: %s", path, err)
					return nil
				}
				if remote == "." {
					return nil
					// remote = ""
				}
				if fs := f.NewFsObjectWithInfo(remote, fi); fs != nil {
					if fs.Storable() {
						out <- fs
					}
				}
			}
			return nil
		})
		if err != nil {
			log.Printf("Failed to open directory: %s: %s", f.root, err)
		}
		close(out)
	}()
	return out
}

// FIXME most of this is generic
// could make it into Copy(dst, src FsObject)

// Puts the FsObject to the local filesystem
//
// FIXME return the object?
func (f *FsLocal) Put(src FsObject) {
	dstRemote := src.Remote()
	dstPath := filepath.Join(f.root, dstRemote)
	log.Printf("Download %s to %s", dstRemote, dstPath)
	// Temporary FsObject under construction
	fs := &FsObjectLocal{remote: dstRemote, path: dstPath}

	dir := path.Dir(dstPath)
	err := os.MkdirAll(dir, 0770)
	if err != nil {
		FsLog(fs, "Couldn't make directory: %s", err)
		return
	}

	out, err := os.Create(dstPath)
	if err != nil {
		FsLog(fs, "Failed to open: %s", err)
		return
	}

	// Close and remove file on error at the end
	defer func() {
		checkClose(out, &err)
		if err != nil {
			FsDebug(fs, "Removing failed download")
			removeErr := os.Remove(dstPath)
			if removeErr != nil {
				FsLog(fs, "Failed to remove failed download: %s", err)
			}
		}
	}()

	in, err := src.Open()
	if err != nil {
		FsLog(fs, "Failed to open: %s", err)
		return
	}
	defer checkClose(in, &err)

	_, err = io.Copy(out, in)
	if err != nil {
		FsLog(fs, "Failed to download: %s", err)
		return
	}

	// Set the mtime
	modTime, err := src.ModTime()
	if err != nil {
		FsDebug(fs, "Failed to read mtime from object: %s", err)
	} else {
		fs.SetModTime(modTime)
	}
}

// Mkdir creates the directory if it doesn't exist
func (f *FsLocal) Mkdir() error {
	return os.MkdirAll(f.root, 0770)
}

// Rmdir removes the directory
//
// If it isn't empty it will return an error
func (f *FsLocal) Rmdir() error {
	return os.Remove(f.root)
}

// ------------------------------------------------------------

// Return the remote path
func (fs *FsObjectLocal) Remote() string {
	return fs.remote
}

// Md5sum calculates the Md5sum of a file returning a lowercase hex string
func (fs *FsObjectLocal) Md5sum() (string, error) {
	in, err := os.Open(fs.path)
	if err != nil {
		FsLog(fs, "Failed to open: %s", err)
		return "", err
	}
	defer in.Close() // FIXME ignoring error
	hash := md5.New()
	_, err = io.Copy(hash, in)
	if err != nil {
		FsLog(fs, "Failed to read: %s", err)
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// Size returns the size of an object in bytes
func (fs *FsObjectLocal) Size() int64 {
	return fs.info.Size()
}

// ModTime returns the modification time of the object
func (fs *FsObjectLocal) ModTime() (modTime time.Time, err error) {
	return fs.info.ModTime(), nil
}

// Sets the modification time of the local fs object
func (fs *FsObjectLocal) SetModTime(modTime time.Time) {
	err := Chtimes(fs.path, modTime, modTime)
	if err != nil {
		FsDebug(fs, "Failed to set mtime on file: %s", err)
	}
}

// Is this object storable
func (fs *FsObjectLocal) Storable() bool {
	mode := fs.info.Mode()
	if mode&(os.ModeSymlink|os.ModeNamedPipe|os.ModeSocket|os.ModeDevice) != 0 {
		FsDebug(fs, "Can't transfer non file/directory")
		return false
	} else if mode&os.ModeDir != 0 {
		FsDebug(fs, "FIXME Skipping directory")
		return false
	}
	return true
}

// Open an object for read
func (fs *FsObjectLocal) Open() (in io.ReadCloser, err error) {
	in, err = os.Open(fs.path)
	return
}

// Stat a FsObject into info
func (fs *FsObjectLocal) lstat() error {
	info, err := os.Lstat(fs.path)
	fs.info = info
	return err
}

// Remove an object
func (fs *FsObjectLocal) Remove() error {
	return os.Remove(fs.path)
}

// Check the interfaces are satisfied
var _ Fs = &FsLocal{}
var _ FsObject = &FsObjectLocal{}
