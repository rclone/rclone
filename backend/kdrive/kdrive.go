// Package kdrive provides an interface to the kDrive
// object storage system.
package kdrive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/kdrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/zeebo/xxh3"
)

const (
	defaultEndpoint = "https://api.infomaniak.com"
	minSleep        = 10 * time.Millisecond
	maxSleep        = 2 * time.Second
	decayConstant   = 2                // bigger for slower decay, exponential
	uploadThreshold = 20 * 1024 * 1024 // 20 Mo
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "kdrive",
		Description: "Infomaniak kDrive",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: encoder.Display | encoder.EncodeLeftSpace | encoder.EncodeRightSpace | encoder.EncodeInvalidUtf8,
		}, {
			Name: "root_folder_id",
			Help: `The folder to use as root.

You may use either one of the convenience shortcuts [private|common|shared]
or an explicit folder ID.

- private: your user private directory
- common: the kDrive common directory, shared among users
- shared: the folder with the files shared with you`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "drive_id",
			Help: `ID of the kDrive to use.

When showing a folder on kdrive, you can find the drive_id here:
https://ksuite.infomaniak.com/{account_id}/kdrive/app/drive/{drive_id}/files/...`,
			Default: "",
		}, {
			Name:       "access_token",
			Help:       `Access token generated in Infomaniak profile manager.`,
			Required:   true,
			IsPassword: true,
			Sensitive:  true,
		}, {
			Name: "endpoint",
			Help: `The API endpoint to use.

Leave blank normally. There is no reason to change the endpoint except for internal use or beta-testing.`,
			Advanced: true,
		},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Enc          encoder.MultiEncoder `config:"encoding"`
	RootFolderID string               `config:"root_folder_id"`
	DriveID      string               `config:"drive_id"`
	AccessToken  string               `config:"access_token"`
	Endpoint     string               `config:"endpoint"`
}

// Fs represents a remote kdrive
type Fs struct {
	name          string             // name of this remote
	root          string             // the path we are working on
	opt           Options            // parsed options
	features      *fs.Features       // optional features
	srv           *rest.Client       // the connection to the server
	cleanupSrv    *rest.Client       // the connection used for the cleanup method
	dirCache      *dircache.DirCache // Map of directory path to directory id
	pacer         *fs.Pacer          // pacer for API calls
	cacheNotFound map[string]cacheEntry
}

// Object describes a kdrive object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	xxh3        string    // XXH3 if known
}

type cacheEntry struct {
	item *api.Item
	err  error
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
	return fmt.Sprintf("kdrive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// clearNotFoundCache removes all entries from the not-found cache
func (f *Fs) clearNotFoundCache() {
	f.cacheNotFound = make(map[string]cacheEntry)
}

// parsePath parses a kdrive 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	408, // Request Timeout
	429, // Too Many Requests
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

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.ResultStatus)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.ErrorDetail.ErrorString == "" {
		errResponse.ErrorDetail.ErrorString = resp.Status
	}
	if errResponse.ErrorDetail.Result == "" {
		errResponse.ErrorDetail.Result = resp.Status
	}
	return errResponse
}

// isNotFoundError checks if the given error is a standard kDrive API "Not Found" error.
func isNotFoundError(err error) bool {
	var apiErr *api.ResultStatus

	return errors.As(err, &apiErr) && apiErr.ErrorDetail.Result == "object_not_found"
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Use the default API endpoint unless explicitly defined;
	// not defined as option default to keep it hidden from end user,
	// since there is little to no reason to change it.
	if opt.Endpoint == "" {
		opt.Endpoint = defaultEndpoint
	}

	accessToken, err := obscure.Reveal(opt.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("couldn't decrypt access token: %w", err)
	}

	root = parsePath(root)
	fs.Debugf(ctx, "NewFs: for root=%s", root)

	f := &Fs{
		name:          name,
		root:          root,
		opt:           *opt,
		srv:           rest.NewClient(fshttp.NewClient(ctx)).SetRoot(opt.Endpoint),
		pacer:         fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		cacheNotFound: make(map[string]cacheEntry),
	}
	f.srv.SetHeader("Authorization", "Bearer "+accessToken)

	f.cleanupSrv = rest.NewClient(fshttp.NewClient(ctx)).SetRoot(opt.Endpoint)
	f.cleanupSrv.SetHeader("Authorization", "Bearer "+accessToken)

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		CanHaveEmptyDirectories: true,
		PartialUploads:          false,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)

	rootID, err := f.computeRootID()
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, "kdriveInitCache", make(map[string]cacheEntry))

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
		_, err = tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if errors.Is(err, fs.ErrorObjectNotFound) {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
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

// computeRootID finds the real RootId of the configured RootFolderID.
func (f *Fs) computeRootID() (rootID string, err error) {
	ctx := context.Background()

	if _, err := strconv.Atoi(f.opt.RootFolderID); err == nil {
		rootID = f.opt.RootFolderID
	} else {
		switch f.opt.RootFolderID {
		case "private":
			rootID, _, err = f.FindLeaf(ctx, "1", "Private")
		case "common":
			rootID, _, err = f.FindLeaf(ctx, "1", "Common documents")
		case "shared":
			rootID, _, err = f.FindLeaf(ctx, "1", "Shared")
		case "":
			rootID, _, err = f.FindLeaf(ctx, "1", "Private")
		default:
			rootID, _, err = f.FindLeaf(ctx, "1", f.opt.RootFolderID)
		}
	}

	fs.Debugf(nil, "ROOTFOLDERID %w ROOTID %s", f.opt.RootFolderID, rootID)

	return
}

// getItem retrieves a file or directory by its ID.
func (f *Fs) getItem(ctx context.Context, id string) (*api.Item, error) {
	// https://developer.infomaniak.com/docs/api/get/2/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D
	opts := rest.Opts{
		Method:     "GET",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s", f.opt.DriveID, id),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("with", "path")

	var result api.ItemResult
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, fmt.Errorf("couldn't get item: %w", err)
	}

	item := result.Data
	if item.ID == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	item.Name = f.opt.Enc.ToStandardName(item.Name)
	return &item, nil
}

// findItemInDir retrieves a file or directory by its name in a specific directory using the API.
func (f *Fs) findItemInDir(ctx context.Context, directoryID string, leaf string) (*api.Item, error) {
	cacheKey := directoryID + "|" + leaf

	if entry, entryExists := f.cacheNotFound[cacheKey]; entryExists {
		return entry.item, entry.err
	}

	cacheInit, cacheInitExist := ctx.Value("kdriveInitCache").(map[string]cacheEntry)
	if cacheInitExist {
		if entry, entryExists := cacheInit[cacheKey]; entryExists {
			return entry.item, entry.err
		}
	}

	// https://developer.infomaniak.com/docs/api/get/3/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/name
	opts := rest.Opts{
		Method:     "GET",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s/name", f.opt.DriveID, directoryID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(leaf))
	opts.Parameters.Set("with", "path,hash")

	var result api.ItemResult
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if isNotFoundError(err) {
			f.cacheNotFound[cacheKey] = cacheEntry{nil, fs.ErrorObjectNotFound}
			return nil, fs.ErrorObjectNotFound
		}
		return nil, fmt.Errorf("couldn't find item in dir: %w", err)
	}

	item := result.Data
	// Check if item is valid (has an ID)
	if item.ID == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	// Normalize the name
	item.Name = f.opt.Enc.ToStandardName(item.Name)

	if cacheInitExist {
		cacheInit[cacheKey] = cacheEntry{&item, nil}
	}

	return &item, nil
}

// findItemByPath retrieves a file or directory by its path
func (f *Fs) findItemByPath(ctx context.Context, remote string) (*api.Item, error) {
	if remote == "" || remote == "." {
		rootID, err := f.dirCache.FindDir(ctx, "", false)
		if err != nil {
			return nil, err
		}
		return f.getItem(ctx, rootID)
	}

	// Get the directoryID of the parent directory
	directory, leaf := path.Split(remote)
	directory = strings.TrimSuffix(directory, "/")

	directoryID, err := f.dirCache.FindDir(ctx, directory, false)
	if err != nil {
		if errors.Is(err, fs.ErrorDirNotFound) {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	return f.findItemInDir(ctx, directoryID, leaf)
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, remote string) (*api.Item, error) {
	// fs.Debugf(ctx, "readMetaDataForPath: remote=%s", remote)

	// Try the new API endpoint first
	info, err := f.findItemByPath(ctx, remote)
	if err != nil {
		return nil, err
	}

	return info, nil
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
	// Use API endpoint to find item directly by name
	item, err := f.findItemInDir(ctx, pathID, leaf)
	if err != nil {
		if errors.Is(err, fs.ErrorObjectNotFound) {
			return "", false, nil
		}
		return "", false, err
	}

	if item.Type == "dir" {
		return strconv.Itoa(item.ID), true, nil
	}

	// Not a directory
	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	var result api.CreateDirResult

	// https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/directory
	opts := rest.Opts{
		Method:     "POST",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s/directory", f.opt.DriveID, pathID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(leaf))
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}

	f.clearNotFoundCache()
	return strconv.Itoa(result.Data.ID), nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.Item) bool

// Lists the directory required calling the user function on each item found.
//
// If the user fn ever returns true then it early exits with found = true.
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, recursive bool, fn listAllFn) (found bool, err error) {
	rootItem, err := f.getItem(ctx, dirID)
	if err != nil {
		return false, err
	}
	rootPath := rootItem.FullPath + "/"

	listSomeFiles := func(currentDirID string, fromCursor string) (api.SearchResult, error) {
		// https://developer.infomaniak.com/docs/api/get/3/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/files
		opts := rest.Opts{
			Method:     "GET",
			Path:       fmt.Sprintf("/3/drive/%s/files/%s/files", f.opt.DriveID, currentDirID),
			Parameters: url.Values{},
		}
		opts.Parameters.Set("limit", "1000")
		opts.Parameters.Set("with", "path")
		if recursive {
			opts.Parameters.Set("depth", "unlimited")
		}

		if len(fromCursor) > 0 {
			opts.Parameters.Set("cursor", fromCursor)
		}

		var result api.SearchResult
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			err = result.ResultStatus.Update(err)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return result, fmt.Errorf("couldn't list files: %w", err)
		}
		return result, nil
	}

	var listErr error
	var recursiveContents func(currentDirID string, currentSubDir string, fromCursor string)

	recursiveContents = func(currentDirID string, currentSubDir string, fromCursor string) {
		if listErr != nil {
			return
		}
		result, err := listSomeFiles(currentDirID, fromCursor)
		if err != nil {
			listErr = err
			return
		}

		// First, analyze what has been returned, and go in-depth if required
		for i := range result.Data {
			item := &result.Data[i]
			if item.Type == "dir" {
				if filesOnly {
					continue
				}
			} else {
				if directoriesOnly {
					continue
				}
			}

			item.Name = f.opt.Enc.ToStandardName(item.Name)
			item.FullPath = f.opt.Enc.ToStandardPath(strings.TrimPrefix(item.FullPath, rootPath))

			if fn(item) {
				found = true
				break
			}

			if recursive && currentDirID == "1" && item.Type == "dir" {
				recursiveContents(strconv.Itoa(item.ID), path.Join(currentSubDir, item.Name), "" /*reset cursor*/)
			}
		}

		// Then load the rest of the files in that folder and apply the same logic
		if result.HasMore {
			recursiveContents(currentDirID, currentSubDir, result.Cursor)
		}
	}

	recursiveContents(dirID, "", "")

	if listErr != nil {
		return found, listErr
	}
	return
}

// listHelper iterates over all items from the directory
// and calls the callback for each element.
func (f *Fs) listHelper(ctx context.Context, dir string, recursive bool, callback func(entries fs.DirEntry) error) (err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	//fs.Debugf(ctx, "listHelper: root=%s dir=%s directoryID=%s", f.root, dir, directoryID)
	if err != nil {
		return err
	}
	var iErr error

	_, err = f.listAll(ctx, directoryID, false, false, recursive, func(info *api.Item) bool {
		remote := path.Join(dir, info.FullPath)

		// When not recursive, only return direct children
		if !recursive {
			itemDir := path.Dir(remote)
			// Normalize: "." becomes "" for root
			if itemDir == "." {
				itemDir = ""
			}
			if itemDir != dir {
				// Skip - not a direct child
				return false
			}
		}

		if info.Type == "dir" {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, strconv.Itoa(info.ID))

			d := fs.NewDir(remote, info.ModTime()).SetID(strconv.Itoa(info.ID))
			d.SetParentID(strconv.Itoa(info.ParentID))
			d.SetSize(info.Size)

			iErr = callback(d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}

			iErr = callback(o)
		}

		if iErr != nil {
			return true
		}
		return false
	})

	if err != nil {
		return err
	}

	if iErr != nil {
		return iErr
	}

	return nil
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
func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	l := list.NewHelper(callback)
	err = f.listHelper(ctx, dir, false, func(o fs.DirEntry) error {
		return l.Add(o)
	})
	if err != nil {
		return err
	}
	return l.Flush()
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	l := list.NewHelper(callback)
	err = f.listHelper(ctx, dir, true, func(o fs.DirEntry) error {
		// fs.Debugf(nil, "ADD OBJECT %s", o.Remote())
		return l.Add(o)
	})
	if err != nil {
		return err
	}
	return l.Flush()
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	// fs.Debugf(ctx, "createObject: remote = %s", remote)
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:      f,
		remote:  remote,
		size:    size,
		modTime: modTime,
	}
	return o, leaf, directoryID, nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, leaf, directoryID, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.update(ctx, in, src, directoryID, leaf, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
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

	nonEmpty, err := f.listAll(ctx, rootID, false, false, false, func(i *api.Item) bool {
		return true
	})
	if (nonEmpty || err != nil) && check {
		return fmt.Errorf("rmdir failed: directory %s not empty", dir)
	}

	// https://developer.infomaniak.com/docs/api/delete/2/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       fmt.Sprintf("/2/drive/%s/files/%s", f.opt.DriveID, rootID),
		Parameters: url.Values{},
	}
	var resp *http.Response
	var result api.CancellableResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("rmdir failed: %w", err)
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
	return time.Second
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
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/copy/%7Bdestination_directory_id%7D
	opts := rest.Opts{
		Method:     "POST",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s/copy/%s", f.opt.DriveID, srcObj.id, directoryID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(leaf))
	//opts.Parameters.Set("mtime", fmt.Sprintf("%d", uint64(srcObj.modTime.Unix())))
	var resp *http.Response
	var result api.FileCopyResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	err = dstObj.setMetaData(&result.Data)
	if err != nil {
		return nil, err
	}

	f.clearNotFoundCache()
	return dstObj, nil
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// CleanUp empties the trash
func (f *Fs) CleanUp(ctx context.Context) error {
	// https://developer.infomaniak.com/docs/api/delete/2/drive/%7Bdrive_id%7D/trash
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       fmt.Sprintf("/2/drive/%s/trash", f.opt.DriveID),
		Parameters: url.Values{},
	}
	var resp *http.Response
	var result api.ResultStatus
	var err error
	return f.pacer.Call(func() (bool, error) {
		resp, err = f.cleanupSrv.CallJSON(ctx, &opts, nil, &result)
		err = result.Update(err)
		return shouldRetry(ctx, resp, err)
	})
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

	// Create temporary object
	moveDst, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/move/%7Bdestination_directory_id%7D
	opts := rest.Opts{
		Method:     "POST",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s/move/%s", f.opt.DriveID, srcObj.id, directoryID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(leaf))
	var resp *http.Response
	var result api.CancellableResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		if err != nil && err.(*api.ResultStatus) != nil {
			if err.(*api.ResultStatus).ErrorDetail.Result == "conflict_error" {
				// Destination already exists => remove if and retry
				err = moveDst.readMetaData(ctx)
				if err != nil {
					return false, err
				}
				err = moveDst.Remove(ctx)
				if err != nil {
					return false, err
				}

				f.clearNotFoundCache()
				return true, nil
			}
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}

	f.clearNotFoundCache()
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

	srcID, _, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	// https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/move/%7Bdestination_directory_id%7D
	opts := rest.Opts{
		Method:     "POST",
		Path:       fmt.Sprintf("/3/drive/%s/files/%s/move/%s", f.opt.DriveID, srcID, dstDirectoryID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("name", f.opt.Enc.FromStandardName(dstLeaf))
	var resp *http.Response
	var result api.CancellableResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

	srcFs.dirCache.FlushDir(srcRemote)
	srcFs.dirCache.FlushDir(dstRemote)
	f.clearNotFoundCache()
	return nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

func (f *Fs) getPublicLink(ctx context.Context, fileID int) (string, bool, error) {
	// https://developer.infomaniak.com/docs/api/get/2/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/link
	opts := rest.Opts{
		Method: "GET",
		Path:   fmt.Sprintf("/2/drive/%s/files/%d/link", f.opt.DriveID, fileID),
	}
	var result api.PubLinkResult
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Update(err)
		return shouldRetry(ctx, resp, err)
	})

	expired := false
	if err != nil {
		return "", expired, fmt.Errorf("failed to get public link: %w", err)
	}

	// check if the shared link is blocked
	if result.Data.AccessBlocked == true {
		return "", expired, nil
	}

	// check if the shared link is expired
	if result.Data.ValidUntil > 0 {
		// ValidUntil is a Unix timestamp (UTC)
		if time.Now().Unix() > int64(result.Data.ValidUntil) {
			// link is expired
			expired = true
		}
	}

	return result.Data.URL, expired, nil
}

func (f *Fs) deletePublicLink(ctx context.Context, fileID int) error {
	// https://developer.infomaniak.com/docs/api/delete/2/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/link
	opts := rest.Opts{
		Method: "DELETE",
		Path:   fmt.Sprintf("/2/drive/%s/files/%d/link", f.opt.DriveID, fileID),
	}
	var result api.ResultStatus
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.Update(err)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return fmt.Errorf("failed to remove public link: %w", err)
	}

	return nil
}

func (f *Fs) createPublicLink(ctx context.Context, fileID int, expire fs.Duration) (string, error) {
	createReq := struct {
		Right      string `json:"right"` // true for read access
		ValidUntil int    `json:"valid_until,omitempty"`
	}{
		Right: "public",
	}

	// Set expiry if provided
	if expire != fs.DurationOff {
		createReq.ValidUntil = int(time.Now().Add(time.Duration(expire)).Unix())
	}

	// https://developer.infomaniak.com/docs/api/post/2/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/link
	opts := rest.Opts{
		Method: "POST",
		Path:   fmt.Sprintf("/2/drive/%s/files/%d/link", f.opt.DriveID, fileID),
	}

	var result api.PubLinkResult
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, createReq, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return "", fmt.Errorf("failed to create public link: %w", err)
	}

	return result.Data.URL, nil
}

func (f *Fs) updatePublicLink(ctx context.Context, fileID int, expire fs.Duration) error {
	createReq := struct {
		ValidUntil int `json:"valid_until,omitempty"`
	}{}

	// Set expiry if provided
	if expire != fs.DurationOff {
		createReq.ValidUntil = int(time.Now().Add(time.Duration(expire)).Unix())
	}

	// https://developer.infomaniak.com/docs/api/put/2/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/link
	opts := rest.Opts{
		Method: "PUT",
		Path:   fmt.Sprintf("/2/drive/%s/files/%d/link", f.opt.DriveID, fileID),
	}

	var result api.PubLinkUpdateResult
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, createReq, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return fmt.Errorf("failed to update public link: %w", err)
	}

	return nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
// If unlink is true, it removes the existing public link.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	item, err := f.findItemByPath(ctx, remote)
	if err != nil {
		return "", err
	}

	if unlink {
		// Remove existing public link
		err = f.deletePublicLink(ctx, item.ID)
		return "", err
	}

	// Check if link exists
	link, expired, err := f.getPublicLink(ctx, item.ID)
	if link != "" {
		// link exist
		if expire != fs.DurationOff {
			// update valid until and return the same link
			err = f.updatePublicLink(ctx, item.ID, expire)
			if err != nil {
				return "", err
			}
			return link, nil
		} else if expired == true {
			// delete and recreate after
			f.deletePublicLink(ctx, item.ID)
		} else {
			// return link
			return link, nil
		}
	}

	// Create or get existing public link
	return f.createPublicLink(ctx, item.ID, expire)
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	// https://developer.infomaniak.com/docs/api/get/2/drive/%7Bdrive_id%7D
	opts := rest.Opts{
		Method:     "GET",
		Path:       fmt.Sprintf("/2/drive/%s", f.opt.DriveID),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("only", "used_size,size")
	var resp *http.Response
	var q api.QuotaInfo
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &q)
		err = q.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	free := max(q.Data.Size-q.Data.UsedSize, 0)
	usage = &fs.Usage{
		Total: fs.NewUsageValue(q.Data.Size),     // quota of bytes that can be used
		Used:  fs.NewUsageValue(q.Data.UsedSize), // bytes in use
		Free:  fs.NewUsageValue(free),            // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// Shutdown shutdown the fs
func (f *Fs) Shutdown(_ context.Context) error {
	return nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	// kDrive only supports xxh3
	return hash.Set(hash.XXH3)
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

// getHashes fetches the hashes into the object
func (o *Object) retrieveHash(ctx context.Context) (err error) {
	var resp *http.Response
	var result api.ChecksumFileResult

	// https://developer.infomaniak.com/docs/api/get/2/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/hash
	opts := rest.Opts{
		Method: "GET",
		Path:   fmt.Sprintf("/2/drive/%s/files/%s/hash", o.fs.opt.DriveID, o.id),
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	o.setHash(strings.TrimPrefix(result.Data.Hash, "xxh3:"))
	return nil
}

// Hash returns the XXH3 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	var pHash *string
	switch t {
	case hash.XXH3:
		pHash = &o.xxh3
	default:
		return "", hash.ErrUnsupported
	}
	if o.xxh3 == "" {
		err := o.retrieveHash(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get hash: %w", err)
		}
	}
	return *pHash, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(context.Background())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Type == "dir" {
		return fs.ErrorIsDir
	}
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = info.ModTime()
	o.id = strconv.Itoa(info.ID)
	if len(o.xxh3) == 0 && len(info.Hash) > 0 {
		o.xxh3 = strings.TrimPrefix(info.Hash, "xxh3:")
	}
	return nil
}

// setHash sets the hashes from that passed in
func (o *Object) setHash(hash string) {
	o.xxh3 = hash
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
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
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	if modTime.Unix() == 0 {
		return fs.ErrorCantSetModTime
	}

	var result api.ResultStatus

	modTimeReq := struct {
		LastModifiedAt int64 `json:"last_modified_at"`
	}{
		LastModifiedAt: modTime.Unix(),
	}

	// https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/last-modified
	opts := rest.Opts{
		Method: "POST",
		Path:   fmt.Sprintf("/3/drive/%s/files/%s/last-modified", o.fs.opt.DriveID, o.id),
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, modTimeReq, &result)
		err = result.Update(err)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return fs.ErrorCantSetModTime
	}

	// Update Object modTime
	o.modTime = modTime
	return nil
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	fs.FixRangeOption(options, o.Size())
	var resp *http.Response

	// https://developer.infomaniak.com/docs/api/get/2/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D/download
	opts := rest.Opts{
		Method:  "GET",
		Path:    fmt.Sprintf("/2/drive/%s/files/%s/download", o.fs.opt.DriveID, o.id),
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
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size() // NB can upload without size
	remote := o.Remote()

	if size < 0 {
		return errors.New("can't upload unknown sizes objects")
	}

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	return o.update(ctx, in, src, directoryID, leaf, options...)
}

func (o *Object) update(ctx context.Context, in io.Reader, src fs.ObjectInfo, directoryID, leaf string, options ...fs.OpenOption) (err error) {
	size := src.Size() // NB can upload without size

	if size < 0 {
		return errors.New("can't upload unknown sizes objects")
	}

	// if file size is less than the threshold, upload direct
	if size <= uploadThreshold {
		return o.updateDirect(ctx, in, directoryID, leaf, src, options...)
	}
	// else, use multipart upload with parallelism
	return o.updateMultipart(ctx, in, src, options...)
}

func (o *Object) updateDirect(ctx context.Context, in io.Reader, directoryID, leaf string, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {

	var resp *http.Response
	var result api.UploadFileResponse

	// Attempt to get the hash from the source object without reading the content
	totalHash, err := src.Hash(ctx, hash.XXH3)
	var body io.Reader
	size := src.Size()

	if err == nil && totalHash != "" {
		// Hash is already known (e.g., local file)
		// Stream directly without loading into memory
		body = in
		totalHash = "xxh3:" + totalHash
	} else {
		// Hash unknown, need to read content to calculate it
		content, err := io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("failed to read file content: %w", err)
		}

		// Calculate xxh3 hash
		hasher := xxh3.New()
		_, _ = hasher.Write(content)
		sum := hasher.Sum(nil)
		totalHash = fmt.Sprintf("xxh3:%x", sum)

		body = bytes.NewReader(content)
	}

	// https://developer.infomaniak.com/docs/api/post/3/drive/%7Bdrive_id%7D/upload
	opts := rest.Opts{
		Method:           "POST",
		Path:             fmt.Sprintf("/3/drive/%s/upload", o.fs.opt.DriveID),
		Body:             body,
		ContentType:      fs.MimeType(ctx, src),
		ContentLength:    &size,
		Parameters:       url.Values{},
		TransferEncoding: []string{"identity"}, // kdrive doesn't like chunked encoding
		Options:          options,
	}

	leaf = o.fs.opt.Enc.FromStandardName(leaf)
	opts.Parameters.Set("file_name", leaf)
	opts.Parameters.Set("directory_id", directoryID)
	opts.Parameters.Set("total_size", fmt.Sprintf("%d", size))
	opts.Parameters.Set("last_modified_at", fmt.Sprintf("%d", uint64(src.ModTime(ctx).Unix())))
	opts.Parameters.Set("conflict", "version")
	opts.Parameters.Set("with", "hash")
	opts.Parameters.Set("total_chunk_hash", totalHash)

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return err
	}

	o.size = size
	o.setHash(strings.TrimPrefix(result.Data.Hash, "xxh3:"))

	o.fs.clearNotFoundCache()
	return o.readMetaData(ctx)
}

// updateMultipart uploads large files using chunked upload with parallel streaming
func (o *Object) updateMultipart(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	f := o.fs

	chunkWriter, err := f.UploadMultipart(ctx, src, in, options)
	if err != nil {
		return err
	}

	// Extract the file info from the chunk writer
	session := chunkWriter.(*uploadSession)
	if session.fileInfo == nil {
		return fmt.Errorf("upload failed: no file info returned")
	}

	o.fs.clearNotFoundCache()
	return o.setMetaData(session.fileInfo)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	// https://developer.infomaniak.com/docs/api/delete/2/drive/%7Bdrive_id%7D/files/%7Bfile_id%7D
	opts := rest.Opts{
		Method: "DELETE",
		Path:   fmt.Sprintf("/2/drive/%s/files/%s", o.fs.opt.DriveID, o.id),
	}
	var result api.CancellableResponse
	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		err = result.ResultStatus.Update(err)
		return shouldRetry(ctx, resp, err)
	})
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.ListPer         = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
