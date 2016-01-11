// Package googlecloudstorage provides an interface to Google Cloud Storage
package googlecloudstorage

/*
Notes

Can't set Updated but can set Metadata on object creation

Patch needs full_control not just read_write

FIXME Patch/Delete/Get isn't working with files with spaces in - giving 404 error
- https://code.google.com/p/google-api-go-client/issues/detail?id=64
*/

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/storage/v1"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
)

const (
	rcloneClientID     = "202264815644.apps.googleusercontent.com"
	rcloneClientSecret = "8p/yms3OlNXE9OTDl/HLypf9gdiJ5cT3"
	timeFormatIn       = time.RFC3339
	timeFormatOut      = "2006-01-02T15:04:05.000000000Z07:00"
	metaMtime          = "mtime" // key to store mtime under in metadata
	listChunks         = 256     // chunk size to read directory listings
)

var (
	// Description of how to auth for this app
	storageConfig = &oauth2.Config{
		Scopes:       []string{storage.DevstorageFullControlScope},
		Endpoint:     google.Endpoint,
		ClientID:     rcloneClientID,
		ClientSecret: fs.Reveal(rcloneClientSecret),
		RedirectURL:  oauthutil.TitleBarRedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.Info{
		Name:  "google cloud storage",
		NewFs: NewFs,
		Config: func(name string) {
			err := oauthutil.Config("google cloud storage", name, storageConfig)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: fs.ConfigClientID,
			Help: "Google Application Client Id - leave blank normally.",
		}, {
			Name: fs.ConfigClientSecret,
			Help: "Google Application Client Secret - leave blank normally.",
		}, {
			Name: "project_number",
			Help: "Project number optional - needed only for list/create/delete buckets - see your developer console.",
		}, {
			Name: "object_acl",
			Help: "Access Control List for new objects.",
			Examples: []fs.OptionExample{{
				Value: "authenticatedRead",
				Help:  "Object owner gets OWNER access, and all Authenticated Users get READER access.",
			}, {
				Value: "bucketOwnerFullControl",
				Help:  "Object owner gets OWNER access, and project team owners get OWNER access.",
			}, {
				Value: "bucketOwnerRead",
				Help:  "Object owner gets OWNER access, and project team owners get READER access.",
			}, {
				Value: "private",
				Help:  "Object owner gets OWNER access [default if left blank].",
			}, {
				Value: "projectPrivate",
				Help:  "Object owner gets OWNER access, and project team members get access according to their roles.",
			}, {
				Value: "publicRead",
				Help:  "Object owner gets OWNER access, and all Users get READER access.",
			}},
		}, {
			Name: "bucket_acl",
			Help: "Access Control List for new buckets.",
			Examples: []fs.OptionExample{{
				Value: "authenticatedRead",
				Help:  "Project team owners get OWNER access, and all Authenticated Users get READER access.",
			}, {
				Value: "private",
				Help:  "Project team owners get OWNER access [default if left blank].",
			}, {
				Value: "projectPrivate",
				Help:  "Project team members get access according to their roles.",
			}, {
				Value: "publicRead",
				Help:  "Project team owners get OWNER access, and all Users get READER access.",
			}, {
				Value: "publicReadWrite",
				Help:  "Project team owners get OWNER access, and all Users get WRITER access.",
			}},
		}},
	})
}

// Fs represents a remote storage server
type Fs struct {
	name          string           // name of this remote
	svc           *storage.Service // the connection to the storage server
	client        *http.Client     // authorized client
	bucket        string           // the bucket we are working on
	root          string           // the path we are working on if any
	projectNumber string           // used for finding buckets
	objectAcl     string           // used when creating new objects
	bucketAcl     string           // used when creating new buckets
}

// Object describes a storage object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs      *Fs       // what this object is part of
	remote  string    // The remote path
	url     string    // download path
	md5sum  string    // The MD5Sum of the object
	bytes   int64     // Bytes in the object
	modTime time.Time // Modified time of the object
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	if f.root == "" {
		return f.bucket
	}
	return f.bucket + "/" + f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
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

// NewFs contstructs an Fs from the path, bucket:path
func NewFs(name, root string) (fs.Fs, error) {
	oAuthClient, err := oauthutil.NewClient(name, storageConfig)
	if err != nil {
		log.Fatalf("Failed to configure Google Cloud Storage: %v", err)
	}

	bucket, directory, err := parsePath(root)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:          name,
		bucket:        bucket,
		root:          directory,
		projectNumber: fs.ConfigFile.MustValue(name, "project_number"),
		objectAcl:     fs.ConfigFile.MustValue(name, "object_acl"),
		bucketAcl:     fs.ConfigFile.MustValue(name, "bucket_acl"),
	}
	if f.objectAcl == "" {
		f.objectAcl = "private"
	}
	if f.bucketAcl == "" {
		f.bucketAcl = "private"
	}

	// Create a new authorized Drive client.
	f.client = oAuthClient
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
func (f *Fs) newFsObjectWithInfo(remote string, info *storage.Object) fs.Object {
	o := &Object{
		fs:     f,
		remote: remote,
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

// NewFsObject returns an FsObject from a path
//
// May return nil if an error occurred
func (f *Fs) NewFsObject(remote string) fs.Object {
	return f.newFsObjectWithInfo(remote, nil)
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
func (f *Fs) list(directories bool, fn func(string, *storage.Object)) {
	list := f.svc.Objects.List(f.bucket).Prefix(f.root).MaxResults(listChunks)
	if directories {
		list = list.Delimiter("/")
	}
	rootLength := len(f.root)
	for {
		objects, err := list.Do()
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't read bucket %q: %s", f.bucket, err)
			return
		}
		if !directories {
			for _, object := range objects.Items {
				if !strings.HasPrefix(object.Name, f.root) {
					fs.Log(f, "Odd name received %q", object.Name)
					continue
				}
				remote := object.Name[rootLength:]
				fn(remote, object)
			}
		} else {
			var object storage.Object
			for _, prefix := range objects.Prefixes {
				if !strings.HasSuffix(prefix, "/") {
					continue
				}
				fn(prefix[:len(prefix)-1], &object)
			}
		}
		if objects.NextPageToken == "" {
			break
		}
		list.PageToken(objects.NextPageToken)
	}
}

// List walks the path returning a channel of FsObjects
func (f *Fs) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
	if f.bucket == "" {
		// Return no objects at top level list
		close(out)
		fs.Stats.Error()
		fs.ErrorLog(f, "Can't list objects at root - choose a bucket using lsd")
	} else {
		// List the objects
		go func() {
			defer close(out)
			f.list(false, func(remote string, object *storage.Object) {
				if fs := f.newFsObjectWithInfo(remote, object); fs != nil {
					out <- fs
				}
			})
		}()
	}
	return out
}

// ListDir lists the buckets
func (f *Fs) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	if f.bucket == "" {
		// List the buckets
		go func() {
			defer close(out)
			if f.projectNumber == "" {
				fs.Stats.Error()
				fs.ErrorLog(f, "Can't list buckets without project number")
				return
			}
			listBuckets := f.svc.Buckets.List(f.projectNumber).MaxResults(listChunks)
			for {
				buckets, err := listBuckets.Do()
				if err != nil {
					fs.Stats.Error()
					fs.ErrorLog(f, "Couldn't list buckets: %v", err)
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
func (f *Fs) Put(in io.Reader, remote string, modTime time.Time, size int64) (fs.Object, error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}
	return o, o.Update(in, modTime, size)
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir() error {
	_, err := f.svc.Buckets.Get(f.bucket).Do()
	if err == nil {
		// Bucket already exists
		return nil
	}

	if f.projectNumber == "" {
		return fmt.Errorf("Can't make bucket without project number")
	}

	bucket := storage.Bucket{
		Name: f.bucket,
	}
	_, err = f.svc.Buckets.Insert(f.projectNumber, &bucket).PredefinedAcl(f.bucketAcl).Do()
	return err
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty: Error 409: The bucket you tried
// to delete was not empty.
func (f *Fs) Rmdir() error {
	if f.root != "" {
		return nil
	}
	return f.svc.Buckets.Delete(f.bucket).Do()
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
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
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debug(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	// Temporary Object under construction
	dstObj := &Object{
		fs:     f,
		remote: remote,
	}

	srcBucket := srcObj.fs.bucket
	srcObject := srcObj.fs.root + srcObj.remote
	dstBucket := f.bucket
	dstObject := f.root + remote
	newObject, err := f.svc.Objects.Copy(srcBucket, srcObject, dstBucket, dstObject, nil).Do()
	if err != nil {
		return nil, err
	}
	// Set the metadata for the new object while we have it
	dstObj.setMetaData(newObject)
	return dstObj, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Fs {
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
	return o.md5sum, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.bytes
}

// setMetaData sets the fs data from a storage.Object
func (o *Object) setMetaData(info *storage.Object) {
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
		modTime, err := time.Parse(timeFormatIn, mtimeString)
		if err == nil {
			o.modTime = modTime
			return
		}
		fs.Debug(o, "Failed to read mtime from metadata: %s", err)
	}

	// Fallback to the Updated time
	modTime, err := time.Parse(timeFormatIn, info.Updated)
	if err != nil {
		fs.Log(o, "Bad time decode: %v", err)
	} else {
		o.modTime = modTime
	}
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	if !o.modTime.IsZero() {
		return nil
	}
	object, err := o.fs.svc.Objects.Get(o.fs.bucket, o.fs.root+o.remote).Do()
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
func (o *Object) ModTime() time.Time {
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
	metadata[metaMtime] = modTime.Format(timeFormatOut)
	return metadata
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) {
	// This only adds metadata so will perserve other metadata
	object := storage.Object{
		Bucket:   o.fs.bucket,
		Name:     o.fs.root + o.remote,
		Metadata: metadataFromModTime(modTime),
	}
	newObject, err := o.fs.svc.Objects.Patch(o.fs.bucket, o.fs.root+o.remote, &object).Do()
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to update remote mtime: %s", err)
	}
	o.setMetaData(newObject)
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open() (in io.ReadCloser, err error) {
	// This is slightly complicated by Go here insisting on
	// decoding the %2F in URLs into / which is legal in http, but
	// unfortunately not what the storage server wants.
	//
	// So first encode all the % into their encoded form
	// URL will decode them giving our original escaped string
	url := strings.Replace(o.url, "%", "%25", -1)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// SetOpaque sets Opaque such that HTTP requests to it don't
	// alter any hex-escaped characters
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", fs.UserAgent)
	res, err := o.fs.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		_ = res.Body.Close() // ignore error
		return nil, fmt.Errorf("Bad response: %d: %s", res.StatusCode, res.Status)
	}
	return res.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, modTime time.Time, size int64) error {
	object := storage.Object{
		Bucket:      o.fs.bucket,
		Name:        o.fs.root + o.remote,
		ContentType: fs.MimeType(o),
		Size:        uint64(size),
		Updated:     modTime.Format(timeFormatOut), // Doesn't get set
		Metadata:    metadataFromModTime(modTime),
	}
	newObject, err := o.fs.svc.Objects.Insert(o.fs.bucket, &object).Media(in).Name(object.Name).PredefinedAcl(o.fs.objectAcl).Do()
	if err != nil {
		return err
	}
	// Set the metadata for the new object while we have it
	o.setMetaData(newObject)
	return nil
}

// Remove an object
func (o *Object) Remove() error {
	return o.fs.svc.Objects.Delete(o.fs.bucket, o.fs.root+o.remote).Do()
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = &Fs{}
	_ fs.Copier = &Fs{}
	_ fs.Object = &Object{}
)
