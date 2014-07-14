// Google Cloud Storage interface
package googlecloudstorage

/*
FIXME can't set updated but can set metadata

FIXME Patch needs full_control not just read_write

FIXME Patch isn't working with files with spaces in - giving 404 error
*/

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"code.google.com/p/google-api-go-client/storage/v1"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/googleauth"
)

const (
	rcloneClientId     = "202264815644.apps.googleusercontent.com"
	rcloneClientSecret = "X4Z3ca8xfWDb1Voo-F9a7ZxJ"
	RFC3339In          = time.RFC3339
	RFC3339Out         = "2006-01-02T15:04:05.000000000Z07:00"
	metaMtime          = "mtime" // key to store mtime under in metadata
	listChunks         = 256     // chunk size to read directory listings
)

var (
	// Description of how to auth for this app
	storageAuth = &googleauth.Auth{
		Scope: "https://www.googleapis.com/auth/devstorage.full_control",
		// Scope:               "https://www.googleapis.com/auth/devstorage.read_write",
		DefaultClientId:     rcloneClientId,
		DefaultClientSecret: rcloneClientSecret,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.FsInfo{
		Name:  "google cloud storage",
		NewFs: NewFs,
		Config: func(name string) {
			storageAuth.Config(name)
		},
		Options: []fs.Option{{
			Name: "project_number",
			Help: "Project number optional - needed only for list/create/delete buckets - see your developer console.",
		}, {
			Name: "client_id",
			Help: "Google Application Client Id - leave blank to use rclone's.",
		}, {
			Name: "client_secret",
			Help: "Google Application Client Secret - leave blank to use rclone's.",
		}},
	})
}

// FsStorage represents a remote storage server
type FsStorage struct {
	svc           *storage.Service // the connection to the storage server
	client        *http.Client     // authorized client
	bucket        string           // the bucket we are working on
	root          string           // the path we are working on if any
	projectNumber string           // used for finding buckets
}

// FsObjectStorage describes a storage object
//
// Will definitely have info but maybe not meta
type FsObjectStorage struct {
	storage *FsStorage // what this object is part of
	remote  string     // The remote path
	url     string     // download path
	md5sum  string     // The MD5Sum of the object
	bytes   int64      // Bytes in the object
	modTime time.Time  // Modified time of the object
}

// ------------------------------------------------------------

// String converts this FsStorage to a string
func (f *FsStorage) String() string {
	if f.root == "" {
		return fmt.Sprintf("Storage bucket %s", f.bucket)
	}
	return fmt.Sprintf("Storage bucket %s path %s", f.bucket, f.root)
}

// Pattern to match a storage path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parseParse parses a storage 'url'
func parsePath(path string) (bucket, directory string, err error) {
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = fmt.Errorf("Couldn't find bucket in storage path %q", path)
	} else {
		bucket, directory = parts[1], parts[2]
		directory = strings.Trim(directory, "/")
	}
	return
}

// NewFs contstructs an FsStorage from the path, bucket:path
func NewFs(name, root string) (fs.Fs, error) {
	t, err := storageAuth.NewTransport(name)
	if err != nil {
		return nil, err
	}

	bucket, directory, err := parsePath(root)
	if err != nil {
		return nil, err
	}

	f := &FsStorage{
		bucket:        bucket,
		root:          directory,
		projectNumber: fs.ConfigFile.MustValue(name, "project_number"),
	}

	// Create a new authorized Drive client.
	f.client = t.Client()
	f.svc, err = storage.New(f.client)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create Google Cloud Storage client: %s", err)
	}

	if f.root != "" {
		f.root += "/"
		// Check to see if the object exists
		_, err = f.svc.Objects.Get(bucket, directory).Do()
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
func (f *FsStorage) NewFsObjectWithInfo(remote string, info *storage.Object) fs.Object {
	o := &FsObjectStorage{
		storage: f,
		remote:  remote,
	}
	if info != nil {
		o.setMetaData(info)
	} else {
		err := o.readMetaData() // reads info and meta, returning an error
		if err != nil {
			// logged already FsDebug("Failed to read info: %s", err)
			return nil
		}
	}
	return o
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsStorage) NewFsObject(remote string) fs.Object {
	return f.NewFsObjectWithInfo(remote, nil)
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
func (f *FsStorage) list(directories bool, fn func(string, *storage.Object)) {
	list := f.svc.Objects.List(f.bucket).Prefix(f.root).MaxResults(listChunks)
	if directories {
		list = list.Delimiter("/")
	}
	rootLength := len(f.root)
	for {
		objects, err := list.Do()
		if err != nil {
			fs.Stats.Error()
			fs.Log(f, "Couldn't read bucket %q: %s", f.bucket, err)
			return
		}
		for _, object := range objects.Items {
			if directories && !strings.HasSuffix(object.Name, "/") {
				continue
			}
			if !strings.HasPrefix(object.Name, f.root) {
				fs.Log(f, "Odd name received %q", object.Name)
				continue
			}
			remote := object.Name[rootLength:]
			fn(remote, object)
		}
		if objects.NextPageToken == "" {
			break
		}
		list.PageToken(objects.NextPageToken)
	}
}

// Walk the path returning a channel of FsObjects
func (f *FsStorage) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
	if f.bucket == "" {
		// Return no objects at top level list
		close(out)
		fs.Stats.Error()
		fs.Log(f, "Can't list objects at root - choose a bucket using lsd")
	} else {
		// List the objects
		go func() {
			defer close(out)
			f.list(false, func(remote string, object *storage.Object) {
				if fs := f.NewFsObjectWithInfo(remote, object); fs != nil {
					out <- fs
				}
			})
		}()
	}
	return out
}

// Lists the buckets
func (f *FsStorage) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	if f.bucket == "" {
		// List the buckets
		go func() {
			defer close(out)
			if f.projectNumber == "" {
				fs.Stats.Error()
				fs.Log(f, "Can't list buckets without project number")
				return
			}
			listBuckets := f.svc.Buckets.List(f.projectNumber).MaxResults(listChunks)
			for {
				buckets, err := listBuckets.Do()
				if err != nil {
					fs.Stats.Error()
					fs.Log(f, "Couldn't list buckets: %v", err)
					break
				} else {
					for _, bucket := range buckets.Items {
						out <- &fs.Dir{
							Name:  bucket.Name,
							Bytes: 0,
							Count: 0,
						}
					}
				}
				if buckets.NextPageToken == "" {
					break
				}
				listBuckets.PageToken(buckets.NextPageToken)
			}
		}()
	} else {
		// List the directories in the path in the bucket
		go func() {
			defer close(out)
			f.list(true, func(remote string, object *storage.Object) {
				out <- &fs.Dir{
					Name:  remote,
					Bytes: int64(object.Size),
					Count: 0,
				}
			})
		}()
	}
	return out
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *FsStorage) Put(in io.Reader, remote string, modTime time.Time, size int64) (fs.Object, error) {
	// Temporary FsObject under construction
	fs := &FsObjectStorage{storage: f, remote: remote}
	return fs, fs.Update(in, modTime, size)
}

// Mkdir creates the bucket if it doesn't exist
func (f *FsStorage) Mkdir() error {
	_, err := f.svc.Buckets.Get(f.bucket).Do()
	if err == nil {
		// Bucket already exists
		return nil
	}

	if f.projectNumber == "" {
		return fmt.Errorf("Can't make bucket without project number")
	}

	//return f.svc.BucketCreate(f.bucket, nil)
	// FIXME is this public or private or what?????
	bucket := storage.Bucket{
		// FIXME other stuff here? ACL??
		Name: f.bucket,
	}
	_, err = f.svc.Buckets.Insert(f.projectNumber, &bucket).Do()
	return err
}

// Rmdir deletes the bucket
//
// Returns an error if it isn't empty: Error 409: The bucket you tried
// to delete was not empty.
func (f *FsStorage) Rmdir() error {
	return f.svc.Buckets.Delete(f.bucket).Do()
}

// Return the precision
func (fs *FsStorage) Precision() time.Duration {
	return time.Nanosecond
}

// ------------------------------------------------------------

// Return the parent Fs
func (o *FsObjectStorage) Fs() fs.Fs {
	return o.storage
}

// Return a string version
func (o *FsObjectStorage) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Return the remote path
func (o *FsObjectStorage) Remote() string {
	return o.remote
}

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (o *FsObjectStorage) Md5sum() (string, error) {
	return o.md5sum, nil
}

// Size returns the size of an object in bytes
func (o *FsObjectStorage) Size() int64 {
	return o.bytes
}

// setMetaData sets the fs data from a storage.Object
func (o *FsObjectStorage) setMetaData(info *storage.Object) {
	o.url = info.MediaLink
	o.bytes = int64(info.Size)

	// Read md5sum
	md5sumData, err := base64.StdEncoding.DecodeString(info.Md5Hash)
	if err != nil {
		fs.Log(o, "Bad MD5 decode: %v", err)
	} else {
		o.md5sum = hex.EncodeToString(md5sumData)
	}

	// read mtime out of metadata if available
	mtimeString, ok := info.Metadata[metaMtime]
	if ok {
		modTime, err := time.Parse(RFC3339In, mtimeString)
		if err == nil {
			o.modTime = modTime
			return
		} else {
			fs.Debug(o, "Failed to read mtime from metadata: %s", err)
		}
	}

	// Fallback to the Updated time
	modTime, err := time.Parse(RFC3339In, info.Updated)
	if err != nil {
		fs.Log(o, "Bad time decode: %v", err)
	} else {
		o.modTime = modTime
	}
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *FsObjectStorage) readMetaData() (err error) {
	if !o.modTime.IsZero() {
		return nil
	}
	object, err := o.storage.svc.Objects.Get(o.storage.bucket, o.storage.root+o.remote).Do()
	if err != nil {
		fs.Debug(o, "Failed to read info: %s", err)
		return err
	}
	o.setMetaData(object)
	return nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *FsObjectStorage) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		// fs.Log(o, "Failed to read metadata: %s", err)
		return time.Now()
	}
	return o.modTime
}

// Returns metadata for an object
func metadataFromModTime(modTime time.Time) map[string]string {
	metadata := make(map[string]string, 1)
	metadata[metaMtime] = modTime.Format(RFC3339Out)
	return metadata
}

// Sets the modification time of the local fs object
func (o *FsObjectStorage) SetModTime(modTime time.Time) {
	// This only adds metadata so will perserve other metadata
	object := storage.Object{
		Bucket:   o.storage.bucket,
		Name:     o.storage.root + o.remote,
		Metadata: metadataFromModTime(modTime),
	}
	_, err := o.storage.svc.Objects.Patch(o.storage.bucket, o.storage.root+o.remote, &object).Do()
	if err != nil {
		fs.Stats.Error()
		fs.Log(o, "Failed to update remote mtime: %s", err)
	}
}

// Is this object storable
func (o *FsObjectStorage) Storable() bool {
	return true
}

// Open an object for read
func (o *FsObjectStorage) Open() (in io.ReadCloser, err error) {
	req, _ := http.NewRequest("GET", o.url, nil)
	req.Header.Set("User-Agent", fs.UserAgent)
	res, err := o.storage.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("Bad response: %d: %s", res.StatusCode, res.Status)
	}
	return res.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *FsObjectStorage) Update(in io.Reader, modTime time.Time, size int64) error {
	// FIXME Set the mtime
	object := storage.Object{
		// FIXME other stuff here? ACL??
		Bucket: o.storage.bucket,
		Name:   o.storage.root + o.remote,
		// ContentType: ???
		// Crc32c: ??? // set this - will it get checked?
		// Md5Hash: will this get checked?
		Size:     uint64(size),
		Updated:  modTime.Format(RFC3339Out), // Doesn't get set
		Metadata: metadataFromModTime(modTime),
	}
	// FIXME ACL????
	_, err := o.storage.svc.Objects.Insert(o.storage.bucket, &object).Media(in).Name(object.Name).Do()
	// FIXME read back the MD5sum out of the returned object and check it?
	return err
}

// Remove an object
func (o *FsObjectStorage) Remove() error {
	return fmt.Errorf("FIXME - not implemented")
	//	return o.storage.svc.ObjectDelete(o.storage.bucket, o.storage.root+o.remote)
}

// Check the interfaces are satisfied
var _ fs.Fs = &FsStorage{}
var _ fs.Object = &FsObjectStorage{}
