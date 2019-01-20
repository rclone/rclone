// Package azureblob provides an interface to the Microsoft Azure blob object storage system

// +build !plan9,!solaris,go1.8

package azureblob

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/walk"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/pkg/errors"
)

const (
	minSleep              = 10 * time.Millisecond
	maxSleep              = 10 * time.Second
	decayConstant         = 1    // bigger for slower decay, exponential
	maxListChunkSize      = 5000 // number of items to read at once
	modTimeKey            = "mtime"
	timeFormatIn          = time.RFC3339
	timeFormatOut         = "2006-01-02T15:04:05.000000000Z07:00"
	maxTotalParts         = 50000 // in multipart upload
	storageDefaultBaseURL = "blob.core.windows.net"
	// maxUncommittedSize = 9 << 30 // can't upload bigger than this
	defaultChunkSize    = 4 * fs.MebiByte
	maxChunkSize        = 100 * fs.MebiByte
	defaultUploadCutoff = 256 * fs.MebiByte
	maxUploadCutoff     = 256 * fs.MebiByte
	defaultAccessTier   = azblob.AccessTierNone
	maxTryTimeout       = time.Hour * 24 * 365 //max time of an azure web request response window (whether or not data is flowing)
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "azureblob",
		Description: "Microsoft Azure Blob Storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "account",
			Help: "Storage Account Name (leave blank to use connection string or SAS URL)",
		}, {
			Name: "key",
			Help: "Storage Account Key (leave blank to use connection string or SAS URL)",
		}, {
			Name: "sas_url",
			Help: "SAS URL for container level access only\n(leave blank if using account/key or connection string)",
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for the service\nLeave blank normally.",
			Advanced: true,
		}, {
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to chunked upload (<= 256MB).",
			Default:  fs.SizeSuffix(defaultUploadCutoff),
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Upload chunk size (<= 100MB).

Note that this is stored in memory and there may be up to
"--transfers" chunks stored at once in memory.`,
			Default:  fs.SizeSuffix(defaultChunkSize),
			Advanced: true,
		}, {
			Name: "list_chunk",
			Help: `Size of blob list.

This sets the number of blobs requested in each listing chunk. Default
is the maximum, 5000. "List blobs" requests are permitted 2 minutes
per megabyte to complete. If an operation is taking longer than 2
minutes per megabyte on average, it will time out (
[source](https://docs.microsoft.com/en-us/rest/api/storageservices/setting-timeouts-for-blob-service-operations#exceptions-to-default-timeout-interval)
). This can be used to limit the number of blobs items to return, to
avoid the time out.`,
			Default:  maxListChunkSize,
			Advanced: true,
		}, {
			Name: "access_tier",
			Help: `Access tier of blob: hot, cool or archive.

Archived blobs can be restored by setting access tier to hot or
cool. Leave blank if you intend to use default access tier, which is
set at account level

If there is no "access tier" specified, rclone doesn't apply any tier.
rclone performs "Set Tier" operation on blobs while uploading, if objects
are not modified, specifying "access tier" to new one will have no effect.
If blobs are in "archive tier" at remote, trying to perform data transfer
operations from remote will not be allowed. User should first restore by
tiering blob to "Hot" or "Cool".`,
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Account       string        `config:"account"`
	Key           string        `config:"key"`
	Endpoint      string        `config:"endpoint"`
	SASURL        string        `config:"sas_url"`
	UploadCutoff  fs.SizeSuffix `config:"upload_cutoff"`
	ChunkSize     fs.SizeSuffix `config:"chunk_size"`
	ListChunkSize uint          `config:"list_chunk"`
	AccessTier    string        `config:"access_tier"`
}

// Fs represents a remote azure server
type Fs struct {
	name             string                // name of this remote
	root             string                // the path we are working on if any
	opt              Options               // parsed config options
	features         *fs.Features          // optional features
	client           *http.Client          // http client we are using
	svcURL           *azblob.ServiceURL    // reference to serviceURL
	cntURL           *azblob.ContainerURL  // reference to containerURL
	container        string                // the container we are working on
	containerOKMu    sync.Mutex            // mutex to protect container OK
	containerOK      bool                  // true if we have created the container
	containerDeleted bool                  // true if we have deleted the container
	pacer            *pacer.Pacer          // To pace and retry the API calls
	uploadToken      *pacer.TokenDispenser // control concurrency
}

// Object describes a azure object
type Object struct {
	fs         *Fs                   // what this object is part of
	remote     string                // The remote path
	modTime    time.Time             // The modified time of the object if known
	md5        string                // MD5 hash if known
	size       int64                 // Size of the object
	mimeType   string                // Content-Type of the object
	accessTier azblob.AccessTierType // Blob Access Tier
	meta       map[string]string     // blob metadata
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
		return fmt.Sprintf("Azure container %s", f.container)
	}
	return fmt.Sprintf("Azure container %s path %s", f.container, f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Pattern to match a azure path
var matcher = regexp.MustCompile(`^/*([^/]*)(.*)$`)

// parseParse parses a azure 'url'
func parsePath(path string) (container, directory string, err error) {
	parts := matcher.FindStringSubmatch(path)
	if parts == nil {
		err = errors.Errorf("couldn't find container in azure path %q", path)
	} else {
		container, directory = parts[1], parts[2]
		directory = strings.Trim(directory, "/")
	}
	return
}

// validateAccessTier checks if azureblob supports user supplied tier
func validateAccessTier(tier string) bool {
	switch tier {
	case string(azblob.AccessTierHot),
		string(azblob.AccessTierCool),
		string(azblob.AccessTierArchive):
		// valid cases
		return true
	default:
		return false
	}
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

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(err error) (bool, error) {
	// FIXME interpret special errors - more to do here
	if storageErr, ok := err.(azblob.StorageError); ok {
		statusCode := storageErr.Response().StatusCode
		for _, e := range retryErrorCodes {
			if statusCode == e {
				return true, err
			}
		}
	}
	return fserrors.ShouldRetry(err), err
}

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	const minChunkSize = fs.Byte
	if cs < minChunkSize {
		return errors.Errorf("%s is less than %s", cs, minChunkSize)
	}
	if cs > maxChunkSize {
		return errors.Errorf("%s is greater than %s", cs, maxChunkSize)
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

func checkUploadCutoff(cs fs.SizeSuffix) error {
	if cs > maxUploadCutoff {
		return errors.Errorf("%v must be less than or equal to %v", cs, maxUploadCutoff)
	}
	return nil
}

func (f *Fs) setUploadCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadCutoff(cs)
	if err == nil {
		old, f.opt.UploadCutoff = f.opt.UploadCutoff, cs
	}
	return
}

// httpClientFactory creates a Factory object that sends HTTP requests
// to a rclone's http.Client.
//
// copied from azblob.newDefaultHTTPClientFactory
func httpClientFactory(client *http.Client) pipeline.Factory {
	return pipeline.FactoryFunc(func(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.PolicyFunc {
		return func(ctx context.Context, request pipeline.Request) (pipeline.Response, error) {
			r, err := client.Do(request.WithContext(ctx))
			if err != nil {
				err = pipeline.NewError(err, "HTTP request failed")
			}
			return pipeline.NewHTTPResponse(r), err
		}
	})
}

// newPipeline creates a Pipeline using the specified credentials and options.
//
// this code was copied from azblob.NewPipeline
func (f *Fs) newPipeline(c azblob.Credential, o azblob.PipelineOptions) pipeline.Pipeline {
	// Closest to API goes first; closest to the wire goes last
	factories := []pipeline.Factory{
		azblob.NewTelemetryPolicyFactory(o.Telemetry),
		azblob.NewUniqueRequestIDPolicyFactory(),
		azblob.NewRetryPolicyFactory(o.Retry),
		c,
		pipeline.MethodFactoryMarker(), // indicates at what stage in the pipeline the method factory is invoked
		azblob.NewRequestLogPolicyFactory(o.RequestLog),
	}
	return pipeline.NewPipeline(factories, pipeline.Options{HTTPSender: httpClientFactory(f.client), Log: o.Log})
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	err = checkUploadCutoff(opt.UploadCutoff)
	if err != nil {
		return nil, errors.Wrap(err, "azure: upload cutoff")
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, errors.Wrap(err, "azure: chunk size")
	}
	if opt.ListChunkSize > maxListChunkSize {
		return nil, errors.Errorf("azure: blob list size can't be greater than %v - was %v", maxListChunkSize, opt.ListChunkSize)
	}
	container, directory, err := parsePath(root)
	if err != nil {
		return nil, err
	}
	if opt.Endpoint == "" {
		opt.Endpoint = storageDefaultBaseURL
	}

	if opt.AccessTier == "" {
		opt.AccessTier = string(defaultAccessTier)
	} else if !validateAccessTier(opt.AccessTier) {
		return nil, errors.Errorf("Azure Blob: Supported access tiers are %s, %s and %s",
			string(azblob.AccessTierHot), string(azblob.AccessTierCool), string(azblob.AccessTierArchive))
	}

	f := &Fs{
		name:        name,
		opt:         *opt,
		container:   container,
		root:        directory,
		pacer:       pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant).SetPacer(pacer.S3Pacer),
		uploadToken: pacer.NewTokenDispenser(fs.Config.Transfers),
		client:      fshttp.NewClient(fs.Config),
	}
	f.features = (&fs.Features{
		ReadMimeType:  true,
		WriteMimeType: true,
		BucketBased:   true,
		SetTier:       true,
		GetTier:       true,
	}).Fill(f)

	var (
		u            *url.URL
		serviceURL   azblob.ServiceURL
		containerURL azblob.ContainerURL
	)
	switch {
	case opt.Account != "" && opt.Key != "":
		credential, err := azblob.NewSharedKeyCredential(opt.Account, opt.Key)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse credentials")
		}

		u, err = url.Parse(fmt.Sprintf("https://%s.%s", opt.Account, opt.Endpoint))
		if err != nil {
			return nil, errors.Wrap(err, "failed to make azure storage url from account and endpoint")
		}
		pipeline := f.newPipeline(credential, azblob.PipelineOptions{Retry: azblob.RetryOptions{TryTimeout: maxTryTimeout}})
		serviceURL = azblob.NewServiceURL(*u, pipeline)
		containerURL = serviceURL.NewContainerURL(container)
	case opt.SASURL != "":
		u, err = url.Parse(opt.SASURL)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse SAS URL")
		}
		// use anonymous credentials in case of sas url
		pipeline := f.newPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{Retry: azblob.RetryOptions{TryTimeout: maxTryTimeout}})
		// Check if we have container level SAS or account level sas
		parts := azblob.NewBlobURLParts(*u)
		if parts.ContainerName != "" {
			if container != "" && parts.ContainerName != container {
				return nil, errors.New("Container name in SAS URL and container provided in command do not match")
			}

			container = parts.ContainerName
			containerURL = azblob.NewContainerURL(*u, pipeline)
		} else {
			serviceURL = azblob.NewServiceURL(*u, pipeline)
			containerURL = serviceURL.NewContainerURL(container)
		}
	default:
		return nil, errors.New("Need account+key or connectionString or sasURL")
	}
	f.svcURL = &serviceURL
	f.cntURL = &containerURL

	if f.root != "" {
		f.root += "/"
		// Check to see if the (container,directory) is actually an existing file
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
			if err == fs.ErrorObjectNotFound || err == fs.ErrorNotAFile {
				// File doesn't exist or is a directory so return old f
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

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *azblob.BlobItem) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		err := o.decodeMetaDataFromBlob(info)
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

// getBlobReference creates an empty blob reference with no metadata
func (f *Fs) getBlobReference(remote string) azblob.BlobURL {
	return f.cntURL.NewBlobURL(f.root + remote)
}

// updateMetadataWithModTime adds the modTime passed in to o.meta.
func (o *Object) updateMetadataWithModTime(modTime time.Time) {
	// Make sure o.meta is not nil
	if o.meta == nil {
		o.meta = make(map[string]string, 1)
	}

	// Set modTimeKey in it
	o.meta[modTimeKey] = modTime.Format(timeFormatOut)
}

// Returns whether file is a directory marker or not
func isDirectoryMarker(size int64, metadata azblob.Metadata, remote string) bool {
	// Directory markers are 0 length
	if size == 0 {
		// Note that metadata with hdi_isfolder = true seems to be a
		// defacto standard for marking blobs as directories.
		endsWithSlash := strings.HasSuffix(remote, "/")
		if endsWithSlash || remote == "" || metadata["hdi_isfolder"] == "true" {
			return true
		}

	}
	return false
}

// listFn is called from list to handle an object
type listFn func(remote string, object *azblob.BlobItem, isDirectory bool) error

// list lists the objects into the function supplied from
// the container and root supplied
//
// dir is the starting directory, "" for root
func (f *Fs) list(dir string, recurse bool, maxResults uint, fn listFn) error {
	f.containerOKMu.Lock()
	deleted := f.containerDeleted
	f.containerOKMu.Unlock()
	if deleted {
		return fs.ErrorDirNotFound
	}
	root := f.root
	if dir != "" {
		root += dir + "/"
	}
	delimiter := ""
	if !recurse {
		delimiter = "/"
	}

	options := azblob.ListBlobsSegmentOptions{
		Details: azblob.BlobListingDetails{
			Copy:             false,
			Metadata:         true,
			Snapshots:        false,
			UncommittedBlobs: false,
			Deleted:          false,
		},
		Prefix:     root,
		MaxResults: int32(maxResults),
	}
	ctx := context.Background()
	directoryMarkers := map[string]struct{}{}
	for marker := (azblob.Marker{}); marker.NotDone(); {
		var response *azblob.ListBlobsHierarchySegmentResponse
		err := f.pacer.Call(func() (bool, error) {
			var err error
			response, err = f.cntURL.ListBlobsHierarchySegment(ctx, marker, delimiter, options)
			return f.shouldRetry(err)
		})

		if err != nil {
			// Check http error code along with service code, current SDK doesn't populate service code correctly sometimes
			if storageErr, ok := err.(azblob.StorageError); ok && (storageErr.ServiceCode() == azblob.ServiceCodeContainerNotFound || storageErr.Response().StatusCode == http.StatusNotFound) {
				return fs.ErrorDirNotFound
			}
			return err
		}
		// Advance marker to next
		marker = response.NextMarker

		for i := range response.Segment.BlobItems {
			file := &response.Segment.BlobItems[i]
			// Finish if file name no longer has prefix
			// if prefix != "" && !strings.HasPrefix(file.Name, prefix) {
			// 	return nil
			// }
			if !strings.HasPrefix(file.Name, f.root) {
				fs.Debugf(f, "Odd name received %q", file.Name)
				continue
			}
			remote := file.Name[len(f.root):]
			if isDirectoryMarker(*file.Properties.ContentLength, file.Metadata, remote) {
				if strings.HasSuffix(remote, "/") {
					remote = remote[:len(remote)-1]
				}
				err = fn(remote, file, true)
				if err != nil {
					return err
				}
				// Keep track of directory markers. If recursing then
				// there will be no Prefixes so no need to keep track
				if !recurse {
					directoryMarkers[remote] = struct{}{}
				}
				continue // skip directory marker
			}
			// Send object
			err = fn(remote, file, false)
			if err != nil {
				return err
			}
		}
		// Send the subdirectories
		for _, remote := range response.Segment.BlobPrefixes {
			remote := strings.TrimRight(remote.Name, "/")
			if !strings.HasPrefix(remote, f.root) {
				fs.Debugf(f, "Odd directory name received %q", remote)
				continue
			}
			remote = remote[len(f.root):]
			// Don't send if already sent as a directory marker
			if _, found := directoryMarkers[remote]; found {
				continue
			}
			// Send object
			err = fn(remote, nil, true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(remote string, object *azblob.BlobItem, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		d := fs.NewDir(remote, time.Time{})
		return d, nil
	}
	o, err := f.newObjectWithInfo(remote, object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// mark the container as being OK
func (f *Fs) markContainerOK() {
	if f.container != "" {
		f.containerOKMu.Lock()
		f.containerOK = true
		f.containerDeleted = false
		f.containerOKMu.Unlock()
	}
}

// listDir lists a single directory
func (f *Fs) listDir(dir string) (entries fs.DirEntries, err error) {
	err = f.list(dir, false, f.opt.ListChunkSize, func(remote string, object *azblob.BlobItem, isDirectory bool) error {
		entry, err := f.itemToDirEntry(remote, object, isDirectory)
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
	// container must be present if listing succeeded
	f.markContainerOK()
	return entries, nil
}

// listContainers returns all the containers to out
func (f *Fs) listContainers(dir string) (entries fs.DirEntries, err error) {
	if dir != "" {
		return nil, fs.ErrorListBucketRequired
	}
	err = f.listContainersToFn(func(container *azblob.ContainerItem) error {
		d := fs.NewDir(container.Name, container.Properties.LastModified)
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
	if f.container == "" {
		return f.listContainers(dir)
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
	if f.container == "" {
		return fs.ErrorListBucketRequired
	}
	list := walk.NewListRHelper(callback)
	err = f.list(dir, true, f.opt.ListChunkSize, func(remote string, object *azblob.BlobItem, isDirectory bool) error {
		entry, err := f.itemToDirEntry(remote, object, isDirectory)
		if err != nil {
			return err
		}
		return list.Add(entry)
	})
	if err != nil {
		return err
	}
	// container must be present if listing succeeded
	f.markContainerOK()
	return list.Flush()
}

// listContainerFn is called from listContainersToFn to handle a container
type listContainerFn func(*azblob.ContainerItem) error

// listContainersToFn lists the containers to the function supplied
func (f *Fs) listContainersToFn(fn listContainerFn) error {
	params := azblob.ListContainersSegmentOptions{
		MaxResults: int32(f.opt.ListChunkSize),
	}
	ctx := context.Background()
	for marker := (azblob.Marker{}); marker.NotDone(); {
		var response *azblob.ListContainersSegmentResponse
		err := f.pacer.Call(func() (bool, error) {
			var err error
			response, err = f.svcURL.ListContainersSegment(ctx, marker, params)
			return f.shouldRetry(err)
		})
		if err != nil {
			return err
		}

		for i := range response.ContainerItems {
			err = fn(&response.ContainerItems[i])
			if err != nil {
				return err
			}
		}
		marker = response.NextMarker
	}

	return nil
}

// Put the object into the container
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

// Check if the container exists
//
// NB this can return incorrect results if called immediately after container deletion
func (f *Fs) dirExists() (bool, error) {
	options := azblob.ListBlobsSegmentOptions{
		Details: azblob.BlobListingDetails{
			Copy:             false,
			Metadata:         false,
			Snapshots:        false,
			UncommittedBlobs: false,
			Deleted:          false,
		},
		MaxResults: 1,
	}
	err := f.pacer.Call(func() (bool, error) {
		ctx := context.Background()
		_, err := f.cntURL.ListBlobsHierarchySegment(ctx, azblob.Marker{}, "", options)
		return f.shouldRetry(err)
	})
	if err == nil {
		return true, nil
	}
	// Check http error code along with service code, current SDK doesn't populate service code correctly sometimes
	if storageErr, ok := err.(azblob.StorageError); ok && (storageErr.ServiceCode() == azblob.ServiceCodeContainerNotFound || storageErr.Response().StatusCode == http.StatusNotFound) {
		return false, nil
	}
	return false, err
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	f.containerOKMu.Lock()
	defer f.containerOKMu.Unlock()
	if f.containerOK {
		return nil
	}
	if !f.containerDeleted {
		exists, err := f.dirExists()
		if err == nil {
			f.containerOK = exists
		}
		if err != nil || exists {
			return err
		}
	}

	// now try to create the container
	err := f.pacer.Call(func() (bool, error) {
		ctx := context.Background()
		_, err := f.cntURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
		if err != nil {
			if storageErr, ok := err.(azblob.StorageError); ok {
				switch storageErr.ServiceCode() {
				case azblob.ServiceCodeContainerAlreadyExists:
					f.containerOK = true
					return false, nil
				case azblob.ServiceCodeContainerBeingDeleted:
					// From https://docs.microsoft.com/en-us/rest/api/storageservices/delete-container
					// When a container is deleted, a container with the same name cannot be created
					// for at least 30 seconds; the container may not be available for more than 30
					// seconds if the service is still processing the request.
					time.Sleep(6 * time.Second) // default 10 retries will be 60 seconds
					f.containerDeleted = true
					return true, err
				}
			}
		}
		return f.shouldRetry(err)
	})
	if err == nil {
		f.containerOK = true
		f.containerDeleted = false
	}
	return errors.Wrap(err, "failed to make container")
}

// isEmpty checks to see if a given directory is empty and returns an error if not
func (f *Fs) isEmpty(dir string) (err error) {
	empty := true
	err = f.list(dir, true, 1, func(remote string, object *azblob.BlobItem, isDirectory bool) error {
		empty = false
		return nil
	})
	if err != nil {
		return err
	}
	if !empty {
		return fs.ErrorDirectoryNotEmpty
	}
	return nil
}

// deleteContainer deletes the container.  It can delete a full
// container so use isEmpty if you don't want that.
func (f *Fs) deleteContainer() error {
	f.containerOKMu.Lock()
	defer f.containerOKMu.Unlock()
	options := azblob.ContainerAccessConditions{}
	ctx := context.Background()
	err := f.pacer.Call(func() (bool, error) {
		_, err := f.cntURL.GetProperties(ctx, azblob.LeaseAccessConditions{})
		if err == nil {
			_, err = f.cntURL.Delete(ctx, options)
		}

		if err != nil {
			// Check http error code along with service code, current SDK doesn't populate service code correctly sometimes
			if storageErr, ok := err.(azblob.StorageError); ok && (storageErr.ServiceCode() == azblob.ServiceCodeContainerNotFound || storageErr.Response().StatusCode == http.StatusNotFound) {
				return false, fs.ErrorDirNotFound
			}

			return f.shouldRetry(err)
		}

		return f.shouldRetry(err)
	})
	if err == nil {
		f.containerOK = false
		f.containerDeleted = true
	}
	return errors.Wrap(err, "failed to delete container")
}

// Rmdir deletes the container if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	err := f.isEmpty(dir)
	if err != nil {
		return err
	}
	if f.root != "" || dir != "" {
		return nil
	}
	return f.deleteContainer()
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// Purge deletes all the files and directories including the old versions.
func (f *Fs) Purge() error {
	dir := "" // forward compat!
	if f.root != "" || dir != "" {
		// Delegate to caller if not root container
		return fs.ErrorCantPurge
	}
	return f.deleteContainer()
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
	err := f.Mkdir("")
	if err != nil {
		return nil, err
	}
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	dstBlobURL := f.getBlobReference(remote)
	srcBlobURL := srcObj.getBlobReference()

	source, err := url.Parse(srcBlobURL.String())
	if err != nil {
		return nil, err
	}

	options := azblob.BlobAccessConditions{}
	ctx := context.Background()
	var startCopy *azblob.BlobStartCopyFromURLResponse

	err = f.pacer.Call(func() (bool, error) {
		startCopy, err = dstBlobURL.StartCopyFromURL(ctx, *source, nil, azblob.ModifiedAccessConditions{}, options)
		return f.shouldRetry(err)
	})
	if err != nil {
		return nil, err
	}

	copyStatus := startCopy.CopyStatus()
	for copyStatus == azblob.CopyStatusPending {
		time.Sleep(1 * time.Second)
		getMetadata, err := dstBlobURL.GetProperties(ctx, options)
		if err != nil {
			return nil, err
		}
		copyStatus = getMetadata.CopyStatus()
	}

	return f.NewObject(remote)
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

// Hash returns the MD5 of an object returning a lowercase hex string
func (o *Object) Hash(t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	// Convert base64 encoded md5 into lower case hex
	if o.md5 == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(o.md5)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to decode Content-MD5: %q", o.md5)
	}
	return hex.EncodeToString(data), nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

func (o *Object) setMetadata(metadata azblob.Metadata) {
	if len(metadata) > 0 {
		o.meta = metadata
		if modTime, ok := metadata[modTimeKey]; ok {
			when, err := time.Parse(timeFormatIn, modTime)
			if err != nil {
				fs.Debugf(o, "Couldn't parse %v = %q: %v", modTimeKey, modTime, err)
			}
			o.modTime = when
		}
	} else {
		o.meta = nil
	}
}

// decodeMetaDataFromPropertiesResponse sets the metadata from the data passed in
//
// Sets
//  o.id
//  o.modTime
//  o.size
//  o.md5
//  o.meta
func (o *Object) decodeMetaDataFromPropertiesResponse(info *azblob.BlobGetPropertiesResponse) (err error) {
	metadata := info.NewMetadata()
	size := info.ContentLength()
	if isDirectoryMarker(size, metadata, o.remote) {
		return fs.ErrorNotAFile
	}
	// NOTE - Client library always returns MD5 as base64 decoded string, Object needs to maintain
	// this as base64 encoded string.
	o.md5 = base64.StdEncoding.EncodeToString(info.ContentMD5())
	o.mimeType = info.ContentType()
	o.size = size
	o.modTime = time.Time(info.LastModified())
	o.accessTier = azblob.AccessTierType(info.AccessTier())
	o.setMetadata(metadata)

	return nil
}

func (o *Object) decodeMetaDataFromBlob(info *azblob.BlobItem) (err error) {
	metadata := info.Metadata
	size := *info.Properties.ContentLength
	if isDirectoryMarker(size, metadata, o.remote) {
		return fs.ErrorNotAFile
	}
	// NOTE - Client library always returns MD5 as base64 decoded string, Object needs to maintain
	// this as base64 encoded string.
	o.md5 = base64.StdEncoding.EncodeToString(info.Properties.ContentMD5)
	o.mimeType = *info.Properties.ContentType
	o.size = size
	o.modTime = info.Properties.LastModified
	o.accessTier = info.Properties.AccessTier
	o.setMetadata(metadata)
	return nil
}

// getBlobReference creates an empty blob reference with no metadata
func (o *Object) getBlobReference() azblob.BlobURL {
	return o.fs.getBlobReference(o.remote)
}

// clearMetaData clears enough metadata so readMetaData will re-read it
func (o *Object) clearMetaData() {
	o.modTime = time.Time{}
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// Sets
//  o.id
//  o.modTime
//  o.size
//  o.md5
func (o *Object) readMetaData() (err error) {
	if !o.modTime.IsZero() {
		return nil
	}
	blob := o.getBlobReference()

	// Read metadata (this includes metadata)
	options := azblob.BlobAccessConditions{}
	ctx := context.Background()
	var blobProperties *azblob.BlobGetPropertiesResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		blobProperties, err = blob.GetProperties(ctx, options)
		return o.fs.shouldRetry(err)
	})
	if err != nil {
		// On directories - GetProperties does not work and current SDK does not populate service code correctly hence check regular http response as well
		if storageErr, ok := err.(azblob.StorageError); ok && (storageErr.ServiceCode() == azblob.ServiceCodeBlobNotFound || storageErr.Response().StatusCode == http.StatusNotFound) {
			return fs.ErrorObjectNotFound
		}
		return err
	}

	return o.decodeMetaDataFromPropertiesResponse(blobProperties)
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
func (o *Object) ModTime() (result time.Time) {
	// The error is logged in readMetaData
	_ = o.readMetaData()
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	// Make sure o.meta is not nil
	if o.meta == nil {
		o.meta = make(map[string]string, 1)
	}
	// Set modTimeKey in it
	o.meta[modTimeKey] = modTime.Format(timeFormatOut)

	blob := o.getBlobReference()
	ctx := context.Background()
	err := o.fs.pacer.Call(func() (bool, error) {
		_, err := blob.SetMetadata(ctx, o.meta, azblob.BlobAccessConditions{})
		return o.fs.shouldRetry(err)
	})
	if err != nil {
		return err
	}
	o.modTime = modTime
	return nil
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	// Offset and Count for range download
	var offset int64
	var count int64
	if o.AccessTier() == azblob.AccessTierArchive {
		return nil, errors.Errorf("Blob in archive tier, you need to set tier to hot or cool first")
	}

	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			offset, count = x.Decode(o.size)
			if count < 0 {
				count = o.size - offset
			}
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	blob := o.getBlobReference()
	ctx := context.Background()
	ac := azblob.BlobAccessConditions{}
	var dowloadResponse *azblob.DownloadResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		dowloadResponse, err = blob.Download(ctx, offset, count, ac, false)
		return o.fs.shouldRetry(err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to open for download")
	}
	in = dowloadResponse.Body(azblob.RetryReaderOptions{})
	return in, nil
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

// readSeeker joins an io.Reader and an io.Seeker
type readSeeker struct {
	io.Reader
	io.Seeker
}

// uploadMultipart uploads a file using multipart upload
//
// Write a larger blob, using CreateBlockBlob, PutBlock, and PutBlockList.
func (o *Object) uploadMultipart(in io.Reader, size int64, blob *azblob.BlobURL, httpHeaders *azblob.BlobHTTPHeaders) (err error) {
	// Calculate correct chunkSize
	chunkSize := int64(o.fs.opt.ChunkSize)
	var totalParts int64
	for {
		// Calculate number of parts
		var remainder int64
		totalParts, remainder = size/chunkSize, size%chunkSize
		if remainder != 0 {
			totalParts++
		}
		if totalParts < maxTotalParts {
			break
		}
		// Double chunk size if the number of parts is too big
		chunkSize *= 2
		if chunkSize > int64(maxChunkSize) {
			return errors.Errorf("can't upload as it is too big %v - takes more than %d chunks of %v", fs.SizeSuffix(size), totalParts, fs.SizeSuffix(chunkSize/2))
		}
	}
	fs.Debugf(o, "Multipart upload session started for %d parts of size %v", totalParts, fs.SizeSuffix(chunkSize))

	// https://godoc.org/github.com/Azure/azure-storage-blob-go/2017-07-29/azblob#example-BlockBlobURL
	// Utilities are cloned from above example
	// These helper functions convert a binary block ID to a base-64 string and vice versa
	// NOTE: The blockID must be <= 64 bytes and ALL blockIDs for the block must be the same length
	blockIDBinaryToBase64 := func(blockID []byte) string { return base64.StdEncoding.EncodeToString(blockID) }
	// These helper functions convert an int block ID to a base-64 string and vice versa
	blockIDIntToBase64 := func(blockID uint64) string {
		binaryBlockID := (&[8]byte{})[:] // All block IDs are 8 bytes long
		binary.LittleEndian.PutUint64(binaryBlockID, blockID)
		return blockIDBinaryToBase64(binaryBlockID)
	}

	// block ID variables
	var (
		rawID   uint64
		blockID = "" // id in base64 encoded form
		blocks  []string
	)

	// increment the blockID
	nextID := func() {
		rawID++
		blockID = blockIDIntToBase64(rawID)
		blocks = append(blocks, blockID)
	}

	// Get BlockBlobURL, we will use default pipeline here
	blockBlobURL := blob.ToBlockBlobURL()
	ctx := context.Background()
	ac := azblob.LeaseAccessConditions{} // Use default lease access conditions

	// unwrap the accounting from the input, we use wrap to put it
	// back on after the buffering
	in, wrap := accounting.UnWrap(in)

	// Upload the chunks
	remaining := size
	position := int64(0)
	errs := make(chan error, 1)
	var wg sync.WaitGroup
outer:
	for part := 0; part < int(totalParts); part++ {
		// Check any errors
		select {
		case err = <-errs:
			break outer
		default:
		}

		reqSize := remaining
		if reqSize >= chunkSize {
			reqSize = chunkSize
		}

		// Make a block of memory
		buf := make([]byte, reqSize)

		// Read the chunk
		_, err = io.ReadFull(in, buf)
		if err != nil {
			err = errors.Wrap(err, "multipart upload failed to read source")
			break outer
		}

		// Transfer the chunk
		nextID()
		wg.Add(1)
		o.fs.uploadToken.Get()
		go func(part int, position int64, blockID string) {
			defer wg.Done()
			defer o.fs.uploadToken.Put()
			fs.Debugf(o, "Uploading part %d/%d offset %v/%v part size %v", part+1, totalParts, fs.SizeSuffix(position), fs.SizeSuffix(size), fs.SizeSuffix(chunkSize))

			// Upload the block, with MD5 for check
			md5sum := md5.Sum(buf)
			transactionalMD5 := md5sum[:]
			err = o.fs.pacer.Call(func() (bool, error) {
				bufferReader := bytes.NewReader(buf)
				wrappedReader := wrap(bufferReader)
				rs := readSeeker{wrappedReader, bufferReader}
				_, err = blockBlobURL.StageBlock(ctx, blockID, &rs, ac, transactionalMD5)
				return o.fs.shouldRetry(err)
			})

			if err != nil {
				err = errors.Wrap(err, "multipart upload failed to upload part")
				select {
				case errs <- err:
				default:
				}
				return
			}
		}(part, position, blockID)

		// ready for next block
		remaining -= chunkSize
		position += chunkSize
	}
	wg.Wait()
	if err == nil {
		select {
		case err = <-errs:
		default:
		}
	}
	if err != nil {
		return err
	}

	// Finalise the upload session
	err = o.fs.pacer.Call(func() (bool, error) {
		_, err := blockBlobURL.CommitBlockList(ctx, blocks, *httpHeaders, o.meta, azblob.BlobAccessConditions{})
		return o.fs.shouldRetry(err)
	})
	if err != nil {
		return errors.Wrap(err, "multipart upload failed to finalize")
	}
	return nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	err = o.fs.Mkdir("")
	if err != nil {
		return err
	}
	size := src.Size()
	// Update Mod time
	o.updateMetadataWithModTime(src.ModTime())
	if err != nil {
		return err
	}

	blob := o.getBlobReference()
	httpHeaders := azblob.BlobHTTPHeaders{}
	httpHeaders.ContentType = fs.MimeType(o)
	// Multipart upload doesn't support MD5 checksums at put block calls, hence calculate
	// MD5 only for PutBlob requests
	if size < int64(o.fs.opt.UploadCutoff) {
		if sourceMD5, _ := src.Hash(hash.MD5); sourceMD5 != "" {
			sourceMD5bytes, err := hex.DecodeString(sourceMD5)
			if err == nil {
				httpHeaders.ContentMD5 = sourceMD5bytes
			} else {
				fs.Debugf(o, "Failed to decode %q as MD5: %v", sourceMD5, err)
			}
		}
	}

	putBlobOptions := azblob.UploadStreamToBlockBlobOptions{
		BufferSize:      int(o.fs.opt.ChunkSize),
		MaxBuffers:      4,
		Metadata:        o.meta,
		BlobHTTPHeaders: httpHeaders,
	}
	// FIXME Until https://github.com/Azure/azure-storage-blob-go/pull/75
	// is merged the SDK can't upload a single blob of exactly the chunk
	// size, so upload with a multpart upload to work around.
	// See: https://github.com/ncw/rclone/issues/2653
	multipartUpload := size >= int64(o.fs.opt.UploadCutoff)
	if size == int64(o.fs.opt.ChunkSize) {
		multipartUpload = true
		fs.Debugf(o, "Setting multipart upload for file of chunk size (%d) to work around SDK bug", size)
	}

	ctx := context.Background()
	// Don't retry, return a retry error instead
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		if multipartUpload {
			// If a large file upload in chunks
			err = o.uploadMultipart(in, size, &blob, &httpHeaders)
		} else {
			// Write a small blob in one transaction
			blockBlobURL := blob.ToBlockBlobURL()
			_, err = azblob.UploadStreamToBlockBlob(ctx, in, blockBlobURL, putBlobOptions)
		}
		return o.fs.shouldRetry(err)
	})
	if err != nil {
		return err
	}
	// Refresh metadata on object
	o.clearMetaData()
	err = o.readMetaData()
	if err != nil {
		return err
	}

	// If tier is not changed or not specified, do not attempt to invoke `SetBlobTier` operation
	if o.fs.opt.AccessTier == string(defaultAccessTier) || o.fs.opt.AccessTier == string(o.AccessTier()) {
		return nil
	}

	// Now, set blob tier based on configured access tier
	return o.SetTier(o.fs.opt.AccessTier)
}

// Remove an object
func (o *Object) Remove() error {
	blob := o.getBlobReference()
	snapShotOptions := azblob.DeleteSnapshotsOptionNone
	ac := azblob.BlobAccessConditions{}
	ctx := context.Background()
	return o.fs.pacer.Call(func() (bool, error) {
		_, err := blob.Delete(ctx, snapShotOptions, ac)
		return o.fs.shouldRetry(err)
	})
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	return o.mimeType
}

// AccessTier of an object, default is of type none
func (o *Object) AccessTier() azblob.AccessTierType {
	return o.accessTier
}

// SetTier performs changing object tier
func (o *Object) SetTier(tier string) error {
	if !validateAccessTier(tier) {
		return errors.Errorf("Tier %s not supported by Azure Blob Storage", tier)
	}

	// Check if current tier already matches with desired tier
	if o.GetTier() == tier {
		return nil
	}
	desiredAccessTier := azblob.AccessTierType(tier)
	blob := o.getBlobReference()
	ctx := context.Background()
	err := o.fs.pacer.Call(func() (bool, error) {
		_, err := blob.SetTier(ctx, desiredAccessTier, azblob.LeaseAccessConditions{})
		return o.fs.shouldRetry(err)
	})

	if err != nil {
		return errors.Wrap(err, "Failed to set Blob Tier")
	}

	// Set access tier on local object also, this typically
	// gets updated on get blob properties
	o.accessTier = desiredAccessTier
	fs.Debugf(o, "Successfully changed object tier to %s", tier)

	return nil
}

// GetTier returns object tier in azure as string
func (o *Object) GetTier() string {
	return string(o.accessTier)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = &Fs{}
	_ fs.Copier    = &Fs{}
	_ fs.Purger    = &Fs{}
	_ fs.ListRer   = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.MimeTyper = &Object{}
)
