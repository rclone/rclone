// Swift interface
package swift

// FIXME need to prevent anything but ListDir working for swift://

import (
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/swift"
)

// Register with Fs
func init() {
	fs.Register(&fs.FsInfo{
		Name:  "swift",
		NewFs: NewFs,
		Options: []fs.Option{{
			Name: "user",
			Help: "User name to log in.",
		}, {
			Name: "key",
			Help: "API key or password.",
		}, {
			Name: "auth",
			Help: "Authentication URL for server.",
			Examples: []fs.OptionExample{{
				Help:  "Rackspace US",
				Value: "https://auth.api.rackspacecloud.com/v1.0",
			}, {
				Help:  "Rackspace UK",
				Value: "https://lon.auth.api.rackspacecloud.com/v1.0",
			}, {
				Help:  "Rackspace v2",
				Value: "https://identity.api.rackspacecloud.com/v2.0",
			}, {
				Help:  "Memset Memstore UK",
				Value: "https://auth.storage.memset.com/v1.0",
			}, {
				Help:  "Memset Memstore UK v2",
				Value: "https://auth.storage.memset.com/v2.0",
			}},
		},
		// snet     = flag.Bool("swift-snet", false, "Use internal service network") // FIXME not implemented
		},
	})
}

// FsSwift represents a remote swift server
type FsSwift struct {
	c         swift.Connection // the connection to the swift server
	container string           // the container we are working on
	root      string           // the path we are working on if any
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

// String converts this FsSwift to a string
func (f *FsSwift) String() string {
	return fmt.Sprintf("Swift container %s", f.container)
}

// Pattern to match a swift path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parseParse parses a swift 'url'
func parsePath(path string) (container, directory string, err error) {
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = fmt.Errorf("Couldn't find container in swift path %q", path)
	} else {
		container, directory = parts[1], parts[2]
		directory = strings.Trim(directory, "/")
	}
	return
}

// swiftConnection makes a connection to swift
func swiftConnection(name string) (*swift.Connection, error) {
	userName := fs.ConfigFile.MustValue(name, "user")
	if userName == "" {
		return nil, errors.New("user not found")
	}
	apiKey := fs.ConfigFile.MustValue(name, "key")
	if apiKey == "" {
		return nil, errors.New("key not found")
	}
	authUrl := fs.ConfigFile.MustValue(name, "auth")
	if authUrl == "" {
		return nil, errors.New("auth not found")
	}
	c := &swift.Connection{
		UserName: userName,
		ApiKey:   apiKey,
		AuthUrl:  authUrl,
	}
	err := c.Authenticate()
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NewFs contstructs an FsSwift from the path, container:path
func NewFs(name, path string) (fs.Fs, error) {
	container, directory, err := parsePath(path)
	if err != nil {
		return nil, err
	}
	if directory != "" {
		return nil, fmt.Errorf("Directories not supported yet in %q", path)
	}
	c, err := swiftConnection(name)
	if err != nil {
		return nil, err
	}
	f := &FsSwift{c: *c, container: container, root: directory}
	return f, nil
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsSwift) NewFsObjectWithInfo(remote string, info *swift.Object) fs.Object {
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
func (f *FsSwift) NewFsObject(remote string) fs.Object {
	return f.NewFsObjectWithInfo(remote, nil)
}

// Walk the path returning a channel of FsObjects
func (f *FsSwift) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
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
			fs.Stats.Error()
			log.Printf("Couldn't read container %q: %s", f.container, err)
		}
		close(out)
	}()
	return out
}

// Lists the containers
func (f *FsSwift) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		containers, err := f.c.ContainersAll(nil)
		if err != nil {
			fs.Stats.Error()
			log.Printf("Couldn't list containers: %s", err)
		} else {
			for _, container := range containers {
				out <- &fs.Dir{
					Name:  container.Name,
					Bytes: container.Bytes,
					Count: container.Count,
				}
			}
		}
	}()
	return out
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *FsSwift) Put(in io.Reader, remote string, modTime time.Time, size int64) (fs.Object, error) {
	// Temporary FsObject under construction
	fs := &FsObjectSwift{swift: f, remote: remote}
	return fs, fs.Update(in, modTime, size)
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

// Return the parent Fs
func (o *FsObjectSwift) Fs() fs.Fs {
	return o.swift
}

// Return a string version
func (o *FsObjectSwift) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Return the remote path
func (o *FsObjectSwift) Remote() string {
	return o.remote
}

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (o *FsObjectSwift) Md5sum() (string, error) {
	return strings.ToLower(o.info.Hash), nil
}

// Size returns the size of an object in bytes
func (o *FsObjectSwift) Size() int64 {
	return o.info.Bytes
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *FsObjectSwift) readMetaData() (err error) {
	if o.meta != nil {
		return nil
	}
	info, h, err := o.swift.c.Object(o.swift.container, o.remote)
	if err != nil {
		fs.Debug(o, "Failed to read info: %s", err)
		return err
	}
	meta := h.ObjectMetadata()
	o.info = info
	o.meta = &meta
	return nil
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *FsObjectSwift) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		// fs.Log(o, "Failed to read metadata: %s", err)
		return o.info.LastModified
	}
	modTime, err := o.meta.GetModTime()
	if err != nil {
		// fs.Log(o, "Failed to read mtime from object: %s", err)
		return o.info.LastModified
	}
	return modTime
}

// Sets the modification time of the local fs object
func (o *FsObjectSwift) SetModTime(modTime time.Time) {
	err := o.readMetaData()
	if err != nil {
		fs.Stats.Error()
		fs.Log(o, "Failed to read metadata: %s", err)
		return
	}
	o.meta.SetModTime(modTime)
	err = o.swift.c.ObjectUpdate(o.swift.container, o.remote, o.meta.ObjectHeaders())
	if err != nil {
		fs.Stats.Error()
		fs.Log(o, "Failed to update remote mtime: %s", err)
	}
}

// Is this object storable
func (o *FsObjectSwift) Storable() bool {
	return true
}

// Open an object for read
func (o *FsObjectSwift) Open() (in io.ReadCloser, err error) {
	in, _, err = o.swift.c.ObjectOpen(o.swift.container, o.remote, true, nil)
	return
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *FsObjectSwift) Update(in io.Reader, modTime time.Time, size int64) error {
	// Set the mtime
	m := swift.Metadata{}
	m.SetModTime(modTime)
	_, err := o.swift.c.ObjectPut(o.swift.container, o.remote, in, true, "", "", m.ObjectHeaders())
	return err
}

// Remove an object
func (o *FsObjectSwift) Remove() error {
	return o.swift.c.ObjectDelete(o.swift.container, o.remote)
}

// Check the interfaces are satisfied
var _ fs.Fs = &FsSwift{}
var _ fs.Object = &FsObjectSwift{}
