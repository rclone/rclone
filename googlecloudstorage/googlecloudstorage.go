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
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/storage/v1"
)

const (
	rcloneClientID              = "202264815644.apps.googleusercontent.com"
	rcloneEncryptedClientSecret = "Uj7C9jGfb9gmeaV70Lh058cNkWvepr-Es9sBm0zdgil7JaOWF1VySw"
	timeFormatIn                = time.RFC3339
	timeFormatOut               = "2006-01-02T15:04:05.000000000Z07:00"
	metaMtime                   = "mtime" // key to store mtime under in metadata
	listChunks                  = 256     // chunk size to read directory listings
)

var (
	// Description of how to auth for this app
	storageConfig = &oauth2.Config{
		Scopes:       []string{storage.DevstorageFullControlScope},
		Endpoint:     google.Endpoint,
		ClientID:     rcloneClientID,
		ClientSecret: fs.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.TitleBarRedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "google cloud storage",
		Description: "Google Cloud Storage (this is not Google Drive)",
		NewFs:       NewFs,
		Config: func(name string) {
			if fs.ConfigFileGet(name, "service_account_file") != "" {
				return
			}
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
			Name: "service_account_file",
			Help: "Service Account Credentials JSON file path - needed only if you want use SA instead of interactive login.",
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
	root          string           // the path we are working on if any
	features      *fs.Features     // optional features
	svc           *storage.Service // the connection to the storage server
	client        *http.Client     // authorized client
	bucket        string           // the bucket we are working on
	projectNumber string           // used for finding buckets
	objectACL     string           // used when creating new objects
	bucketACL     string           // used when creating new buckets
}

// Object describes a storage object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs       *Fs       // what this object is part of
	remote   string    // The remote path
	url      string    // download path
	md5sum   string    // The MD5Sum of the object
	bytes    int64     // Bytes in the object
	modTime  time.Time // Modified time of the object
	mimeType string
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

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Pattern to match a storage path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parseParse parses a storage 'url'
func parsePath(path string) (bucket, directory string, err error) {
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = errors.Errorf("couldn't find bucket in storage path %q", path)
	} else {
		bucket, directory = parts[1], parts[2]
		directory = strings.Trim(directory, "/")
	}
	return
}

func getServiceAccountClient(keyJsonfilePath string) (*http.Client, error) {
	data, err := ioutil.ReadFile(os.ExpandEnv(keyJsonfilePath))
	if err != nil {
		return nil, errors.Wrap(err, "error opening credentials file")
	}
	conf, err := google.JWTConfigFromJSON(data, storageConfig.Scopes...)
	if err != nil {
		return nil, errors.Wrap(err, "error processing credentials")
	}
	ctxWithSpecialClient := oauthutil.Context()
	return oauth2.NewClient(ctxWithSpecialClient, conf.TokenSource(ctxWithSpecialClient)), nil
}

// NewFs contstructs an Fs from the path, bucket:path
func NewFs(name, root string) (fs.Fs, error) {
	var oAuthClient *http.Client
	var err error

	serviceAccountPath := fs.ConfigFileGet(name, "service_account_file")
	if serviceAccountPath != "" {
		oAuthClient, err = getServiceAccountClient(serviceAccountPath)
		if err != nil {
			log.Fatalf("Failed configuring Google Cloud Storage Service Account: %v", err)
		}
	} else {
		oAuthClient, _, err = oauthutil.NewClient(name, storageConfig)
		if err != nil {
			log.Fatalf("Failed to configure Google Cloud Storage: %v", err)
		}
	}

	bucket, directory, err := parsePath(root)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:          name,
		bucket:        bucket,
		root:          directory,
		projectNumber: fs.ConfigFileGet(name, "project_number"),
		objectACL:     fs.ConfigFileGet(name, "object_acl"),
		bucketACL:     fs.ConfigFileGet(name, "bucket_acl"),
	}
	f.features = (&fs.Features{ReadMimeType: true, WriteMimeType: true}).Fill(f)
	if f.objectACL == "" {
		f.objectACL = "private"
	}
	if f.bucketACL == "" {
		f.bucketACL = "private"
	}

	// Create a new authorized Drive client.
	f.client = oAuthClient
	f.svc, err = storage.New(f.client)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create Google Cloud Storage client")
	}

	if f.root != "" {
		f.root += "/"
		// Check to see if the object exists
		_, err = f.svc.Objects.Get(bucket, directory).Do()
		if err == nil {
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

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *storage.Object) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		o.setMetaData(info)
	} else {
		err := o.readMetaData() // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// listFn is called from list to handle an object.
type listFn func(remote string, object *storage.Object, isDirectory bool) error

// list the objects into the function supplied
//
// dir is the starting directory, "" for root
//
// If directories is set it only sends directories
func (f *Fs) list(dir string, level int, fn listFn) error {
	root := f.root
	rootLength := len(root)
	if dir != "" {
		root += dir + "/"
	}
	list := f.svc.Objects.List(f.bucket).Prefix(root).MaxResults(listChunks)
	switch level {
	case 1:
		list = list.Delimiter("/")
	case fs.MaxLevel:
	default:
		return fs.ErrorLevelNotSupported
	}
	for {
		objects, err := list.Do()
		if err != nil {
			return err
		}
		if level == 1 {
			var object storage.Object
			for _, prefix := range objects.Prefixes {
				if !strings.HasSuffix(prefix, "/") {
					continue
				}
				err = fn(prefix[:len(prefix)-1], &object, true)
				if err != nil {
					return err
				}
			}
		}
		for _, object := range objects.Items {
			if !strings.HasPrefix(object.Name, root) {
				fs.Log(f, "Odd name received %q", object.Name)
				continue
			}
			remote := object.Name[rootLength:]
			err = fn(remote, object, false)
			if err != nil {
				return err
			}
		}
		if objects.NextPageToken == "" {
			break
		}
		list.PageToken(objects.NextPageToken)
	}
	return nil
}

// listFiles lists files and directories to out
func (f *Fs) listFiles(out fs.ListOpts, dir string) {
	defer out.Finished()
	if f.bucket == "" {
		out.SetError(errors.New("can't list objects at root - choose a bucket using lsd"))
		return
	}
	// List the objects
	err := f.list(dir, out.Level(), func(remote string, object *storage.Object, isDirectory bool) error {
		if isDirectory {
			dir := &fs.Dir{
				Name:  remote,
				Bytes: int64(object.Size),
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
			if out.Add(o) {
				return fs.ErrorListAborted
			}
		}
		return nil
	})
	if err != nil {
		if gErr, ok := err.(*googleapi.Error); ok {
			if gErr.Code == http.StatusNotFound {
				err = fs.ErrorDirNotFound
			}
		}
		out.SetError(err)
	}
}

// listBuckets lists the buckets to out
func (f *Fs) listBuckets(out fs.ListOpts, dir string) {
	defer out.Finished()
	if dir != "" {
		out.SetError(fs.ErrorListOnlyRoot)
		return
	}
	if f.projectNumber == "" {
		out.SetError(errors.New("can't list buckets without project number"))
		return
	}
	listBuckets := f.svc.Buckets.List(f.projectNumber).MaxResults(listChunks)
	for {
		buckets, err := listBuckets.Do()
		if err != nil {
			out.SetError(err)
			return
		}
		for _, bucket := range buckets.Items {
			dir := &fs.Dir{
				Name:  bucket.Name,
				Bytes: 0,
				Count: 0,
			}
			if out.AddDir(dir) {
				return
			}
		}
		if buckets.NextPageToken == "" {
			break
		}
		listBuckets.PageToken(buckets.NextPageToken)
	}
}

// List lists the path to out
func (f *Fs) List(out fs.ListOpts, dir string) {
	if f.bucket == "" {
		f.listBuckets(out, dir)
	} else {
		f.listFiles(out, dir)
	}
	return
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return o, o.Update(in, src)
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	// Can't create subdirs
	if dir != "" {
		return nil
	}
	_, err := f.svc.Buckets.Get(f.bucket).Do()
	if err == nil {
		// Bucket already exists
		return nil
	}

	if f.projectNumber == "" {
		return errors.New("can't make bucket without project number")
	}

	bucket := storage.Bucket{
		Name: f.bucket,
	}
	_, err = f.svc.Buckets.Insert(f.projectNumber, &bucket).PredefinedAcl(f.bucketACL).Do()
	return err
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty: Error 409: The bucket you tried
// to delete was not empty.
func (f *Fs) Rmdir(dir string) error {
	if f.root != "" || dir != "" {
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
	o.mimeType = info.ContentType

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
		if gErr, ok := err.(*googleapi.Error); ok {
			if gErr.Code == http.StatusNotFound {
				return fs.ErrorObjectNotFound
			}
		}
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
		// fs.Log(o, "Failed to read metadata: %v", err)
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
func (o *Object) SetModTime(modTime time.Time) error {
	// This only adds metadata so will perserve other metadata
	object := storage.Object{
		Bucket:   o.fs.bucket,
		Name:     o.fs.root + o.remote,
		Metadata: metadataFromModTime(modTime),
	}
	newObject, err := o.fs.svc.Objects.Patch(o.fs.bucket, o.fs.root+o.remote, &object).Do()
	if err != nil {
		return err
	}
	o.setMetaData(newObject)
	return nil
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	req, err := http.NewRequest("GET", o.url, nil)
	if err != nil {
		return nil, err
	}
	fs.OpenOptionAddHTTPHeaders(req.Header, options)
	res, err := o.fs.client.Do(req)
	if err != nil {
		return nil, err
	}
	_, isRanging := req.Header["Range"]
	if !(res.StatusCode == http.StatusOK || (isRanging && res.StatusCode == http.StatusPartialContent)) {
		_ = res.Body.Close() // ignore error
		return nil, errors.Errorf("bad response: %d: %s", res.StatusCode, res.Status)
	}
	return res.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	size := src.Size()
	modTime := src.ModTime()

	object := storage.Object{
		Bucket:      o.fs.bucket,
		Name:        o.fs.root + o.remote,
		ContentType: fs.MimeType(src),
		Size:        uint64(size),
		Updated:     modTime.Format(timeFormatOut), // Doesn't get set
		Metadata:    metadataFromModTime(modTime),
	}
	newObject, err := o.fs.svc.Objects.Insert(o.fs.bucket, &object).Media(in, googleapi.ContentType("")).Name(object.Name).PredefinedAcl(o.fs.objectACL).Do()
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

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = &Fs{}
	_ fs.Copier    = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.MimeTyper = &Object{}
)
