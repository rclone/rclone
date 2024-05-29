// Package linkbox provides an interface to the linkbox.to Cloud storage system.
//
// API docs: https://www.linkbox.to/api-docs
package linkbox

/*
   Extras
   - PublicLink - NO - sharing doesn't share the actual file, only a page with it on
   - Move - YES - have Move and Rename file APIs so is possible
   - MoveDir - NO - probably not possible - have Move but no Rename
*/

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	maxEntitiesPerPage = 1000
	minSleep           = 200 * time.Millisecond
	maxSleep           = 2 * time.Second
	pacerBurst         = 1
	linkboxAPIURL      = "https://www.linkbox.to/api/open/"
	rootID             = "0" // ID of root directory
)

func init() {
	fsi := &fs.RegInfo{
		Name:        "linkbox",
		Description: "Linkbox",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "token",
			Help:      "Token from https://www.linkbox.to/admin/account",
			Sensitive: true,
			Required:  true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Token string `config:"token"`
}

// Fs stores the interface to the remote Linkbox files
type Fs struct {
	name     string
	root     string
	opt      Options            // options for this backend
	features *fs.Features       // optional features
	ci       *fs.ConfigInfo     // global config
	srv      *rest.Client       // the connection to the server
	dirCache *dircache.DirCache // Map of directory path to directory id
	pacer    *fs.Pacer
}

// Object is a remote object that has been stat'd (so it exists, but is not necessarily open for reading)
type Object struct {
	fs          *Fs
	remote      string
	size        int64
	modTime     time.Time
	contentType string
	fullURL     string
	dirID       int64
	itemID      string // and these IDs are for files
	id          int64  // these IDs appear to apply to directories
	isDir       bool
}

// NewFs creates a new Fs object from the name and root. It connects to
// the host specified in the config file.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	root = strings.Trim(root, "/")
	// Parse config into Options struct

	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	ci := fs.GetConfig(ctx)

	f := &Fs{
		name: name,
		opt:  *opt,
		root: root,
		ci:   ci,
		srv:  rest.NewClient(fshttp.NewClient(ctx)),

		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep))),
	}
	f.dirCache = dircache.New(root, rootID, f)

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CaseInsensitive:         true,
	}).Fill(ctx, f)

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
		_, err := tempF.NewObject(ctx, remote)
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

type entity struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Ctime  int64  `json:"ctime"`
	Size   int64  `json:"size"`
	ID     int64  `json:"id"`
	Pid    int64  `json:"pid"`
	ItemID string `json:"item_id"`
}

// Return true if the entity is a directory
func (e *entity) isDir() bool {
	return e.Type == "dir" || e.Type == "sdir"
}

type data struct {
	Entities []entity `json:"list"`
}
type fileSearchRes struct {
	response
	SearchData data `json:"data"`
}

// Set an object info from an entity
func (o *Object) set(e *entity) {
	o.modTime = time.Unix(e.Ctime, 0)
	o.contentType = e.Type
	o.size = e.Size
	o.fullURL = e.URL
	o.isDir = e.isDir()
	o.id = e.ID
	o.itemID = e.ItemID
	o.dirID = e.Pid
}

// Call linkbox with the query in opts and return result
//
// This will be checked for error and an error will be returned if Status != 1
func getUnmarshaledResponse(ctx context.Context, f *Fs, opts *rest.Opts, result interface{}) error {
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	responser := result.(responser)
	if responser.IsError() {
		return responser
	}
	return nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*entity) bool

// Search is a bit fussy about which characters match
//
// If the name doesn't match this then do an dir list instead
// N.B.: Linkbox doesn't support search by name that is longer than 50 chars
var searchOK = regexp.MustCompile(`^[a-zA-Z0-9_ -.]{1,50}$`)

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
//
// If you set name then search ignores dirID. name is a substring
// search also so name="dir" matches "sub dir" also. This filters it
// down so it only returns items in dirID
func (f *Fs) listAll(ctx context.Context, dirID string, name string, fn listAllFn) (found bool, err error) {
	var (
		pageNumber       = 0
		numberOfEntities = maxEntitiesPerPage
	)
	name = strings.TrimSpace(name) // search doesn't like spaces
	if !searchOK.MatchString(name) {
		// If name isn't good then do an unbounded search
		name = ""
	}

OUTER:
	for numberOfEntities == maxEntitiesPerPage {
		pageNumber++
		opts := &rest.Opts{
			Method:  "GET",
			RootURL: linkboxAPIURL,
			Path:    "file_search",
			Parameters: url.Values{
				"token":    {f.opt.Token},
				"name":     {name},
				"pid":      {dirID},
				"pageNo":   {itoa(pageNumber)},
				"pageSize": {itoa64(maxEntitiesPerPage)},
			},
		}

		var responseResult fileSearchRes
		err = getUnmarshaledResponse(ctx, f, opts, &responseResult)
		if err != nil {
			return false, fmt.Errorf("getting files failed: %w", err)
		}

		numberOfEntities = len(responseResult.SearchData.Entities)

		for _, entity := range responseResult.SearchData.Entities {
			if itoa64(entity.Pid) != dirID {
				// when name != "" this returns from all directories, so ignore not this one
				continue
			}
			if fn(&entity) {
				found = true
				break OUTER
			}
		}
		if pageNumber > 100000 {
			return false, fmt.Errorf("too many results")
		}
	}
	return found, nil
}

// Turn 64 bit int to string
func itoa64(i int64) string {
	return strconv.FormatInt(i, 10)
}

// Turn int to string
func itoa(i int) string {
	return itoa64(int64(i))
}

func splitDirAndName(remote string) (dir string, name string) {
	lastSlashPosition := strings.LastIndex(remote, "/")
	if lastSlashPosition == -1 {
		dir = ""
		name = remote
	} else {
		dir = remote[:lastSlashPosition]
		name = remote[lastSlashPosition+1:]
	}

	// fs.Debugf(nil, "splitDirAndName remote = {%s}, dir = {%s}, name = {%s}", remote, dir, name)

	return dir, name
}

// FindLeaf finds a directory of name leaf in the folder with ID directoryID
func (f *Fs) FindLeaf(ctx context.Context, directoryID, leaf string) (directoryIDOut string, found bool, err error) {
	// Find the leaf in directoryID
	found, err = f.listAll(ctx, directoryID, leaf, func(entity *entity) bool {
		if entity.isDir() && strings.EqualFold(entity.Name, leaf) {
			directoryIDOut = itoa64(entity.ID)
			return true
		}
		return false
	})
	return directoryIDOut, found, err
}

// Returned from "folder_create"
type folderCreateRes struct {
	response
	Data struct {
		DirID int64 `json:"dirId"`
	} `json:"data"`
}

// CreateDir makes a directory with dirID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, dirID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", dirID, leaf)
	opts := &rest.Opts{
		Method:  "GET",
		RootURL: linkboxAPIURL,
		Path:    "folder_create",
		Parameters: url.Values{
			"token":       {f.opt.Token},
			"name":        {leaf},
			"pid":         {dirID},
			"isShare":     {"0"},
			"canInvite":   {"1"},
			"canShare":    {"1"},
			"withBodyImg": {"1"},
			"desc":        {""},
		},
	}

	response := folderCreateRes{}
	err = getUnmarshaledResponse(ctx, f, opts, &response)
	if err != nil {
		// response status 1501 means that directory already exists
		if response.Status == 1501 {
			return newID, fmt.Errorf("couldn't find already created directory: %w", fs.ErrorDirNotFound)
		}
		return newID, fmt.Errorf("CreateDir failed: %w", err)

	}
	if response.Data.DirID == 0 {
		return newID, fmt.Errorf("API returned 0 for ID of newly created directory")
	}
	return itoa64(response.Data.DirID), nil
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
	// fs.Debugf(f, "List method dir = {%s}", dir)
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	_, err = f.listAll(ctx, directoryID, "", func(entity *entity) bool {
		remote := path.Join(dir, entity.Name)
		if entity.isDir() {
			id := itoa64(entity.ID)
			modTime := time.Unix(entity.Ctime, 0)
			d := fs.NewDir(remote, modTime).SetID(id).SetParentID(itoa64(entity.Pid))
			entries = append(entries, d)
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, id)
		} else {
			o := &Object{
				fs:     f,
				remote: remote,
			}
			o.set(entity)
			entries = append(entries, o)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// get an entity with leaf from dirID
func getEntity(ctx context.Context, f *Fs, leaf string, directoryID string, token string) (*entity, error) {
	var result *entity
	var resultErr = fs.ErrorObjectNotFound
	_, err := f.listAll(ctx, directoryID, leaf, func(entity *entity) bool {
		if strings.EqualFold(entity.Name, leaf) {
			// fs.Debugf(f, "getObject found entity.Name {%s} name {%s}", entity.Name, name)
			if entity.isDir() {
				result = nil
				resultErr = fs.ErrorIsDir
			} else {
				result = entity
				resultErr = nil
			}
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	return result, resultErr
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
//
// If remote points to a directory then it should return
// ErrorIsDir if possible without doing any extra work,
// otherwise ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	entity, err := getEntity(ctx, f, leaf, dirID, f.opt.Token)
	if err != nil {
		return nil, err
	}
	o := &Object{
		fs:     f,
		remote: remote,
	}
	o.set(entity)
	return o, nil
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	if check {
		entries, err := f.List(ctx, dir)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	opts := &rest.Opts{
		Method:  "GET",
		RootURL: linkboxAPIURL,
		Path:    "folder_del",
		Parameters: url.Values{
			"token":  {f.opt.Token},
			"dirIds": {directoryID},
		},
	}

	response := response{}
	err = getUnmarshaledResponse(ctx, f, opts, &response)
	if err != nil {
		// Linkbox has some odd error returns here
		if response.Status == 403 || response.Status == 500 {
			return fs.ErrorDirNotFound
		}
		return fmt.Errorf("purge error: %w", err)
	}

	f.dirCache.FlushDir(dir)
	if err != nil {
		return err
	}
	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// SetModTime sets modTime on a particular file
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var res *http.Response
	downloadURL := o.fullURL
	if downloadURL == "" {
		_, name := splitDirAndName(o.Remote())
		newObject, err := getEntity(ctx, o.fs, name, itoa64(o.dirID), o.fs.opt.Token)
		if err != nil {
			return nil, err
		}
		if newObject == nil {
			// fs.Debugf(o.fs, "Open entity is empty: name = {%s}", name)
			return nil, fs.ErrorObjectNotFound
		}

		downloadURL = newObject.URL
	}

	opts := &rest.Opts{
		Method:  "GET",
		RootURL: downloadURL,
		Options: options,
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		var err error
		res, err = o.fs.srv.Call(ctx, opts)
		return o.fs.shouldRetry(ctx, res, err)
	})

	if err != nil {
		return nil, fmt.Errorf("Open failed: %w", err)
	}

	return res.Body, nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	if size == 0 {
		return fs.ErrorCantUploadEmptyFiles
	} else if size < 0 {
		return fmt.Errorf("can't upload files of unknown length")
	}

	remote := o.Remote()

	// remove the file if it exists
	if o.itemID != "" {
		fs.Debugf(o, "Update: removing old file")
		err = o.Remove(ctx)
		if err != nil {
			fs.Errorf(o, "Update: failed to remove existing file: %v", err)
		}
		o.itemID = ""
	} else {
		tmpObject, err := o.fs.NewObject(ctx, remote)
		if err == nil {
			fs.Debugf(o, "Update: removing old file")
			err = tmpObject.Remove(ctx)
			if err != nil {
				fs.Errorf(o, "Update: failed to remove existing file: %v", err)
			}
		}
	}

	first10m := io.LimitReader(in, 10_485_760)
	first10mBytes, err := io.ReadAll(first10m)
	if err != nil {
		return fmt.Errorf("Update err in reading file: %w", err)
	}

	// get upload authorization (step 1)
	opts := &rest.Opts{
		Method:  "GET",
		RootURL: linkboxAPIURL,
		Path:    "get_upload_url",
		Options: options,
		Parameters: url.Values{
			"token":           {o.fs.opt.Token},
			"fileMd5ofPre10m": {fmt.Sprintf("%x", md5.Sum(first10mBytes))},
			"fileSize":        {itoa64(size)},
		},
	}

	getFirstStepResult := getUploadURLResponse{}
	err = getUnmarshaledResponse(ctx, o.fs, opts, &getFirstStepResult)
	if err != nil {
		if getFirstStepResult.Status != 600 {
			return fmt.Errorf("Update err in unmarshaling response: %w", err)
		}
	}

	switch getFirstStepResult.Status {
	case 1:
		// upload file using link from first step
		var res *http.Response

		file := io.MultiReader(bytes.NewReader(first10mBytes), in)

		opts := &rest.Opts{
			Method:        "PUT",
			RootURL:       getFirstStepResult.Data.SignURL,
			Options:       options,
			Body:          file,
			ContentLength: &size,
		}

		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			res, err = o.fs.srv.Call(ctx, opts)
			return o.fs.shouldRetry(ctx, res, err)
		})

		if err != nil {
			return fmt.Errorf("update err in uploading file: %w", err)
		}

		_, err = io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("update err in reading response: %w", err)
		}

	case 600:
		// Status means that we don't need to upload file
		// We need only to make second step
	default:
		return fmt.Errorf("got unexpected message from Linkbox: %s", getFirstStepResult.Message)
	}

	leaf, dirID, err := o.fs.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		return err
	}

	// create file item at Linkbox (second step)
	opts = &rest.Opts{
		Method:  "GET",
		RootURL: linkboxAPIURL,
		Path:    "folder_upload_file",
		Options: options,
		Parameters: url.Values{
			"token":           {o.fs.opt.Token},
			"fileMd5ofPre10m": {fmt.Sprintf("%x", md5.Sum(first10mBytes))},
			"fileSize":        {itoa64(size)},
			"pid":             {dirID},
			"diyName":         {leaf},
		},
	}

	getSecondStepResult := getUploadURLResponse{}
	err = getUnmarshaledResponse(ctx, o.fs, opts, &getSecondStepResult)
	if err != nil {
		return fmt.Errorf("Update second step failed: %w", err)
	}

	// Try a few times to read the object after upload for eventual consistency
	const maxTries = 10
	var sleepTime = 100 * time.Millisecond
	var entity *entity
	for try := 1; try <= maxTries; try++ {
		entity, err = getEntity(ctx, o.fs, leaf, dirID, o.fs.opt.Token)
		if err == nil {
			break
		}
		if err != fs.ErrorObjectNotFound {
			return fmt.Errorf("Update failed to read object: %w", err)
		}
		fs.Debugf(o, "Trying to read object after upload: try again in %v (%d/%d)", sleepTime, try, maxTries)
		time.Sleep(sleepTime)
		sleepTime *= 2
	}
	if err != nil {
		return err
	}
	o.set(entity)
	return nil
}

// Remove this object
func (o *Object) Remove(ctx context.Context) error {
	opts := &rest.Opts{
		Method:  "GET",
		RootURL: linkboxAPIURL,
		Path:    "file_del",
		Parameters: url.Values{
			"token":   {o.fs.opt.Token},
			"itemIds": {o.itemID},
		},
	}
	requestResult := getUploadURLResponse{}
	err := getUnmarshaledResponse(ctx, o.fs, opts, &requestResult)
	if err != nil {
		return fmt.Errorf("could not Remove: %w", err)

	}
	return nil
}

// ModTime returns the modification time of the remote http file
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Remote the name of the remote HTTP file, relative to the fs root
func (o *Object) Remote() string {
	return o.remote
}

// Size returns the size in bytes of the remote http file
func (o *Object) Size() int64 {
	return o.size
}

// String returns the URL to the remote HTTP file
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Fs is the filesystem this remote http file object is located within
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns "" since HTTP (in Go or OpenSSH) doesn't support remote calculation of hashes
func (o *Object) Hash(ctx context.Context, r hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable returns whether the remote http file is a regular file
// (not a directory, symbolic link, block device, character device, named pipe, etc.)
func (o *Object) Storable() bool {
	return true
}

// Features returns the optional features of this Fs
// Info provides a read only interface to information about a filesystem.
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Name of the remote (as passed into NewFs)
// Name returns the configured name of the file system
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Linkbox root '%s'", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
// Returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

/*
	{
	  "data": {
	    "signUrl": "http://xx -- Then CURL PUT your file with sign url "
	  },
	  "msg": "please use this url to upload (PUT method)",
	  "status": 1
	}
*/

// All messages have these items
type response struct {
	Message string `json:"msg"`
	Status  int    `json:"status"`
}

// IsError returns whether response represents an error
func (r *response) IsError() bool {
	return r.Status != 1
}

// Error returns the error state of this response
func (r *response) Error() string {
	return fmt.Sprintf("Linkbox error %d: %s", r.Status, r.Message)
}

// responser is interface covering the response so we can use it when it is embedded.
type responser interface {
	IsError() bool
	Error() string
}

type getUploadURLData struct {
	SignURL string `json:"signUrl"`
}

type getUploadURLResponse struct {
	response
	Data getUploadURLData `json:"data"`
}

// Put in to the remote path with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Put should either
// return an error or upload it properly (rather than e.g. calling panic).
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: src.Remote(),
		size:   src.Size(),
	}
	dir, _ := splitDirAndName(src.Remote())
	err := f.Mkdir(ctx, dir)
	if err != nil {
		return nil, err
	}
	err = o.Update(ctx, in, src, options...)
	return o, err
}

// Purge all files in the directory specified
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
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

// shouldRetry determines whether a given err rates being retried
func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.Purger          = &Fs{}
	_ fs.DirCacheFlusher = &Fs{}
	_ fs.Object          = &Object{}
)
