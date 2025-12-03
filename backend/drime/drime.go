// Package drime provides an interface to the Drime
// object storage system.
package drime

/*
Return results give

  X-Ratelimit-Limit: 2000
  X-Ratelimit-Remaining: 1999

The rate limit headers indicate the number of allowed API requests per
minute. The limit is two thousand requests per minute, and rclone
should stay under that.
*/

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/backend/drime/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep            = 10 * time.Millisecond
	maxSleep            = 20 * time.Second
	decayConstant       = 1 // bigger for slower decay, exponential
	baseURL             = "https://app.drime.cloud/"
	rootURL             = baseURL + "api/v1"
	maxUploadParts      = 10000 // maximum allowed number of parts in a multi-part upload
	minChunkSize        = fs.SizeSuffix(1024 * 1024 * 5)
	defaultUploadCutoff = fs.SizeSuffix(200 * 1024 * 1024)
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "drime",
		Description: "Drime",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "access_token",
			Help: `API Access token

You can get this from the web control panel.`,
			Sensitive: true,
		}, {
			Name: "root_folder_id",
			Help: `ID of the root folder

Leave this blank normally, rclone will fill it in automatically.

If you want rclone to be restricted to a particular folder you can
fill it in - see the docs for more info.
`,
			Default:   "",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "workspace_id",
			Help: `Account ID

Leave this blank normally unless you wish to specify a Workspace ID.
`,
			Default:   "",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:     "list_chunk",
			Help:     `Number of items to list in each call`,
			Default:  1000,
			Advanced: true,
		}, {
			Name:     "hard_delete",
			Help:     "Delete files permanently rather than putting them into the trash.",
			Default:  false,
			Advanced: true,
		}, {
			Name: "upload_cutoff",
			Help: `Cutoff for switching to chunked upload.

Any files larger than this will be uploaded in chunks of chunk_size.
The minimum is 0 and the maximum is 5 GiB.`,
			Default:  defaultUploadCutoff,
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Chunk size to use for uploading.

When uploading files larger than upload_cutoff or files with unknown
size (e.g. from "rclone rcat" or uploaded with "rclone mount" or google
photos or google docs) they will be uploaded as multipart uploads
using this chunk size.

Note that "--drime-upload-concurrency" chunks of this size are buffered
in memory per transfer.

If you are transferring large files over high-speed links and you have
enough memory, then increasing this will speed up the transfers.

Rclone will automatically increase the chunk size when uploading a
large file of known size to stay below the 10,000 chunks limit.

Files of unknown size are uploaded with the configured
chunk_size. Since the default chunk size is 5 MiB and there can be at
most 10,000 chunks, this means that by default the maximum size of
a file you can stream upload is 48 GiB.  If you wish to stream upload
larger files then you will need to increase chunk_size.
`,
			Default:  minChunkSize,
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads and copies.

This is the number of chunks of the same file that are uploaded
concurrently for multipart uploads and copies.

If you are uploading small numbers of large files over high-speed links
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.`,
			Default:  4,
			Advanced: true,
		}, {
			Name: "upload_cutoff",
			Help: `Cutoff for switching to chunked upload.

Any files larger than this will be uploaded in chunks of chunk_size.
The minimum is 0 and the maximum is 5 GiB.`,
			Default:  defaultUploadCutoff,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display | // Slash Control Delete Dot
				encoder.EncodeLeftSpace |
				encoder.EncodeBackSlash |
				encoder.EncodeRightSpace |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

/*
TestDrime{sb0-v}
stringNeedsEscaping = []rune{
	'/', '\\', '\a', '\b', '\f', '\n', '\r', '\t', '\v', '\x00', '\x01', '\x02', '\x03', '\x04', '\x05', '\x06', '\x0e', '\x0f', '\x10', '\x11', '\x12', '\x13', '\x14',
	'\x15', '\x16', '\x17', '\x18', '\x19', '\x1a', '\x1b', '\x1c', '\x1d', '\x1e', '\x1f', '\x7f', '\xbf', '\xfe'
}
maxFileLength = 255 // for 1 byte unicode characters
maxFileLength = 127 // for 2 byte unicode characters
maxFileLength = 85 // for 3 byte unicode characters
maxFileLength = 63 // for 4 byte unicode characters
canWriteUnnormalized = true
canReadUnnormalized   = true
canReadRenormalized   = false
canStream = true
base32768isOK = true // make sure maxFileLength for 2 byte unicode chars is the same as for 1 byte characters
*/

// Options defines the configuration for this backend
type Options struct {
	AccessToken       string               `config:"access_token"`
	RootFolderID      string               `config:"root_folder_id"`
	WorkspaceID       string               `config:"workspace_id"`
	UploadConcurrency int                  `config:"upload_concurrency"`
	ChunkSize         fs.SizeSuffix        `config:"chunk_size"`
	HardDelete        bool                 `config:"hard_delete"`
	UploadCutoff      fs.SizeSuffix        `config:"upload_cutoff"`
	ListChunk         int                  `config:"list_chunk"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote drime
type Fs struct {
	name     string             // name of this remote
	root     string             // the path we are working on
	opt      Options            // parsed options
	features *fs.Features       // optional features
	srv      *rest.Client       // the connection to the server
	dirCache *dircache.DirCache // Map of directory path to directory id
	pacer    *fs.Pacer          // pacer for API calls
}

// Object describes a drime object
//
// The full set of metadata will always be present
type Object struct {
	fs       *Fs       // what this object is part of
	remote   string    // The remote path
	size     int64     // size of the object
	modTime  time.Time // modification time of the object
	id       string    // ID of the object
	dirID    string    // ID of the object's directory
	mimeType string    // mime type of the object
	url      string    // where to download this object
}

// ------------------------------------------------------------

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

func (f *Fs) setUploadCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	old, f.opt.UploadCutoff = f.opt.UploadCutoff, cs
	return
}

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
	return fmt.Sprintf("drime root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a drime 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Item, err error) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, directoryID, false, true, leaf, func(item *api.Item) bool {
		if item.Name == leaf {
			info = item
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fs.ErrorObjectNotFound
	}
	return info, nil
}

// getItem reads item for ID given
func (f *Fs) getItem(ctx context.Context, id string, dirID string, leaf string) (info *api.Item, err error) {
	found, err := f.listAll(ctx, dirID, false, true, leaf, func(item *api.Item) bool {
		if item.ID.String() == id {
			info = item
			return true
		}
		return false
	})
	if !found {
		return nil, fs.ErrorObjectNotFound
	}
	return info, err
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		fs.Debugf(nil, "Couldn't read error out of body: %v", err)
		body = nil
	}
	// Decode error response if there was one - they can be blank
	var errResponse api.Error
	if len(body) > 0 {
		err = json.Unmarshal(body, &errResponse)
		if err != nil {
			fs.Debugf(nil, "Couldn't decode error response: %v", err)
		}
	}
	if errResponse.Message == "" {
		errResponse.Message = fmt.Sprintf("%s (%d): %s", resp.Status, resp.StatusCode, string(body))
	}
	return errResponse
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, fmt.Errorf("drime: chunk size: %w", err)
	}

	root = parsePath(root)

	client := fshttp.NewClient(ctx)

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   rest.NewClient(client).SetRoot(rootURL),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
		WriteMimeType:           true,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)
	f.srv.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)
	f.srv.SetHeader("Accept", "application/json")

	// Get rootFolderID
	rootID := f.opt.RootFolderID
	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// rootSlash returns root with a slash on if it is empty, otherwise empty string
func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, true, false, leaf, func(item *api.Item) bool {
		if item.Name == leaf {
			pathIDOut = item.ID.String()
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// createDir makes a directory with pathID as parent and name leaf and modTime
func (f *Fs) createDir(ctx context.Context, pathID, leaf string, modTime time.Time) (item *api.Item, err error) {
	var resp *http.Response
	var result api.CreateFolderResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/folders",
	}
	mkdir := api.CreateFolderRequest{
		Name:     f.opt.Enc.FromStandardName(leaf),
		ParentID: json.Number(pathID),
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mkdir, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}
	return &result.Folder, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	item, err := f.createDir(ctx, pathID, leaf, time.Now())
	if err != nil {
		return "", err
	}
	return item.ID.String(), nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.Item) bool

// Lists the directory required calling the user function on each item found
//
// If name is set then the server will limit the returned items to those
// with that name.
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, name string, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/drive/file-entries",
		Parameters: url.Values{},
	}
	if dirID != "" {
		opts.Parameters.Add("parentIds", dirID)
	}
	if directoriesOnly {
		opts.Parameters.Add("type", api.ItemTypeFolder)
	}
	if f.opt.WorkspaceID != "" {
		opts.Parameters.Set("workspaceId", f.opt.WorkspaceID)
	}
	opts.Parameters.Set("perPage", strconv.Itoa(f.opt.ListChunk))
	page := 1
OUTER:
	for {
		opts.Parameters.Set("page", strconv.Itoa(page))
		var result api.Listing
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return found, fmt.Errorf("couldn't list files: %w", err)
		}
		for _, item := range result.Data {
			if item.Type == api.ItemTypeFolder {
				if filesOnly {
					continue
				}
			} else {
				if directoriesOnly {
					continue
				}
			}
			item.Name = f.opt.Enc.ToStandardName(item.Name)
			if fn(&item) {
				found = true
				break OUTER
			}
		}
		if result.NextPage == 0 {
			break
		}
		page = result.NextPage
	}
	return found, err
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, info *api.Item) (entry fs.DirEntry, err error) {
	if info.Type == api.ItemTypeFolder {
		// cache the directory ID for later lookups
		f.dirCache.Put(remote, info.ID.String())
		entry = fs.NewDir(remote, info.UpdatedAt).
			SetSize(info.FileSize).
			SetID(info.ID.String()).
			SetParentID(info.ParentID.String())
	} else {
		entry, err = f.newObjectWithInfo(ctx, remote, info)
		if err != nil {
			return nil, err
		}
	}
	return entry, nil
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
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, false, false, "", func(info *api.Item) bool {
		remote := path.Join(dir, info.Name)
		entry, err := f.itemToDirEntry(ctx, remote, info)
		if err != nil {
			iErr = err
			return true
		}
		entries = append(entries, entry)
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, leaf, directoryID, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// PutUnchecked the object into the container
//
// This will produce a duplicate if the object already exists.
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, _, _, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/file-entries/delete",
	}
	request := api.DeleteRequest{
		EntryIDs:      []string{id},
		DeleteForever: f.opt.HardDelete,
	}
	var result api.DeleteResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}
	// Check the individual result codes also
	for name, errstring := range result.Errors {
		return fmt.Errorf("failed to delete item %q: %s", name, errstring)
	}
	return nil
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dc := f.dirCache
	rootID, err := dc.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// Check to see if there is contents in the directory
	if check {
		found, err := f.listAll(ctx, rootID, false, false, "", func(item *api.Item) bool {
			return true
		})
		if err != nil {
			return err
		}
		if found {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	// Delete the directory
	err = f.deleteObject(ctx, rootID)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// patch an attribute on an object to value
func (f *Fs) patch(ctx context.Context, id, attribute string, value string) (item *api.Item, err error) {
	var resp *http.Response
	var request = api.UpdateItemRequest{
		Name: value,
	}
	var result api.UpdateItemResponse
	opts := rest.Opts{
		Method: "PUT",
		Path:   "/file-entries/" + id,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to patch item %q to %v: %w", attribute, value, err)
	}
	return &result.FileEntry, nil
}

// rename a file or a folder
func (f *Fs) rename(ctx context.Context, id, newLeaf string) (item *api.Item, err error) {
	return f.patch(ctx, id, "name", f.opt.Enc.FromStandardName(newLeaf))
}

// move a file or a folder to a new directory
// func (f *Fs) move(ctx context.Context, id, newDirID string, dstLeaf string) (item *api.Item, err error) {
func (f *Fs) move(ctx context.Context, id, newDirID string) (err error) {
	var resp *http.Response
	var request = api.MoveRequest{
		EntryIDs:      []string{id},
		DestinationID: newDirID,
	}
	var result api.MoveResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/file-entries/move",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to move item: %w", err)
	}

	return nil
}

// move and rename a file or folder to directoryID with leaf
func (f *Fs) moveTo(ctx context.Context, id, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID string) (info *api.Item, err error) {
	newLeaf := f.opt.Enc.FromStandardName(dstLeaf)
	oldLeaf := f.opt.Enc.FromStandardName(srcLeaf)
	doRenameLeaf := oldLeaf != newLeaf
	doMove := srcDirectoryID != dstDirectoryID

	// Now rename the leaf to a temporary name if we are moving to
	// another directory to make sure we don't overwrite something
	// in the destination directory by accident
	if doRenameLeaf && doMove {
		tmpLeaf := newLeaf + "." + random.String(8)
		info, err = f.rename(ctx, id, tmpLeaf)
		if err != nil {
			return nil, fmt.Errorf("Move rename leaf: %w", err)
		}
	}

	// Move the object to a new directory (with the existing name)
	// if required
	if doMove {
		err = f.move(ctx, id, dstDirectoryID)
		if err != nil {
			return nil, err
		}
	}

	// Rename the leaf to its final name if required
	if doRenameLeaf {
		info, err = f.rename(ctx, id, newLeaf)
		if err != nil {
			return nil, fmt.Errorf("Move rename leaf: %w", err)
		}
	}

	if info == nil {
		info, err = f.getItem(ctx, id, dstDirectoryID, dstLeaf)
		if err != nil {
			return nil, err
		}
	}

	return info, nil
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Find existing object
	srcLeaf, srcDirectoryID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObj, dstLeaf, dstDirectoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Do the move
	info, err := f.moveTo(ctx, srcObj.id, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID)
	if err != nil {
		return nil, err
	}

	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	// Do the move
	_, err = f.moveTo(ctx, srcID, srcLeaf, dstLeaf, srcDirectoryID, dstDirectoryID)
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// copy a file or a folder to a new directory
func (f *Fs) copy(ctx context.Context, id, newDirID string) (item *api.Item, err error) {
	var resp *http.Response
	var request = api.CopyRequest{
		EntryIDs:      []string{id},
		DestinationID: newDirID,
	}
	var result api.CopyResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/file-entries/duplicate",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to copy item: %w", err)
	}
	itemResult := result.Entries[0]
	return &itemResult, nil
}

// copy and rename a file or folder to directoryID with leaf
func (f *Fs) copyTo(ctx context.Context, srcID, srcLeaf, dstLeaf, dstDirectoryID string) (info *api.Item, err error) {
	// Can have duplicates so don't have to be careful here

	// Copy to dstDirectoryID first
	info, err = f.copy(ctx, srcID, dstDirectoryID)
	if err != nil {
		return nil, err
	}

	// Rename if required
	if srcLeaf != dstLeaf {
		info, err = f.rename(ctx, info.ID.String(), dstLeaf)
		if err != nil {
			return nil, err
		}
	}
	return info, nil
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
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (dst fs.Object, err error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcLeaf := path.Base(srcObj.remote)

	srcPath := srcObj.fs.rootSlash() + srcObj.remote
	dstPath := f.rootSlash() + remote
	if srcPath == dstPath {
		return nil, fmt.Errorf("can't copy %q -> %q as are same name", srcPath, dstPath)
	}

	// Find existing object
	existingObj, err := f.NewObject(ctx, remote)
	if err == nil {
		defer func() {
			// Don't remove existing object if returning an error
			if err != nil {
				return
			}
			fs.Debugf(existingObj, "Server side copy: removing existing object after successful copy")
			err = existingObj.Remove(ctx)
		}()
	}

	// Create temporary object
	dstObj, dstLeaf, dstDirectoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	info, err := f.copyTo(ctx, srcObj.id, srcLeaf, dstLeaf, dstDirectoryID)
	if err != nil {
		return nil, err
	}
	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}

	return dstObj, nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

var warnStreamUpload sync.Once

// Status of open chunked upload
type drimeChunkWriter struct {
	uploadID  string
	key       string
	chunkSize int64
	size      int64
	f         *Fs
	o         *Object
	written   atomic.Int64
	fileEntry api.Item

	uploadName   string
	leaf         string
	mime         string
	extension    string
	parentID     json.Number
	relativePath string

	completedPartsMu sync.Mutex
	completedParts   []api.CompletedPart
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return info, nil, err
	}

	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}

	size := src.Size()
	fs.FixRangeOption(options, size)

	// calculate size of parts
	chunkSize := f.opt.ChunkSize

	// size can be -1 here meaning we don't know the size of the incoming file. We use ChunkSize
	// buffers here (default 5 MB). With a maximum number of parts (10,000) this will be a file of
	// 48 GB.
	if size == -1 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				chunkSize, fs.SizeSuffix(int64(chunkSize)*int64(maxUploadParts)))
		})
	} else {
		chunkSize = chunksize.Calculator(src, size, maxUploadParts, chunkSize)
	}

	createSize := max(0, size)

	// Initiate multipart upload
	req := api.MultiPartCreateRequest{
		Filename:     leaf,
		Mime:         fs.MimeType(ctx, src),
		Size:         createSize,
		Extension:    strings.TrimPrefix(path.Ext(leaf), `.`),
		ParentID:     json.Number(directoryID),
		RelativePath: f.opt.Enc.FromStandardPath(path.Join(f.root, remote)),
	}

	var resp api.MultiPartCreateResponse

	opts := rest.Opts{
		Method:  "POST",
		Path:    "/s3/multipart/create",
		Options: options,
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		res, err := o.fs.srv.CallJSON(ctx, &opts, req, &resp)
		return shouldRetry(ctx, res, err)
	})

	if err != nil {
		return info, nil, fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	mime := fs.MimeType(ctx, src)
	ext := strings.TrimPrefix(path.Ext(leaf), ".")
	// must have file extension for multipart upload
	if ext == "" {
		ext = "bin"
	}
	rel := f.opt.Enc.FromStandardPath(path.Join(f.root, remote))

	chunkWriter := &drimeChunkWriter{
		uploadID:     resp.UploadID,
		key:          resp.Key,
		chunkSize:    int64(chunkSize),
		size:         size,
		f:            f,
		o:            o,
		uploadName:   path.Base(resp.Key),
		leaf:         leaf,
		mime:         mime,
		extension:    ext,
		parentID:     json.Number(directoryID),
		relativePath: rel,
	}
	info = fs.ChunkWriterInfo{
		ChunkSize:         int64(chunkSize),
		Concurrency:       f.opt.UploadConcurrency,
		LeavePartsOnError: false,
	}
	return info, chunkWriter, err
}

// WriteChunk will write chunk number with reader bytes, where chunk number >= 0
func (s *drimeChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (bytesWritten int64, err error) {
	// chunk numbers between 1 and 100000
	chunkNumber++

	// find size of chunk
	chunkSize, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to seek chunk: %w", err)
	}

	if chunkSize == 0 && chunkNumber != 1 {
		return 0, nil
	}

	partOpts := rest.Opts{
		Method: "POST",
		Path:   "/s3/multipart/batch-sign-part-urls",
	}

	req := api.MultiPartGetURLsRequest{
		UploadID: s.uploadID,
		Key:      s.key,
		PartNumbers: []int{
			chunkNumber,
		},
	}

	var resp api.MultiPartGetURLsResponse

	err = s.f.pacer.Call(func() (bool, error) {
		res, err := s.f.srv.CallJSON(ctx, &partOpts, req, &resp)
		return shouldRetry(ctx, res, err)
	})

	if err != nil {
		return 0, fmt.Errorf("failed to get part URL: %w", err)
	}

	if len(resp.URLs) != 1 {
		return 0, fmt.Errorf("expecting 1 URL  but got %d", len(resp.URLs))
	}
	partURL := resp.URLs[0].URL

	opts := rest.Opts{
		Method:        "PUT",
		RootURL:       partURL,
		Body:          reader,
		ContentType:   "application/octet-stream",
		ContentLength: &chunkSize,
		NoResponse:    true,
		ExtraHeaders: map[string]string{
			"Authorization": "", // clear the default auth
		},
	}

	var uploadRes *http.Response

	err = s.f.pacer.Call(func() (bool, error) {
		_, err = reader.Seek(0, io.SeekStart)
		if err != nil {
			return false, fmt.Errorf("failed to seek chunk: %w", err)
		}
		uploadRes, err = s.f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, uploadRes, err)
	})

	if err != nil {
		return 0, fmt.Errorf("failed to upload part %d: %w", chunkNumber, err)
	}

	// Get ETag from response
	etag := uploadRes.Header.Get("ETag")
	fs.CheckClose(uploadRes.Body, &err)

	s.completedPartsMu.Lock()
	defer s.completedPartsMu.Unlock()
	s.completedParts = append(s.completedParts, api.CompletedPart{
		PartNumber: int32(chunkNumber),
		ETag:       etag,
	})

	// Count size written for unknown file sizes
	s.written.Add(chunkSize)

	return chunkSize, nil
}

// Close complete chunked writer finalising the file.
func (s *drimeChunkWriter) Close(ctx context.Context) error {
	s.completedPartsMu.Lock()
	defer s.completedPartsMu.Unlock()

	// Complete multipart upload
	sort.Slice(s.completedParts, func(i, j int) bool {
		return s.completedParts[i].PartNumber < s.completedParts[j].PartNumber
	})

	completeBody := api.MultiPartCompleteRequest{
		UploadID: s.uploadID,
		Key:      s.key,
		Parts:    s.completedParts,
	}

	completeOpts := rest.Opts{
		Method: "POST",
		Path:   "/s3/multipart/complete",
	}

	var response api.MultiPartCompleteResponse

	err := s.f.pacer.Call(func() (bool, error) {
		res, err := s.f.srv.CallJSON(ctx, &completeOpts, completeBody, &response)
		return shouldRetry(ctx, res, err)
	})

	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	finalSize := s.size
	if finalSize < 0 {
		finalSize = s.written.Load()
	}

	// s3/entries request to create drime object from multipart upload
	req := api.MultiPartEntriesRequest{
		ClientMime:      s.mime,
		ClientName:      s.leaf,
		Filename:        s.uploadName,
		Size:            finalSize,
		ClientExtension: s.extension,
		ParentID:        s.parentID,
		RelativePath:    s.relativePath,
	}

	entriesOpts := rest.Opts{
		Method: "POST",
		Path:   "/s3/entries",
	}

	var res api.MultiPartEntriesResponse
	err = s.f.pacer.Call(func() (bool, error) {
		res, err := s.f.srv.CallJSON(ctx, &entriesOpts, req, &res)
		return shouldRetry(ctx, res, err)
	})
	if err != nil {
		return fmt.Errorf("failed to create entry after multipart upload: %w", err)
	}
	s.fileEntry = res.FileEntry

	return nil
}

// Abort chunk write
//
// You can and should call Abort without calling Close.
func (s *drimeChunkWriter) Abort(ctx context.Context) error {
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/s3/multipart/abort",
		NoResponse: true,
	}

	req := api.MultiPartAbort{
		UploadID: s.uploadID,
		Key:      s.key,
	}

	err := s.f.pacer.Call(func() (bool, error) {
		res, err := s.f.srv.CallJSON(ctx, &opts, req, nil)
		return shouldRetry(ctx, res, err)
	})

	if err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}

	return nil
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns a string version
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

// Hash returns the hash of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// setMetaDataAny sets the metadata from info but doesn't check the type
func (o *Object) setMetaDataAny(info *api.Item) {
	o.size = info.FileSize
	o.modTime = info.UpdatedAt
	o.id = info.ID.String()
	o.dirID = info.ParentID.String()
	o.mimeType = info.Mime
	o.url = info.URL
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Type == api.ItemTypeFolder {
		return fs.ErrorIsDir
	}
	if info.ID == "" {
		return fmt.Errorf("ID not found in response")
	}
	o.setMetaDataAny(info)
	return nil
}

// readMetaData gets the metadata unconditionally as we expect Object
// to always have the full set of metadata
func (o *Object) readMetaData(ctx context.Context) (err error) {
	var info *api.Item
	info, err = o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	if o.url == "" {
		// On upload an Object is returned with no url, so fetch it here if needed
		err = o.readMetaData(ctx)
		if err != nil {
			return nil, fmt.Errorf("read metadata: %w", err)
		}
	}
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: baseURL + o.url,
		Options: options,
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	remote := o.Remote()
	size := src.Size()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// If the file exists, delete it after a successful upload
	if o.id != "" {
		id := o.id
		o.id = ""
		defer func() {
			if err != nil {
				return
			}
			fs.Debugf(o, "Removing old object on successful upload")
			deleteErr := o.fs.deleteObject(ctx, id)
			if deleteErr != nil {
				err = fmt.Errorf("failed to delete existing object: %w", deleteErr)
			}
		}()
	}

	if size < 0 || size > int64(o.fs.opt.UploadCutoff) {
		chunkWriter, err := multipart.UploadMultipart(ctx, src, in, multipart.UploadMultipartOptions{
			Open:        o.fs,
			OpenOptions: options,
		})
		if err != nil {
			return err
		}
		s := chunkWriter.(*drimeChunkWriter)

		return o.setMetaData(&s.fileEntry)
	}

	// Do the upload
	var resp *http.Response
	var result api.UploadResponse
	var encodedLeaf = o.fs.opt.Enc.FromStandardName(leaf)
	opts := rest.Opts{
		Method: "POST",
		Body:   in,
		MultipartParams: url.Values{
			"parentId":     {directoryID},
			"relativePath": {encodedLeaf},
		},
		MultipartContentName: "file",
		MultipartFileName:    encodedLeaf,
		MultipartContentType: fs.MimeType(ctx, src),
		Path:                 "/uploads",
		Options:              options,
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	return o.setMetaData(&result.FileEntry)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// MimeType returns the content type of the Object if known, or "" if not
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// ParentID returns the ID of the Object parent if known, or "" if not
func (o *Object) ParentID() string {
	return o.dirID
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.OpenChunkWriter = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.ParentIDer      = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
)
