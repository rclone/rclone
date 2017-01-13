// Package swift provides an interface to the Swift object storage system
package swift

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/swift"
	"github.com/pkg/errors"
)

// Constants
const (
	directoryMarkerContentType = "application/directory" // content type of directory marker objects
)

// Globals
var (
	chunkSize = fs.SizeSuffix(5 * 1024 * 1024 * 1024)
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "swift",
		Description: "Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)",
		NewFs:       NewFs,
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
			}, {
				Help:  "OVH",
				Value: "https://auth.cloud.ovh.net/v2.0",
			}},
		}, {
			Name: "domain",
			Help: "User domain - optional (v3 auth)",
		}, {
			Name: "tenant",
			Help: "Tenant name - optional for v1 auth, required otherwise",
		}, {
			Name: "tenant_domain",
			Help: "Tenant domain - optional (v3 auth)",
		}, {
			Name: "region",
			Help: "Region name - optional",
		}, {
			Name: "storage_url",
			Help: "Storage URL - optional",
		}, {
			Name: "auth_version",
			Help: "AuthVersion - optional - set to (1,2,3) if your auth URL has no version",
		},
		},
	})
	// snet     = flag.Bool("swift-snet", false, "Use internal service network") // FIXME not implemented
	fs.VarP(&chunkSize, "swift-chunk-size", "", "Above this size files will be chunked into a _segments container.")
}

// Fs represents a remote swift server
type Fs struct {
	name              string            // name of this remote
	root              string            // the path we are working on if any
	features          *fs.Features      // optional features
	c                 *swift.Connection // the connection to the swift server
	container         string            // the container we are working on
	segmentsContainer string            // container to store the segments (if any) in
}

// Object describes a swift object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs      *Fs            // what this object is part of
	remote  string         // The remote path
	info    swift.Object   // Info from the swift object if known
	headers *swift.Headers // The object headers if known
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	if f.root == "" {
		return f.container
	}
	return f.container + "/" + f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	if f.root == "" {
		return fmt.Sprintf("Swift container %s", f.container)
	}
	return fmt.Sprintf("Swift container %s path %s", f.container, f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Pattern to match a swift path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parseParse parses a swift 'url'
func parsePath(path string) (container, directory string, err error) {
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = errors.Errorf("couldn't find container in swift path %q", path)
	} else {
		container, directory = parts[1], parts[2]
		directory = strings.Trim(directory, "/")
	}
	return
}

// swiftConnection makes a connection to swift
func swiftConnection(name string) (*swift.Connection, error) {
	userName := fs.ConfigFileGet(name, "user")
	if userName == "" {
		return nil, errors.New("user not found")
	}
	apiKey := fs.ConfigFileGet(name, "key")
	if apiKey == "" {
		return nil, errors.New("key not found")
	}
	authURL := fs.ConfigFileGet(name, "auth")
	if authURL == "" {
		return nil, errors.New("auth not found")
	}
	c := &swift.Connection{
		UserName:       userName,
		ApiKey:         apiKey,
		AuthUrl:        authURL,
		AuthVersion:    fs.ConfigFileGetInt(name, "auth_version", 0),
		Tenant:         fs.ConfigFileGet(name, "tenant"),
		Region:         fs.ConfigFileGet(name, "region"),
		Domain:         fs.ConfigFileGet(name, "domain"),
		TenantDomain:   fs.ConfigFileGet(name, "tenant_domain"),
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

// NewFsWithConnection contstructs an Fs from the path, container:path
// and authenticated connection
func NewFsWithConnection(name, root string, c *swift.Connection) (fs.Fs, error) {
	container, directory, err := parsePath(root)
	if err != nil {
		return nil, err
	}
	f := &Fs{
		name:              name,
		c:                 c,
		container:         container,
		segmentsContainer: container + "_segments",
		root:              directory,
	}
	f.features = (&fs.Features{ReadMimeType: true, WriteMimeType: true}).Fill(f)
	// StorageURL overloading
	storageURL := fs.ConfigFileGet(name, "storage_url")
	if storageURL != "" {
		f.c.StorageUrl = storageURL
		f.c.Auth = newAuth(f.c.Auth, storageURL)
	}
	if f.root != "" {
		f.root += "/"
		// Check to see if the object exists - ignoring directory markers
		info, _, err := f.c.Object(container, directory)
		if err == nil && info.ContentType != directoryMarkerContentType {
			f.root = path.Dir(directory)
			if f.root == "." {
				f.root = ""
			} else {
				f.root += "/"
			}
			// return an error with an fs which points to the parent
			return f, fs.ErrorIsFile
		}
	}
	return f, nil
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	c, err := swiftConnection(name)
	if err != nil {
		return nil, err
	}
	return NewFsWithConnection(name, root, c)
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *swift.Object) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	// Note that due to a quirk of swift, dynamic large objects are
	// returned as 0 bytes in the listing.  Correct this here by
	// making sure we read the full metadata for all 0 byte files.
	// We don't read the metadata for directory marker objects.
	if info != nil && info.Bytes == 0 && info.ContentType != "application/directory" {
		info = nil
	}
	if info != nil {
		// Set info but not headers
		o.info = *info
	} else {
		err := o.readMetaData() // reads info and headers, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found it
// returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// listFn is called from list and listContainerRoot to handle an object.
type listFn func(remote string, object *swift.Object, isDirectory bool) error

// listContainerRoot lists the objects into the function supplied from
// the container and root supplied
//
// Level is the level of the recursion
func (f *Fs) listContainerRoot(container, root string, dir string, level int, fn listFn) error {
	prefix := root
	if dir != "" {
		prefix += dir + "/"
	}
	// Options for ObjectsWalk
	opts := swift.ObjectsOpts{
		Prefix: prefix,
		Limit:  256,
	}
	switch level {
	case 1:
		opts.Delimiter = '/'
	case fs.MaxLevel:
	default:
		return fs.ErrorLevelNotSupported
	}
	rootLength := len(root)
	return f.c.ObjectsWalk(container, &opts, func(opts *swift.ObjectsOpts) (interface{}, error) {
		objects, err := f.c.Objects(container, opts)
		if err == nil {
			for i := range objects {
				object := &objects[i]
				isDirectory := false
				if level == 1 {
					if strings.HasSuffix(object.Name, "/") {
						isDirectory = true
						object.Name = object.Name[:len(object.Name)-1]
					}
				}
				if !strings.HasPrefix(object.Name, root) {
					fs.Log(f, "Odd name received %q", object.Name)
					continue
				}
				remote := object.Name[rootLength:]
				err = fn(remote, object, isDirectory)
				if err != nil {
					break
				}
			}
		}
		return objects, err
	})
}

// list the objects into the function supplied
func (f *Fs) list(dir string, level int, fn listFn) error {
	return f.listContainerRoot(f.container, f.root, dir, level, fn)
}

// listFiles walks the path returning a channel of Objects
func (f *Fs) listFiles(out fs.ListOpts, dir string) {
	defer out.Finished()
	if f.container == "" {
		out.SetError(errors.New("can't list objects at root - choose a container using lsd"))
		return
	}
	// List the objects
	err := f.list(dir, out.Level(), func(remote string, object *swift.Object, isDirectory bool) error {
		if isDirectory {
			dir := &fs.Dir{
				Name:  remote,
				Bytes: object.Bytes,
				Count: 0,
			}
			if out.AddDir(dir) {
				return fs.ErrorListAborted
			}
		} else {
			o, err := f.newObjectWithInfo(remote, object)
			if err != nil {
				return err
			}
			// Storable does a full metadata read on 0 size objects which might be dynamic large objects
			if o.Storable() {
				if out.Add(o) {
					return fs.ErrorListAborted
				}
			}
		}
		return nil
	})
	if err != nil {
		if err == swift.ContainerNotFound {
			err = fs.ErrorDirNotFound
		}
		out.SetError(err)
	}
}

// listContainers lists the containers
func (f *Fs) listContainers(out fs.ListOpts, dir string) {
	defer out.Finished()
	if dir != "" {
		out.SetError(fs.ErrorListOnlyRoot)
		return
	}
	containers, err := f.c.ContainersAll(nil)
	if err != nil {
		out.SetError(err)
		return
	}
	for _, container := range containers {
		dir := &fs.Dir{
			Name:  container.Name,
			Bytes: container.Bytes,
			Count: container.Count,
		}
		if out.AddDir(dir) {
			break
		}
	}
}

// List walks the path returning files and directories to out
func (f *Fs) List(out fs.ListOpts, dir string) {
	if f.container == "" {
		f.listContainers(out, dir)
	} else {
		f.listFiles(out, dir)
	}
	return
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs, fs.Update(in, src)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	// Can't create subdirs
	if dir != "" {
		return nil
	}
	// Check to see if container exists first
	_, _, err := f.c.Container(f.container)
	if err == nil {
		return nil
	}
	if err == swift.ContainerNotFound {
		return f.c.ContainerCreate(f.container, nil)
	}
	return err

}

// Rmdir deletes the container if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	if f.root != "" || dir != "" {
		return nil
	}
	return f.c.ContainerDelete(f.container)
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Purge deletes all the files and directories
//
// Implemented here so we can make sure we delete directory markers
func (f *Fs) Purge() error {
	// Delete all the files including the directory markers
	toBeDeleted := make(chan fs.Object, fs.Config.Transfers)
	delErr := make(chan error, 1)
	go func() {
		delErr <- fs.DeleteFiles(toBeDeleted)
	}()
	err := f.list("", fs.MaxLevel, func(remote string, object *swift.Object, isDirectory bool) error {
		if !isDirectory {
			o, err := f.newObjectWithInfo(remote, object)
			if err != nil {
				return err
			}
			toBeDeleted <- o
		}
		return nil
	})
	close(toBeDeleted)
	delError := <-delErr
	if err == nil {
		err = delError
	}
	if err != nil {
		return err
	}
	return f.Rmdir("")
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
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debug(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcFs := srcObj.fs
	_, err := f.c.ObjectCopy(srcFs.container, srcFs.root+srcObj.remote, f.container, f.root+remote, nil)
	if err != nil {
		return nil, err
	}
	return f.NewObject(remote)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(t fs.HashType) (string, error) {
	if t != fs.HashMD5 {
		return "", fs.ErrHashUnsupported
	}
	isDynamicLargeObject, err := o.isDynamicLargeObject()
	if err != nil {
		return "", err
	}
	isStaticLargeObject, err := o.isStaticLargeObject()
	if err != nil {
		return "", err
	}
	if isDynamicLargeObject || isStaticLargeObject {
		fs.Debug(o, "Returning empty Md5sum for swift large object")
		return "", nil
	}
	return strings.ToLower(o.info.Hash), nil
}

// hasHeader checks for the header passed in returning false if the
// object isn't found.
func (o *Object) hasHeader(header string) (bool, error) {
	err := o.readMetaData()
	if err != nil {
		if err == fs.ErrorObjectNotFound {
			return false, nil
		}
		return false, err
	}
	_, isDynamicLargeObject := (*o.headers)[header]
	return isDynamicLargeObject, nil
}

// isDynamicLargeObject checks for X-Object-Manifest header
func (o *Object) isDynamicLargeObject() (bool, error) {
	return o.hasHeader("X-Object-Manifest")
}

// isStaticLargeObjectFile checks for the X-Static-Large-Object header
func (o *Object) isStaticLargeObject() (bool, error) {
	return o.hasHeader("X-Static-Large-Object")
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.info.Bytes
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
//
// it returns fs.ErrorObjectNotFound if the object isn't found
func (o *Object) readMetaData() (err error) {
	if o.headers != nil {
		return nil
	}
	info, h, err := o.fs.c.Object(o.fs.container, o.fs.root+o.remote)
	if err != nil {
		if err == swift.ObjectNotFound {
			return fs.ErrorObjectNotFound
		}
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
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Debug(o, "Failed to read metadata: %s", err)
		return o.info.LastModified
	}
	modTime, err := o.headers.ObjectMetadata().GetModTime()
	if err != nil {
		// fs.Log(o, "Failed to read mtime from object: %v", err)
		return o.info.LastModified
	}
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	err := o.readMetaData()
	if err != nil {
		return err
	}
	meta := o.headers.ObjectMetadata()
	meta.SetModTime(modTime)
	newHeaders := meta.ObjectHeaders()
	for k, v := range newHeaders {
		(*o.headers)[k] = v
	}
	// Include any other metadata from request
	for k, v := range *o.headers {
		if strings.HasPrefix(k, "X-Object-") {
			newHeaders[k] = v
		}
	}
	return o.fs.c.ObjectUpdate(o.fs.container, o.fs.root+o.remote, newHeaders)
}

// Storable returns if this object is storable
//
// It compares the Content-Type to directoryMarkerContentType - that
// makes it a directory marker which is not storable.
func (o *Object) Storable() bool {
	return o.info.ContentType != directoryMarkerContentType
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	headers := fs.OpenOptionHeaders(options)
	_, isRanging := headers["Range"]
	in, _, err = o.fs.c.ObjectOpen(o.fs.container, o.fs.root+o.remote, !isRanging, headers)
	return
}

// min returns the smallest of x, y
func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

// removeSegments removes any old segments from o
//
// if except is passed in then segments with that prefix won't be deleted
func (o *Object) removeSegments(except string) error {
	segmentsRoot := o.fs.root + o.remote + "/"
	err := o.fs.listContainerRoot(o.fs.segmentsContainer, segmentsRoot, "", fs.MaxLevel, func(remote string, object *swift.Object, isDirectory bool) error {
		if isDirectory {
			return nil
		}
		if except != "" && strings.HasPrefix(remote, except) {
			// fs.Debug(o, "Ignoring current segment file %q in container %q", segmentsRoot+remote, o.fs.segmentsContainer)
			return nil
		}
		segmentPath := segmentsRoot + remote
		fs.Debug(o, "Removing segment file %q in container %q", segmentPath, o.fs.segmentsContainer)
		return o.fs.c.ObjectDelete(o.fs.segmentsContainer, segmentPath)
	})
	if err != nil {
		return err
	}
	// remove the segments container if empty, ignore errors
	err = o.fs.c.ContainerDelete(o.fs.segmentsContainer)
	if err == nil {
		fs.Debug(o, "Removed empty container %q", o.fs.segmentsContainer)
	}
	return nil
}

// urlEncode encodes a string so that it is a valid URL
//
// We don't use any of Go's standard methods as we need `/` not
// encoded but we need '&' encoded.
func urlEncode(str string) string {
	var buf bytes.Buffer
	for i := 0; i < len(str); i++ {
		c := str[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '/' || c == '.' {
			_ = buf.WriteByte(c)
		} else {
			_, _ = buf.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return buf.String()
}

// updateChunks updates the existing object using chunks to a separate
// container.  It returns a string which prefixes current segments.
func (o *Object) updateChunks(in io.Reader, headers swift.Headers, size int64, contentType string) (string, error) {
	// Create the segmentsContainer if it doesn't exist
	err := o.fs.c.ContainerCreate(o.fs.segmentsContainer, nil)
	if err != nil {
		return "", err
	}
	// Upload the chunks
	left := size
	i := 0
	uniquePrefix := fmt.Sprintf("%s/%d", swift.TimeToFloatString(time.Now()), size)
	segmentsPath := fmt.Sprintf("%s%s/%s", o.fs.root, o.remote, uniquePrefix)
	for left > 0 {
		n := min(left, int64(chunkSize))
		headers["Content-Length"] = strconv.FormatInt(n, 10) // set Content-Length as we know it
		segmentReader := io.LimitReader(in, n)
		segmentPath := fmt.Sprintf("%s/%08d", segmentsPath, i)
		fs.Debug(o, "Uploading segment file %q into %q", segmentPath, o.fs.segmentsContainer)
		_, err := o.fs.c.ObjectPut(o.fs.segmentsContainer, segmentPath, segmentReader, true, "", "", headers)
		if err != nil {
			return "", err
		}
		left -= n
		i++
	}
	// Upload the manifest
	headers["X-Object-Manifest"] = urlEncode(fmt.Sprintf("%s/%s", o.fs.segmentsContainer, segmentsPath))
	headers["Content-Length"] = "0" // set Content-Length as we know it
	emptyReader := bytes.NewReader(nil)
	manifestName := o.fs.root + o.remote
	_, err = o.fs.c.ObjectPut(o.fs.container, manifestName, emptyReader, true, "", contentType, headers)
	return uniquePrefix + "/", err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	size := src.Size()
	modTime := src.ModTime()

	// Note whether this is a dynamic large object before starting
	isDynamicLargeObject, err := o.isDynamicLargeObject()
	if err != nil {
		return err
	}

	// Set the mtime
	m := swift.Metadata{}
	m.SetModTime(modTime)
	contentType := fs.MimeType(src)
	headers := m.ObjectHeaders()
	uniquePrefix := ""
	if size > int64(chunkSize) {
		uniquePrefix, err = o.updateChunks(in, headers, size, contentType)
		if err != nil {
			return err
		}
	} else {
		headers["Content-Length"] = strconv.FormatInt(size, 10) // set Content-Length as we know it
		_, err := o.fs.c.ObjectPut(o.fs.container, o.fs.root+o.remote, in, true, "", contentType, headers)
		if err != nil {
			return err
		}
	}

	// If file was a dynamic large object then remove old/all segments
	if isDynamicLargeObject {
		err = o.removeSegments(uniquePrefix)
		if err != nil {
			fs.Log(o, "Failed to remove old segments - carrying on with upload: %v", err)
		}
	}

	// Read the metadata from the newly created object
	o.headers = nil // wipe old metadata
	return o.readMetaData()
}

// Remove an object
func (o *Object) Remove() error {
	isDynamicLargeObject, err := o.isDynamicLargeObject()
	if err != nil {
		return err
	}
	// Remove file/manifest first
	err = o.fs.c.ObjectDelete(o.fs.container, o.fs.root+o.remote)
	if err != nil {
		return err
	}
	// ...then segments if required
	if isDynamicLargeObject {
		err = o.removeSegments("")
		if err != nil {
			return err
		}
	}
	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	return o.info.ContentType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = &Fs{}
	_ fs.Purger    = &Fs{}
	_ fs.Copier    = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.MimeTyper = &Object{}
)
