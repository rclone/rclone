//go:build !plan9 && !solaris && !js

// Package oracleobjectstorage provides an interface to the OCI object storage system.
package oracleobjectstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ncw/swift/v2"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/pacer"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "oracleobjectstorage",
		Description: "Oracle Cloud Infrastructure Object Storage",
		Prefix:      "oos",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options:     newOptions(),
		MetadataInfo: &fs.MetadataInfo{
			System: systemMetadataInfo,
			Help:   `User metadata is stored as opc-meta- keys.`,
		},
	})
}

var systemMetadataInfo = map[string]fs.MetadataHelp{
	"opc-meta-mode": {
		Help:    "File type and mode",
		Type:    "octal, unix style",
		Example: "0100664",
	},
	"opc-meta-uid": {
		Help:    "User ID of owner",
		Type:    "decimal number",
		Example: "500",
	},
	"opc-meta-gid": {
		Help:    "Group ID of owner",
		Type:    "decimal number",
		Example: "500",
	},
	"opc-meta-atime": {
		Help:    "Time of last access",
		Type:    "ISO 8601",
		Example: "2025-06-30T22:27:43-04:00",
	},
	"opc-meta-mtime": {
		Help:    "Time of last modification",
		Type:    "ISO 8601",
		Example: "2025-06-30T22:27:43-04:00",
	},
	"opc-meta-btime": {
		Help:    "Time of file birth (creation)",
		Type:    "ISO 8601",
		Example: "2025-06-30T22:27:43-04:00",
	},
}

// Fs represents a remote object storage server
type Fs struct {
	name          string                             // name of this remote
	root          string                             // the path we are working on if any
	opt           Options                            // parsed config options
	ci            *fs.ConfigInfo                     // global config
	features      *fs.Features                       // optional features
	srv           *objectstorage.ObjectStorageClient // the connection to the object storage
	rootBucket    string                             // bucket part of root (if any)
	rootDirectory string                             // directory part of root (if any)
	cache         *bucket.Cache                      // cache for bucket creation status
	pacer         *fs.Pacer                          // To pace the API calls
}

// NewFs Initialize backend
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	err = validateSSECustomerKeyOptions(opt)
	if err != nil {
		return nil, err
	}
	ci := fs.GetConfig(ctx)
	objectStorageClient, err := newObjectStorageClient(ctx, opt)
	if err != nil {
		return nil, err
	}
	pc := fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep)))
	// Set pacer retries to 2 (1 try and 1 retry) because we are
	// relying on SDK retry mechanism, but we allow 2 attempts to
	// retry directory listings after XMLSyntaxError
	pc.SetRetries(2)
	f := &Fs{
		name:  name,
		opt:   *opt,
		ci:    ci,
		srv:   objectStorageClient,
		cache: bucket.NewCache(),
		pacer: pc,
	}
	f.setRoot(root)
	f.features = (&fs.Features{
		ReadMetadata:      true,
		ReadMimeType:      true,
		WriteMimeType:     true,
		BucketBased:       true,
		BucketBasedRootOK: true,
		SetTier:           true,
		GetTier:           true,
		SlowModTime:       true,
	}).Fill(ctx, f)
	if f.rootBucket != "" && f.rootDirectory != "" && !strings.HasSuffix(root, "/") {
		// Check to see if the (bucket,directory) is actually an existing file
		oldRoot := f.root
		newRoot, leaf := path.Split(oldRoot)
		f.setRoot(newRoot)
		_, err := f.NewObject(ctx, leaf)
		if err != nil {
			// File doesn't exist or is a directory so return old f
			f.setRoot(oldRoot)
			return f, nil
		}
		// return an error with fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, err
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

func checkUploadCutoff(cs fs.SizeSuffix) error {
	if cs > maxUploadCutoff {
		return fmt.Errorf("%s is greater than %s", cs, maxUploadCutoff)
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

func (f *Fs) setCopyCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.CopyCutoff = f.opt.CopyCutoff, cs
	}
	return
}

// ------------------------------------------------------------
// Implement backed that represents a remote object storage server
// Fs is the interface a cloud storage system must provide
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
		return "oos:root"
	}
	if f.rootDirectory == "" {
		return fmt.Sprintf("oos:bucket %s", f.rootBucket)
	}
	return fmt.Sprintf("oos:bucket %s, path %s", f.rootBucket, f.rootDirectory)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootBucket, f.rootDirectory = bucket.Split(f.root)
}

// parsePath parses a remote 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// split returns bucket and bucketPath from the rootRelativePath
// relative to f.root
func (f *Fs) split(rootRelativePath string) (bucketName, bucketPath string) {
	bucketName, bucketPath = bucket.Split(path.Join(f.root, rootRelativePath))
	return f.opt.Enc.FromStandardName(bucketName), f.opt.Enc.FromStandardPath(bucketPath)
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
	return list.WithListP(ctx, dir, f)
}

// ListP lists the objects and directories of the Fs starting
// from dir non recursively into out.
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
func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) error {
	list := list.NewHelper(callback)
	bucketName, directory := f.split(dir)
	fs.Debugf(f, "listing: bucket : %v, directory: %v", bucketName, dir)
	if bucketName == "" {
		if directory != "" {
			return fs.ErrorListBucketRequired
		}
		entries, err := f.listBuckets(ctx)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = list.Add(entry)
			if err != nil {
				return err
			}
		}
	} else {
		err := f.listDir(ctx, bucketName, directory, f.rootDirectory, f.rootBucket == "", list.Add)
		if err != nil {
			return err
		}
	}
	return list.Flush()
}

// listFn is called from list to handle an object.
type listFn func(remote string, object *objectstorage.ObjectSummary, isDirectory bool) error

const listingCheckpointVersion = 1

// listingCheckpoint stores backend-managed resume state for a specific OCI
// listing scope. Marker is the OCI pagination resume token; the other fields
// guard against applying that marker to a different listing. Version is kept
// advisory so checkpoint loading can remain backward compatible across schema
// changes as long as the required fields are still present.
type listingCheckpoint struct {
	Version   int       `json:"version"`
	Bucket    string    `json:"bucket"`
	Prefix    string    `json:"prefix"`
	Delimiter string    `json:"delimiter"`
	Marker    string    `json:"marker"`
	UpdatedAt time.Time `json:"updated_at"`
}

type listResumeError struct {
	Code      string `json:"code"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	File      string `json:"file,omitempty"`
	Option    string `json:"option,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Delimiter string `json:"delimiter,omitempty"`
	Marker    string `json:"marker,omitempty"`
	Cause     string `json:"cause,omitempty"`
}

// Error returns a stable, parseable payload for callers orchestrating resume logic.
func (e *listResumeError) Error() string {
	payload, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintf("oos_list_resume_error code=%q source=%q message=%q", e.Code, e.Source, e.Message)
	}
	return "oos_list_resume_error " + string(payload)
}

func newListStartResumeError(code, bucket, marker, message string, cause error) error {
	err := &listResumeError{
		Code:    code,
		Source:  "list_start",
		Message: message,
		Option:  "--oos-list-start",
		Bucket:  bucket,
		Marker:  marker,
	}
	if cause != nil {
		err.Cause = cause.Error()
	}
	return err
}

func newCheckpointResumeError(code, filePath, bucket, prefix, delimiter, marker, message string, cause error) error {
	err := &listResumeError{
		Code:      code,
		Source:    "checkpoint_file",
		Message:   message,
		File:      filePath,
		Option:    "--oos-list-checkpoint-file",
		Bucket:    bucket,
		Prefix:    prefix,
		Delimiter: delimiter,
		Marker:    marker,
	}
	if cause != nil {
		err.Cause = cause.Error()
	}
	return err
}

func loadListingCheckpoint(filePath, bucket, prefix, delimiter string) (marker string, found bool, err error) {
	if filePath == "" {
		return "", false, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, newCheckpointResumeError("checkpoint_read_failed", filePath, bucket, prefix, delimiter, "", "failed to read checkpoint file", err)
	}

	state := listingCheckpoint{}
	if err := json.Unmarshal(data, &state); err != nil {
		return "", false, newCheckpointResumeError("checkpoint_corrupt", filePath, bucket, prefix, delimiter, "", "checkpoint file is not valid JSON", err)
	}

	if state.Version != 0 && state.Version != listingCheckpointVersion {
		fs.Debugf(nil, "Loading listing checkpoint at %q with version mismatch (stored=%d current=%d); attempting compatible load", filePath, state.Version, listingCheckpointVersion)
	}

	if state.Bucket != bucket || state.Prefix != prefix || state.Delimiter != delimiter {
		return "", false, newCheckpointResumeError(
			"checkpoint_scope_mismatch",
			filePath,
			bucket,
			prefix,
			delimiter,
			state.Marker,
			fmt.Sprintf("checkpoint scope mismatch: stored bucket=%q prefix=%q delimiter=%q", state.Bucket, state.Prefix, state.Delimiter),
			nil,
		)
	}

	return state.Marker, true, nil
}

func saveListingCheckpoint(filePath string, state listingCheckpoint) error {
	if filePath == "" {
		return nil
	}

	state.UpdatedAt = time.Now().UTC()
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return newCheckpointResumeError("checkpoint_save_failed", filePath, state.Bucket, state.Prefix, state.Delimiter, state.Marker, "failed to marshal checkpoint file", err)
	}

	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return newCheckpointResumeError("checkpoint_save_failed", filePath, state.Bucket, state.Prefix, state.Delimiter, state.Marker, "failed to create checkpoint directory", err)
		}
	}

	tmpFile, err := os.CreateTemp(dir, ".rclone-oos-list-checkpoint-*.tmp")
	if err != nil {
		return newCheckpointResumeError("checkpoint_save_failed", filePath, state.Bucket, state.Prefix, state.Delimiter, state.Marker, "failed to create temporary checkpoint file", err)
	}
	tmpPath := tmpFile.Name()

	_, writeErr := tmpFile.Write(payload)
	closeErr := tmpFile.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return newCheckpointResumeError("checkpoint_save_failed", filePath, state.Bucket, state.Prefix, state.Delimiter, state.Marker, "failed to write checkpoint file", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return newCheckpointResumeError("checkpoint_save_failed", filePath, state.Bucket, state.Prefix, state.Delimiter, state.Marker, "failed to close checkpoint file", closeErr)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return newCheckpointResumeError("checkpoint_save_failed", filePath, state.Bucket, state.Prefix, state.Delimiter, state.Marker, "failed to replace checkpoint file", err)
	}

	fs.Debugf(nil, "Saved listing checkpoint to %q with marker %q for bucket=%q prefix=%q delimiter=%q", filePath, state.Marker, state.Bucket, state.Prefix, state.Delimiter)
	return nil
}

func (f *Fs) listStartExists(ctx context.Context, bucket, start string) (bool, error) {
	if start == "" {
		return false, nil
	}

	req := objectstorage.HeadObjectRequest{
		NamespaceName: common.String(f.opt.Namespace),
		BucketName:    common.String(bucket),
		ObjectName:    common.String(start),
	}
	useBYOKHeadObject(f, &req)

	var resp objectstorage.HeadObjectResponse
	err := f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.srv.HeadObject(ctx, req)
		return shouldRetry(ctx, resp.HTTPResponse(), err)
	})
	if err != nil {
		if svcErr, ok := err.(common.ServiceError); ok && svcErr.GetHTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// list the objects into the function supplied from
// the bucket and root supplied
// (bucket, directory) is the starting directory
// If prefix is set then it is removed from all file names
// If addBucket is set then it adds the bucket to the start of the remotes generated
// If recurse is set the function will recursively list
// If limit is > 0 then it limits to that many files (must be less than 1000)
// If hidden is set then it will list the hidden (deleted) files too.
// if findFile is set it will look for files called (bucket, directory)
func (f *Fs) list(ctx context.Context, bucket, directory, prefix string, addBucket bool, recurse bool, limit int,
	fn listFn) (err error) {
	if prefix != "" {
		prefix += "/"
	}
	if directory != "" {
		directory += "/"
	}

	delimiter := ""
	if !recurse {
		delimiter = "/"
	}
	chunkSize := 1000
	if limit > 0 {
		chunkSize = limit
	}
	var request = objectstorage.ListObjectsRequest{
		NamespaceName: common.String(f.opt.Namespace),
		BucketName:    common.String(bucket),
		Prefix:        common.String(directory),
		Limit:         common.Int(chunkSize),
		Fields:        common.String("name,size,etag,timeCreated,md5,timeModified,storageTier,archivalState"),
	}
	if delimiter != "" {
		request.Delimiter = common.String(delimiter)
	}

	// A matching checkpoint file owns resume behavior for this listing scope.
	checkpointState := listingCheckpoint{
		Version:   listingCheckpointVersion,
		Bucket:    bucket,
		Prefix:    directory,
		Delimiter: delimiter,
	}
	checkpointFound := false
	if f.opt.ListCheckpointFile != "" {
		fs.Debugf(f, "Checking list checkpoint file %q for bucket=%q prefix=%q delimiter=%q", f.opt.ListCheckpointFile, checkpointState.Bucket, checkpointState.Prefix, checkpointState.Delimiter)
		marker, found, err := loadListingCheckpoint(f.opt.ListCheckpointFile, checkpointState.Bucket,
			checkpointState.Prefix, checkpointState.Delimiter)
		if err != nil {
			return err
		}
		if found {
			checkpointFound = true
			if marker == "" {
				request.Start = nil
			} else {
				markerExists, err := f.listStartExists(ctx, bucket, marker)
				if err != nil {
					return newCheckpointResumeError("checkpoint_marker_validation_failed", f.opt.ListCheckpointFile, bucket, checkpointState.Prefix, checkpointState.Delimiter, marker, "failed to validate checkpoint marker", err)
				}
				if markerExists {
					request.Start = common.String(marker)
				} else {
					return newCheckpointResumeError("checkpoint_marker_not_found", f.opt.ListCheckpointFile, bucket, checkpointState.Prefix, checkpointState.Delimiter, marker, "checkpoint marker object does not exist", nil)
				}
			}
			if request.Start == nil {
				fs.Infof(f, "Resuming listing from checkpoint file %q from the beginning", f.opt.ListCheckpointFile)
			} else {
				fs.Infof(f, "Resuming listing from checkpoint file %q with marker %q", f.opt.ListCheckpointFile, marker)
			}
			if f.opt.ListStart != "" {
				fs.Debugf(f, "Ignoring --oos-list-start %q because the checkpoint file %q takes precedence", f.opt.ListStart, f.opt.ListCheckpointFile)
			}
		} else {
			fs.Debugf(f, "No applicable listing checkpoint found in %q", f.opt.ListCheckpointFile)
		}
	}

	// Only honor the manual list-start marker when no checkpoint already applies.
	if !checkpointFound && f.opt.ListStart != "" {
		startExists, err := f.listStartExists(ctx, bucket, f.opt.ListStart)
		if err != nil {
			return newListStartResumeError("list_start_validation_failed", bucket, f.opt.ListStart, "failed to validate list start marker", err)
		}
		if startExists {
			request.Start = common.String(f.opt.ListStart)
		} else {
			return newListStartResumeError("list_start_not_found", bucket, f.opt.ListStart, "list start marker object does not exist", nil)
		}
	}

	for {
		var resp objectstorage.ListObjectsResponse
		err = f.pacer.Call(func() (bool, error) {
			var err error
			resp, err = f.srv.ListObjects(ctx, request)
			return shouldRetry(ctx, resp.HTTPResponse(), err)
		})
		if err != nil {
			if ociError, ok := err.(common.ServiceError); ok {
				// If it is a timeout then we want to retry that
				if ociError.GetHTTPStatusCode() == http.StatusNotFound {
					err = fs.ErrorDirNotFound
				}
			}
			if f.rootBucket == "" {
				// if listing from the root ignore wrong region requests returning
				// empty directory
				if reqErr, ok := err.(common.ServiceError); ok {
					// 301 if wrong region for bucket
					if reqErr.GetHTTPStatusCode() == http.StatusMovedPermanently {
						fs.Errorf(f, "Can't change region for bucket %q with no bucket specified", bucket)
						return nil
					}
				}
			}
			return err
		}
		if !recurse {
			for _, commonPrefix := range resp.ListObjects.Prefixes {
				if commonPrefix == "" {
					fs.Logf(f, "Nil common prefix received")
					continue
				}
				remote := commonPrefix
				remote = f.opt.Enc.ToStandardPath(remote)
				if !strings.HasPrefix(remote, prefix) {
					fs.Logf(f, "Odd name received %q", remote)
					continue
				}
				remote = remote[len(prefix):]
				if addBucket {
					remote = path.Join(bucket, remote)
				}
				remote = strings.TrimSuffix(remote, "/")
				err = fn(remote, &objectstorage.ObjectSummary{Name: &remote}, true)
				if err != nil {
					return err
				}
			}
		}
		for i := range resp.Objects {
			object := &resp.Objects[i]
			// Finish if file name no longer has prefix
			//if prefix != "" && !strings.HasPrefix(file.Name, prefix) {
			//	return nil
			//}
			remote := *object.Name
			remote = f.opt.Enc.ToStandardPath(remote)
			if !strings.HasPrefix(remote, prefix) {
				continue
			}
			remote = remote[len(prefix):]
			// Check for directory
			isDirectory := remote == "" || strings.HasSuffix(remote, "/")
			if addBucket {
				remote = path.Join(bucket, remote)
			}
			// is this a directory marker?
			if isDirectory && object.Size != nil && *object.Size == 0 {
				continue // skip directory marker
			}
			if isDirectory && len(remote) > 1 {
				remote = remote[:len(remote)-1]
			}
			err = fn(remote, object, isDirectory)
			if err != nil {
				return err
			}
		}
		nextMarker := ""
		if resp.NextStartWith != nil {
			nextMarker = *resp.NextStartWith
		}

		if f.opt.ListCheckpointFile != "" {
			checkpointState.Marker = nextMarker
			if err := saveListingCheckpoint(f.opt.ListCheckpointFile, checkpointState); err != nil {
				return err
			}
		}

		// end if no NextFileName
		if resp.NextStartWith == nil {
			break
		}
		request.Start = resp.NextStartWith
	}
	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, object *objectstorage.ObjectSummary, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		size := int64(0)
		if object.Size != nil {
			size = *object.Size
		}
		d := fs.NewDir(remote, time.Time{}).SetSize(size)
		return d, nil
	}
	o, err := f.newObjectWithInfo(ctx, remote, object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// listDir lists a single directory
func (f *Fs) listDir(ctx context.Context, bucket, directory, prefix string, addBucket bool, callback func(fs.DirEntry) error) (err error) {
	fn := func(remote string, object *objectstorage.ObjectSummary, isDirectory bool) error {
		entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory)
		if err != nil {
			return err
		}
		if entry != nil {
			return callback(entry)
		}
		return nil
	}
	err = f.list(ctx, bucket, directory, prefix, addBucket, false, 0, fn)
	if err != nil {
		return err
	}
	// bucket must be present if listing succeeded
	f.cache.MarkOK(bucket)
	return nil
}

// listBuckets returns all the buckets to out
func (f *Fs) listBuckets(ctx context.Context) (entries fs.DirEntries, err error) {
	if f.opt.Provider == noAuth {
		return nil, fmt.Errorf("can't list buckets with %v provider, use a valid auth provider in config file", noAuth)
	}
	var request = objectstorage.ListBucketsRequest{
		NamespaceName: common.String(f.opt.Namespace),
		CompartmentId: common.String(f.opt.Compartment),
	}
	var resp objectstorage.ListBucketsResponse
	for {
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.ListBuckets(ctx, request)
			return shouldRetry(ctx, resp.HTTPResponse(), err)
		})
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Items {
			bucketName := f.opt.Enc.ToStandardName(*item.Name)
			f.cache.MarkOK(bucketName)
			d := fs.NewDir(bucketName, item.TimeCreated.Time)
			entries = append(entries, d)
		}
		if resp.OpcNextPage == nil {
			break
		}
		request.Page = resp.OpcNextPage
	}
	return entries, nil
}

// Return an Object from a path
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *objectstorage.ObjectSummary) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		// Set info but not meta
		if info.TimeModified == nil {
			fs.Logf(o, "Failed to read last modified")
			o.lastModified = time.Now()
		} else {
			o.lastModified = info.TimeModified.Time
		}
		if info.Md5 != nil {
			md5, err := o.base64ToMd5(*info.Md5)
			if err != nil {
				o.md5 = md5
			}
		}
		o.bytes = *info.Size
		o.storageTier = storageTierMap[strings.ToLower(string(info.StorageTier))]
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

// Put the object into the bucket
// Copy the reader in to the new object which is returned
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return o, o.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	bucketName, _ := f.split(dir)
	return f.makeBucket(ctx, bucketName)
}

// makeBucket creates the bucket if it doesn't exist
func (f *Fs) makeBucket(ctx context.Context, bucketName string) error {
	if f.opt.NoCheckBucket {
		return nil
	}
	return f.cache.Create(bucketName, func() error {
		details := objectstorage.CreateBucketDetails{
			Name:             common.String(bucketName),
			CompartmentId:    common.String(f.opt.Compartment),
			PublicAccessType: objectstorage.CreateBucketDetailsPublicAccessTypeNopublicaccess,
		}
		req := objectstorage.CreateBucketRequest{
			NamespaceName:       common.String(f.opt.Namespace),
			CreateBucketDetails: details,
		}
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.CreateBucket(ctx, req)
			return shouldRetry(ctx, resp.HTTPResponse(), err)
		})
		if err == nil {
			fs.Infof(f, "Bucket %q created with accessType %q", bucketName,
				objectstorage.CreateBucketDetailsPublicAccessTypeNopublicaccess)
		}
		if svcErr, ok := err.(common.ServiceError); ok {
			if code := svcErr.GetCode(); code == "BucketAlreadyOwnedByYou" || code == "BucketAlreadyExists" {
				err = nil
			}
		}
		return err
	}, func() (bool, error) {
		return f.bucketExists(ctx, bucketName)
	})
}

// Check if the bucket exists
//
// NB this can return incorrect results if called immediately after bucket deletion
func (f *Fs) bucketExists(ctx context.Context, bucketName string) (bool, error) {
	req := objectstorage.HeadBucketRequest{
		NamespaceName: common.String(f.opt.Namespace),
		BucketName:    common.String(bucketName),
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.HeadBucket(ctx, req)
		return shouldRetry(ctx, resp.HTTPResponse(), err)
	})
	if err == nil {
		return true, nil
	}
	if err, ok := err.(common.ServiceError); ok {
		if err.GetHTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}
	}
	return false, err
}

// Rmdir delete an empty bucket. if bucket is not empty this is will fail with appropriate error
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	bucketName, directory := f.split(dir)
	if bucketName == "" || directory != "" {
		return nil
	}
	return f.cache.Remove(bucketName, func() error {
		req := objectstorage.DeleteBucketRequest{
			NamespaceName: common.String(f.opt.Namespace),
			BucketName:    common.String(bucketName),
		}
		err := f.pacer.Call(func() (bool, error) {
			resp, err := f.srv.DeleteBucket(ctx, req)
			return shouldRetry(ctx, resp.HTTPResponse(), err)
		})
		if err == nil {
			fs.Infof(f, "Bucket %q deleted", bucketName)
		}
		return err
	})
}

func (f *Fs) abortMultiPartUpload(ctx context.Context, bucketName, bucketPath, uploadID *string) (err error) {
	if uploadID == nil || *uploadID == "" {
		return nil
	}
	request := objectstorage.AbortMultipartUploadRequest{
		NamespaceName: common.String(f.opt.Namespace),
		BucketName:    bucketName,
		ObjectName:    bucketPath,
		UploadId:      uploadID,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.AbortMultipartUpload(ctx, request)
		return shouldRetry(ctx, resp.HTTPResponse(), err)
	})
	return err
}

// cleanUpBucket removes all pending multipart uploads for a given bucket over the age of maxAge
func (f *Fs) cleanUpBucket(ctx context.Context, bucket string, maxAge time.Duration,
	uploads []*objectstorage.MultipartUpload) (err error) {
	fs.Infof(f, "cleaning bucket %q of pending multipart uploads older than %v", bucket, maxAge)
	for _, upload := range uploads {
		if upload.TimeCreated != nil && upload.Object != nil && upload.UploadId != nil {
			age := time.Since(upload.TimeCreated.Time)
			what := fmt.Sprintf("pending multipart upload for bucket %q key %q dated %v (%v ago)", bucket, *upload.Object,
				upload.TimeCreated, age)
			if age > maxAge {
				fs.Infof(f, "removing %s", what)
				if operations.SkipDestructive(ctx, what, "remove pending upload") {
					continue
				}
				_ = f.abortMultiPartUpload(ctx, upload.Bucket, upload.Object, upload.UploadId)
			}
		} else {
			fs.Infof(f, "MultipartUpload doesn't have sufficient details to abort.")
		}
	}
	return err
}

// CleanUp removes all pending multipart uploads
func (f *Fs) cleanUp(ctx context.Context, maxAge time.Duration) (err error) {
	uploadsMap, err := f.listMultipartUploadsAll(ctx)
	if err != nil {
		return err
	}
	for bucketName, uploads := range uploadsMap {
		cleanErr := f.cleanUpBucket(ctx, bucketName, maxAge, uploads)
		if err != nil {
			fs.Errorf(f, "Failed to cleanup bucket %q: %v", bucketName, cleanErr)
			err = cleanErr
		}
	}
	return err
}

// CleanUp removes all pending multipart uploads older than 24 hours
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	return f.cleanUp(ctx, 24*time.Hour)
}

// ------------------------------------------------------------
// Implement ListRer is an optional interfaces for Fs
//------------------------------------------------------------

/*
ListR lists the objects and directories of the Fs starting
from dir recursively into out.

dir should be "" to start from the root, and should not
have trailing slashes.

This should return ErrDirNotFound if the directory isn't
found.

It should call callback for each tranche of entries read.
These need not be returned in any particular order.  If
callback returns an error then the listing will stop
immediately.

Don't implement this unless you have a more efficient way
of listing recursively that doing a directory traversal.
*/
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	bucketName, directory := f.split(dir)
	list := list.NewHelper(callback)
	listR := func(bucket, directory, prefix string, addBucket bool) error {
		return f.list(ctx, bucket, directory, prefix, addBucket, true, 0, func(remote string, object *objectstorage.ObjectSummary, isDirectory bool) error {
			entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory)
			if err != nil {
				return err
			}
			return list.Add(entry)
		})
	}
	if bucketName == "" {
		entries, err := f.listBuckets(ctx)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = list.Add(entry)
			if err != nil {
				return err
			}
			bucketName := entry.Remote()
			err = listR(bucketName, "", f.rootDirectory, true)
			if err != nil {
				return err
			}
			// bucket must be present if listing succeeded
			f.cache.MarkOK(bucketName)
		}
	} else {
		err = listR(bucketName, directory, f.rootDirectory, f.rootBucket == "")
		if err != nil {
			return err
		}
		// bucket must be present if listing succeeded
		f.cache.MarkOK(bucketName)
	}
	return list.Flush()
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *Object) Metadata(ctx context.Context) (metadata fs.Metadata, err error) {
	err = o.readMetaData(ctx)
	if err != nil {
		return nil, err
	}
	metadata = make(fs.Metadata, len(o.meta)+7)
	for k, v := range o.meta {
		switch k {
		case metaMtime:
			if modTime, err := swift.FloatStringToTime(v); err == nil {
				metadata["mtime"] = modTime.Format(time.RFC3339Nano)
			}
		case metaMD5Hash:
			// don't write hash metadata
		default:
			metadata[k] = v
		}
	}
	if o.mimeType != "" {
		metadata["content-type"] = o.mimeType
	}

	if !o.lastModified.IsZero() {
		metadata["btime"] = o.lastModified.Format(time.RFC3339Nano)
	}

	return metadata, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.Copier          = &Fs{}
	_ fs.PutStreamer     = &Fs{}
	_ fs.ListRer         = &Fs{}
	_ fs.ListPer         = &Fs{}
	_ fs.Commander       = &Fs{}
	_ fs.CleanUpper      = &Fs{}
	_ fs.OpenChunkWriter = &Fs{}

	_ fs.Object    = &Object{}
	_ fs.MimeTyper = &Object{}
	_ fs.GetTierer = &Object{}
	_ fs.SetTierer = &Object{}
)
