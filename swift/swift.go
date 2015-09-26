// Package swift provides an interface to the Swift object storage system
package swift

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/swift"
	"github.com/spf13/pflag"
)

// Globals
var (
	chunkSize = fs.SizeSuffix(5 * 1024 * 1024 * 1024)
)

// Register with Fs
func init() {
	fs.Register(&fs.Info{
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
		}, {
			Name: "tenant",
			Help: "Tenant name - optional",
		}, {
			Name: "region",
			Help: "Region name - optional",
		},
		},
	})
	// snet     = flag.Bool("swift-snet", false, "Use internal service network") // FIXME not implemented
	pflag.VarP(&chunkSize, "swift-chunk-size", "", "Above this size files will be chunked into a _segments container.")
}

// FsSwift represents a remote swift server
type FsSwift struct {
	name      string           // name of this remote
	c         swift.Connection // the connection to the swift server
	container string           // the container we are working on
	root      string           // the path we are working on if any
}

// FsObjectSwift describes a swift object
//
// Will definitely have info but maybe not meta
type FsObjectSwift struct {
	swift   *FsSwift       // what this object is part of
	remote  string         // The remote path
	info    swift.Object   // Info from the swift object if known
	headers *swift.Headers // The object headers if known
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *FsSwift) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *FsSwift) Root() string {
	if f.root == "" {
		return f.container
	}
	return f.container + "/" + f.root
}

// String converts this FsSwift to a string
func (f *FsSwift) String() string {
	if f.root == "" {
		return fmt.Sprintf("Swift container %s", f.container)
	}
	return fmt.Sprintf("Swift container %s path %s", f.container, f.root)
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
	authURL := fs.ConfigFile.MustValue(name, "auth")
	if authURL == "" {
		return nil, errors.New("auth not found")
	}
	c := &swift.Connection{
		UserName:       userName,
		ApiKey:         apiKey,
		AuthUrl:        authURL,
		UserAgent:      fs.UserAgent,
		Tenant:         fs.ConfigFile.MustValue(name, "tenant"),
		Region:         fs.ConfigFile.MustValue(name, "region"),
		ConnectTimeout: 10 * fs.Config.ConnectTimeout, // Use the timeouts in the transport
		Timeout:        10 * fs.Config.Timeout,        // Use the timeouts in the transport
		Transport:      fs.Config.Transport(),
	}
	err := c.Authenticate()
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NewFs contstructs an FsSwift from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	container, directory, err := parsePath(root)
	if err != nil {
		return nil, err
	}
	c, err := swiftConnection(name)
	if err != nil {
		return nil, err
	}
	f := &FsSwift{
		name:      name,
		c:         *c,
		container: container,
		root:      directory,
	}
	if f.root != "" {
		f.root += "/"
		// Check to see if the object exists
		_, _, err = f.c.Object(container, directory)
		if err == nil {
			remote := path.Base(directory)
			f.root = path.Dir(directory)
			if f.root == "." {
				f.root = ""
			} else {
				f.root += "/"
			}
			obj := f.NewFsObject(remote)
			// return a Fs Limited to this object
			return fs.NewLimited(f, obj), nil
		}
	}
	return f, nil
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsSwift) newFsObjectWithInfo(remote string, info *swift.Object) fs.Object {
	fs := &FsObjectSwift{
		swift:  f,
		remote: remote,
	}
	if info != nil {
		// Set info but not headers
		fs.info = *info
	} else {
		err := fs.readMetaData() // reads info and headers, returning an error
		if err != nil {
			// logged already FsDebug("Failed to read info: %s", err)
			return nil
		}
	}
	return fs
}

// NewFsObject returns an FsObject from a path
//
// May return nil if an error occurred
func (f *FsSwift) NewFsObject(remote string) fs.Object {
	return f.newFsObjectWithInfo(remote, nil)
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
func (f *FsSwift) list(directories bool, fn func(string, *swift.Object)) {
	// Options for ObjectsWalk
	opts := swift.ObjectsOpts{
		Prefix: f.root,
		Limit:  256,
	}
	if directories {
		opts.Delimiter = '/'
	}
	rootLength := len(f.root)
	err := f.c.ObjectsWalk(f.container, &opts, func(opts *swift.ObjectsOpts) (interface{}, error) {
		objects, err := f.c.Objects(f.container, opts)
		if err == nil {
			for i := range objects {
				object := &objects[i]
				// FIXME if there are no directories, swift gives back the files for some reason!
				if directories {
					if !strings.HasSuffix(object.Name, "/") {
						continue
					}
					object.Name = object.Name[:len(object.Name)-1]
				}
				if !strings.HasPrefix(object.Name, f.root) {
					fs.Log(f, "Odd name received %q", object.Name)
					continue
				}
				remote := object.Name[rootLength:]
				fn(remote, object)
			}
		}
		return objects, err
	})
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(f, "Couldn't read container %q: %s", f.container, err)
	}
}

// List walks the path returning a channel of FsObjects
func (f *FsSwift) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
	if f.container == "" {
		// Return no objects at top level list
		close(out)
		fs.Stats.Error()
		fs.ErrorLog(f, "Can't list objects at root - choose a container using lsd")
	} else {
		// List the objects
		go func() {
			defer close(out)
			f.list(false, func(remote string, object *swift.Object) {
				if fs := f.newFsObjectWithInfo(remote, object); fs != nil {
					out <- fs
				}
			})
		}()
	}
	return out
}

// ListDir lists the containers
func (f *FsSwift) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	if f.container == "" {
		// List the containers
		go func() {
			defer close(out)
			containers, err := f.c.ContainersAll(nil)
			if err != nil {
				fs.Stats.Error()
				fs.ErrorLog(f, "Couldn't list containers: %v", err)
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
	} else {
		// List the directories in the path in the container
		go func() {
			defer close(out)
			f.list(true, func(remote string, object *swift.Object) {
				out <- &fs.Dir{
					Name:  remote,
					Bytes: object.Bytes,
					Count: 0,
				}
			})
		}()
	}
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

// Precision of the remote
func (f *FsSwift) Precision() time.Duration {
	return time.Nanosecond
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *FsSwift) Copy(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*FsObjectSwift)
	if !ok {
		fs.Debug(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcFs := srcObj.swift
	_, err := f.c.ObjectCopy(srcFs.container, srcFs.root+srcObj.remote, f.container, f.root+remote, nil)
	if err != nil {
		return nil, err
	}
	return f.NewFsObject(remote), nil
}

// ------------------------------------------------------------

// Fs returns the parent Fs
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

// Remote returns the remote path
func (o *FsObjectSwift) Remote() string {
	return o.remote
}

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (o *FsObjectSwift) Md5sum() (string, error) {
	isManifest, err := o.IsManifestFile()
	if err != nil {
		return "", err
	}
	if isManifest {
		fs.Debug(o, "Return empty md5 for swift manifest file. Md5 of manifest file calculate as md5 of md5 of it's parts, so it's not original md5")
		return "", nil
	} else {
		return strings.ToLower(o.info.Hash), nil
	}
}

// Check manifest header
func (o *FsObjectSwift) IsManifestFile() (bool, error) {
	err := o.readMetaData()
	if err != nil {
		return false, err
	}
	_, isManifestFile := (*o.headers)["X-Object-Manifest"]
	return isManifestFile, nil
}

// Size returns the size of an object in bytes
func (o *FsObjectSwift) Size() int64 {
	return o.info.Bytes
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *FsObjectSwift) readMetaData() (err error) {
	if o.headers != nil {
		return nil
	}
	info, h, err := o.swift.c.Object(o.swift.container, o.swift.root+o.remote)
	if err != nil {
		fs.Debug(o, "Failed to read info: %s", err)
		return err
	}
	o.info = info
	o.headers = &h
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
	modTime, err := o.headers.ObjectMetadata().GetModTime()
	if err != nil {
		// fs.Log(o, "Failed to read mtime from object: %s", err)
		return o.info.LastModified
	}
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *FsObjectSwift) SetModTime(modTime time.Time) {
	err := o.readMetaData()
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to read metadata: %s", err)
		return
	}
	meta := o.headers.ObjectMetadata()
	meta.SetModTime(modTime)
	err = o.swift.c.ObjectUpdate(o.swift.container, o.swift.root+o.remote, meta.ObjectHeaders())
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to update remote mtime: %s", err)
	}
}

// Storable returns if this object is storable
func (o *FsObjectSwift) Storable() bool {
	return true
}

// Open an object for read
func (o *FsObjectSwift) Open() (in io.ReadCloser, err error) {
	in, _, err = o.swift.c.ObjectOpen(o.swift.container, o.swift.root+o.remote, true, nil)
	return
}

func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *FsObjectSwift) Update(in io.Reader, modTime time.Time, size int64) error {
	// Set the mtime
	m := swift.Metadata{}
	m.SetModTime(modTime)
	if size > int64(chunkSize) {
		segmentsContainerName := o.swift.container + "_segments"
		left := size
		i := 0
		nowFloat := swift.TimeToFloatString(time.Now())
		for left > 0 {
			n := min(left, int64(chunkSize))
			segmentReader := io.LimitReader(in, n)
			segmentPath := fmt.Sprintf("%s%s/%s/%d/%09d", o.swift.root, o.remote, nowFloat, size, i)
			_, err := o.swift.c.ObjectPut(segmentsContainerName, segmentPath, segmentReader, true, "", "", m.ObjectHeaders())
			if err != nil {
				return err
			}
			left -= n
			i += 1
		}
		manifestHeaders := swift.Headers{"X-Object-Manifest": fmt.Sprintf("%s/%s%s/%s/%d", segmentsContainerName, o.swift.root, o.remote, nowFloat, size)}
		for k, v := range m.ObjectHeaders() {
			manifestHeaders[k] = v
		}
		emptyReader := bytes.NewReader(nil)
		manifestName := o.swift.root + o.remote
		_, err := o.swift.c.ObjectPut(o.swift.container, manifestName, emptyReader, true, "", "", manifestHeaders)
		if err != nil {
			return err
		}
		// remove old segments
		segmentsPath := fmt.Sprintf("%s/%s%s/", segmentsContainerName, o.swift.root, o.remote)
		segmentsFs, err := NewFs(o.swift.name, segmentsPath)
		if err != nil {
			return err
		}
		for o := range segmentsFs.List() {
			if !strings.HasPrefix(o.Remote(), nowFloat) {
				fs.Log(o, "Remove old file segment '%s'", o.Remote())
				err := o.Remove()
				if err != nil {
                 		       return err
                		}
			}
		}
	} else {
		_, err := o.swift.c.ObjectPut(o.swift.container, o.swift.root+o.remote, in, true, "", "", m.ObjectHeaders())
		if err != nil {
			return err
		}
	}
	// Read the metadata from the newly created object
	o.headers = nil // wipe old metadata
	return o.readMetaData()
}

// Remove an object
func (o *FsObjectSwift) Remove() error {
	isManifestFile, err := o.IsManifestFile()
	if err != nil {
		return err
	}
	if isManifestFile {
		// remove segments
		segmentsContainerName := o.swift.container + "_segments"
		segmentsPath := fmt.Sprintf("%s/%s%s/", segmentsContainerName, o.swift.root, o.remote)
		segmentsFs, err := NewFs(o.swift.name, segmentsPath)
		if err != nil {
			return err
		}
		for o := range segmentsFs.List() {
			err := o.Remove()
			if err != nil {
				return err
			}
		}
	}
	return o.swift.c.ObjectDelete(o.swift.container, o.swift.root+o.remote)
}

// Check the interfaces are satisfied
var _ fs.Fs = &FsSwift{}
var _ fs.Copier = &FsSwift{}
var _ fs.Object = &FsObjectSwift{}

