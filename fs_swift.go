// Swift interface
package main

import (
	"fmt"
	"github.com/ncw/swift"
	"io"
	"log"
	"strings"
	"time"
)

// FsSwift represents a remote swift server
type FsSwift struct {
	c         swift.Connection // the connection to the swift server
	container string           // the container we are working on
}

// FsObjectSwift describes a swift object
//
// Will definitely have info but maybe not meta
type FsObjectSwift struct {
	swift  *FsSwift        // what this object is part of
	remote string          // The remote path
	info   swift.Object    // Info from the swift object if known
	meta   *swift.Metadata // The object metadata if known
}

// ------------------------------------------------------------

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsSwift) NewFsObjectWithInfo(remote string, info *swift.Object) FsObject {
	fs := &FsObjectSwift{
		swift:  f,
		remote: remote,
	}
	if info != nil {
		// Set info but not meta
		fs.info = *info
	} else {
		err := fs.readMetaData() // reads info and meta, returning an error
		if err != nil {
			// logged already fs.Debugf("Failed to read info: %s", err)
			return nil
		}
	}
	return fs
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsSwift) NewFsObject(remote string) FsObject {
	return f.NewFsObjectWithInfo(remote, nil)
}

// Walk the path returning a channel of FsObjects
func (f *FsSwift) List() FsObjectsChan {
	out := make(FsObjectsChan, *checkers)
	go func() {
		// FIXME use a smaller limit?
		err := f.c.ObjectsWalk(f.container, nil, func(opts *swift.ObjectsOpts) (interface{}, error) {
			objects, err := f.c.Objects(f.container, opts)
			if err == nil {
				for i := range objects {
					object := &objects[i]
					if fs := f.NewFsObjectWithInfo(object.Name, object); fs != nil {
						out <- fs
					}
				}
			}
			return objects, err
		})
		if err != nil {
			log.Printf("Couldn't read container %q: %s", f.container, err)
		}
		close(out)
	}()
	return out
}

// Put the FsObject into the container
func (f *FsSwift) Put(src FsObject) {
	// Temporary FsObject under construction
	fs := &FsObjectSwift{swift: f, remote: src.Remote()}
	// FIXME content type
	in, err := src.Open()
	if err != nil {
		fs.Debugf("Failed to open: %s", err)
		return
	}
	defer in.Close()

	// Set the mtime
	m := swift.Metadata{}
	modTime, err := src.ModTime()
	if err != nil {
		fs.Debugf("Failed to read mtime from object: %s", err)
	} else {
		m.SetModTime(modTime)
	}

	_, err = fs.swift.c.ObjectPut(fs.swift.container, fs.remote, in, true, "", "", m.ObjectHeaders())
	if err != nil {
		fs.Debugf("Failed to upload: %s", err)
		return
	}
	fs.Debugf("Uploaded")
}

// Mkdir creates the container if it doesn't exist
func (f *FsSwift) Mkdir() error {
	return f.c.ContainerCreate(f.container, nil)
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *FsSwift) Rmdir() error {
	return f.c.ContainerDelete(f.container)
}

// ------------------------------------------------------------

// Return the remote path
func (fs *FsObjectSwift) Remote() string {
	return fs.remote
}

// Write debuging output for this FsObject
func (fs *FsObjectSwift) Debugf(text string, args ...interface{}) {
	out := fmt.Sprintf(text, args...)
	log.Printf("%s: %s", fs.remote, out)
}

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (fs *FsObjectSwift) Md5sum() (string, error) {
	return strings.ToLower(fs.info.Hash), nil
}

// Size returns the size of an object in bytes
func (fs *FsObjectSwift) Size() int64 {
	return fs.info.Bytes
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (fs *FsObjectSwift) readMetaData() (err error) {
	if fs.meta != nil {
		return nil
	}
	info, h, err := fs.swift.c.Object(fs.swift.container, fs.remote)
	if err != nil {
		fs.Debugf("Failed to read info: %s", err)
		return err
	}
	meta := h.ObjectMetadata()
	fs.info = info
	fs.meta = &meta
	return nil
}

// ModTime returns the modification time of the object
func (fs *FsObjectSwift) ModTime() (modTime time.Time, err error) {
	err = fs.readMetaData()
	if err != nil {
		fs.Debugf("Failed to read metadata: %s", err)
		return
	}
	modTime, err = fs.meta.GetModTime()
	if err != nil {
		fs.Debugf("Failed to read mtime from object: %s", err)
		return
	}
	return
}

// Sets the modification time of the local fs object
func (fs *FsObjectSwift) SetModTime(modTime time.Time) {
	err := fs.readMetaData()
	if err != nil {
		fs.Debugf("Failed to read metadata: %s", err)
		return
	}
	fs.meta.SetModTime(modTime)
	err = fs.swift.c.ObjectUpdate(fs.swift.container, fs.remote, fs.meta.ObjectHeaders())
	if err != nil {
		fs.Debugf("Failed to update remote mtime: %s", err)
	}
}

// Is this object storable
func (fs *FsObjectSwift) Storable() bool {
	return true
}

// Open an object for read
func (fs *FsObjectSwift) Open() (in io.ReadCloser, err error) {
	in, _, err = fs.swift.c.ObjectOpen(fs.swift.container, fs.info.Name, true, nil)
	return
}

// Remove an object
func (fs *FsObjectSwift) Remove() error {
	return fs.swift.c.ObjectDelete(fs.swift.container, fs.remote)
}

// Check the interfaces are satisfied
var _ Fs = &FsSwift{}
var _ FsObject = &FsObjectSwift{}
