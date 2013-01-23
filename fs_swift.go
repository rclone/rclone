// Swift interface
package main

// FIXME need to prevent anything but ListDir working for swift://

import (
	"errors"
	"flag"
	"fmt"
	"github.com/ncw/swift"
	"io"
	"log"
	"os"
	"regexp"
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

// Globals
var (
	// Flags
	// FIXME make these part of swift so we get a standard set of flags?
	authUrl  = flag.String("auth", os.Getenv("ST_AUTH"), "Auth URL for server. Defaults to environment var ST_AUTH.")
	userName = flag.String("user", os.Getenv("ST_USER"), "User name. Defaults to environment var ST_USER.")
	apiKey   = flag.String("key", os.Getenv("ST_KEY"), "API key (password). Defaults to environment var ST_KEY.")
	snet     = flag.Bool("snet", false, "Use internal service network") // FIXME not implemented
)

// String converts this FsSwift to a string
func (f *FsSwift) String() string {
	return fmt.Sprintf("Swift container %s", f.container)
}

// Pattern to match a swift url
var swiftMatch = regexp.MustCompile(`^swift://([^/]*)(.*)$`)

// parseParse parses a swift 'url'
func parsePath(path string) (container, directory string, err error) {
	parts := swiftMatch.FindAllStringSubmatch(path, -1)
	if len(parts) != 1 || len(parts[0]) != 3 {
		err = fmt.Errorf("Couldn't parse swift url %q", path)
	} else {
		container, directory = parts[0][1], parts[0][2]
		directory = strings.Trim(directory, "/")
	}
	return
}

// swiftConnection makes a connection to swift
func swiftConnection() (*swift.Connection, error) {
	if *userName == "" {
		return nil, errors.New("Need -user or environmental variable ST_USER")
	}
	if *apiKey == "" {
		return nil, errors.New("Need -key or environmental variable ST_KEY")
	}
	if *authUrl == "" {
		return nil, errors.New("Need -auth or environmental variable ST_AUTH")
	}
	c := &swift.Connection{
		UserName: *userName,
		ApiKey:   *apiKey,
		AuthUrl:  *authUrl,
	}
	err := c.Authenticate()
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NewFsSwift contstructs an FsSwift from the path, container:path
func NewFsSwift(path string) (*FsSwift, error) {
	container, directory, err := parsePath(path)
	if err != nil {
		return nil, err
	}
	if directory != "" {
		return nil, fmt.Errorf("Directories not supported yet in %q", path)
	}
	c, err := swiftConnection()
	if err != nil {
		return nil, err
	}
	f := &FsSwift{c: *c, container: container}
	return f, nil
}

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
			// logged already FsDebug("Failed to read info: %s", err)
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
			stats.Error()
			log.Printf("Couldn't read container %q: %s", f.container, err)
		}
		close(out)
	}()
	return out
}

// Lists the containers
func (f *FsSwift) ListDir() FsDirChan {
	out := make(FsDirChan, *checkers)
	go func() {
		defer close(out)
		containers, err := f.c.ContainersAll(nil)
		if err != nil {
			stats.Error()
			log.Printf("Couldn't list containers: %s", err)
		} else {
			for _, container := range containers {
				out <- &FsDir{
					Name:  container.Name,
					Bytes: container.Bytes,
					Count: container.Count,
				}
			}
		}
	}()
	return out
}

// Put the FsObject into the container
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created
func (f *FsSwift) Put(in io.Reader, remote string, modTime time.Time, size int64) (FsObject, error) {
	// Temporary FsObject under construction
	fs := &FsObjectSwift{swift: f, remote: remote}

	// Set the mtime
	m := swift.Metadata{}
	m.SetModTime(modTime)
	_, err := f.c.ObjectPut(f.container, remote, in, true, "", "", m.ObjectHeaders())
	return fs, err
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

// Return the precision
func (fs *FsSwift) Precision() time.Duration {
	return time.Nanosecond
}

// ------------------------------------------------------------

// Return the remote path
func (fs *FsObjectSwift) Remote() string {
	return fs.remote
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
		FsDebug(fs, "Failed to read info: %s", err)
		return err
	}
	meta := h.ObjectMetadata()
	fs.info = info
	fs.meta = &meta
	return nil
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (fs *FsObjectSwift) ModTime() time.Time {
	err := fs.readMetaData()
	if err != nil {
		// FsLog(fs, "Failed to read metadata: %s", err)
		return fs.info.LastModified
	}
	modTime, err := fs.meta.GetModTime()
	if err != nil {
		// FsLog(fs, "Failed to read mtime from object: %s", err)
		return fs.info.LastModified
	}
	return modTime
}

// Sets the modification time of the local fs object
func (fs *FsObjectSwift) SetModTime(modTime time.Time) {
	err := fs.readMetaData()
	if err != nil {
		stats.Error()
		FsLog(fs, "Failed to read metadata: %s", err)
		return
	}
	fs.meta.SetModTime(modTime)
	err = fs.swift.c.ObjectUpdate(fs.swift.container, fs.remote, fs.meta.ObjectHeaders())
	if err != nil {
		stats.Error()
		FsLog(fs, "Failed to update remote mtime: %s", err)
	}
}

// Is this object storable
func (fs *FsObjectSwift) Storable() bool {
	return true
}

// Open an object for read
func (fs *FsObjectSwift) Open() (in io.ReadCloser, err error) {
	in, _, err = fs.swift.c.ObjectOpen(fs.swift.container, fs.remote, true, nil)
	return
}

// Remove an object
func (fs *FsObjectSwift) Remove() error {
	return fs.swift.c.ObjectDelete(fs.swift.container, fs.remote)
}

// Check the interfaces are satisfied
var _ Fs = &FsSwift{}
var _ FsObject = &FsObjectSwift{}
