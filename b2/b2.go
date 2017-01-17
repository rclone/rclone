// Package b2 provides an interface to the Backblaze B2 object storage system
package b2

// FIXME should we remove sha1 checks from here as rclone now supports
// checking SHA1s?

import (
	"bytes"
	"crypto/sha1"
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
	"github.com/ncw/rclone/pacer"
	"github.com/ncw/rclone/rest"
	"github.com/pkg/errors"
)

const (
	defaultEndpoint  = "https://api.backblazeb2.com"
	headerPrefix     = "x-bz-info-" // lower case as that is what the server returns
	timeKey          = "src_last_modified_millis"
	timeHeader       = headerPrefix + timeKey
	sha1Key          = "large_file_sha1"
	sha1Header       = "X-Bz-Content-Sha1"
	sha1InfoHeader   = headerPrefix + sha1Key
	testModeHeader   = "X-Bz-Test-Mode"
	retryAfterHeader = "Retry-After"
	minSleep         = 10 * time.Millisecond
	maxSleep         = 5 * time.Minute
	decayConstant    = 1 // bigger for slower decay, exponential
	maxParts         = 10000
	maxVersions      = 100 // maximum number of versions we search in --b2-versions mode
)

// Globals
var (
	minChunkSize       = fs.SizeSuffix(100E6)
	chunkSize          = fs.SizeSuffix(96 * 1024 * 1024)
	uploadCutoff       = fs.SizeSuffix(200E6)
	b2TestMode         = fs.StringP("b2-test-mode", "", "", "A flag string for X-Bz-Test-Mode header.")
	b2Versions         = fs.BoolP("b2-versions", "", false, "Include old versions in directory listings.")
	errNotWithVersions = errors.New("can't modify or delete files in --b2-versions mode")
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "b2",
		Description: "Backblaze B2",
		NewFs:       NewFs,
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
	fs.VarP(&uploadCutoff, "b2-upload-cutoff", "", "Cutoff for switching to chunked upload")
	fs.VarP(&chunkSize, "b2-chunk-size", "", "Upload chunk size. Must fit in memory.")
}

// Fs represents a remote b2 server
type Fs struct {
	name          string                       // name of this remote
	root          string                       // the path we are working on if any
	features      *fs.Features                 // optional features
	account       string                       // account name
	key           string                       // auth key
	endpoint      string                       // name of the starting api endpoint
	srv           *rest.Client                 // the connection to the b2 server
	bucket        string                       // the bucket we are working on
	bucketIDMutex sync.Mutex                   // mutex to protect _bucketID
	_bucketID     string                       // the ID of the bucket we are working on
	info          api.AuthorizeAccountResponse // result of authorize call
	uploadMu      sync.Mutex                   // lock for upload variable
	uploads       []*api.GetUploadURLResponse  // result of get upload URL calls
	authMu        sync.Mutex                   // lock for authorizing the account
	pacer         *pacer.Pacer                 // To pace and retry the API calls
	uploadTokens  chan struct{}                // control concurrency of uploads
	extraTokens   chan struct{}                // extra tokens for multipart uploads
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
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

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
				fs.ErrorLog(f, "Malformed %s header %q: %v", retryAfterHeader, retryAfterString, err)
			}
		}
		retryAfterDuration := time.Duration(retryAfter) * time.Second
		if f.pacer.GetSleep() < retryAfterDuration {
			fs.Debug(f, "Setting sleep to %v after error: %v", retryAfterDuration, err)
			// We set 1/2 the value here because the pacer will double it immediately
			f.pacer.SetSleep(retryAfterDuration / 2)
		}
		return true, err
	}
	return fs.ShouldRetry(err) || fs.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(resp *http.Response, err error) (bool, error) {
	if resp != nil && resp.StatusCode == 401 {
		fs.Debug(f, "Unauthorized: %v", err)
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
	if uploadCutoff < chunkSize {
		return nil, errors.Errorf("b2: upload cutoff must be less than chunk size %v - was %v", chunkSize, uploadCutoff)
	}
	if chunkSize < minChunkSize {
		return nil, errors.Errorf("b2: chunk size can't be less than %v - was %v", minChunkSize, chunkSize)
	}
	bucket, directory, err := parsePath(root)
	if err != nil {
		return nil, err
	}
	account := fs.ConfigFileGet(name, "account")
	if account == "" {
		return nil, errors.New("account not found")
	}
	key := fs.ConfigFileGet(name, "key")
	if key == "" {
		return nil, errors.New("key not found")
	}
	endpoint := fs.ConfigFileGet(name, "endpoint", defaultEndpoint)
	f := &Fs{
		name:         name,
		bucket:       bucket,
		root:         directory,
		account:      account,
		key:          key,
		endpoint:     endpoint,
		srv:          rest.NewClient(fs.Config.Client()).SetErrorHandler(errorHandler),
		pacer:        pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
		uploadTokens: make(chan struct{}, fs.Config.Transfers),
		extraTokens:  make(chan struct{}, fs.Config.Transfers),
	}
	f.features = (&fs.Features{ReadMimeType: true, WriteMimeType: true}).Fill(f)
	// Set the test flag if required
	if *b2TestMode != "" {
		testMode := strings.TrimSpace(*b2TestMode)
		f.srv.SetHeader(testModeHeader, testMode)
		fs.Debug(f, "Setting test header \"%s: %s\"", testModeHeader, testMode)
	}
	// Fill up the upload and extra tokens
	for i := 0; i < fs.Config.Transfers; i++ {
		f.returnUploadToken()
		f.extraTokens <- struct{}{}
	}
	err = f.authorizeAccount()
	if err != nil {
		return nil, errors.Wrap(err, "failed to authorize account")
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
		Absolute:     true,
		Method:       "GET",
		Path:         f.endpoint + "/b2api/v1/b2_authorize_account",
		UserName:     f.account,
		Password:     f.key,
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

// Gets an upload token to control the concurrency
func (f *Fs) getUploadToken() {
	<-f.uploadTokens
}

// Return an upload token
func (f *Fs) returnUploadToken() {
	f.uploadTokens <- struct{}{}
}

// Help count the multipart uploads
type multipartUploadCounter struct {
	f           *Fs
	uploadToken chan struct{}
}

// Create a new upload counter. This gets an upload token for
// exclusive use by this multipart upload - the other tokens are
// shared between all the multipart uploads.
//
// Call .finished() when done to return the upload token.
func (f *Fs) newMultipartUploadCounter() *multipartUploadCounter {
	m := &multipartUploadCounter{
		f:           f,
		uploadToken: make(chan struct{}, 1),
	}
	f.getUploadToken()
	m.uploadToken <- struct{}{}
	return m
}

// Gets an upload token for a multipart upload
//
// This gets one token only from the first class tokens.  This means
// that the multiplart upload is guaranteed at least one token as
// there is one first class token per possible upload.
//
// Pass the return value to returnMultipartUploadToken
func (m *multipartUploadCounter) getMultipartUploadToken() bool {
	// get the upload token by preference
	select {
	case <-m.uploadToken:
		return true
	default:
	}
	// ...otherwise wait for the first one to appear.
	//
	// If both uploadToken and extraTokens are ready at this point
	// (unlikely but possible) and we get an extraToken instead of
	// an uploadToken this will not cause any harm - this
	// multipart upload will get an extra upload slot temporarily.
	select {
	case <-m.uploadToken:
		return true
	case <-m.f.extraTokens:
		return false
	}
}

// Return a multipart upload token retreived from getMultipartUploadToken
func (m *multipartUploadCounter) returnMultipartUploadToken(firstClass bool) {
	if firstClass {
		m.uploadToken <- struct{}{}
	} else {
		m.f.extraTokens <- struct{}{}
	}
}

// Mark us finished with this upload counter
func (m *multipartUploadCounter) finished() {
	m.f.returnUploadToken()
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
func (f *Fs) list(dir string, level int, prefix string, limit int, hidden bool, fn listFn) error {
	root := f.root
	if dir != "" {
		root += dir + "/"
	}
	delimiter := ""
	switch level {
	case 1:
		delimiter = "/"
	case fs.MaxLevel:
	default:
		return fs.ErrorLevelNotSupported
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
	var response api.ListFileNamesResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_list_file_names",
	}
	if hidden {
		opts.Path = "/b2_list_file_versions"
	}
	for {
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
				fs.Log(f, "Odd name received %q", file.Name)
				continue
			}
			remote := file.Name[len(f.root):]
			// Check for directory
			isDirectory := level != 0 && strings.HasSuffix(remote, "/")
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

// listFiles walks the path returning files and directories to out
func (f *Fs) listFiles(out fs.ListOpts, dir string) {
	defer out.Finished()
	// List the objects
	last := ""
	err := f.list(dir, out.Level(), "", 0, *b2Versions, func(remote string, object *api.File, isDirectory bool) error {
		if isDirectory {
			dir := &fs.Dir{
				Name:  remote,
				Bytes: -1,
				Count: -1,
			}
			if out.AddDir(dir) {
				return fs.ErrorListAborted
			}
		} else {
			if remote == last {
				remote = object.UploadTimestamp.AddVersion(remote)
			} else {
				last = remote
			}
			// hide objects represent deleted files which we don't list
			if object.Action == "hide" {
				return nil
			}
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
		out.SetError(err)
	}
}

// listBuckets returns all the buckets to out
func (f *Fs) listBuckets(out fs.ListOpts, dir string) {
	defer out.Finished()
	if dir != "" {
		out.SetError(fs.ErrorListOnlyRoot)
		return
	}
	err := f.listBucketsToFn(func(bucket *api.Bucket) error {
		dir := &fs.Dir{
			Name:  bucket.Name,
			Bytes: -1,
			Count: -1,
		}
		if out.AddDir(dir) {
			return fs.ErrorListAborted
		}
		return nil
	})
	if err != nil {
		out.SetError(err)
	}
}

// List walks the path returning files and directories to out
func (f *Fs) List(out fs.ListOpts, dir string) {
	if f.bucket == "" {
		f.listBuckets(out, dir)
	} else {
		f.listFiles(out, dir)
	}
	return
}

// listBucketFn is called from listBucketsToFn to handle a bucket
type listBucketFn func(*api.Bucket) error

// listBucketsToFn lists the buckets to the function supplied
func (f *Fs) listBucketsToFn(fn listBucketFn) error {
	var account = api.Account{ID: f.info.AccountID}
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
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs, fs.Update(in, src)
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	// Can't create subdirs
	if dir != "" {
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
					return nil
				}
				if getBucketErr != fs.ErrorDirNotFound {
					fs.Debug(f, "Error checking bucket exists: %v", getBucketErr)
				}
			}
		}
		return errors.Wrap(err, "failed to create bucket")
	}
	f.setBucketID(response.ID)
	return nil
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
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
	f.clearBucketID()
	f.clearUploadURL()
	return nil
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
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

	// Delete Config.Transfers in parallel
	toBeDeleted := make(chan *api.File, fs.Config.Transfers)
	var wg sync.WaitGroup
	wg.Add(fs.Config.Transfers)
	for i := 0; i < fs.Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for object := range toBeDeleted {
				fs.Stats.Checking(object.Name)
				checkErr(f.deleteByID(object.ID, object.Name))
				fs.Stats.DoneChecking(object.Name)
			}
		}()
	}
	last := ""
	checkErr(f.list("", fs.MaxLevel, "", 0, true, func(remote string, object *api.File, isDirectory bool) error {
		if !isDirectory {
			fs.Stats.Checking(remote)
			if oldOnly && last != remote {
				if object.Action == "hide" {
					fs.Debug(remote, "Deleting current version (id %q) as it is a hide marker", object.ID)
					toBeDeleted <- object
				} else {
					fs.Debug(remote, "Not deleting current version (id %q) %q", object.ID, object.Action)
				}
			} else {
				fs.Debug(remote, "Deleting (id %q)", object.ID)
				toBeDeleted <- object
			}
			last = remote
			fs.Stats.DoneChecking(remote)
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
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashSHA1)
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
func (o *Object) Hash(t fs.HashType) (string, error) {
	if t != fs.HashSHA1 {
		return "", fs.ErrHashUnsupported
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
	if *b2Versions {
		timestamp, baseRemote = api.RemoveVersion(baseRemote)
		maxSearched = maxVersions
	}
	var info *api.File
	err = o.fs.list("", fs.MaxLevel, baseRemote, maxSearched, *b2Versions, func(remote string, object *api.File, isDirectory bool) error {
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
		fs.Debug(o, "Failed to parse mod time string %q: %v", timeString, err)
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
		return errors.Errorf("object corrupted on transfer - length mismatch (want %d got %d)", file.o.Size(), file.bytes)
	}

	// Check the SHA1
	receivedSHA1 := file.o.sha1
	calculatedSHA1 := fmt.Sprintf("%x", file.hash.Sum(nil))
	if receivedSHA1 != calculatedSHA1 {
		return errors.Errorf("object corrupted on transfer - SHA1 mismatch (want %q got %q)", receivedSHA1, calculatedSHA1)
	}

	return nil
}

// Check it satisfies the interfaces
var _ io.ReadCloser = &openFile{}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	opts := rest.Opts{
		Method:   "GET",
		Absolute: true,
		Path:     o.fs.info.DownloadURL,
		Options:  options,
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
		fs.Debug(o, "Reading sha1 from header - %q", o.sha1)
		// if sha1 header is "none" (in big files), then need
		// to read it from the metadata
		if o.sha1 == "none" {
			o.sha1 = resp.Header.Get(sha1InfoHeader)
			fs.Debug(o, "Reading sha1 from info - %q", o.sha1)
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
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) (err error) {
	if *b2Versions {
		return errNotWithVersions
	}
	size := src.Size()

	// If a large file upload in chunks - see upload.go
	if size >= int64(uploadCutoff) {
		up, err := o.fs.newLargeUpload(o, in, src)
		if err != nil {
			return err
		}
		return up.Upload()
	}

	modTime := src.ModTime()
	calculatedSha1, _ := src.Hash(fs.HashSHA1)

	// If source cannot provide the hash, copy to a temporary file
	// and calculate the hash while doing so.
	// Then we serve the temporary file.
	if calculatedSha1 == "" {
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
			return errors.Errorf("read %d bytes expecting %d", n, size)
		}
		calculatedSha1 = fmt.Sprintf("%x", hash.Sum(nil))

		// Rewind the temporary file
		_, err = fd.Seek(0, 0)
		if err != nil {
			return err
		}
		// Set input to temporary file
		in = fd
	}

	// Get upload Token
	o.fs.getUploadToken()
	defer o.fs.returnUploadToken()

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
		Method:   "POST",
		Absolute: true,
		Path:     upload.UploadURL,
		Body:     in,
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
			fs.Debug(o, "Clearing upload URL because of error: %v", err)
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
	if *b2Versions {
		return errNotWithVersions
	}
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
		Name:     o.fs.root + o.remote,
	}
	var response api.File
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(&opts, &request, &response)
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to delete file")
	}
	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs         = &Fs{}
	_ fs.Purger     = &Fs{}
	_ fs.CleanUpper = &Fs{}
	_ fs.Object     = &Object{}
	_ fs.MimeTyper  = &Object{}
)
