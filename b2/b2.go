// Package b2 provides an interface to the Backblaze B2 object storage system
package b2

// FIXME if b2 could set the mod time then it has everything else to
// implement mod times.  It is just missing that bit of API.

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/b2/api"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/rest"
)

const (
	defaultEndpoint = "https://api.backblaze.com"
	headerPrefix    = "x-bz-info-" // lower case as that is what the server returns
	timeKey         = "src_last_modified_millis"
	timeHeader      = headerPrefix + timeKey
	sha1Header      = "X-Bz-Content-Sha1"
)

// Register with Fs
func init() {
	fs.Register(&fs.Info{
		Name:  "b2",
		NewFs: NewFs,
		Options: []fs.Option{{
			Name: "account",
			Help: "Account ID",
		}, {
			Name: "key",
			Help: "Application Key",
		}, {
			Name: "endpoint",
			Help: "Endpoint for the service - leave blank normally.",
		},
		},
	})
}

// Fs represents a remote b2 server
type Fs struct {
	name          string                       // name of this remote
	srv           *rest.Client                 // the connection to the b2 server
	bucket        string                       // the bucket we are working on
	bucketIDMutex sync.Mutex                   // mutex to protect _bucketID
	_bucketID     string                       // the ID of the bucket we are working on
	root          string                       // the path we are working on if any
	info          api.AuthorizeAccountResponse // result of authorize call
	uploadMu      sync.Mutex                   // lock for upload variable
	upload        api.GetUploadURLResponse     // result of get upload URL call
}

// Object describes a b2 object
//
// Will definitely have info
type Object struct {
	fs      *Fs       // what this object is part of
	remote  string    // The remote path
	info    api.File  // Info from the b2 object if known
	modTime time.Time // The modified time of the object if known
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
		return fmt.Sprintf("B2 bucket %s", f.bucket)
	}
	return fmt.Sprintf("B2 bucket %s path %s", f.bucket, f.root)
}

// Pattern to match a b2 path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parseParse parses a b2 'url'
func parsePath(path string) (bucket, directory string, err error) {
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = fmt.Errorf("Couldn't find bucket in b2 path %q", path)
	} else {
		bucket, directory = parts[1], parts[2]
		directory = strings.Trim(directory, "/")
	}
	return
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debug(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.Code == "" {
		errResponse.Code = "unknown"
	}
	if errResponse.Status == 0 {
		errResponse.Status = resp.StatusCode
	}
	if errResponse.Message == "" {
		errResponse.Message = "Unknown " + resp.Status
	}
	return errResponse
}

// NewFs contstructs an Fs from the path, bucket:path
func NewFs(name, root string) (fs.Fs, error) {
	bucket, directory, err := parsePath(root)
	if err != nil {
		return nil, err
	}
	f := &Fs{
		name:   name,
		bucket: bucket,
		root:   directory,
	}

	account := fs.ConfigFile.MustValue(name, "account")
	if account == "" {
		return nil, errors.New("account not found")
	}
	key := fs.ConfigFile.MustValue(name, "key")
	if key == "" {
		return nil, errors.New("key not found")
	}
	endpoint := fs.ConfigFile.MustValue(name, "endpoint", defaultEndpoint)

	f.srv = rest.NewClient(fs.Config.Client()).SetRoot(endpoint + "/b2api/v1").SetErrorHandler(errorHandler)

	opts := rest.Opts{
		Method:   "GET",
		Path:     "/b2_authorize_account",
		UserName: account,
		Password: key,
	}
	_, err = f.srv.CallJSON(&opts, nil, &f.info)
	if err != nil {
		return nil, fmt.Errorf("Failed to authenticate: %v", err)
	}
	f.srv.SetRoot(f.info.APIURL+"/b2api/v1").SetHeader("Authorization", f.info.AuthorizationToken)

	if f.root != "" {
		f.root += "/"
		// Check to see if the (bucket,directory) is actually an existing file
		oldRoot := f.root
		remote := path.Base(directory)
		f.root = path.Dir(directory)
		if f.root == "." {
			f.root = ""
		} else {
			f.root += "/"
		}
		obj := f.NewFsObject(remote)
		if obj != nil {
			return fs.NewLimited(f, obj), nil
		}
		f.root = oldRoot
	}
	return f, nil
}

// getUploadURL returns the UploadURL and the AuthorizationToken
func (f *Fs) getUploadURL() (string, string, error) {
	f.uploadMu.Lock()
	defer f.uploadMu.Unlock()
	bucketID, err := f.getBucketID()
	if err != nil {
		return "", "", err
	}
	if f.upload.UploadURL == "" || f.upload.AuthorizationToken == "" {
		opts := rest.Opts{
			Method: "POST",
			Path:   "/b2_get_upload_url",
		}
		var request = api.GetUploadURLRequest{
			BucketID: bucketID,
		}
		_, err := f.srv.CallJSON(&opts, &request, &f.upload)
		if err != nil {
			return "", "", fmt.Errorf("Failed to get upload URL: %v", err)
		}
	}
	return f.upload.UploadURL, f.upload.AuthorizationToken, nil
}

// clearUploadURL clears the current UploadURL and the AuthorizationToken
func (f *Fs) clearUploadURL() {
	f.uploadMu.Lock()
	f.upload = api.GetUploadURLResponse{}
	defer f.uploadMu.Unlock()
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *Fs) newFsObjectWithInfo(remote string, info *api.File) fs.Object {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		// Set info but not headers
		o.info = *info
	} else {
		err := o.readMetaData() // reads info and headers, returning an error
		if err != nil {
			fs.Debug(o, "Failed to read metadata: %s", err)
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

// listFn is called from list to handle an object
type listFn func(string, *api.File) error

// list lists the objects into the function supplied from
// the bucket and root supplied
//
// If prefix is set then startFileName is used as a prefix which all
// files must have
//
// If limit is > 0 then it limits to that many files (must be less
// than 1000)
//
// If hidden is set then it will list the hidden (deleted) files too.
func (f *Fs) list(prefix string, limit int, hidden bool, fn listFn) error {
	bucketID, err := f.getBucketID()
	if err != nil {
		return err
	}
	chunkSize := 1000
	if limit > 0 {
		chunkSize = limit
	}
	var request = api.ListFileNamesRequest{
		BucketID:     bucketID,
		MaxFileCount: chunkSize,
	}
	prefix = f.root + prefix
	if prefix != "" {
		request.StartFileName = prefix
	}
	var response api.ListFileNamesResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_list_file_names",
	}
	if hidden {
		opts.Path = "/b2_list_file_versions"
	}
	for {
		_, err = f.srv.CallJSON(&opts, &request, &response)
		if err != nil {
			return err
		}
		for i := range response.Files {
			file := &response.Files[i]
			// Finish if file name no longer has prefix
			if !strings.HasPrefix(file.Name, prefix) {
				return nil
			}
			err = fn(file.Name[len(prefix):], file)
			if err != nil {
				return err
			}
		}
		// end if no NextFileName
		if response.NextFileName == nil {
			break
		}
		request.StartFileName = *response.NextFileName
		if response.NextFileID != nil {
			request.StartFileID = *response.NextFileID
		}
	}
	return nil
}

// List walks the path returning a channel of FsObjects
func (f *Fs) List(out fs.ListOpts) {
	defer out.Finished()
	if f.bucket == "" {
		out.SetError(fmt.Errorf("Can't list objects at root - choose a bucket using lsd"))
		// Return no objects at top level list
		fs.Stats.Error()
		return
	}
	// List the objects
	err := f.list("", 0, false, func(remote string, object *api.File) error {
		if o := f.newFsObjectWithInfo(remote, object); o != nil {
			if out.Add(o) {
				return fs.ErrListAborted
			}
		}
		return nil
	})
	if err != nil {
		out.SetError(err)
		fs.Stats.Error()
		fs.ErrorLog(f, "Couldn't list bucket %q: %s", f.bucket, err)
	}

}

// listBucketFn is called from listBuckets to handle a bucket
type listBucketFn func(*api.Bucket)

// listBuckets lists the buckets to the function supplied
func (f *Fs) listBuckets(fn listBucketFn) error {
	var account = api.Account{ID: f.info.AccountID}
	var response api.ListBucketsResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_list_buckets",
	}
	_, err := f.srv.CallJSON(&opts, &account, &response)
	if err != nil {
		return err
	}
	for i := range response.Buckets {
		fn(&response.Buckets[i])
	}
	return nil
}

// getBucketID finds the ID for the current bucket name
func (f *Fs) getBucketID() (bucketID string, err error) {
	f.bucketIDMutex.Lock()
	defer f.bucketIDMutex.Unlock()
	if f._bucketID != "" {
		return f._bucketID, nil
	}
	err = f.listBuckets(func(bucket *api.Bucket) {
		if bucket.Name == f.bucket {
			bucketID = bucket.ID
		}
	})
	if bucketID == "" {
		err = fmt.Errorf("Couldn't find bucket %q", f.bucket)
	}
	f._bucketID = bucketID
	return bucketID, err
}

// setBucketID sets the ID for the current bucket name
func (f *Fs) setBucketID(ID string) {
	f.bucketIDMutex.Lock()
	f._bucketID = ID
	f.bucketIDMutex.Unlock()
}

// clearBucketID clears the ID for the current bucket name
func (f *Fs) clearBucketID() {
	f.bucketIDMutex.Lock()
	f._bucketID = ""
	f.bucketIDMutex.Unlock()
}

// ListDir lists the buckets
func (f *Fs) ListDir(out fs.ListDirOpts) {
	defer out.Finished()
	if f.bucket == "" {
		err := f.listBuckets(func(bucket *api.Bucket) {
			out <- &fs.Dir{
				Name:  bucket.Name,
				Bytes: -1,
				Count: -1,
			}
		})
		if err != nil {
			out.SetError(err)
			fs.Stats.Error()
			fs.ErrorLog(f, "Error listing buckets: %v", err)
		}
	} else {
		lastDir := ""
		err := f.list("", 0, false, func(remote string, object *api.File) error {
			slash := strings.IndexRune(remote, '/')
			if slash < 0 {
				return nil
			}
			dir := remote[:slash]
			if dir == lastDir {
				return nil
			}
			if out.Add(&fs.Dir{
				Name:  dir,
				Bytes: -1,
				Count: -1,
			}) {
				return fs.ErrListAborted
			}
			lastDir = dir
			return nil
		})
		if err != nil {
			out.SetError(err)
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't list bucket %q: %s", f.bucket, err)
		}
	}
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, remote string, modTime time.Time, size int64) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: remote,
	}
	return fs, fs.Update(in, modTime, size)
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir() error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_create_bucket",
	}
	var request = api.CreateBucketRequest{
		AccountID: f.info.AccountID,
		Name:      f.bucket,
		Type:      "allPrivate",
	}
	var response api.Bucket
	_, err := f.srv.CallJSON(&opts, &request, &response)
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			if apiErr.Code == "duplicate_bucket_name" {
				return nil
			}
		}
		return fmt.Errorf("Failed to create bucket: %v", err)
	}
	f.setBucketID(response.ID)
	return nil
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir() error {
	if f.root != "" {
		return nil
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_delete_bucket",
	}
	bucketID, err := f.getBucketID()
	if err != nil {
		return err
	}
	var request = api.DeleteBucketRequest{
		ID:        bucketID,
		AccountID: f.info.AccountID,
	}
	var response api.Bucket
	_, err = f.srv.CallJSON(&opts, &request, &response)
	if err != nil {
		return fmt.Errorf("Failed to delete bucket: %v", err)
	}
	f.clearBucketID()
	f.clearUploadURL()
	return nil
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// deleteByID deletes a file version given Name and ID
func (f *Fs) deleteByID(ID, Name string) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_delete_file_version",
	}
	var request = api.DeleteFileRequest{
		ID:   ID,
		Name: Name,
	}
	var response api.File
	_, err := f.srv.CallJSON(&opts, &request, &response)
	if err != nil {
		return fmt.Errorf("Failed to delete %q: %v", Name, err)
	}
	return nil
}

// Purge deletes all the files and directories
//
// Implemented here so we can make sure we delete old versions.
func (f *Fs) Purge() error {
	var errReturn error
	var checkErrMutex sync.Mutex
	var checkErr = func(err error) {
		if err == nil {
			return
		}
		checkErrMutex.Lock()
		defer checkErrMutex.Unlock()
		fs.Stats.Error()
		fs.ErrorLog(f, "Purge error: %v", err)
		if errReturn == nil {
			errReturn = err
		}
	}

	// Delete Config.Transfers in parallel
	toBeDeleted := make(chan *api.File, fs.Config.Transfers)
	var wg sync.WaitGroup
	wg.Add(fs.Config.Transfers)
	for i := 0; i < fs.Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for object := range toBeDeleted {
				checkErr(f.deleteByID(object.ID, object.Name))
			}
		}()
	}
	checkErr(f.list("", 0, true, func(remote string, object *api.File) error {
		fs.Debug(remote, "Deleting (id %q)", object.ID)
		toBeDeleted <- object
		return nil
	}))
	close(toBeDeleted)
	wg.Wait()

	checkErr(f.Rmdir())
	return errReturn
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

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Md5sum() (string, error) {
	return "", nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.info.Size
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	if o.info.ID != "" {
		return nil
	}
	err = o.fs.list(o.remote, 1, false, func(remote string, object *api.File) error {
		if remote == "" {
			o.info = *object
		}
		return nil
	})
	if o.info.ID != "" {
		return nil
	}
	return fmt.Errorf("Object %q not found", o.remote)
}

// timeString returns modTime as the number of milliseconds
// elapsed since January 1, 1970 UTC as a decimal string.
func timeString(modTime time.Time) string {
	return strconv.FormatInt(modTime.UnixNano()/1E6, 10)
}

// parseTimeString converts a decimal string number of milliseconds
// elapsed since January 1, 1970 UTC into a time.Time
func parseTimeString(timeString string) (result time.Time, err error) {
	if timeString == "" {
		return result, fmt.Errorf("%q not found in metadata", timeKey)
	}
	unixMilliseconds, err := strconv.ParseInt(timeString, 10, 64)
	if err != nil {
		return result, err
	}
	return time.Unix(unixMilliseconds/1E3, (unixMilliseconds%1E3)*1E6).UTC(), nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() (result time.Time) {
	if !o.modTime.IsZero() {
		return o.modTime
	}

	// Return the current time if can't read metadata
	result = time.Now()

	// Read metadata (need ID)
	err := o.readMetaData()
	if err != nil {
		fs.Debug(o, "Failed to read metadata: %v", err)
		return result
	}

	// Return the UploadTimestamp if can't get file info
	result = time.Time(o.info.UploadTimestamp)

	// Now read the metadata for the modified time
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_get_file_info",
	}
	var request = api.GetFileInfoRequest{
		ID: o.info.ID,
	}
	var response api.FileInfo
	_, err = o.fs.srv.CallJSON(&opts, &request, &response)
	if err != nil {
		fs.Debug(o, "Failed to get file info: %v", err)
		return result
	}

	// Parse the result
	timeString := response.Info[timeKey]
	parsed, err := parseTimeString(timeString)
	if err != nil {
		fs.Debug(o, "Failed to parse mod time string %q: %v", timeString, err)
		return result
	}
	return parsed
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) {
	// Not possible with B2
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// openFile represents an Object open for reading
type openFile struct {
	o     *Object        // Object we are reading for
	resp  *http.Response // response of the GET
	body  io.Reader      // reading from here
	hash  hash.Hash      // currently accumulating SHA1
	bytes int64          // number of bytes read on this connection
	eof   bool           // whether we have read end of file
}

// newOpenFile wraps an io.ReadCloser and checks the sha1sum
func newOpenFile(o *Object, resp *http.Response) *openFile {
	file := &openFile{
		o:    o,
		resp: resp,
		hash: sha1.New(),
	}
	file.body = io.TeeReader(resp.Body, file.hash)
	return file
}

// Read bytes from the object - see io.Reader
func (file *openFile) Read(p []byte) (n int, err error) {
	n, err = file.body.Read(p)
	file.bytes += int64(n)
	if err == io.EOF {
		file.eof = true
	}
	return
}

// Close the object and checks the length and SHA1 if all the object
// was read
func (file *openFile) Close() (err error) {
	// Close the body at the end
	defer fs.CheckClose(file.resp.Body, &err)

	// If not end of file then can't check SHA1
	if !file.eof {
		return nil
	}

	// Check to see we read the correct number of bytes
	if file.o.Size() != file.bytes {
		return fmt.Errorf("Object corrupted on transfer - length mismatch (want %d got %d)", file.o.Size(), file.bytes)
	}

	// Check the SHA1
	receivedSHA1 := file.resp.Header.Get(sha1Header)
	calculatedSHA1 := fmt.Sprintf("%x", file.hash.Sum(nil))
	if receivedSHA1 != calculatedSHA1 {
		return fmt.Errorf("Object corrupted on transfer - SHA1 mismatch (want %q got %q)", receivedSHA1, calculatedSHA1)
	}

	return nil
}

// Check it satisfies the interfaces
var _ io.ReadCloser = &openFile{}

// Open an object for read
func (o *Object) Open() (in io.ReadCloser, err error) {
	opts := rest.Opts{
		Method:   "GET",
		Absolute: true,
		Path:     o.fs.info.DownloadURL + "/file/" + urlEncode(o.fs.bucket) + "/" + urlEncode(o.fs.root+o.remote),
	}
	resp, err := o.fs.srv.Call(&opts)
	if err != nil {
		return nil, fmt.Errorf("Failed to open for download: %v", err)
	}

	// Parse the time out of the headers if possible
	timeString := resp.Header.Get(timeHeader)
	parsed, err := parseTimeString(timeString)
	if err != nil {
		fs.Debug(o, "Failed to parse mod time string %q: %v", timeString, err)
	} else {
		o.modTime = parsed
	}
	return newOpenFile(o, resp), nil
}

// dontEncode is the characters that do not need percent-encoding
//
// The characters that do not need percent-encoding are a subset of
// the printable ASCII characters: upper-case letters, lower-case
// letters, digits, ".", "_", "-", "/", "~", "!", "$", "'", "(", ")",
// "*", ";", "=", ":", and "@". All other byte values in a UTF-8 must
// be replaced with "%" and the two-digit hex value of the byte.
const dontEncode = (`abcdefghijklmnopqrstuvwxyz` +
	`ABCDEFGHIJKLMNOPQRSTUVWXYZ` +
	`0123456789` +
	`._-/~!$'()*;=:@`)

// noNeedToEncode is a bitmap of characters which don't need % encoding
var noNeedToEncode [256]bool

func init() {
	for _, c := range dontEncode {
		noNeedToEncode[c] = true
	}
}

// urlEncode encodes in with % encoding
func urlEncode(in string) string {
	var out bytes.Buffer
	for i := 0; i < len(in); i++ {
		c := in[i]
		if noNeedToEncode[c] {
			_ = out.WriteByte(c)
		} else {
			_, _ = out.WriteString(fmt.Sprintf("%%%2X", c))
		}
	}
	return out.String()
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, modTime time.Time, size int64) (err error) {
	// Open a temp file to copy the input
	fd, err := ioutil.TempFile("", "rclone-b2-")
	if err != nil {
		return err
	}
	_ = os.Remove(fd.Name()) // Delete the file - may not work on Windows
	defer func() {
		_ = fd.Close()           // Ignore error may have been closed already
		_ = os.Remove(fd.Name()) // Delete the file - may have been deleted already
	}()

	// Copy the input while calculating the sha1
	hash := sha1.New()
	teed := io.TeeReader(in, hash)
	n, err := io.Copy(fd, teed)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("Read %d bytes expecting %d", n, size)
	}
	calculatedSha1 := fmt.Sprintf("%x", hash.Sum(nil))

	// Rewind the temporary file
	_, err = fd.Seek(0, 0)
	if err != nil {
		return err
	}

	// Get upload URL
	UploadURL, AuthorizationToken, err := o.fs.getUploadURL()
	if err != nil {
		return err
	}

	// Headers for upload file
	//
	// Authorization
	// required
	// An upload authorization token, from b2_get_upload_url.
	//
	// X-Bz-File-Name
	// required
	//
	// The name of the file, in percent-encoded UTF-8. See Files for requirements on file names. See String Encoding.
	//
	// Content-Type
	// required
	//
	// The MIME type of the content of the file, which will be returned in
	// the Content-Type header when downloading the file. Use the
	// Content-Type b2/x-auto to automatically set the stored Content-Type
	// post upload. In the case where a file extension is absent or the
	// lookup fails, the Content-Type is set to application/octet-stream. The
	// Content-Type mappings can be purused here.
	//
	// X-Bz-Content-Sha1
	// required
	//
	// The SHA1 checksum of the content of the file. B2 will check this when
	// the file is uploaded, to make sure that the file arrived correctly. It
	// will be returned in the X-Bz-Content-Sha1 header when the file is
	// downloaded.
	//
	// X-Bz-Info-src_last_modified_millis
	// optional
	//
	// If the original source of the file being uploaded has a last modified
	// time concept, Backblaze recommends using this spelling of one of your
	// ten X-Bz-Info-* headers (see below). Using a standard spelling allows
	// different B2 clients and the B2 web user interface to interoperate
	// correctly. The value should be a base 10 number which represents a UTC
	// time when the original source file was last modified. It is a base 10
	// number of milliseconds since midnight, January 1, 1970 UTC. This fits
	// in a 64 bit integer such as the type "long" in the programming
	// language Java. It is intended to be compatible with Java's time
	// long. For example, it can be passed directly into the Java call
	// Date.setTime(long time).
	//
	// X-Bz-Info-*
	// optional
	//
	// Up to 10 of these headers may be present. The * part of the header
	// name is replace with the name of a custom field in the file
	// information stored with the file, and the value is an arbitrary UTF-8
	// string, percent-encoded. The same info headers sent with the upload
	// will be returned with the download.

	opts := rest.Opts{
		Method:   "POST",
		Absolute: true,
		Path:     UploadURL,
		Body:     fd,
		ExtraHeaders: map[string]string{
			"Authorization":  AuthorizationToken,
			"X-Bz-File-Name": urlEncode(o.fs.root + o.remote),
			"Content-Type":   fs.MimeType(o),
			sha1Header:       calculatedSha1,
			timeHeader:       timeString(modTime),
		},
		ContentLength: &size,
	}
	var response api.FileInfo
	_, err = o.fs.srv.CallJSON(&opts, nil, &response)
	if err != nil {
		return fmt.Errorf("Failed to upload: %v", err)
	}
	o.info.ID = response.ID
	o.info.Name = response.Name
	o.info.Action = "upload"
	o.info.Size = response.Size
	o.info.UploadTimestamp = api.Timestamp(time.Now()) // FIXME not quite right
	return nil
}

// Remove an object
func (o *Object) Remove() error {
	bucketID, err := o.fs.getBucketID()
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_hide_file",
	}
	var request = api.HideFileRequest{
		BucketID: bucketID,
		Name:     o.info.Name,
	}
	var response api.File
	_, err = o.fs.srv.CallJSON(&opts, &request, &response)
	if err != nil {
		return fmt.Errorf("Failed to delete file: %v", err)
	}
	return nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = &Fs{}
	_ fs.Purger = &Fs{}
	_ fs.Object = &Object{}
)
