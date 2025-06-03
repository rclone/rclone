// Package b2 provides an interface to the Backblaze B2 object storage system.
package b2

// FIXME should we remove sha1 checks from here as rclone now supports
// checking SHA1s?

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	gohash "hash"
	"io"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/b2/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/pool"
	"github.com/rclone/rclone/lib/rest"
)

const (
	defaultEndpoint     = "https://api.backblazeb2.com"
	headerPrefix        = "x-bz-info-" // lower case as that is what the server returns
	timeKey             = "src_last_modified_millis"
	timeHeader          = headerPrefix + timeKey
	sha1Key             = "large_file_sha1"
	sha1Header          = "X-Bz-Content-Sha1"
	testModeHeader      = "X-Bz-Test-Mode"
	idHeader            = "X-Bz-File-Id"
	nameHeader          = "X-Bz-File-Name"
	timestampHeader     = "X-Bz-Upload-Timestamp"
	retryAfterHeader    = "Retry-After"
	minSleep            = 10 * time.Millisecond
	maxSleep            = 5 * time.Minute
	decayConstant       = 1 // bigger for slower decay, exponential
	maxParts            = 10000
	maxVersions         = 100 // maximum number of versions we search in --b2-versions mode
	minChunkSize        = 5 * fs.Mebi
	defaultChunkSize    = 96 * fs.Mebi
	defaultUploadCutoff = 200 * fs.Mebi
	largeFileCopyCutoff = 4 * fs.Gibi // 5E9 is the max
	defaultMaxAge       = 24 * time.Hour
)

// Globals
var (
	errNotWithVersions  = errors.New("can't modify or delete files in --b2-versions mode")
	errNotWithVersionAt = errors.New("can't modify or delete files in --b2-version-at mode")
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "b2",
		Description: "Backblaze B2",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name:      "account",
			Help:      "Account ID or Application Key ID.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:      "key",
			Help:      "Application Key.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for the service.\n\nLeave blank normally.",
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
in the [b2 integrations checklist](https://www.backblaze.com/docs/cloud-storage-integration-checklist).`,
			Default:  "",
			Hide:     fs.OptionHideConfigurator,
			Advanced: true,
		}, {
			Name:     "versions",
			Help:     "Include old versions in directory listings.\n\nNote that when using this no file write operations are permitted,\nso you can't upload files or delete them.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "version_at",
			Help:     "Show file versions as they were at the specified time.\n\nNote that when using this no file write operations are permitted,\nso you can't upload files or delete them.",
			Default:  fs.Time{},
			Advanced: true,
		}, {
			Name:    "hard_delete",
			Help:    "Permanently delete files on remote removal, otherwise hide files.",
			Default: false,
		}, {
			Name: "upload_cutoff",
			Help: `Cutoff for switching to chunked upload.

Files above this size will be uploaded in chunks of "--b2-chunk-size".

This value should be set no larger than 4.657 GiB (== 5 GB).`,
			Default:  defaultUploadCutoff,
			Advanced: true,
		}, {
			Name: "copy_cutoff",
			Help: `Cutoff for switching to multipart copy.

Any files larger than this that need to be server-side copied will be
copied in chunks of this size.

The minimum is 0 and the maximum is 4.6 GiB.`,
			Default:  largeFileCopyCutoff,
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Upload chunk size.

When uploading large files, chunk the file into this size.

Must fit in memory. These chunks are buffered in memory and there
might a maximum of "--transfers" chunks in progress at once.

5,000,000 Bytes is the minimum size.`,
			Default:  defaultChunkSize,
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

Note that chunks are stored in memory and there may be up to
"--transfers" * "--b2-upload-concurrency" chunks stored at once
in memory.`,
			Default:  4,
			Advanced: true,
		}, {
			Name: "disable_checksum",
			Help: `Disable checksums for large (> upload cutoff) files.

Normally rclone will calculate the SHA1 checksum of the input before
uploading it so it can add it to metadata on the object. This is great
for data integrity checking but can cause long delays for large files
to start uploading.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "download_url",
			Help: `Custom endpoint for downloads.

This is usually set to a Cloudflare CDN URL as Backblaze offers
free egress for data downloaded through the Cloudflare network.
Rclone works with private buckets by sending an "Authorization" header.
If the custom endpoint rewrites the requests for authentication,
e.g., in Cloudflare Workers, this header needs to be handled properly.
Leave blank if you want to use the endpoint provided by Backblaze.

The URL provided here SHOULD have the protocol and SHOULD NOT have
a trailing slash or specify the /file/bucket subpath as rclone will
request files with "{download_url}/file/{bucket_name}/{path}".

Example:
> https://mysubdomain.mydomain.tld
(No trailing "/", "file" or "bucket")`,
			Advanced: true,
		}, {
			Name: "download_auth_duration",
			Help: `Time before the public link authorization token will expire in s or suffix ms|s|m|h|d.

This is used in combination with "rclone link" for making files
accessible to the public and sets the duration before the download
authorization token will expire.

The minimum value is 1 second. The maximum value is one week.`,
			Default:  fs.Duration(7 * 24 * time.Hour),
			Advanced: true,
		}, {
			Name:     "memory_pool_flush_time",
			Default:  fs.Duration(time.Minute),
			Advanced: true,
			Hide:     fs.OptionHideBoth,
			Help:     `How often internal memory buffer pools will be flushed. (no longer used)`,
		}, {
			Name:     "memory_pool_use_mmap",
			Default:  false,
			Advanced: true,
			Hide:     fs.OptionHideBoth,
			Help:     `Whether to use mmap buffers in internal memory pool. (no longer used)`,
		}, {
			Name: "lifecycle",
			Help: `Set the number of days deleted files should be kept when creating a bucket.

On bucket creation, this parameter is used to create a lifecycle rule
for the entire bucket.

If lifecycle is 0 (the default) it does not create a lifecycle rule so
the default B2 behaviour applies. This is to create versions of files
on delete and overwrite and to keep them indefinitely.

If lifecycle is >0 then it creates a single rule setting the number of
days before a file that is deleted or overwritten is deleted
permanently. This is known as daysFromHidingToDeleting in the b2 docs.

The minimum value for this parameter is 1 day.

You can also enable hard_delete in the config also which will mean
deletions won't cause versions but overwrites will still cause
versions to be made.

See: [rclone backend lifecycle](#lifecycle) for setting lifecycles after bucket creation.
`,
			Default:  0,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// See: https://www.backblaze.com/docs/cloud-storage-files
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			// FIXME: allow /, but not leading, trailing or double
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Account                       string               `config:"account"`
	Key                           string               `config:"key"`
	Endpoint                      string               `config:"endpoint"`
	TestMode                      string               `config:"test_mode"`
	Versions                      bool                 `config:"versions"`
	VersionAt                     fs.Time              `config:"version_at"`
	HardDelete                    bool                 `config:"hard_delete"`
	UploadCutoff                  fs.SizeSuffix        `config:"upload_cutoff"`
	CopyCutoff                    fs.SizeSuffix        `config:"copy_cutoff"`
	ChunkSize                     fs.SizeSuffix        `config:"chunk_size"`
	UploadConcurrency             int                  `config:"upload_concurrency"`
	DisableCheckSum               bool                 `config:"disable_checksum"`
	DownloadURL                   string               `config:"download_url"`
	DownloadAuthorizationDuration fs.Duration          `config:"download_auth_duration"`
	Lifecycle                     int                  `config:"lifecycle"`
	Enc                           encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote b2 server
type Fs struct {
	name            string                                 // name of this remote
	root            string                                 // the path we are working on if any
	opt             Options                                // parsed config options
	ci              *fs.ConfigInfo                         // global config
	features        *fs.Features                           // optional features
	srv             *rest.Client                           // the connection to the b2 server
	rootBucket      string                                 // bucket part of root (if any)
	rootDirectory   string                                 // directory part of root (if any)
	cache           *bucket.Cache                          // cache for bucket creation status
	bucketIDMutex   sync.Mutex                             // mutex to protect _bucketID
	_bucketID       map[string]string                      // the ID of the bucket we are working on
	bucketTypeMutex sync.Mutex                             // mutex to protect _bucketType
	_bucketType     map[string]string                      // the Type of the bucket we are working on
	info            api.AuthorizeAccountResponse           // result of authorize call
	uploadMu        sync.Mutex                             // lock for upload variable
	uploads         map[string][]*api.GetUploadURLResponse // Upload URLs by buckedID
	authMu          sync.Mutex                             // lock for authorizing the account
	pacer           *fs.Pacer                              // To pace and retry the API calls
	uploadToken     *pacer.TokenDispenser                  // control concurrency
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
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	if f.rootBucket == "" {
		return "B2 root"
	}
	if f.rootDirectory == "" {
		return fmt.Sprintf("B2 bucket %s", f.rootBucket)
	}
	return fmt.Sprintf("B2 bucket %s path %s", f.rootBucket, f.rootDirectory)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a remote 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// split returns bucket and bucketPath from the rootRelativePath
// relative to f.root
func (f *Fs) split(rootRelativePath string) (bucketName, bucketPath string) {
	return bucket.Split(path.Join(f.root, rootRelativePath))
}

// split returns bucket and bucketPath from the object
func (o *Object) split() (bucket, bucketPath string) {
	return o.fs.split(o.remote)
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	401, // Unauthorized (e.g. "Token has expired")
	408, // Request Timeout
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	503, // Service Unavailable
	504, // Gateway Time-out
}

// shouldRetryNoReauth returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetryNoReauth(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
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
		return true, pacer.RetryAfterError(err, time.Duration(retryAfter)*time.Second)
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if resp != nil && resp.StatusCode == 401 {
		fs.Debugf(f, "Unauthorized: %v", err)
		// Reauth
		authErr := f.authorizeAccount(ctx)
		if authErr != nil {
			err = authErr
		}
		return true, err
	}
	return f.shouldRetryNoReauth(ctx, resp, err)
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		fs.Errorf(nil, "Couldn't read error out of body: %v", err)
		body = nil
	}
	// Decode error response if there was one - they can be blank
	errResponse := new(api.Error)
	if len(body) > 0 {
		err = json.Unmarshal(body, errResponse)
		if err != nil {
			fs.Errorf(nil, "Couldn't decode error response: %v", err)
		}
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
		return fmt.Errorf("%s is less than %s", cs, minChunkSize)
	}
	return nil
}

func (f *Fs) setUploadChunkSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	}
	return
}

func checkUploadCutoff(opt *Options, cs fs.SizeSuffix) error {
	if cs < opt.ChunkSize {
		return fmt.Errorf("%v is less than chunk size %v", cs, opt.ChunkSize)
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

func (f *Fs) setCopyCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.CopyCutoff = f.opt.CopyCutoff, cs
	}
	return
}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootBucket, f.rootDirectory = bucket.Split(f.root)
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.UploadCutoff < opt.ChunkSize {
		opt.UploadCutoff = opt.ChunkSize
		fs.Infof(nil, "b2: raising upload cutoff to chunk size: %v", opt.UploadCutoff)
	}
	err = checkUploadCutoff(opt, opt.UploadCutoff)
	if err != nil {
		return nil, fmt.Errorf("b2: upload cutoff: %w", err)
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, fmt.Errorf("b2: chunk size: %w", err)
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
	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:        name,
		opt:         *opt,
		ci:          ci,
		srv:         rest.NewClient(fshttp.NewClient(ctx)).SetErrorHandler(errorHandler),
		cache:       bucket.NewCache(),
		_bucketID:   make(map[string]string, 1),
		_bucketType: make(map[string]string, 1),
		uploads:     make(map[string][]*api.GetUploadURLResponse),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		uploadToken: pacer.NewTokenDispenser(ci.Transfers),
	}
	f.setRoot(root)
	f.features = (&fs.Features{
		ReadMimeType:          true,
		WriteMimeType:         true,
		BucketBased:           true,
		BucketBasedRootOK:     true,
		ChunkWriterDoesntSeek: true,
	}).Fill(ctx, f)
	// Set the test flag if required
	if opt.TestMode != "" {
		testMode := strings.TrimSpace(opt.TestMode)
		f.srv.SetHeader(testModeHeader, testMode)
		fs.Debugf(f, "Setting test header \"%s: %s\"", testModeHeader, testMode)
	}
	err = f.authorizeAccount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to authorize account: %w", err)
	}
	// If this is a key limited to a single bucket, it must exist already
	if f.rootBucket != "" && f.info.Allowed.BucketID != "" {
		allowedBucket := f.opt.Enc.ToStandardName(f.info.Allowed.BucketName)
		if allowedBucket == "" {
			return nil, errors.New("bucket that application key is restricted to no longer exists")
		}
		if allowedBucket != f.rootBucket {
			return nil, fmt.Errorf("you must use bucket %q with this application key", allowedBucket)
		}
		f.cache.MarkOK(f.rootBucket)
		f.setBucketID(f.rootBucket, f.info.Allowed.BucketID)
	}
	if f.rootBucket != "" && f.rootDirectory != "" {
		// Check to see if the (bucket,directory) is actually an existing file
		oldRoot := f.root
		newRoot, leaf := path.Split(oldRoot)
		f.setRoot(newRoot)
		_, err := f.NewObject(ctx, leaf)
		if err != nil {
			// File doesn't exist so return old f
			f.setRoot(oldRoot)
			return f, nil
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// authorizeAccount gets the API endpoint and auth token.  Can be used
// for reauthentication too.
func (f *Fs) authorizeAccount(ctx context.Context) error {
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
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &f.info)
		return f.shouldRetryNoReauth(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}
	f.srv.SetRoot(f.info.APIURL+"/b2api/v1").SetHeader("Authorization", f.info.AuthorizationToken)
	return nil
}

// hasPermission returns if the current AuthorizationToken has the selected permission
func (f *Fs) hasPermission(permission string) bool {
	return slices.Contains(f.info.Allowed.Capabilities, permission)
}

// getUploadURL returns the upload info with the UploadURL and the AuthorizationToken
//
// This should be returned with returnUploadURL when finished
func (f *Fs) getUploadURL(ctx context.Context, bucket string) (upload *api.GetUploadURLResponse, err error) {
	f.uploadMu.Lock()
	defer f.uploadMu.Unlock()
	bucketID, err := f.getBucketID(ctx, bucket)
	if err != nil {
		return nil, err
	}
	// look for a stored upload URL for the correct bucketID
	uploads := f.uploads[bucketID]
	if len(uploads) > 0 {
		upload, uploads = uploads[0], uploads[1:]
		f.uploads[bucketID] = uploads
		return upload, nil
	}
	// get a new upload URL since not found
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_get_upload_url",
	}
	var request = api.GetUploadURLRequest{
		BucketID: bucketID,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &upload)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get upload URL: %w", err)
	}
	return upload, nil
}

// returnUploadURL returns the UploadURL to the cache
func (f *Fs) returnUploadURL(upload *api.GetUploadURLResponse) {
	if upload == nil {
		return
	}
	f.uploadMu.Lock()
	f.uploads[upload.BucketID] = append(f.uploads[upload.BucketID], upload)
	f.uploadMu.Unlock()
}

// clearUploadURL clears the current UploadURL and the AuthorizationToken
func (f *Fs) clearUploadURL(bucketID string) {
	f.uploadMu.Lock()
	delete(f.uploads, bucketID)
	f.uploadMu.Unlock()
}

// getRW gets a RW buffer and an upload token
//
// If noBuf is set then it just gets an upload token
func (f *Fs) getRW(noBuf bool) (rw *pool.RW) {
	f.uploadToken.Get()
	if !noBuf {
		rw = multipart.NewRW()
	}
	return rw
}

// putRW returns a RW buffer to the memory pool and returns an upload
// token
//
// If buf is nil then it just returns the upload token
func (f *Fs) putRW(rw *pool.RW) {
	if rw != nil {
		_ = rw.Close()
	}
	f.uploadToken.Put()
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.File) (fs.Object, error) {
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
		err := o.readMetaData(ctx) // reads info and headers, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// listFn is called from list to handle an object
type listFn func(remote string, object *api.File, isDirectory bool) error

// errEndList is a sentinel used to end the list iteration now.
// listFn should return it to end the iteration with no errors.
var errEndList = errors.New("end list")

// list lists the objects into the function supplied from
// the bucket and root supplied
//
// (bucket, directory) is the starting directory
//
// If prefix is set then it is removed from all file names.
//
// If addBucket is set then it adds the bucket to the start of the
// remotes generated.
//
// If recurse is set the function will recursively list.
//
// If limit is > 0 then it limits to that many files (must be less
// than 1000).
//
// If hidden is set then it will list the hidden (deleted) files too.
//
// if findFile is set it will look for files called (bucket, directory)
func (f *Fs) list(ctx context.Context, bucket, directory, prefix string, addBucket bool, recurse bool, limit int, hidden bool, findFile bool, fn listFn) error {
	if !findFile {
		if prefix != "" {
			prefix += "/"
		}
		if directory != "" {
			directory += "/"
		}
	}
	delimiter := ""
	if !recurse {
		delimiter = "/"
	}
	bucketID, err := f.getBucketID(ctx, bucket)
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
		Prefix:       f.opt.Enc.FromStandardPath(directory),
		Delimiter:    delimiter,
	}
	if directory != "" {
		request.StartFileName = f.opt.Enc.FromStandardPath(directory)
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_list_file_names",
	}
	if hidden || f.opt.VersionAt.IsSet() {
		opts.Path = "/b2_list_file_versions"
	}

	lastFileName := ""

	for {
		var response api.ListFileNamesResponse
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
			return f.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return err
		}
		for i := range response.Files {
			file := &response.Files[i]
			file.Name = f.opt.Enc.ToStandardPath(file.Name)
			// Finish if file name no longer has prefix
			if prefix != "" && !strings.HasPrefix(file.Name, prefix) {
				return nil
			}
			if !strings.HasPrefix(file.Name, prefix) {
				fs.Debugf(f, "Odd name received %q", file.Name)
				continue
			}
			remote := file.Name[len(prefix):]
			// Check for directory
			isDirectory := remote == "" || strings.HasSuffix(remote, "/")
			if isDirectory && len(remote) > 1 {
				remote = remote[:len(remote)-1]
			}
			if addBucket {
				remote = path.Join(bucket, remote)
			}

			if f.opt.VersionAt.IsSet() {
				if time.Time(file.UploadTimestamp).After(time.Time(f.opt.VersionAt)) {
					// Ignore versions that were created after the specified time
					continue
				}

				if file.Name == lastFileName {
					// Ignore versions before the already returned version
					continue
				}
			}

			// Send object
			lastFileName = file.Name
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
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, object *api.File, isDirectory bool, last *string) (fs.DirEntry, error) {
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
	o, err := f.newObjectWithInfo(ctx, remote, object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// listDir lists a single directory
func (f *Fs) listDir(ctx context.Context, bucket, directory, prefix string, addBucket bool) (entries fs.DirEntries, err error) {
	last := ""
	err = f.list(ctx, bucket, directory, prefix, f.rootBucket == "", false, 0, f.opt.Versions, false, func(remote string, object *api.File, isDirectory bool) error {
		entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory, &last)
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
	f.cache.MarkOK(bucket)
	return entries, nil
}

// listBuckets returns all the buckets to out
func (f *Fs) listBuckets(ctx context.Context) (entries fs.DirEntries, err error) {
	err = f.listBucketsToFn(ctx, "", func(bucket *api.Bucket) error {
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	bucket, directory := f.split(dir)
	if bucket == "" {
		if directory != "" {
			return nil, fs.ErrorListBucketRequired
		}
		return f.listBuckets(ctx)
	}
	return f.listDir(ctx, bucket, directory, f.rootDirectory, f.rootBucket == "")
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
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	bucket, directory := f.split(dir)
	list := list.NewHelper(callback)
	listR := func(bucket, directory, prefix string, addBucket bool) error {
		last := ""
		return f.list(ctx, bucket, directory, prefix, addBucket, true, 0, f.opt.Versions, false, func(remote string, object *api.File, isDirectory bool) error {
			entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory, &last)
			if err != nil {
				return err
			}
			return list.Add(entry)
		})
	}
	if bucket == "" {
		entries, err := f.listBuckets(ctx)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = list.Add(entry)
			if err != nil {
				return err
			}
			bucket := entry.Remote()
			err = listR(bucket, "", f.rootDirectory, true)
			if err != nil {
				return err
			}
			// bucket must be present if listing succeeded
			f.cache.MarkOK(bucket)
		}
	} else {
		err = listR(bucket, directory, f.rootDirectory, f.rootBucket == "")
		if err != nil {
			return err
		}
		// bucket must be present if listing succeeded
		f.cache.MarkOK(bucket)
	}
	return list.Flush()
}

// listBucketFn is called from listBucketsToFn to handle a bucket
type listBucketFn func(*api.Bucket) error

// listBucketsToFn lists the buckets to the function supplied
func (f *Fs) listBucketsToFn(ctx context.Context, bucketName string, fn listBucketFn) error {
	var account = api.ListBucketsRequest{
		AccountID: f.info.AccountID,
		BucketID:  f.info.Allowed.BucketID,
	}
	if bucketName != "" && account.BucketID == "" {
		account.BucketName = f.opt.Enc.FromStandardName(bucketName)
	}

	var response api.ListBucketsResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_list_buckets",
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &account, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	f.bucketIDMutex.Lock()
	f.bucketTypeMutex.Lock()
	f._bucketID = make(map[string]string, 1)
	f._bucketType = make(map[string]string, 1)
	for i := range response.Buckets {
		bucket := &response.Buckets[i]
		bucket.Name = f.opt.Enc.ToStandardName(bucket.Name)
		f.cache.MarkOK(bucket.Name)
		f._bucketID[bucket.Name] = bucket.ID
		f._bucketType[bucket.Name] = bucket.Type
	}
	f.bucketTypeMutex.Unlock()
	f.bucketIDMutex.Unlock()
	for i := range response.Buckets {
		bucket := &response.Buckets[i]
		err = fn(bucket)
		if err != nil {
			return err
		}
	}
	return nil
}

// getbucketType finds the bucketType for the current bucket name
// can be one of allPublic. allPrivate, or snapshot
func (f *Fs) getbucketType(ctx context.Context, bucket string) (bucketType string, err error) {
	f.bucketTypeMutex.Lock()
	bucketType = f._bucketType[bucket]
	f.bucketTypeMutex.Unlock()
	if bucketType != "" {
		return bucketType, nil
	}
	err = f.listBucketsToFn(ctx, bucket, func(bucket *api.Bucket) error {
		// listBucketsToFn reads bucket Types
		return nil
	})
	f.bucketTypeMutex.Lock()
	bucketType = f._bucketType[bucket]
	f.bucketTypeMutex.Unlock()
	if bucketType == "" {
		err = fs.ErrorDirNotFound
	}
	return bucketType, err
}

// setBucketType sets the Type for the current bucket name
func (f *Fs) setBucketType(bucket string, Type string) {
	f.bucketTypeMutex.Lock()
	f._bucketType[bucket] = Type
	f.bucketTypeMutex.Unlock()
}

// clearBucketType clears the Type for the current bucket name
func (f *Fs) clearBucketType(bucket string) {
	f.bucketTypeMutex.Lock()
	delete(f._bucketType, bucket)
	f.bucketTypeMutex.Unlock()
}

// getBucketID finds the ID for the current bucket name
func (f *Fs) getBucketID(ctx context.Context, bucket string) (bucketID string, err error) {
	f.bucketIDMutex.Lock()
	bucketID = f._bucketID[bucket]
	f.bucketIDMutex.Unlock()
	if bucketID != "" {
		return bucketID, nil
	}
	err = f.listBucketsToFn(ctx, bucket, func(bucket *api.Bucket) error {
		// listBucketsToFn sets IDs
		return nil
	})
	f.bucketIDMutex.Lock()
	bucketID = f._bucketID[bucket]
	f.bucketIDMutex.Unlock()
	if bucketID == "" {
		err = fs.ErrorDirNotFound
	}
	return bucketID, err
}

// setBucketID sets the ID for the current bucket name
func (f *Fs) setBucketID(bucket, ID string) {
	f.bucketIDMutex.Lock()
	f._bucketID[bucket] = ID
	f.bucketIDMutex.Unlock()
}

// clearBucketID clears the ID for the current bucket name
func (f *Fs) clearBucketID(bucket string) {
	f.bucketIDMutex.Lock()
	delete(f._bucketID, bucket)
	f.bucketIDMutex.Unlock()
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs, fs.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	bucket, _ := f.split(dir)
	return f.makeBucket(ctx, bucket)
}

// makeBucket creates the bucket if it doesn't exist
func (f *Fs) makeBucket(ctx context.Context, bucket string) error {
	return f.cache.Create(bucket, func() error {
		opts := rest.Opts{
			Method: "POST",
			Path:   "/b2_create_bucket",
		}
		var request = api.CreateBucketRequest{
			AccountID: f.info.AccountID,
			Name:      f.opt.Enc.FromStandardName(bucket),
			Type:      "allPrivate",
		}
		if f.opt.Lifecycle > 0 {
			request.LifecycleRules = []api.LifecycleRule{{
				DaysFromHidingToDeleting: &f.opt.Lifecycle,
			}}
		}
		var response api.Bucket
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
			return f.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			if apiErr, ok := err.(*api.Error); ok {
				if apiErr.Code == "duplicate_bucket_name" {
					// Check this is our bucket - buckets are globally unique and this
					// might be someone elses.
					_, getBucketErr := f.getBucketID(ctx, bucket)
					if getBucketErr == nil {
						// found so it is our bucket
						return nil
					}
					if getBucketErr != fs.ErrorDirNotFound {
						fs.Debugf(f, "Error checking bucket exists: %v", getBucketErr)
					}
				}
			}
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		f.setBucketID(bucket, response.ID)
		f.setBucketType(bucket, response.Type)
		return nil
	}, nil)
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	bucket, directory := f.split(dir)
	if bucket == "" || directory != "" {
		return nil
	}
	return f.cache.Remove(bucket, func() error {
		opts := rest.Opts{
			Method: "POST",
			Path:   "/b2_delete_bucket",
		}
		bucketID, err := f.getBucketID(ctx, bucket)
		if err != nil {
			return err
		}
		var request = api.DeleteBucketRequest{
			ID:        bucketID,
			AccountID: f.info.AccountID,
		}
		var response api.Bucket
		err = f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
			return f.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("failed to delete bucket: %w", err)
		}
		f.clearBucketID(bucket)
		f.clearBucketType(bucket)
		f.clearUploadURL(bucketID)
		return nil
	})
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

// hide hides a file on the remote
func (f *Fs) hide(ctx context.Context, bucket, bucketPath string) error {
	bucketID, err := f.getBucketID(ctx, bucket)
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_hide_file",
	}
	var request = api.HideFileRequest{
		BucketID: bucketID,
		Name:     f.opt.Enc.FromStandardPath(bucketPath),
	}
	var response api.File
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			if apiErr.Code == "already_hidden" {
				// sometimes eventual consistency causes this, so
				// ignore this error since it is harmless
				return nil
			}
		}
		return fmt.Errorf("failed to hide %q: %w", bucketPath, err)
	}
	return nil
}

// deleteByID deletes a file version given Name and ID
func (f *Fs) deleteByID(ctx context.Context, ID, Name string) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_delete_file_version",
	}
	var request = api.DeleteFileRequest{
		ID:   ID,
		Name: f.opt.Enc.FromStandardPath(Name),
	}
	var response api.File
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to delete %q: %w", Name, err)
	}
	return nil
}

// purge deletes all the files and directories
//
// if oldOnly is true then it deletes only non current files.
//
// Implemented here so we can make sure we delete old versions.
func (f *Fs) purge(ctx context.Context, dir string, oldOnly bool, deleteHidden bool, deleteUnfinished bool, maxAge time.Duration) error {
	bucket, directory := f.split(dir)
	if bucket == "" {
		return errors.New("can't purge from root")
	}
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
		return time.Since(time.Time(timestamp)) > maxAge
	}

	// Delete Config.Transfers in parallel
	toBeDeleted := make(chan *api.File, f.ci.Transfers)
	var wg sync.WaitGroup
	wg.Add(f.ci.Transfers)
	for range f.ci.Transfers {
		go func() {
			defer wg.Done()
			for object := range toBeDeleted {
				oi, err := f.newObjectWithInfo(ctx, object.Name, object)
				if err != nil {
					fs.Errorf(object.Name, "Can't create object %v", err)
					continue
				}
				tr := accounting.Stats(ctx).NewCheckingTransfer(oi, "deleting")
				err = f.deleteByID(ctx, object.ID, object.Name)
				checkErr(err)
				tr.Done(ctx, err)
			}
		}()
	}
	if oldOnly {
		if deleteHidden && deleteUnfinished {
			fs.Infof(f, "cleaning bucket %q of all hidden files, and pending multipart uploads older than %v", bucket, maxAge)
		} else if deleteHidden {
			fs.Infof(f, "cleaning bucket %q of all hidden files", bucket)
		} else if deleteUnfinished {
			fs.Infof(f, "cleaning bucket %q of pending multipart uploads older than %v", bucket, maxAge)
		} else {
			fs.Errorf(f, "cleaning bucket %q of nothing. This should never happen!", bucket)
			return nil
		}
	} else {
		fs.Infof(f, "cleaning bucket %q of all files", bucket)
	}

	last := ""
	checkErr(f.list(ctx, bucket, directory, f.rootDirectory, f.rootBucket == "", true, 0, true, false, func(remote string, object *api.File, isDirectory bool) error {
		if !isDirectory {
			oi, err := f.newObjectWithInfo(ctx, object.Name, object)
			if err != nil {
				fs.Errorf(object, "Can't create object %+v", err)
			}
			tr := accounting.Stats(ctx).NewCheckingTransfer(oi, "checking")
			if oldOnly && last != remote {
				// Check current version of the file
				if deleteHidden && object.Action == "hide" {
					fs.Debugf(remote, "Deleting current version (id %q) as it is a hide marker", object.ID)
					if !operations.SkipDestructive(ctx, object.Name, "remove hide marker") {
						toBeDeleted <- object
					}
				} else if deleteUnfinished && object.Action == "start" && isUnfinishedUploadStale(object.UploadTimestamp) {
					fs.Debugf(remote, "Deleting current version (id %q) as it is a start marker (upload started at %s)", object.ID, time.Time(object.UploadTimestamp).Local())
					if !operations.SkipDestructive(ctx, object.Name, "remove pending upload") {
						toBeDeleted <- object
					}
				} else {
					fs.Debugf(remote, "Not deleting current version (id %q) %q dated %v (%v ago)", object.ID, object.Action, time.Time(object.UploadTimestamp).Local(), time.Since(time.Time(object.UploadTimestamp)))
				}
			} else {
				fs.Debugf(remote, "Deleting (id %q)", object.ID)
				if !operations.SkipDestructive(ctx, object.Name, "delete") {
					toBeDeleted <- object
				}
			}
			last = remote
			tr.Done(ctx, nil)
		}
		return nil
	}))
	close(toBeDeleted)
	wg.Wait()

	if !oldOnly {
		checkErr(f.Rmdir(ctx, dir))
	}
	return errReturn
}

// Purge deletes all the files and directories including the old versions.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purge(ctx, dir, false, false, false, defaultMaxAge)
}

// CleanUp deletes all hidden files and pending multipart uploads older than 24 hours.
func (f *Fs) CleanUp(ctx context.Context) error {
	return f.purge(ctx, "", true, true, true, defaultMaxAge)
}

// cleanUp deletes all hidden files and/or pending multipart uploads older than the specified age.
func (f *Fs) cleanUp(ctx context.Context, deleteHidden bool, deleteUnfinished bool, maxAge time.Duration) (err error) {
	return f.purge(ctx, "", true, deleteHidden, deleteUnfinished, maxAge)
}

// copy does a server-side copy from dstObj <- srcObj
//
// If newInfo is nil then the metadata will be copied otherwise it
// will be replaced with newInfo
func (f *Fs) copy(ctx context.Context, dstObj *Object, srcObj *Object, newInfo *api.File) (err error) {
	if srcObj.size > int64(f.opt.CopyCutoff) {
		if newInfo == nil {
			newInfo, err = srcObj.getMetaData(ctx)
			if err != nil {
				return err
			}
		}
		up, err := f.newLargeUpload(ctx, dstObj, nil, srcObj, f.opt.CopyCutoff, true, newInfo)
		if err != nil {
			return err
		}
		err = up.Copy(ctx)
		if err != nil {
			return err
		}
		return dstObj.decodeMetaDataFileInfo(up.info)
	}

	dstBucket, dstPath := dstObj.split()
	err = f.makeBucket(ctx, dstBucket)
	if err != nil {
		return err
	}

	destBucketID, err := f.getBucketID(ctx, dstBucket)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_copy_file",
	}
	var request = api.CopyFileRequest{
		SourceID:     srcObj.id,
		Name:         f.opt.Enc.FromStandardPath(dstPath),
		DestBucketID: destBucketID,
	}
	if newInfo == nil {
		request.MetadataDirective = "COPY"
	} else {
		request.MetadataDirective = "REPLACE"
		request.ContentType = newInfo.ContentType
		request.Info = newInfo.Info
	}
	var response api.FileInfo
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	return dstObj.decodeMetaDataFileInfo(&response)
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	// Temporary Object under construction
	dstObj := &Object{
		fs:     f,
		remote: remote,
	}
	err := f.copy(ctx, dstObj, srcObj, nil)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA1)
}

// getDownloadAuthorization returns authorization token for downloading
// without account.
func (f *Fs) getDownloadAuthorization(ctx context.Context, bucket, remote string) (authorization string, err error) {
	validDurationInSeconds := time.Duration(f.opt.DownloadAuthorizationDuration).Nanoseconds() / 1e9
	if validDurationInSeconds <= 0 || validDurationInSeconds > 604800 {
		return "", errors.New("--b2-download-auth-duration must be between 1 sec and 1 week")
	}
	if !f.hasPermission("shareFiles") {
		return "", errors.New("sharing a file link requires the shareFiles permission")
	}
	bucketID, err := f.getBucketID(ctx, bucket)
	if err != nil {
		return "", err
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/b2_get_download_authorization",
	}
	var request = api.GetDownloadAuthorizationRequest{
		BucketID:               bucketID,
		FileNamePrefix:         f.opt.Enc.FromStandardPath(path.Join(f.rootDirectory, remote)),
		ValidDurationInSeconds: validDurationInSeconds,
	}
	var response api.GetDownloadAuthorizationResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("failed to get download authorization: %w", err)
	}
	return response.AuthorizationToken, nil
}

// PublicLink returns a link for downloading without account
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	bucket, bucketPath := f.split(remote)
	var RootURL string
	if f.opt.DownloadURL == "" {
		RootURL = f.info.DownloadURL
	} else {
		RootURL = f.opt.DownloadURL
	}
	_, err = f.NewObject(ctx, remote)
	if err == fs.ErrorObjectNotFound || err == fs.ErrorNotAFile {
		err2 := f.list(ctx, bucket, bucketPath, f.rootDirectory, f.rootBucket == "", false, 1, f.opt.Versions, false, func(remote string, object *api.File, isDirectory bool) error {
			err = nil
			return nil
		})
		if err2 != nil {
			return "", err2
		}
	}
	if err != nil {
		return "", err
	}
	absPath := "/" + urlEncode(bucketPath)
	link = RootURL + "/file/" + urlEncode(bucket) + absPath
	bucketType, err := f.getbucketType(ctx, bucket)
	if err != nil {
		return "", err
	}
	if bucketType == "allPrivate" || bucketType == "snapshot" {
		AuthorizationToken, err := f.getDownloadAuthorization(ctx, bucket, remote)
		if err != nil {
			return "", err
		}
		link += "?Authorization=" + AuthorizationToken
	}
	return link, nil
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
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA1 {
		return "", hash.ErrUnsupported
	}
	if o.sha1 == "" {
		// Error is logged in readMetaData
		err := o.readMetaData(ctx)
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

// Clean the SHA1
//
// Make sure it is lower case.
//
// Remove unverified prefix - see https://www.backblaze.com/docs/cloud-storage-upload-files-with-the-native-api
// Some tools (e.g. Cyberduck) use this
func cleanSHA1(sha1 string) string {
	const unverified = "unverified:"
	return strings.TrimPrefix(strings.ToLower(sha1), unverified)
}

// decodeMetaDataRaw sets the metadata from the data passed in
//
// Sets
//
//	o.id
//	o.modTime
//	o.size
//	o.sha1
func (o *Object) decodeMetaDataRaw(ID, SHA1 string, Size int64, UploadTimestamp api.Timestamp, Info map[string]string, mimeType string) (err error) {
	o.id = ID
	o.sha1 = SHA1
	o.mimeType = mimeType
	// Read SHA1 from metadata if it exists and isn't set
	if o.sha1 == "" || o.sha1 == "none" {
		o.sha1 = Info[sha1Key]
	}
	o.sha1 = cleanSHA1(o.sha1)
	o.size = Size
	// Use the UploadTimestamp if can't get file info
	o.modTime = time.Time(UploadTimestamp)
	err = o.parseTimeString(Info[timeKey])
	if err != nil {
		return err
	}
	return nil
}

// decodeMetaData sets the metadata in the object from an api.File
//
// Sets
//
//	o.id
//	o.modTime
//	o.size
//	o.sha1
func (o *Object) decodeMetaData(info *api.File) (err error) {
	return o.decodeMetaDataRaw(info.ID, info.SHA1, info.Size, info.UploadTimestamp, info.Info, info.ContentType)
}

// decodeMetaDataFileInfo sets the metadata in the object from an api.FileInfo
//
// Sets
//
//	o.id
//	o.modTime
//	o.size
//	o.sha1
func (o *Object) decodeMetaDataFileInfo(info *api.FileInfo) (err error) {
	return o.decodeMetaDataRaw(info.ID, info.SHA1, info.Size, info.UploadTimestamp, info.Info, info.ContentType)
}

// getMetaDataListing gets the metadata from the object unconditionally from the listing
//
// Note that listing is a class C transaction which costs more than
// the B transaction used in getMetaData
func (o *Object) getMetaDataListing(ctx context.Context) (info *api.File, err error) {
	bucket, bucketPath := o.split()
	maxSearched := 1
	var timestamp api.Timestamp
	if o.fs.opt.Versions {
		timestamp, bucketPath = api.RemoveVersion(bucketPath)
		maxSearched = maxVersions
	}

	err = o.fs.list(ctx, bucket, bucketPath, "", false, true, maxSearched, o.fs.opt.Versions, true, func(remote string, object *api.File, isDirectory bool) error {
		if isDirectory {
			return nil
		}
		if remote == bucketPath {
			if !timestamp.IsZero() && !timestamp.Equal(object.UploadTimestamp) {
				return nil
			}
			info = object
		}
		return errEndList // read only 1 item
	})
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	if info == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return info, nil
}

// getMetaData gets the metadata from the object unconditionally
func (o *Object) getMetaData(ctx context.Context) (info *api.File, err error) {
	// If using versions and have a version suffix, need to list the directory to find the correct versions
	if o.fs.opt.Versions {
		timestamp, _ := api.RemoveVersion(o.remote)
		if !timestamp.IsZero() {
			return o.getMetaDataListing(ctx)
		}
	}
	_, info, err = o.getOrHead(ctx, "HEAD", nil)
	return info, err
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// Sets
//
//	o.id
//	o.modTime
//	o.size
//	o.sha1
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.id != "" {
		return nil
	}
	info, err := o.getMetaData(ctx)
	if err != nil {
		return err
	}
	return o.decodeMetaData(info)
}

// timeString returns modTime as the number of milliseconds
// elapsed since January 1, 1970 UTC as a decimal string.
func timeString(modTime time.Time) string {
	return strconv.FormatInt(modTime.UnixNano()/1e6, 10)
}

// parseTimeStringHelper converts a decimal string number of milliseconds
// elapsed since January 1, 1970 UTC into a time.Time
func parseTimeStringHelper(timeString string) (time.Time, error) {
	unixMilliseconds, err := strconv.ParseInt(timeString, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(unixMilliseconds/1e3, (unixMilliseconds%1e3)*1e6).UTC(), nil
}

// parseTimeString converts a decimal string number of milliseconds
// elapsed since January 1, 1970 UTC into a time.Time and stores it in
// the modTime variable.
func (o *Object) parseTimeString(timeString string) (err error) {
	if timeString == "" {
		return nil
	}
	modTime, err := parseTimeStringHelper(timeString)
	if err != nil {
		fs.Debugf(o, "Failed to parse mod time string %q: %v", timeString, err)
		return nil
	}
	o.modTime = modTime
	return nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
//
// SHA-1 will also be updated once the request has completed.
func (o *Object) ModTime(ctx context.Context) (result time.Time) {
	// The error is logged in readMetaData
	_ = o.readMetaData(ctx)
	return o.modTime
}

// SetModTime sets the modification time of the Object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	info, err := o.getMetaData(ctx)
	if err != nil {
		return err
	}
	info.Info[timeKey] = timeString(modTime)

	// Copy to the same name, overwriting the metadata only
	return o.fs.copy(ctx, o, o, info)
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
		return fmt.Errorf("corrupted on transfer: lengths differ want %d vs got %d", file.o.Size(), file.bytes)
	}

	// Check the SHA1
	receivedSHA1 := file.o.sha1
	calculatedSHA1 := fmt.Sprintf("%x", file.hash.Sum(nil))
	if receivedSHA1 != "" && receivedSHA1 != calculatedSHA1 {
		return fmt.Errorf("corrupted on transfer: SHA1 hashes differ want %q vs got %q", receivedSHA1, calculatedSHA1)
	}

	return nil
}

// Check it satisfies the interfaces
var _ io.ReadCloser = &openFile{}

func (o *Object) getOrHead(ctx context.Context, method string, options []fs.OpenOption) (resp *http.Response, info *api.File, err error) {
	opts := rest.Opts{
		Method:     method,
		Options:    options,
		NoResponse: method == "HEAD",
	}

	// Use downloadUrl from backblaze if downloadUrl is not set
	// otherwise use the custom downloadUrl
	if o.fs.opt.DownloadURL == "" {
		opts.RootURL = o.fs.info.DownloadURL
	} else {
		opts.RootURL = o.fs.opt.DownloadURL
	}

	// Download by id if set and not using DownloadURL otherwise by name
	if o.id != "" && o.fs.opt.DownloadURL == "" {
		opts.Path += "/b2api/v1/b2_download_file_by_id?fileId=" + urlEncode(o.id)
	} else {
		bucket, bucketPath := o.split()
		opts.Path += "/file/" + urlEncode(o.fs.opt.Enc.FromStandardName(bucket)) + "/" + urlEncode(o.fs.opt.Enc.FromStandardPath(bucketPath))
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		// 404 for files, 400 for directories
		if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest) {
			return nil, nil, fs.ErrorObjectNotFound
		}
		return nil, nil, fmt.Errorf("failed to %s for download: %w", method, err)
	}

	// NB resp may be Open here - don't return err != nil without closing

	// Convert the Headers into an api.File
	var uploadTimestamp api.Timestamp
	err = uploadTimestamp.UnmarshalJSON([]byte(resp.Header.Get(timestampHeader)))
	if err != nil {
		fs.Debugf(o, "Bad "+timestampHeader+" header: %v", err)
	}
	var Info = make(map[string]string)
	for k, vs := range resp.Header {
		k = strings.ToLower(k)
		for _, v := range vs {
			if strings.HasPrefix(k, headerPrefix) {
				Info[k[len(headerPrefix):]] = v
			}
		}
	}
	info = &api.File{
		ID:              resp.Header.Get(idHeader),
		Name:            resp.Header.Get(nameHeader),
		Action:          "upload",
		Size:            resp.ContentLength,
		UploadTimestamp: uploadTimestamp,
		SHA1:            resp.Header.Get(sha1Header),
		ContentType:     resp.Header.Get("Content-Type"),
		Info:            Info,
	}

	// When reading files from B2 via cloudflare using
	// --b2-download-url cloudflare strips the Content-Length
	// headers (presumably so it can inject stuff) so use the old
	// length read from the listing.
	if info.Size < 0 {
		info.Size = o.size
	}
	return resp, info, nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	fs.FixRangeOption(options, o.size)

	resp, info, err := o.getOrHead(ctx, "GET", options)
	if err != nil {
		return nil, err
	}

	// Don't check length or hash or metadata on partial content
	if resp.StatusCode == http.StatusPartialContent {
		return resp.Body, nil
	}

	err = o.decodeMetaData(info)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
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
	for i := range len(in) {
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
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	if o.fs.opt.Versions {
		return errNotWithVersions
	}
	if o.fs.opt.VersionAt.IsSet() {
		return errNotWithVersionAt
	}
	size := src.Size()

	bucket, bucketPath := o.split()
	err = o.fs.makeBucket(ctx, bucket)
	if err != nil {
		return err
	}
	if size < 0 {
		// Check if the file is large enough for a chunked upload (needs to be at least two chunks)
		rw := o.fs.getRW(false)

		n, err := io.CopyN(rw, in, int64(o.fs.opt.ChunkSize))
		if err == nil {
			bufReader := bufio.NewReader(in)
			in = bufReader
			_, err = bufReader.Peek(1)
		}

		if err == nil {
			fs.Debugf(o, "File is big enough for chunked streaming")
			up, err := o.fs.newLargeUpload(ctx, o, in, src, o.fs.opt.ChunkSize, false, nil, options...)
			if err != nil {
				o.fs.putRW(rw)
				return err
			}
			// NB Stream returns the buffer and token
			err = up.Stream(ctx, rw)
			if err != nil {
				return err
			}
			return o.decodeMetaDataFileInfo(up.info)
		} else if err == io.EOF {
			fs.Debugf(o, "File has %d bytes, which makes only one chunk. Using direct upload.", n)
			defer o.fs.putRW(rw)
			size = n
			in = rw
		} else {
			o.fs.putRW(rw)
			return err
		}
	} else if size > int64(o.fs.opt.UploadCutoff) {
		chunkWriter, err := multipart.UploadMultipart(ctx, src, in, multipart.UploadMultipartOptions{
			Open:        o.fs,
			OpenOptions: options,
		})
		if err != nil {
			return err
		}
		up := chunkWriter.(*largeUpload)
		return o.decodeMetaDataFileInfo(up.info)
	}

	modTime, err := o.getModTime(ctx, src, options)
	if err != nil {
		return err
	}

	calculatedSha1, _ := src.Hash(ctx, hash.SHA1)
	if calculatedSha1 == "" {
		calculatedSha1 = "hex_digits_at_end"
		har := newHashAppendingReader(in, sha1.New())
		size += int64(har.AdditionalLength())
		in = har
	}

	// Get upload URL
	upload, err := o.fs.getUploadURL(ctx, bucket)
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
	// Content-Type mappings can be pursued here.
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
		Options: options,
		ExtraHeaders: map[string]string{
			"Authorization":  upload.AuthorizationToken,
			"X-Bz-File-Name": urlEncode(o.fs.opt.Enc.FromStandardPath(bucketPath)),
			"Content-Type":   fs.MimeType(ctx, src),
			sha1Header:       calculatedSha1,
			timeHeader:       timeString(modTime),
		},
		ContentLength: &size,
	}
	var response api.FileInfo
	// Don't retry, return a retry error instead
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &response)
		retry, err := o.fs.shouldRetry(ctx, resp, err)
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

// Get modTime from the source; if --metadata is set, fetch the src metadata and get it from there.
// When metadata support is added to b2, this method will need a more generic name
func (o *Object) getModTime(ctx context.Context, src fs.ObjectInfo, options []fs.OpenOption) (time.Time, error) {
	modTime := src.ModTime(ctx)

	// Fetch metadata if --metadata is in use
	meta, err := fs.GetMetadataOptions(ctx, o.fs, src, options)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read metadata from source object: %w", err)
	}
	// merge metadata into request and user metadata
	for k, v := range meta {
		k = strings.ToLower(k)
		// For now, the only metadata we're concerned with is "mtime"
		switch k {
		case "mtime":
			// mtime in meta overrides source ModTime
			metaModTime, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				fs.Debugf(o, "failed to parse metadata %s: %q: %v", k, v, err)
			} else {
				modTime = metaModTime
			}
		default:
			// Do nothing for now
		}
	}
	return modTime, nil
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	// FIXME what if file is smaller than 1 chunk?
	if f.opt.Versions {
		return info, nil, errNotWithVersions
	}
	if f.opt.VersionAt.IsSet() {
		return info, nil, errNotWithVersionAt
	}
	//size := src.Size()

	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}

	bucket, _ := o.split()
	err = f.makeBucket(ctx, bucket)
	if err != nil {
		return info, nil, err
	}

	info = fs.ChunkWriterInfo{
		ChunkSize:   int64(f.opt.ChunkSize),
		Concurrency: o.fs.opt.UploadConcurrency,
		//LeavePartsOnError: o.fs.opt.LeavePartsOnError,
	}
	up, err := f.newLargeUpload(ctx, o, nil, src, f.opt.ChunkSize, false, nil, options...)
	return info, up, err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	bucket, bucketPath := o.split()
	if o.fs.opt.Versions {
		return errNotWithVersions
	}
	if o.fs.opt.VersionAt.IsSet() {
		return errNotWithVersionAt
	}
	if o.fs.opt.HardDelete {
		return o.fs.deleteByID(ctx, o.id, bucketPath)
	}
	return o.fs.hide(ctx, bucket, bucketPath)
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

var lifecycleHelp = fs.CommandHelp{
	Name:  "lifecycle",
	Short: "Read or set the lifecycle for a bucket",
	Long: `This command can be used to read or set the lifecycle for a bucket.

Usage Examples:

To show the current lifecycle rules:

    rclone backend lifecycle b2:bucket

This will dump something like this showing the lifecycle rules.

    [
        {
            "daysFromHidingToDeleting": 1,
            "daysFromUploadingToHiding": null,
            "daysFromStartingToCancelingUnfinishedLargeFiles": null,
            "fileNamePrefix": ""
        }
    ]

If there are no lifecycle rules (the default) then it will just return [].

To reset the current lifecycle rules:

    rclone backend lifecycle b2:bucket -o daysFromHidingToDeleting=30
    rclone backend lifecycle b2:bucket -o daysFromUploadingToHiding=5 -o daysFromHidingToDeleting=1

This will run and then print the new lifecycle rules as above.

Rclone only lets you set lifecycles for the whole bucket with the
fileNamePrefix = "".

You can't disable versioning with B2. The best you can do is to set
the daysFromHidingToDeleting to 1 day. You can enable hard_delete in
the config also which will mean deletions won't cause versions but
overwrites will still cause versions to be made.

    rclone backend lifecycle b2:bucket -o daysFromHidingToDeleting=1

See: https://www.backblaze.com/docs/cloud-storage-lifecycle-rules
`,
	Opts: map[string]string{
		"daysFromHidingToDeleting":                        "After a file has been hidden for this many days it is deleted. 0 is off.",
		"daysFromUploadingToHiding":                       "This many days after uploading a file is hidden",
		"daysFromStartingToCancelingUnfinishedLargeFiles": "Cancels any unfinished large file versions after this many days",
	},
}

func (f *Fs) lifecycleCommand(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
	var newRule api.LifecycleRule
	if daysStr := opt["daysFromHidingToDeleting"]; daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return nil, fmt.Errorf("bad daysFromHidingToDeleting: %w", err)
		}
		newRule.DaysFromHidingToDeleting = &days
	}
	if daysStr := opt["daysFromUploadingToHiding"]; daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return nil, fmt.Errorf("bad daysFromUploadingToHiding: %w", err)
		}
		newRule.DaysFromUploadingToHiding = &days
	}
	if daysStr := opt["daysFromStartingToCancelingUnfinishedLargeFiles"]; daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return nil, fmt.Errorf("bad daysFromStartingToCancelingUnfinishedLargeFiles: %w", err)
		}
		newRule.DaysFromStartingToCancelingUnfinishedLargeFiles = &days
	}
	bucketName, _ := f.split("")
	if bucketName == "" {
		return nil, errors.New("bucket required")

	}

	skip := operations.SkipDestructive(ctx, name, "update lifecycle rules")

	var bucket *api.Bucket
	if !skip && (newRule.DaysFromHidingToDeleting != nil || newRule.DaysFromUploadingToHiding != nil || newRule.DaysFromStartingToCancelingUnfinishedLargeFiles != nil) {
		bucketID, err := f.getBucketID(ctx, bucketName)
		if err != nil {
			return nil, err
		}
		opts := rest.Opts{
			Method: "POST",
			Path:   "/b2_update_bucket",
		}
		var request = api.UpdateBucketRequest{
			ID:             bucketID,
			AccountID:      f.info.AccountID,
			LifecycleRules: []api.LifecycleRule{newRule},
		}
		var response api.Bucket
		err = f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CallJSON(ctx, &opts, &request, &response)
			return f.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return nil, err
		}
		bucket = &response
	} else {
		err = f.listBucketsToFn(ctx, bucketName, func(b *api.Bucket) error {
			bucket = b
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if bucket == nil {
		return nil, fs.ErrorDirNotFound
	}
	return bucket.LifecycleRules, nil
}

var cleanupHelp = fs.CommandHelp{
	Name:  "cleanup",
	Short: "Remove unfinished large file uploads.",
	Long: `This command removes unfinished large file uploads of age greater than
max-age, which defaults to 24 hours.

Note that you can use --interactive/-i or --dry-run with this command to see what
it would do.

    rclone backend cleanup b2:bucket/path/to/object
    rclone backend cleanup -o max-age=7w b2:bucket/path/to/object

Durations are parsed as per the rest of rclone, 2h, 7d, 7w etc.
`,
	Opts: map[string]string{
		"max-age": "Max age of upload to delete",
	},
}

func (f *Fs) cleanupCommand(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
	maxAge := defaultMaxAge
	if opt["max-age"] != "" {
		maxAge, err = fs.ParseDuration(opt["max-age"])
		if err != nil {
			return nil, fmt.Errorf("bad max-age: %w", err)
		}
	}
	return nil, f.cleanUp(ctx, false, true, maxAge)
}

var cleanupHiddenHelp = fs.CommandHelp{
	Name:  "cleanup-hidden",
	Short: "Remove old versions of files.",
	Long: `This command removes any old hidden versions of files.

Note that you can use --interactive/-i or --dry-run with this command to see what
it would do.

    rclone backend cleanup-hidden b2:bucket/path/to/dir
`,
}

func (f *Fs) cleanupHiddenCommand(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
	return nil, f.cleanUp(ctx, true, false, 0)
}

var commandHelp = []fs.CommandHelp{
	lifecycleHelp,
	cleanupHelp,
	cleanupHiddenHelp,
}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
	switch name {
	case "lifecycle":
		return f.lifecycleCommand(ctx, name, arg, opt)
	case "cleanup":
		return f.cleanupCommand(ctx, name, arg, opt)
	case "cleanup-hidden":
		return f.cleanupHiddenCommand(ctx, name, arg, opt)
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.Purger          = &Fs{}
	_ fs.Copier          = &Fs{}
	_ fs.PutStreamer     = &Fs{}
	_ fs.CleanUpper      = &Fs{}
	_ fs.ListRer         = &Fs{}
	_ fs.PublicLinker    = &Fs{}
	_ fs.OpenChunkWriter = &Fs{}
	_ fs.Commander       = &Fs{}
	_ fs.Object          = &Object{}
	_ fs.MimeTyper       = &Object{}
	_ fs.IDer            = &Object{}
)
