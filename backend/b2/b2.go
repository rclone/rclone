// Package b2 provides an interface to the Backblaze B2 object storage system
package b2

// FIXME should we remove sha1 checks from here as rclone now supports
// checking SHA1s?

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	gohash "hash"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/backend/b2/api"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
)

const (
	defaultEndpoint     = "https://api.backblazeb2.com"
	headerPrefix        = "x-bz-info-" // lower case as that is what the server returns
	timeKey             = "src_last_modified_millis"
	timeHeader          = headerPrefix + timeKey
	sha1Key             = "large_file_sha1"
	sha1Header          = "X-Bz-Content-Sha1"
	sha1InfoHeader      = headerPrefix + sha1Key
	testModeHeader      = "X-Bz-Test-Mode"
	retryAfterHeader    = "Retry-After"
	minSleep            = 10 * time.Millisecond
	maxSleep            = 5 * time.Minute
	decayConstant       = 1 // bigger for slower decay, exponential
	maxParts            = 10000
	maxVersions         = 100 // maximum number of versions we search in --b2-versions mode
	minChunkSize        = 5 * fs.MebiByte
	defaultChunkSize    = 96 * fs.MebiByte
	defaultUploadCutoff = 200 * fs.MebiByte
)

// Globals
var (
	errNotWithVersions = errors.New("can't modify or delete files in --b2-versions mode")
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "b2",
		Description: "Backblaze B2",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "account",
			Help:     "Account ID or Application Key ID",
			Required: true,
		}, {
			Name:     "key",
			Help:     "Application Key",
			Required: true,
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for the service.\nLeave blank normally.",
			Advanced: true,
		}, {
			Name: "test_mode",
			Help: `A flag string for X-Bz-Test-Mode header for debugging.

This is for debugging purposes only. Setting it to one of the strings
below will cause b2 to return specific errors:

  * "fail_some_uploads"
  * "expire_some_account_authorization_tokens"
  * "force_cap_exceeded"

These will be set in the "X-Bz-Test-Mode" header which is documented
in the [b2 integrations checklist](https://www.backblaze.com/b2/docs/integration_checklist.html).`,
			Default:  "",
			Hide:     fs.OptionHideConfigurator,
			Advanced: true,
		}, {
			Name:     "versions",
			Help:     "Include old versions in directory listings.\nNote that when using this no file write operations are permitted,\nso you can't upload files or delete them.",
			Default:  false,
			Advanced: true,
		}, {
			Name:    "hard_delete",
			Help:    "Permanently delete files on remote removal, otherwise hide files.",
			Default: false,
		}, {
			Name: "upload_cutoff",
			Help: `Cutoff for switching to chunked upload.

Files above this size will be uploaded in chunks of "--b2-chunk-size".

This value should be set no larger than 4.657GiB (== 5GB).`,
			Default:  fs.SizeSuffix(defaultUploadCutoff),
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Upload chunk size. Must fit in memory.

When uploading large files, chunk the file into this size.  Note that
these chunks are buffered in memory and there might a maximum of
"--transfers" chunks in progress at once.  5,000,000 Bytes is the
minimim size.`,
			Default:  fs.SizeSuffix(defaultChunkSize),
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Account      string        `config:"account"`
	Key          string        `config:"key"`
	Endpoint     string        `config:"endpoint"`
	TestMode     string        `config:"test_mode"`
	Versions     bool          `config:"versions"`
	HardDelete   bool          `config:"hard_delete"`
	UploadCutoff fs.SizeSuffix `config:"upload_cutoff"`
	ChunkSize    fs.SizeSuffix `config:"chunk_size"`
}

// Fs represents a remote b2 server
type Fs struct {
	name          string                       // name of this remote
	root          string                       // the path we are working on if any
	opt           Options                      // parsed config options
	features      *fs.Features                 // optional features
	srv           *rest.Client                 // the connection to the b2 server
	bucket        string                       // the bucket we are working on
	bucketOKMu    sync.Mutex                   // mutex to protect bucket OK
	bucketOK      bool                         // true if we have created the bucket
	bucketIDMutex sync.Mutex                   // mutex to protect _bucketID
	_bucketID     string                       // the ID of the bucket we are working on
	info          api.AuthorizeAccountResponse // result of authorize call
	uploadMu      sync.Mutex                   // lock for upload variable
	uploads       []*api.GetUploadURLResponse  // result of get upload URL calls
	authMu        sync.Mutex                   // lock for authorizing the account
	pacer         *pacer.Pacer                 // To pace and retry the API calls
	bufferTokens  chan []byte                  // control concurrency of multipart uploads
}

// Object describes a b2 object
type Object struct {
	fs       *Fs       // what this object is part of
	remote   string    // The remote path
	id       string    // b2 id of the file
	modTime  time.Time // The modified time of the object if known
	sha1     string    // SHA-1 hash if known
	size     int64     // Size of the object
	mimeType string    // Content-Type of the object
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

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Pattern to match a b2 path
var matcher = regexp.MustCompile(`^/*([^/]*)(.*)$`)

// parseParse parses a b2 'url'
func parsePath(path string) (bucket, directory string, err error) {
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = errors.Errorf("couldn't find bucket in b2 path %q", path)
	} else {
		bucket, directory = parts[1], parts[2]
		directory = strings.Trim(directory, "/")
	}
	return
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	401, // Unauthorized (eg "Token has expired")
	408, // Request Timeout
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	503, // Service Unavailable
	504, // Gateway Time-out
}

// shouldRetryNoAuth returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetryNoReauth(resp *http.Response, err error) (bool, error) {
	// For 429 or 503 errors look at the Retry-After: header and
	// set the retry appropriately, starting with a minimum of 1
	// second if it isn't set.
	if resp != nil && (resp.StatusCode == 429 || resp.StatusCode == 503) {
		var retryAfter = 1
		retryAfterString := resp.Header.Get(retryAfterHeader)
		if retryAfterString != "" {
			var err error
			retryAfter, err = strconv.Atoi(retryAfterString)
			if err != nil {
				fs.Errorf(f, "Malformed %s header %q: %v", retryAfterHeader, retryAfterString, err)
			}
		}
		retryAfterDuration := time.Duration(retryAfter) * time.Second
		if f.pacer.GetSleep() < retryAfterDuration {
			fs.Debugf(f, "Setting sleep to %v after error: %v", retryAfterDuration, err)
			// We set 1/2 the value here because the pacer will double it immediately
			f.pacer.SetSleep(retryAfterDuration / 2)
		}
		return true, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(resp *http.Response, err error) (bool, error) {
	if resp != nil && resp.StatusCode == 401 {
		fs.Debugf(f, "Unauthorized: %v", err)
		// Reauth
		authErr := f.authorizeAccount()
		if authErr != nil {
			err = authErr
		}
		return true, err
	}
	return f.shouldRetryNoReauth(resp, err)
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
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

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	if cs < minChunkSize {
		return errors.Errorf("%s is less than %s", cs, minChunkSize)
	}
	return nil
}

func (f *Fs) setUploadChunkSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.ChunkSize = f.opt.ChunkSize, cs
		f.fillBufferTokens() // reset the buffer tokens
	}
	return
}

func checkUploadCutoff(opt *Options, cs fs.SizeSuffix) error {
	if cs < opt.ChunkSize {
		return errors.Errorf("%v is less than chunk size %v", cs, opt.ChunkSize)
	}
	return nil
}

func (f *Fs) setUploadCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadCutoff(&f.opt, cs)
	if err == nil {
		old, f.opt.UploadCutoff = f.opt.UploadCutoff, cs
	}
	return
}

// NewFs contstructs an Fs from the path, bucket:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	err = checkUploadCutoff(opt, opt.UploadCutoff)
	if err != nil {
		return nil, errors.Wrap(err, "b2: upload cutoff")
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, errors.Wrap(err, "b2: chunk size")
	}
	bucket, directory, err := parsePath(root)
	if err != nil {
		return nil, err
	}
	if opt.Account == "" {
		return nil, errors.New("account not found")
	}
	if opt.Key == "" {
		return nil, errors.New("key not found")
	}
	if opt.Endpoint == "" {
		opt.Endpoint = defaultEndpoint
	}
	f := &Fs{
		name:   name,
		opt:    *opt,
		bucket: bucket,
		root:   directory,
		srv:    rest.NewClient(fshttp.NewClient(fs.Config)).SetErrorHandler(errorHandler),
		pacer:  pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
	}
	f.features = (&fs.Features{
		ReadMimeType:  true,
		WriteMimeType: true,
		BucketBased:   true,
	}).Fill(f)
	// Set the test flag if required
	if opt.TestMode != "" {
		testMode := strings.TrimSpace(opt.TestMode)
		f.srv.SetHeader(testModeHeader, testMode)
		fs.Debugf(f, "Setting test header \"%s: %s\"", testModeHeader, testMode)
	}
	f.fillBufferTokens()
	err = f.authorizeAccount()
	if err != nil {
		return nil, errors.Wrap(err, "failed to authorize account")
	}
	// If this is a key limited to a single bucket, it must exist already
	if f.bucket != "" && f.info.Allowed.BucketID != "" {
		allowedBucket := f.info.Allowed.BucketName
		if allowedBucket == "" {
			return nil, errors.New("bucket that application key is restricted to no longer exists")
		}
		if allowedBucket != f.bucket {
			return nil, errors.Errorf("you must use bucket %q with this application key", allowedBucket)
		}
		f.markBucketOK()
		f.setBucketID(f.info.Allowed.BucketID)
	}
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
		_, err := f.NewObject(remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				f.root = oldRoot
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// authorizeAccount gets the API endpoint and auth token.  Can be used
// for reauthentication too.
func (f *Fs) authorizeAccount() error {
	f.authMu.Lock()
	defer f.authMu.Unlock()
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/b2api/v1/b2_authorize_account",
		RootURL:      f.opt.Endpoint,
		UserName:     f.opt.Account,
		Password:     f.opt.Key,
		ExtraHeaders: map[string]string{"Authorization": ""}, // unset the Authorization for this request
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(&opts, nil, &f.info)
		return f.shouldRetryNoReauth(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to authenticate")
	}
	f.srv.SetRoot(f.info.APIURL+"/b2api/v1").SetHeader("Authorization", f.info.AuthorizationToken)
	return nil
}

// getUploadURL returns the upload info with the UploadURL and the AuthorizationToken
//
// This should be returned with returnUploadURL when finished
func (f *Fs) getUploadURL() (upload *api.GetUploadURLResponse, err error) {
	f.uploadMu.Lock()
	defer f.uploadMu.Unlock()
	bucketID, err := f.getBucketID()
	if err != nil {
		return nil, err
	}
	if len(f.uploads) == 0 {
		opts := rest.Opts{
			Method: "POST",
			Path:   "/b2_get_upload_url",
		}
		var request = api.GetUploadURLRequest{
			BucketID: bucketID,
		}
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(&opts, &request, &upload)
			return f.shouldRetry(resp, err)
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get upload URL")
		}
	} else {
		upload, f.uploads = f.uploads[0], f.uploads[1:]
	}
	return upload, nil
}

// returnUploadURL returns the UploadURL to the cache
func (f *Fs) returnUploadURL(upload *api.GetUploadURLResponse) {
	if upload == nil {
		return
	}
	f.uploadMu.Lock()
	f.uploads = append(f.uploads, upload)
	f.uploadMu.Unlock()
}

// clearUploadURL clears the current UploadURL and the AuthorizationToken
func (f *Fs) clearUploadURL() {
	f.uploadMu.Lock()
	f.uploads = nil
	f.uploadMu.Unlock()
}

// Fill up (or reset) the buffer tokens
func (f *Fs) fillBufferTokens() {
	f.bufferTokens = make(chan []byte, fs.Config.Transfers)
	for i := 0; i < fs.Config.Transfers; i++ {
		f.bufferTokens <- nil
	}
}

// getUploadBlock gets a block from the pool of size chunkSize
func (f *Fs) getUploadBlock() []byte {
	buf := <-f.bufferTokens
	if buf == nil {
		buf = make([]byte, f.opt.ChunkSize)
	}
	// fs.Debugf(f, "Getting upload block %p", buf)
	return buf
}

// putUploadBlock returns a block to the pool of size chunkSize
func (f *Fs) putUploadBlock(buf []byte) {
	buf = buf[:cap(buf)]
	if len(buf) != int(f.opt.ChunkSize) {
		panic("bad blocksize returned to pool")
	}
	// fs.Debugf(f, "Returning upload block %p", buf)
	f.bufferTokens <- buf
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *api.File) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		err := o.decodeMetaData(info)
		if err != nil {
			return nil, err
		}
	} else {
		err := o.readMetaData() // reads info and headers, returning an error
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

// listFn is called from list to handle an object
type listFn func(remote string, object *api.File, isDirectory bool) error

// errEndList is a sentinel used to end the list iteration now.
// listFn should return it to end the iteration with no errors.
var errEndList = errors.New("end list")

// list lists the objects into the function supplied from
// the bucket and root supplied
//
// dir is the starting directory, "" for root
//
// level is the depth to search to
//
// If prefix is set then startFileName is used as a prefix which all
// files must have
//
// If limit is > 0 then it limits to that many files (must be less
// than 1000)
//
// If hidden is set then it will list the hidden (deleted) files too.
func (f *Fs) list(dir string, recurse bool, prefix string, limit int, hidden bool, fn listFn) error {
	root := f.root
	if dir != "" {
		root += dir + "/"
	}
	delimiter := ""
	if !recurse {
		delimiter = "/"
	}
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
		Prefix:       root,
		Delimiter:    delimiter,
	}
	prefix = root + prefix
	if prefix != "" {
		request.StartFileName = prefix
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_list_file_names",
	}
	if hidden {
		opts.Path = "/b2_list_file_versions"
	}
	for {
		var response api.ListFileNamesResponse
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(&opts, &request, &response)
			return f.shouldRetry(resp, err)
		})
		if err != nil {
			return err
		}
		for i := range response.Files {
			file := &response.Files[i]
			// Finish if file name no longer has prefix
			if prefix != "" && !strings.HasPrefix(file.Name, prefix) {
				return nil
			}
			if !strings.HasPrefix(file.Name, f.root) {
				fs.Debugf(f, "Odd name received %q", file.Name)
				continue
			}
			remote := file.Name[len(f.root):]
			// Check for directory
			isDirectory := strings.HasSuffix(remote, "/")
			if isDirectory {
				remote = remote[:len(remote)-1]
			}
			// Send object
			err = fn(remote, file, isDirectory)
			if err != nil {
				if err == errEndList {
					return nil
				}
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

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(remote string, object *api.File, isDirectory bool, last *string) (fs.DirEntry, error) {
	if isDirectory {
		d := fs.NewDir(remote, time.Time{})
		return d, nil
	}
	if remote == *last {
		remote = object.UploadTimestamp.AddVersion(remote)
	} else {
		*last = remote
	}
	// hide objects represent deleted files which we don't list
	if object.Action == "hide" {
		return nil, nil
	}
	o, err := f.newObjectWithInfo(remote, object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// mark the bucket as being OK
func (f *Fs) markBucketOK() {
	if f.bucket != "" {
		f.bucketOKMu.Lock()
		f.bucketOK = true
		f.bucketOKMu.Unlock()
	}
}

// listDir lists a single directory
func (f *Fs) listDir(dir string) (entries fs.DirEntries, err error) {
	last := ""
	err = f.list(dir, false, "", 0, f.opt.Versions, func(remote string, object *api.File, isDirectory bool) error {
		entry, err := f.itemToDirEntry(remote, object, isDirectory, &last)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// bucket must be present if listing succeeded
	f.markBucketOK()
	return entries, nil
}

// listBuckets returns all the buckets to out
func (f *Fs) listBuckets(dir string) (entries fs.DirEntries, err error) {
	if dir != "" {
		return nil, fs.ErrorListBucketRequired
	}
	err = f.listBucketsToFn(func(bucket *api.Bucket) error {
		d := fs.NewDir(bucket.Name, time.Time{})
		entries = append(entries, d)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	if f.bucket == "" {
		return f.listBuckets(dir)
	}
	return f.listDir(dir)
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(dir string, callback fs.ListRCallback) (err error) {
	if f.bucket == "" {
		return fs.ErrorListBucketRequired
	}
	list := walk.NewListRHelper(callback)
	last := ""
	err = f.list(dir, true, "", 0, f.opt.Versions, func(remote string, object *api.File, isDirectory bool) error {
		entry, err := f.itemToDirEntry(remote, object, isDirectory, &last)
		if err != nil {
			return err
		}
		return list.Add(entry)
	})
	if err != nil {
		return err
	}
	// bucket must be present if listing succeeded
	f.markBucketOK()
	return list.Flush()
}

// listBucketFn is called from listBucketsToFn to handle a bucket
type listBucketFn func(*api.Bucket) error

// listBucketsToFn lists the buckets to the function supplied
func (f *Fs) listBucketsToFn(fn listBucketFn) error {
	var account = api.ListBucketsRequest{
		AccountID: f.info.AccountID,
		BucketID:  f.info.Allowed.BucketID,
	}

	var response api.ListBucketsResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_list_buckets",
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(&opts, &account, &response)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}
	for i := range response.Buckets {
		err = fn(&response.Buckets[i])
		if err != nil {
			return err
		}
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
	err = f.listBucketsToFn(func(bucket *api.Bucket) error {
		if bucket.Name == f.bucket {
			bucketID = bucket.ID
		}
		return nil

	})
	if bucketID == "" {
		err = fs.ErrorDirNotFound
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

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs, fs.Update(in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(in, src, options...)
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	f.bucketOKMu.Lock()
	defer f.bucketOKMu.Unlock()
	if f.bucketOK {
		return nil
	}
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
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(&opts, &request, &response)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			if apiErr.Code == "duplicate_bucket_name" {
				// Check this is our bucket - buckets are globally unique and this
				// might be someone elses.
				_, getBucketErr := f.getBucketID()
				if getBucketErr == nil {
					// found so it is our bucket
					f.bucketOK = true
					return nil
				}
				if getBucketErr != fs.ErrorDirNotFound {
					fs.Debugf(f, "Error checking bucket exists: %v", getBucketErr)
				}
			}
		}
		return errors.Wrap(err, "failed to create bucket")
	}
	f.setBucketID(response.ID)
	f.bucketOK = true
	return nil
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	f.bucketOKMu.Lock()
	defer f.bucketOKMu.Unlock()
	if f.root != "" || dir != "" {
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
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(&opts, &request, &response)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to delete bucket")
	}
	f.bucketOK = false
	f.clearBucketID()
	f.clearUploadURL()
	return nil
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

// hide hides a file on the remote
func (f *Fs) hide(Name string) error {
	bucketID, err := f.getBucketID()
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_hide_file",
	}
	var request = api.HideFileRequest{
		BucketID: bucketID,
		Name:     Name,
	}
	var response api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(&opts, &request, &response)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrapf(err, "failed to hide %q", Name)
	}
	return nil
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
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(&opts, &request, &response)
		return f.shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete %q", Name)
	}
	return nil
}

// purge deletes all the files and directories
//
// if oldOnly is true then it deletes only non current files.
//
// Implemented here so we can make sure we delete old versions.
func (f *Fs) purge(oldOnly bool) error {
	var errReturn error
	var checkErrMutex sync.Mutex
	var checkErr = func(err error) {
		if err == nil {
			return
		}
		checkErrMutex.Lock()
		defer checkErrMutex.Unlock()
		if errReturn == nil {
			errReturn = err
		}
	}
	var isUnfinishedUploadStale = func(timestamp api.Timestamp) bool {
		if time.Since(time.Time(timestamp)).Hours() > 24 {
			return true
		}
		return false
	}

	// Delete Config.Transfers in parallel
	toBeDeleted := make(chan *api.File, fs.Config.Transfers)
	var wg sync.WaitGroup
	wg.Add(fs.Config.Transfers)
	for i := 0; i < fs.Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for object := range toBeDeleted {
				accounting.Stats.Checking(object.Name)
				checkErr(f.deleteByID(object.ID, object.Name))
				accounting.Stats.DoneChecking(object.Name)
			}
		}()
	}
	last := ""
	checkErr(f.list("", true, "", 0, true, func(remote string, object *api.File, isDirectory bool) error {
		if !isDirectory {
			accounting.Stats.Checking(remote)
			if oldOnly && last != remote {
				if object.Action == "hide" {
					fs.Debugf(remote, "Deleting current version (id %q) as it is a hide marker", object.ID)
					toBeDeleted <- object
				} else if object.Action == "start" && isUnfinishedUploadStale(object.UploadTimestamp) {
					fs.Debugf(remote, "Deleting current version (id %q) as it is a start marker (upload started at %s)", object.ID, time.Time(object.UploadTimestamp).Local())
					toBeDeleted <- object
				} else {
					fs.Debugf(remote, "Not deleting current version (id %q) %q", object.ID, object.Action)
				}
			} else {
				fs.Debugf(remote, "Deleting (id %q)", object.ID)
				toBeDeleted <- object
			}
			last = remote
			accounting.Stats.DoneChecking(remote)
		}
		return nil
	}))
	close(toBeDeleted)
	wg.Wait()

	if !oldOnly {
		checkErr(f.Rmdir(""))
	}
	return errReturn
}

// Purge deletes all the files and directories including the old versions.
func (f *Fs) Purge() error {
	return f.purge(false)
}

// CleanUp deletes all the hidden files.
func (f *Fs) CleanUp() error {
	return f.purge(true)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA1)
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

// Hash returns the Sha-1 of an object returning a lowercase hex string
func (o *Object) Hash(t hash.Type) (string, error) {
	if t != hash.SHA1 {
		return "", hash.ErrUnsupported
	}
	if o.sha1 == "" {
		// Error is logged in readMetaData
		err := o.readMetaData()
		if err != nil {
			return "", err
		}
	}
	return o.sha1, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// decodeMetaDataRaw sets the metadata from the data passed in
//
// Sets
//  o.id
//  o.modTime
//  o.size
//  o.sha1
func (o *Object) decodeMetaDataRaw(ID, SHA1 string, Size int64, UploadTimestamp api.Timestamp, Info map[string]string, mimeType string) (err error) {
	o.id = ID
	o.sha1 = SHA1
	o.mimeType = mimeType
	// Read SHA1 from metadata if it exists and isn't set
	if o.sha1 == "" || o.sha1 == "none" {
		o.sha1 = Info[sha1Key]
	}
	o.size = Size
	// Use the UploadTimestamp if can't get file info
	o.modTime = time.Time(UploadTimestamp)
	return o.parseTimeString(Info[timeKey])
}

// decodeMetaData sets the metadata in the object from an api.File
//
// Sets
//  o.id
//  o.modTime
//  o.size
//  o.sha1
func (o *Object) decodeMetaData(info *api.File) (err error) {
	return o.decodeMetaDataRaw(info.ID, info.SHA1, info.Size, info.UploadTimestamp, info.Info, info.ContentType)
}

// decodeMetaDataFileInfo sets the metadata in the object from an api.FileInfo
//
// Sets
//  o.id
//  o.modTime
//  o.size
//  o.sha1
func (o *Object) decodeMetaDataFileInfo(info *api.FileInfo) (err error) {
	return o.decodeMetaDataRaw(info.ID, info.SHA1, info.Size, info.UploadTimestamp, info.Info, info.ContentType)
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// Sets
//  o.id
//  o.modTime
//  o.size
//  o.sha1
func (o *Object) readMetaData() (err error) {
	if o.id != "" {
		return nil
	}
	maxSearched := 1
	var timestamp api.Timestamp
	baseRemote := o.remote
	if o.fs.opt.Versions {
		timestamp, baseRemote = api.RemoveVersion(baseRemote)
		maxSearched = maxVersions
	}
	var info *api.File
	err = o.fs.list("", true, baseRemote, maxSearched, o.fs.opt.Versions, func(remote string, object *api.File, isDirectory bool) error {
		if isDirectory {
			return nil
		}
		if remote == baseRemote {
			if !timestamp.IsZero() && !timestamp.Equal(object.UploadTimestamp) {
				return nil
			}
			info = object
		}
		return errEndList // read only 1 item
	})
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	if info == nil {
		return fs.ErrorObjectNotFound
	}
	return o.decodeMetaData(info)
}

// timeString returns modTime as the number of milliseconds
// elapsed since January 1, 1970 UTC as a decimal string.
func timeString(modTime time.Time) string {
	return strconv.FormatInt(modTime.UnixNano()/1E6, 10)
}

// parseTimeString converts a decimal string number of milliseconds
// elapsed since January 1, 1970 UTC into a time.Time and stores it in
// the modTime variable.
func (o *Object) parseTimeString(timeString string) (err error) {
	if timeString == "" {
		return nil
	}
	unixMilliseconds, err := strconv.ParseInt(timeString, 10, 64)
	if err != nil {
		fs.Debugf(o, "Failed to parse mod time string %q: %v", timeString, err)
		return err
	}
	o.modTime = time.Unix(unixMilliseconds/1E3, (unixMilliseconds%1E3)*1E6).UTC()
	return nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
//
// SHA-1 will also be updated once the request has completed.
func (o *Object) ModTime() (result time.Time) {
	// The error is logged in readMetaData
	_ = o.readMetaData()
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	// Not possible with B2
	return fs.ErrorCantSetModTime
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
	hash  gohash.Hash    // currently accumulating SHA1
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
		return errors.Errorf("object corrupted on transfer - length mismatch (want %d got %d)", file.o.Size(), file.bytes)
	}

	// Check the SHA1
	receivedSHA1 := file.o.sha1
	calculatedSHA1 := fmt.Sprintf("%x", file.hash.Sum(nil))
	if receivedSHA1 != "" && receivedSHA1 != calculatedSHA1 {
		return errors.Errorf("object corrupted on transfer - SHA1 mismatch (want %q got %q)", receivedSHA1, calculatedSHA1)
	}

	return nil
}

// Check it satisfies the interfaces
var _ io.ReadCloser = &openFile{}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	opts := rest.Opts{
		Method:  "GET",
		RootURL: o.fs.info.DownloadURL,
		Options: options,
	}
	// Download by id if set otherwise by name
	if o.id != "" {
		opts.Path += "/b2api/v1/b2_download_file_by_id?fileId=" + urlEncode(o.id)
	} else {
		opts.Path += "/file/" + urlEncode(o.fs.bucket) + "/" + urlEncode(o.fs.root+o.remote)
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(&opts)
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to open for download")
	}

	// Parse the time out of the headers if possible
	err = o.parseTimeString(resp.Header.Get(timeHeader))
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	// Read sha1 from header if it isn't set
	if o.sha1 == "" {
		o.sha1 = resp.Header.Get(sha1Header)
		fs.Debugf(o, "Reading sha1 from header - %q", o.sha1)
		// if sha1 header is "none" (in big files), then need
		// to read it from the metadata
		if o.sha1 == "none" {
			o.sha1 = resp.Header.Get(sha1InfoHeader)
			fs.Debugf(o, "Reading sha1 from info - %q", o.sha1)
		}
	}
	// Don't check length or hash on partial content
	if resp.StatusCode == http.StatusPartialContent {
		return resp.Body, nil
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
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	if o.fs.opt.Versions {
		return errNotWithVersions
	}
	err = o.fs.Mkdir("")
	if err != nil {
		return err
	}
	size := src.Size()

	if size == -1 {
		// Check if the file is large enough for a chunked upload (needs to be at least two chunks)
		buf := o.fs.getUploadBlock()
		n, err := io.ReadFull(in, buf)
		if err == nil {
			bufReader := bufio.NewReader(in)
			in = bufReader
			_, err = bufReader.Peek(1)
		}

		if err == nil {
			fs.Debugf(o, "File is big enough for chunked streaming")
			up, err := o.fs.newLargeUpload(o, in, src)
			if err != nil {
				o.fs.putUploadBlock(buf)
				return err
			}
			return up.Stream(buf)
		} else if err == io.EOF || err == io.ErrUnexpectedEOF {
			fs.Debugf(o, "File has %d bytes, which makes only one chunk. Using direct upload.", n)
			defer o.fs.putUploadBlock(buf)
			size = int64(n)
			in = bytes.NewReader(buf[:n])
		} else {
			return err
		}
	} else if size > int64(o.fs.opt.UploadCutoff) {
		up, err := o.fs.newLargeUpload(o, in, src)
		if err != nil {
			return err
		}
		return up.Upload()
	}

	modTime := src.ModTime()

	calculatedSha1, _ := src.Hash(hash.SHA1)
	if calculatedSha1 == "" {
		calculatedSha1 = "hex_digits_at_end"
		har := newHashAppendingReader(in, sha1.New())
		size += int64(har.AdditionalLength())
		in = har
	}

	// Get upload URL
	upload, err := o.fs.getUploadURL()
	if err != nil {
		return err
	}
	defer func() {
		// return it like this because we might nil it out
		o.fs.returnUploadURL(upload)
	}()

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
		Method:  "POST",
		RootURL: upload.UploadURL,
		Body:    in,
		ExtraHeaders: map[string]string{
			"Authorization":  upload.AuthorizationToken,
			"X-Bz-File-Name": urlEncode(o.fs.root + o.remote),
			"Content-Type":   fs.MimeType(src),
			sha1Header:       calculatedSha1,
			timeHeader:       timeString(modTime),
		},
		ContentLength: &size,
	}
	var response api.FileInfo
	// Don't retry, return a retry error instead
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(&opts, nil, &response)
		retry, err := o.fs.shouldRetry(resp, err)
		// On retryable error clear UploadURL
		if retry {
			fs.Debugf(o, "Clearing upload URL because of error: %v", err)
			upload = nil
		}
		return retry, err
	})
	if err != nil {
		return err
	}
	return o.decodeMetaDataFileInfo(&response)
}

// Remove an object
func (o *Object) Remove() error {
	if o.fs.opt.Versions {
		return errNotWithVersions
	}
	if o.fs.opt.HardDelete {
		return o.fs.deleteByID(o.id, o.fs.root+o.remote)
	}
	return o.fs.hide(o.fs.root + o.remote)
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	return o.mimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.Purger      = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.CleanUpper  = &Fs{}
	_ fs.ListRer     = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
	_ fs.IDer        = &Object{}
)
