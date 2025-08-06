package filejump

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/filejump/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	apiBaseURL          = "https://drive.filejump.com/api/v1"
	defaultUploadCutoff = 50 * 1024 * 1024
	smallFileCutoff     = 15 * 1024 * 1024 // 15 MiB
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "filejump",
		Description: "FileJump",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "access_token",
			Help:      "You should create an API access token here: https://drive.filejump.com/account-settings",
			Required:  true,
			Sensitive: true,
		}, {
			Name:     "workspace_id",
			Help:     "The ID of the workspace to use. Defaults to personal workspace if not set or 0.",
			Default:  "0",
			Advanced: true,
		}, {
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to multipart upload (>= 50 MiB).",
			Default:  fs.SizeSuffix(defaultUploadCutoff),
			Advanced: true,
		}, {
			Name:     "hard_delete",
			Help:     "Delete files permanently instead of moving them to trash.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeLeftSpace |
				encoder.EncodeRightSpace),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	UploadCutoff fs.SizeSuffix        `config:"upload_cutoff"`
	Enc          encoder.MultiEncoder `config:"encoding"`
	AccessToken  string               `config:"access_token"`
	WorkspaceID  string               `config:"workspace_id"`
	HardDelete   bool                 `config:"hard_delete"`
}

// Fs represents a remote filejump server
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	srv      *rest.Client
	pacer    *fs.Pacer
	dirCache *dircache.DirCache
}

// Object describes a filejump object
type Object struct {
	fs          *Fs
	remote      string
	hasMetaData bool
	size        int64
	modTime     time.Time
	id          string
	mimeType    string
}

// callJSON is a generic function for API calls
/*
POST-Request Template:
--------------------
type Request struct {
    Field1 string `json:"field1"`
    Field2 int    `json:"field2"`
}
request := Request{
    Field1: "wert1",
    Field2: 42,
}

type Response struct {
    Status string `json:"status"`
    Data   string `json:"data,omitempty"`
}

result, err := CallJSON[Response, Request](f, ctx, "POST", "/api/endpoint", nil, &request)
if err != nil {
    return err
}

GET-Request Template:
-------------------
type Response struct {
    Status string `json:"status"`
    Data   string `json:"data,omitempty"`
}
values := url.Values{}
values.Set("param1", "wert1")
values.Set("param2", "wert2")
values.Set("EntryIds", fmt.Sprintf("[%s]", dir))
values.Set("DeleteForever", strconv.FormatBool(true)))

result, err := CallJSON[Response, struct{}](f, ctx, "GET", "/file-entries/delete", &values, nil)
if err != nil {
    return err
}
*/
func CallJSON[T any, B any](f *Fs, ctx context.Context, method string, path string, params *url.Values, body *B) (*T, error) {
	// Input parameters in one line
	logMsg := fmt.Sprintf("CallJSON: %s %s", method, path)
	if params != nil && len(*params) > 0 {
		logMsg += fmt.Sprintf(" params=%s", params.Encode())
	}
	if body != nil {
		bodyJSON, _ := json.Marshal(body)
		logMsg += fmt.Sprintf(" body=%s", string(bodyJSON))
	}
	fs.Debugf(f, "%v", logMsg)

	var result T
	opts := rest.Opts{
		Method: method,
		Path:   path,
		ExtraHeaders: map[string]string{
			"Accept":          "application/json",
			"Accept-Encoding": "gzip",
		},
	}

	if params != nil {
		opts.Parameters = *params
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, body, &result)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return nil, err
	}

	return &result, nil
}

// CallJSONGet executes a GET request
/*
Example call:
--------------
type Response struct {
    Status string `json:"status"`
    Data   string `json:"data,omitempty"`
}

values := url.Values{}
values.Set("param1", "wert1")
values.Set("param2", "wert2")

result, err := CallJSONGet[Response](f, ctx, "/api/endpoint", &values)
if err != nil {
    return err
}
*/
func CallJSONGet[T any](f *Fs, ctx context.Context, path string, params *url.Values) (*T, error) {
	return CallJSON[T, struct{}](f, ctx, "GET", path, params, nil)
}

// CallJSONPost executes a POST request
/*
Example call:
--------------
type RequestDelete struct {
	EntryIds      []int `json:"entryIds"`
	DeleteForever bool  `json:"deleteForever"`
}

type ResultDelete struct {
	Status string `json:"status,omitempty"`
}

iDir, _ := strconv.Atoi(dir)
request := RequestDelete{
	EntryIds:      []int{iDir},
	DeleteForever: false,
}

result, err := CallJSONPost[ResultDelete, RequestDelete](f, ctx, "/file-entries/delete", &request)
if err != nil {
    return err
}
*/
func CallJSONPost[T any, B any](f *Fs, ctx context.Context, path string, body *B) (*T, error) {
	return CallJSON[T, B](f, ctx, "POST", path, &url.Values{}, body)
}

// padNameForFileJump pads names shorter than 3 characters with encoded spaces
// This is required by FileJump's API which enforces a minimum 3-character name length
func (f *Fs) padNameForFileJump(name string) string {
	if name == "" {
		return name
	}

	encodedName := f.opt.Enc.FromStandardName(name)
	for len([]rune(encodedName)) < 3 {
		encodedName += "␠"
	}
	return encodedName
}

// unpadNameForFileJump unpads padded names, which were shorter than 3 characters
// This is required by FileJump's API which enforces a minimum 3-character name length
func (f *Fs) unpadNameForFileJump(name string) string {
	if len([]rune(name)) == 3 {
		// Remove trailing encoded spaces that were added for padding
		name = strings.TrimRight(name, "␠")
	}
	return name
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")

	client := fshttp.NewClient(ctx)

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		srv:  rest.NewClient(client).SetRoot(apiBaseURL),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(
			pacer.MinSleep(10*time.Millisecond),
			pacer.MaxSleep(2*time.Second),
			pacer.DecayConstant(2),
		)),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CleanUp:                 f.CleanUp,
		Move:                    f.Move,
		DirMove:                 f.DirMove,
		PublicLink:              f.PublicLink,
		About:                   f.About,
	}).Fill(ctx, f)
	f.srv.SetHeader("Authorization", "Bearer "+opt.AccessToken)

	// Initialize dirCache without an ID
	// var rootID string
	rootID := opt.WorkspaceID
	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it's a file
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
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// listWorkspaces retrieves the list of workspaces for the current user and returns a formatted error
func (f *Fs) listWorkspaces(ctx context.Context) error {
	type Workspace struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	type WorkspacesResponse struct {
		Workspaces []Workspace `json:"workspaces"`
	}

	response, err := CallJSONGet[WorkspacesResponse](f, ctx, "/me/workspaces", nil)
	if err != nil {
		return fmt.Errorf("could not retrieve workspaces. Please check your access token. Original error: %w", err)
	}

	var workspaceInfo strings.Builder
	workspaceInfo.WriteString("\nAvailable workspaces:\n")
	workspaceInfo.WriteString("-------------------\n")
	// Add personal workspace manually as it's not in the API response
	workspaceInfo.WriteString(fmt.Sprintf("Personal Space | ID: 0\n"))

	for _, ws := range response.Workspaces {
		workspaceInfo.WriteString(fmt.Sprintf("%-14s | ID: %d\n", ws.Name, ws.ID))
	}
	workspaceInfo.WriteString("-------------------\n")
	workspaceInfo.WriteString("Please add the correct 'workspace_id' to your rclone config.")

	return errors.New(workspaceInfo.String())
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
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, activeOnly bool, fn listAllFn) (found bool, err error) {
	values := url.Values{}
	values.Set("folderId", dirID)
	values.Set("perPage", "1000")
	var page *uint
OUTER:
	for {
		if page != nil {
			values.Set("page", strconv.FormatUint(uint64(*page), 10))
		}

		result, err := CallJSONGet[api.FileEntries](f, ctx, "/drive/file-entries", &values)
		if err != nil {
			return found, fmt.Errorf("couldn't list files: %w", err)
		}
		for i := range result.Data {
			item := &result.Data[i]
			if item.Type == api.ItemTypeFolder {
				if filesOnly {
					continue
				}
			} else if item.Type != api.ItemTypeFolder {
				if directoriesOnly {
					continue
				}
			} else {
				fs.Debugf(f, "Ignoring %q - unknown type %q", item.Name, item.Type)
				continue
			}
			// Handle padding removal for names that were padded to meet 3-character minimum
			itemName := f.unpadNameForFileJump(item.Name)
			item.Name = f.opt.Enc.ToStandardName(itemName)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		page = result.NextPage
		if page == nil {
			break
		}
	}
	return
}

// type Fs interface:

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
	_, err = f.listAll(ctx, directoryID, false, false, true, func(info *api.Item) bool {
		remote := path.Join(dir, info.Name)
		if info.Type == api.ItemTypeFolder {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.GetID())
			d := fs.NewDir(remote, info.ModTime()).SetID(info.GetID())
			// FIXME more info from dir?
			entries = append(entries, d)
		} else if info.Type != api.ItemTypeFolder {
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		}

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
	authRetry := false

	if resp != nil && resp.StatusCode == 401 && strings.Contains(resp.Header.Get("Www-Authenticate"), "expired_token") {
		authRetry = true
		fs.Debugf(nil, "Should retry: %v", err)
	}

	return authRetry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
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

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists.
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	// For unknown size uploads, we need to read the data first to determine the size
	if size < 0 {
		// Read all data into memory to determine size
		data, err := io.ReadAll(in)
		if err != nil {
			return nil, fmt.Errorf("failed to read data for unknown size upload: %w", err)
		}
		size = int64(len(data))
		// Create a new reader from the data
		in = strings.NewReader(string(data))
	}

	o, _, _, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
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

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	// For unknown size uploads, we need to read the data first to determine the size
	if size < 0 {
		// Read all data into memory to determine size
		data, err := io.ReadAll(in)
		if err != nil {
			return nil, fmt.Errorf("failed to read data for unknown size upload: %w", err)
		}
		size = int64(len(data))
		// Create a new reader from the data
		in = strings.NewReader(string(data))
	}

	o, _, _, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
}

// // Mkdir makes the directory (container, bucket)
// //
// // Shouldn't return an error if it already exists
// func (f *Fs) Mkdir(ctx context.Context, dir string) error {
// 	presignResult, err := CallJSON[PresignResult](f, ctx, "POST", "/folders", url.Values{
// 		"name": []string{dir}
// :
// "test123"
// parentId
// :
// 		"Filename":     []string{leaf},
// 		"Mime":         []string{"application/octet-stream"},
// 		"Disk":         []string{"uploads"},
// 		"Size":         []string{strconv.FormatInt(size, 10)},
// 		"Extension":    []string{"bin"},
// 		"WorkspaceID":  []string{"0"},
// 		"ParentID":     []string{directoryID},
// 		"RelativePath": []string{""},
// 	})

// 	if err != nil {
// 		// Fehlerbehandlung
// 	}
// }

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

	// If check is true, verify the directory is empty before deleting
	if check {
		isEmpty := true
		_, err := f.listAll(ctx, rootID, false, false, true, func(item *api.Item) bool {
			isEmpty = false
			return true // Stop on first item found
		})
		if err != nil {
			return fmt.Errorf("failed to check if directory is empty: %w", err)
		}
		if !isEmpty {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	type RequestDelete struct {
		EntryIds      []int `json:"entryIds"`
		DeleteForever bool  `json:"deleteForever"`
	}

	type ResultDelete struct {
		Status string `json:"status,omitempty"`
	}

	iDir, _ := strconv.Atoi(rootID)
	request := RequestDelete{
		EntryIds:      []int{iDir},
		DeleteForever: f.opt.HardDelete,
	}

	result, err := CallJSONPost[ResultDelete, RequestDelete](f, ctx, "/file-entries/delete", &request)

	if err != nil {
		return fmt.Errorf("rmdir failed: %w", err)
	}
	if result.Status != "success" {
		return errors.New("delete, no api success")
	}
	f.dirCache.FlushDir(dir)
	if err != nil {
		return errors.New("rmdir failed, no success response")
	}
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// CleanUp empties the trash
func (f *Fs) CleanUp(ctx context.Context) error {
	type RequestEmptyTrash struct {
		EntryIds   []int `json:"entryIds"`
		EmptyTrash bool  `json:"emptyTrash"`
	}

	type ResultEmptyTrash struct {
		Status string `json:"status,omitempty"`
	}

	request := RequestEmptyTrash{
		EntryIds:   []int{}, // Empty array as specified in the API
		EmptyTrash: true,
	}

	result, err := CallJSONPost[ResultEmptyTrash, RequestEmptyTrash](f, ctx, "/file-entries/delete", &request)
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	if result.Status != "success" {
		return fmt.Errorf("cleanup failed with status: %s", result.Status)
	}

	return nil
}

// type Info interface:
// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return "filejump root '" + f.root + "'"
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	// FileJump doesn't support setting custom modification times
	// Return ModTimeNotSupported to disable mod time checks
	return fs.ModTimeNotSupported
}

// Returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, true, false, true, func(item *api.Item) bool {
		if item.Name == leaf {
			pathIDOut = item.GetID()
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// Encode and pad the leaf name for FileJump API requirements
	encodedLeaf := f.padNameForFileJump(leaf)

	type RequestCreateDir struct {
		Name     string `json:"name"`
		ParentID *int64 `json:"parentId,omitempty"` // Use omitempty to send null when nil
	}

	var parentID *int64
	// The API expects `null` for the root directory, not 0.
	// dircache uses "0" as the ID for the root.
	if pathID != "" && pathID != "0" {
		iPathId, err := strconv.ParseInt(pathID, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid parent ID %q: %w", pathID, err)
		}
		parentID = &iPathId
	}

	requestCreateDir := RequestCreateDir{
		Name:     encodedLeaf,
		ParentID: parentID,
	}

	type ResultCreateDir struct {
		Folder struct {
			ID int `json:"id,omitempty"`
		} `json:"folder,omitempty"`
		Status string `json:"status,omitempty"`
	}

	result, err := CallJSONPost[ResultCreateDir, RequestCreateDir](f, ctx, "/folders", &requestCreateDir)

	if err != nil {
		// Check if this is the specific FileJump 3-character limitation error
		if strings.Contains(err.Error(), "The name must be at least 3 characters") {
			return "", fmt.Errorf("directory name '%s' is too short: FileJump requires at least 3 characters", leaf)
		}
		return "", err
	}
	if result.Status != "success" {
		return "", fmt.Errorf("failed to create directory, status: %s", result.Status)
	}

	return strconv.Itoa(result.Folder.ID), nil
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

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Type == api.ItemTypeFolder {
		return fs.ErrorIsDir
	}
	if info.Type == api.ItemTypeFolder {
		return fmt.Errorf("%q is %q: %w", o.remote, info.Type, fs.ErrorNotAFile)
	}
	o.hasMetaData = true
	o.size = int64(info.FileSize)
	// o.sha1 = info.SHA1
	o.modTime = info.ModTime()
	o.id = info.GetID()
	return nil
}

// Copy src to this remote using server-side copy operations.
//
// # This is stored with the remote path given
//
// # It returns the destination Object and a possible error
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

	// Get the source object ID
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	if srcObj.id == "" {
		return nil, fs.ErrorCantCopy
	}

	// Find destination directory
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	// Check if destination file already exists and remove it first
	// This ensures we don't create duplicates
	var idsToDelete []int
	_, listErr := f.listAll(ctx, directoryID, false, true, true, func(item *api.Item) bool {
		if item.Name == leaf {
			id, convErr := strconv.Atoi(item.GetID())
			if convErr == nil {
				idsToDelete = append(idsToDelete, id)
			}
		}
		return false // find all duplicates
	})
	if listErr != nil {
		return nil, fmt.Errorf("copy: failed to check for existing files: %w", listErr)
	}

	if len(idsToDelete) > 0 {
		// Delete existing files with the same name
		type RequestDelete struct {
			EntryIds      []int `json:"entryIds"`
			DeleteForever bool  `json:"deleteForever"`
		}
		request := RequestDelete{
			EntryIds:      idsToDelete,
			DeleteForever: f.opt.HardDelete,
		}
		type ResultDelete struct {
			Status string `json:"status,omitempty"`
		}
		result, deleteErr := CallJSONPost[ResultDelete, RequestDelete](f, ctx, "/file-entries/delete", &request)
		if deleteErr != nil {
			return nil, fmt.Errorf("copy: failed to delete existing files: %w", deleteErr)
		}
		if result.Status != "success" {
			return nil, fmt.Errorf("copy: deleting existing files failed with status: %s", result.Status)
		}
	}

	// Convert IDs to integers
	srcID, err := strconv.Atoi(srcObj.id)
	if err != nil {
		return nil, fmt.Errorf("invalid source ID: %w", err)
	}

	var destID *int
	if directoryID != "" && directoryID != "0" {
		destIDInt, err := strconv.Atoi(directoryID)
		if err != nil {
			return nil, fmt.Errorf("invalid destination ID: %w", err)
		}
		destID = &destIDInt
	}

	// API request structure for copy/duplicate
	type RequestCopy struct {
		EntryIds      []int `json:"entryIds"`
		DestinationId *int  `json:"destinationId"`
	}

	type ResultCopy struct {
		Status  string     `json:"status"`
		Entries []api.Item `json:"entries"`
	}

	request := RequestCopy{
		EntryIds:      []int{srcID},
		DestinationId: destID,
	}

	result, err := CallJSONPost[ResultCopy, RequestCopy](f, ctx, "/file-entries/duplicate", &request)
	if err != nil {
		return nil, fmt.Errorf("copy failed: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("copy failed with status: %s", result.Status)
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("no entries returned from copy operation")
	}

	// Get the copied entry
	copiedEntry := &result.Entries[0]

	// If the name doesn't match what we want, rename it
	if copiedEntry.Name != leaf {
		// Update the entry name via API
		type RequestUpdate struct {
			Name string `json:"name"`
		}

		updateRequest := RequestUpdate{
			Name: f.padNameForFileJump(leaf),
		}

		type ResultUpdate struct {
			Status    string   `json:"status"`
			FileEntry api.Item `json:"fileEntry"`
		}

		updateResult, err := CallJSON[ResultUpdate, RequestUpdate](f, ctx, "PUT", "/file-entries/"+copiedEntry.GetID(), nil, &updateRequest)
		if err != nil {
			// If rename fails, we still have a successful copy, just with wrong name
			fs.Debugf(f, "Failed to rename copied file to %q: %v", leaf, err)
		} else if updateResult.Status == "success" {
			copiedEntry = &updateResult.FileEntry
		}
	}

	// Create new object from the copied entry
	return f.newObjectWithInfo(ctx, remote, copiedEntry)
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// # It returns the destination Object and a possible error
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
	srcFs := src.Fs()

	// Defer flushing the cache for the source and destination directories
	defer f.dirCache.FlushDir(path.Dir(src.Remote()))
	defer f.dirCache.FlushDir(path.Dir(remote))

	// Read metadata to get the ID and current name
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}
	if srcObj.id == "" {
		return nil, fs.ErrorCantMove
	}

	// Find destination directory
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	// 1. Move the file to the new directory if needed
	srcAbsPath := path.Join(srcFs.Root(), srcObj.Remote())
	dstAbsPath := path.Join(f.Root(), remote)
	srcAbsDir := path.Dir(srcAbsPath)
	dstAbsDir := path.Dir(dstAbsPath)

	if srcAbsDir != dstAbsDir {
		// Determine destination directory
		destID := directoryID
		if destID == "" {
			destID = "0"
		}
		entryIDInt, err := strconv.Atoi(srcObj.id)
		if err != nil {
			return nil, errors.New("invalid source file ID")
		}
		destIDInt, err := strconv.Atoi(destID)
		if err != nil {
			return nil, errors.New("invalid destination directory ID")
		}
		type RequestMove struct {
			DestinationId int   `json:"destinationId"`
			EntryIds      []int `json:"entryIds"`
		}
		type ResultMove struct {
			Status string `json:"status"`
		}
		request := RequestMove{
			DestinationId: destIDInt,
			EntryIds:      []int{entryIDInt},
		}
		result, err := CallJSONPost[ResultMove, RequestMove](f, ctx, "/file-entries/move", &request)
		if err != nil {
			return nil, err
		}
		if result.Status != "success" {
			return nil, errors.New("move failed: " + result.Status)
		}
	}

	// 2. Rename if necessary
	oldLeaf := path.Base(srcObj.remote)
	if leaf != oldLeaf {
		type RequestRename struct {
			Name        string `json:"name"`
			InitialName string `json:"initialName"`
		}
		type ResultRename struct {
			FileEntry api.Item `json:"fileEntry"`
			Status    string   `json:"status"`
		}
		request := RequestRename{
			Name:        f.padNameForFileJump(leaf),
			InitialName: f.padNameForFileJump(oldLeaf),
		}
		result, err := CallJSON[ResultRename, RequestRename](f, ctx, "POST", "/file-entries/"+srcObj.id+"?_method=PUT", nil, &request)
		if err != nil {
			return nil, err
		}
		if result.Status != "success" {
			return nil, errors.New("rename failed: " + result.Status)
		}
		return f.newObjectWithInfo(ctx, remote, &result.FileEntry)
	}

	// Reload metadata and return object
	return f.newObjectWithInfo(ctx, remote, nil)
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

	srcID, err := srcFs.dirCache.FindDir(ctx, srcRemote, false)
	if err != nil {
		return err
	}

	// This backend can't move a directory into another directory which has a file with the same name.
	// So we must check for the destination directory existing
	if _, err := f.dirCache.FindDir(ctx, dstRemote, false); err == nil {
		return fs.ErrorDirExists
	} else if !errors.Is(err, fs.ErrorDirNotFound) {
		return err
	}

	// Special case for moving contents of a directory
	if dstRemote == "" {
		dstID, err := f.dirCache.FindDir(ctx, "", true)
		if err != nil {
			return fmt.Errorf("DirMove: failed to find destination root: %w", err)
		}

		var entriesToMove []int
		_, err = srcFs.listAll(ctx, srcID, false, false, true, func(item *api.Item) bool {
			id, err := strconv.Atoi(item.GetID())
			if err == nil {
				entriesToMove = append(entriesToMove, id)
			}
			return false
		})
		if err != nil {
			return fmt.Errorf("DirMove: failed to list source directory %q: %w", srcRemote, err)
		}

		if len(entriesToMove) > 0 {
			destIDInt, err := strconv.Atoi(dstID)
			if err != nil {
				return errors.New("invalid destination directory ID")
			}
			type RequestMove struct {
				DestinationId int   `json:"destinationId"`
				EntryIds      []int `json:"entryIds"`
			}
			type ResultMove struct {
				Status string `json:"status"`
			}
			moveReq := RequestMove{
				DestinationId: destIDInt,
				EntryIds:      entriesToMove,
			}
			_, err = CallJSONPost[ResultMove, RequestMove](f, ctx, "/file-entries/move", &moveReq)
			if err != nil {
				return err
			}
		}

		err = srcFs.Purge(ctx, srcRemote)
		if err != nil {
			return fmt.Errorf("DirMove: failed to purge source directory %q: %w", srcRemote, err)
		}

		srcFs.dirCache.FlushDir(srcRemote)
		f.dirCache.FlushDir("")
		return nil
	}

	// Normal directory move
	leafSrc := path.Base(srcRemote)
	leafDst := path.Base(dstRemote)
	dstParentRemote := path.Dir(dstRemote)
	if dstParentRemote == "." {
		dstParentRemote = ""
	}

	dstParentID, err := f.dirCache.FindDir(ctx, dstParentRemote, true)
	if err != nil {
		return err
	}

	destID := dstParentID
	if destID == "" {
		destID = "0"
	}
	srcIDInt, err := strconv.Atoi(srcID)
	if err != nil {
		return errors.New("invalid source directory ID")
	}
	destIDInt, err := strconv.Atoi(destID)
	if err != nil {
		return errors.New("invalid destination directory ID")
	}
	type RequestMove struct {
		DestinationId int   `json:"destinationId"`
		EntryIds      []int `json:"entryIds"`
	}
	type ResultMove struct {
		Status string `json:"status"`
	}
	moveReq := RequestMove{
		DestinationId: destIDInt,
		EntryIds:      []int{srcIDInt},
	}
	_, err = CallJSONPost[ResultMove, RequestMove](f, ctx, "/file-entries/move", &moveReq)
	if err != nil {
		return err
	}

	if leafDst != leafSrc {
		type RequestRename struct {
			Name        string `json:"name"`
			InitialName string `json:"initialName"`
		}
		type ResultRename struct {
			FolderEntry api.Item `json:"folderEntry"`
			Status      string   `json:"status"`
		}
		renameReq := RequestRename{
			Name:        f.padNameForFileJump(leafDst),
			InitialName: f.padNameForFileJump(leafSrc),
		}
		_, err := CallJSON[ResultRename, RequestRename](
			f, ctx,
			"POST",
			"/file-entries/"+srcID+"?_method=PUT",
			nil,
			&renameReq,
		)
		if err != nil {
			return err
		}
	}

	srcFs.dirCache.FlushDir(srcRemote)
	f.dirCache.FlushDir(dstParentRemote)
	return nil
}

// About gets the space usage of the drive
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	type SpaceUsageResponse struct {
		Used      int64  `json:"used"`
		Available int64  `json:"available"`
		Status    string `json:"status"`
	}

	response, err := CallJSONGet[SpaceUsageResponse](f, ctx, "/user/space-usage", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get space usage: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("failed to get space usage: status %s", response.Status)
	}

	usage := &fs.Usage{
		Used:  fs.NewUsageValue(response.Used),
		Free:  fs.NewUsageValue(response.Available),
		Total: fs.NewUsageValue(response.Used + response.Available),
	}
	return usage, nil
}

// PublicLink generates a public link to the remote path (usually a file).
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	info, err := f.readMetaDataForPath(ctx, remote)
	if err != nil {
		return "", err
	}

	type ShareableLinkResponse struct {
		Link struct {
			Hash string `json:"hash"`
		} `json:"link"`
		Status string `json:"status"`
	}

	var result ShareableLinkResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   "/file-entries/" + info.GetID() + "/shareable-link",
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", fmt.Errorf("failed to create shareable link: %w", err)
	}

	if result.Status != "success" {
		return "", fmt.Errorf("failed to create shareable link: status %s", result.Status)
	}

	if result.Link.Hash == "" {
		return "", errors.New("failed to create shareable link: empty hash in response")
	}

	return "https://drive.filejump.com/drive/s/" + result.Link.Hash, nil
}

// type Object interface:

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	// FileJump doesn't support setting custom modification times via API
	// The modification time is set by the server when the file is uploaded
	// Return this error to indicate that setting mod time requires re-upload
	return fs.ErrorCantSetModTime
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("Download not possible - no ID available")
	}

	// Ensure we have metadata to check file size
	err = o.readMetaData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Special handling for zero-length files
	if o.size == 0 {
		// Return an empty reader for zero-length files
		return io.NopCloser(strings.NewReader("")), nil
	}

	fs.FixRangeOption(options, o.size)

	// Try to get file metadata to see if it contains a download URL
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err == nil && info != nil {
		// Check if the URL field contains a download URL
		if urlStr, ok := info.URL.(string); ok && urlStr != "" {
			// Construct full URL - the URL field contains a relative path
			fullURL := "https://drive.filejump.com/" + strings.TrimPrefix(urlStr, "/")

			req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create download request: %w", err)
			}

			// Add authorization header
			req.Header.Set("Authorization", "Bearer "+o.fs.opt.AccessToken)

			// Apply range options if any
			for _, option := range options {
				if rangeOption, ok := option.(*fs.RangeOption); ok {
					req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeOption.Start, rangeOption.End))
				}
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("failed to download file: %w", err)
			}

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
				resp.Body.Close()
				return nil, fmt.Errorf("download failed with status: %s", resp.Status)
			}

			return resp.Body, nil
		}
	}

	// Fallback to direct download endpoint
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		Path:    "/file-entries/" + o.id + "/download",
		Options: options,
		ExtraHeaders: map[string]string{
			"Authorization": "Bearer " + o.fs.opt.AccessToken,
		},
	}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)

		if err != nil {
			return shouldRetry(ctx, resp, err)
		}

		// Check for redirects
		if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
			redirectURL := resp.Header.Get("Location")
			if redirectURL == "" {
				return false, errors.New("Redirect URL not found")
			}

			// Follow the redirect
			redirectReq, redirectErr := http.NewRequestWithContext(ctx, "GET", redirectURL, nil)
			if redirectErr != nil {
				return false, redirectErr
			}

			// Apply range options to redirect request
			for _, option := range options {
				if rangeOption, ok := option.(*fs.RangeOption); ok {
					redirectReq.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeOption.Start, rangeOption.End))
				}
			}

			redirectResp, redirectErr := http.DefaultClient.Do(redirectReq)
			if redirectErr != nil {
				return shouldRetry(ctx, redirectResp, redirectErr)
			}

			// Replace the original response with the redirect response
			resp.Body.Close() // Close the original response body
			resp = redirectResp
		}

		return false, nil
	})

	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	remote := o.Remote()
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, false)
	if err != nil && err != fs.ErrorDirNotFound {
		return fmt.Errorf("Update: failed to find path: %w", err)
	}

	// If directory exists, check for and remove duplicates
	if err != fs.ErrorDirNotFound {
		var idsToDelete []int
		_, listErr := o.fs.listAll(ctx, directoryID, false, true, true, func(item *api.Item) bool {
			if item.Name == leaf {
				id, convErr := strconv.Atoi(item.GetID())
				if convErr == nil {
					idsToDelete = append(idsToDelete, id)
				}
			}
			return false // find all duplicates
		})
		if listErr != nil {
			return fmt.Errorf("Update: failed to list duplicates: %w", listErr)
		}

		if len(idsToDelete) > 0 {
			type RequestDelete struct {
				EntryIds      []int `json:"entryIds"`
				DeleteForever bool  `json:"deleteForever"`
			}
			request := RequestDelete{
				EntryIds:      idsToDelete,
				DeleteForever: o.fs.opt.HardDelete,
			}
			type ResultDelete struct {
				Status string `json:"status,omitempty"`
			}
			result, deleteErr := CallJSONPost[ResultDelete, RequestDelete](o.fs, ctx, "/file-entries/delete", &request)
			if deleteErr != nil {
				return fmt.Errorf("Update: failed to delete duplicates: %w", deleteErr)
			}
			if result.Status != "success" {
				return fmt.Errorf("Update: deleting duplicates failed with status: %s", result.Status)
			}
		}
	}

	// Now, proceed with creating the directory if it doesn't exist and uploading
	leaf, directoryID, err = o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	size := src.Size()
	modTime := src.ModTime(ctx)

	// For unknown size uploads, we need to read the data first to determine the size
	if size < 0 {
		// Read all data into memory to determine size
		data, err := io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("failed to read data for unknown size upload: %w", err)
		}
		size = int64(len(data))
		// Create a new reader from the data
		in = strings.NewReader(string(data))
	}

	err = o.upload(ctx, in, leaf, directoryID, size, modTime, options...)
	return err
}

func getExtensionAndMime(filename string) (extension, mimeType string) {
	ext := path.Ext(filename)
	if ext == "" {
		return "bin", "application/octet-stream"
	}

	// File extension without dot
	extension = strings.TrimPrefix(ext, ".")

	// Determine MIME type
	mimeType = mime.TypeByExtension(ext)
	if mimeType == "" {
		// If no MIME type was found, return default
		mimeType = "application/octet-stream"
	}

	return extension, mimeType
}

func (o *Object) upload(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time, options ...fs.OpenOption) (err error) {
	// There are two ways to upload a file in the Filejump backend:
	// 1. Using the REST API – this is a single request.
	// 2. Using a presigned URL – first fetch the URL, upload the file directly to Wasabi,
	//    then make a follow-up request to report metadata (e.g. MIME type) to Filejump.

	// Large files are currently uploaded via presigned URLs, since direct upload to Wasabi
	// is likely faster. The REST API is just a wrapper that also uploads to Wasabi,
	// but it doesn't currently support setting a MIME type explicitly.

	// As a result, files uploaded via the REST API default to a binary MIME type.
	// This causes issues when previewing e.g. text files in the web interface,
	// since they can only be downloaded, not displayed.

	// To avoid this, all files are currently uploaded via presigned URL,
	// even though it requires 3 separate requests per file.

	// In the future, we might consider uploading small *binary* files via the REST API
	// to reduce request overhead — since MIME type is irrelevant for those anyway.

	// However, after testing the upload speed, it turned out that using the presigned URL
	// was actually faster.
	// Therefore, the working code that uploads via the REST API wrapper is currently commented out.

	// // Use REST API for small files
	// if size <= smallFileCutoff {
	// 	fs.Debugf(o, "Uploading small file via REST API")
	// 	body := &bytes.Buffer{}
	// 	writer := multipart.NewWriter(body)

	// 	// Encode and pad the filename for FileJump API requirements
	// 	encodedLeaf := o.fs.padNameForFileJump(leaf)
	// 	part, err := writer.CreateFormFile("file", encodedLeaf)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to create form file: %w", err)
	// 	}
	// 	_, err = io.Copy(part, in)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to copy file to multipart buffer: %w", err)
	// 	}

	// 	if directoryID != "" && directoryID != "0" {
	// 		_ = writer.WriteField("parentId", directoryID)
	// 	}

	// 	// The API uses parentId to place the file, so relativePath can be empty.
	// 	// This now matches the large file upload logic.
	// 	_ = writer.WriteField("relativePath", "")

	// 	err = writer.Close()
	// 	if err != nil {
	// 		return fmt.Errorf("failed to close multipart writer: %w", err)
	// 	}

	// 	var result api.FileUploadResponse
	// 	var resp *http.Response
	// 	err = o.fs.pacer.Call(func() (bool, error) {
	// 		opts := rest.Opts{
	// 			Method: "POST",
	// 			Path:   "/uploads",
	// 			Body:   body,
	// 			ExtraHeaders: map[string]string{
	// 				"Content-Type": writer.FormDataContentType(),
	// 				"Accept":       "application/json",
	// 			},
	// 		}
	// 		resp, err = o.fs.srv.Call(ctx, &opts)
	// 		if err != nil {
	// 			return shouldRetry(ctx, resp, err)
	// 		}

	// 		// Decode the response
	// 		defer fs.CheckClose(resp.Body, &err)
	// 		err = json.NewDecoder(resp.Body).Decode(&result)
	// 		if err != nil {
	// 			return false, fmt.Errorf("failed to decode upload response: %w", err)
	// 		}

	// 		return shouldRetry(ctx, resp, err)
	// 	})

	// 	if err != nil {
	// 		return fmt.Errorf("small file upload failed: %w", err)
	// 	}
	// 	if result.Status != "success" {
	// 		return fmt.Errorf("small file upload failed with status: %s", result.Status)
	// 	}

	// 	err = o.setMetaData(&result.FileEntry)
	// 	if err != nil {
	// 		return fmt.Errorf("error setting metadata after small file upload: %w", err)
	// 	}
	// 	o.size = size
	// 	o.modTime = modTime
	// 	return nil
	// }

	// Use S3 presigned URL for large files
	fs.Debugf(o, "Uploading large file via S3 presigned URL")
	directoryIDInt, _ := strconv.Atoi(directoryID)
	workspaceIDInt, err := strconv.Atoi(o.fs.opt.WorkspaceID)
	if err != nil {
		fs.Debugf(o, "Could not parse workspace_id %q, defaulting to 0: %v", o.fs.opt.WorkspaceID, err)
		workspaceIDInt = 0
	}

	// Requesting the pre-subscribed URL
	type ResultPresign struct {
		URL    string `json:"url"`
		Key    string `json:"key"`
		ACL    string `json:"acl"`
		Status string `json:"status"`
	}

	type RequestPresign struct {
		Filename     string `json:"filename"`
		Mime         string `json:"mime"`
		Disk         string `json:"disk"`
		Size         int64  `json:"size"`
		Extension    string `json:"extension"`
		WorkspaceID  int    `json:"workspaceId"`
		ParentID     int    `json:"parentId"`
		RelativePath string `json:"relativePath"`
	}

	ext, mime := getExtensionAndMime(leaf)

	// Encode and pad the filename for FileJump API requirements
	encodedLeaf := o.fs.padNameForFileJump(leaf)

	requestPresign := RequestPresign{
		Filename:     encodedLeaf,
		Mime:         mime,
		Disk:         "uploads",
		Size:         size,
		Extension:    ext,
		WorkspaceID:  workspaceIDInt,
		ParentID:     directoryIDInt,
		RelativePath: "",
	}

	resultPresign, err := CallJSONPost[ResultPresign, RequestPresign](o.fs, ctx, "/s3/simple/presign", &requestPresign)

	if err != nil {
		return fmt.Errorf("error requesting presigned URL: %w", err)
	}

	if resultPresign.Status != "success" {
		return fmt.Errorf("error requesting presigned URL: status is not 'success'")
	}

	// PUT-Request
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, resultPresign.URL, in)
	if err != nil {
		return fmt.Errorf("error creating PUT request: %w", err)
	}

	// Set the necessary headers
	putReq.Header.Set("Content-Type", "application/octet-stream")
	putReq.Header.Set("x-amz-acl", resultPresign.ACL)

	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		return fmt.Errorf("error uploading file: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("error uploading file: HTTP %d: %s", putResp.StatusCode, string(body))
	}

	type RequestEntries struct {
		WorkspaceID     int         `json:"workspaceId"`
		ParentID        interface{} `json:"parentId"`
		RelativePath    string      `json:"relativePath"`
		Disk            string      `json:"disk"`
		ClientMime      string      `json:"clientMime"`
		ClientName      string      `json:"clientName"`
		Filename        string      `json:"filename"`
		Size            int64       `json:"size"`
		ClientExtension string      `json:"clientExtension"`
	}
	requestEntries := RequestEntries{
		WorkspaceID: workspaceIDInt,
		ParentID: func() interface{} {
			if directoryIDInt == 0 {
				return ""
			}
			return directoryIDInt
		}(),
		RelativePath:    "",
		Disk:            "uploads",
		ClientMime:      "application/octet-stream",
		ClientName:      encodedLeaf,
		Filename:        path.Base(resultPresign.Key),
		Size:            size,
		ClientExtension: "bin",
	}

	resultEntries, err := CallJSONPost[api.Item, RequestEntries](o.fs, ctx, "/s3/entries", &requestEntries)

	if err != nil {
		return fmt.Errorf("error requesting file data URL: %w", err)
	}

	// If the upload response doesn't contain valid metadata, try to read it from the API
	if resultEntries.ID == 0 || resultEntries.Name == "" {
		// Set basic metadata first
		o.size = size
		o.modTime = modTime
		o.hasMetaData = false // Force readMetaData to fetch from API
		// Try to read metadata from the API
		err = o.readMetaData(ctx)
		if err != nil {
			// Set minimal metadata to allow the object to function
			o.hasMetaData = true
			o.id = "" // Will be empty, but object still works for most operations
		}
	} else {
		// Set object metadata
		err = o.setMetaData(resultEntries)
		if err != nil {
			return fmt.Errorf("error setting metadata: %w", err)
		}
		o.size = size
		o.modTime = modTime
	}

	return nil
}

// Removes this object
func (o *Object) Remove(ctx context.Context) error {
	type RequestDelete struct {
		EntryIds      []int `json:"entryIds"`
		DeleteForever bool  `json:"deleteForever"`
	}

	type ResultDelete struct {
		Status string `json:"status,omitempty"`
	}

	entryId, _ := strconv.Atoi(o.id)
	request := RequestDelete{
		EntryIds:      []int{entryId},
		DeleteForever: o.fs.opt.HardDelete,
	}

	result, err := CallJSONPost[ResultDelete, RequestDelete](o.fs, ctx, "/file-entries/delete", &request)

	if err != nil {
		return err
	}
	if result.Status != "success" {
		return errors.New(result.Status)
	}
	return nil
}

// type ObjectInfo interface:

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// type DirEntry interface:
// String returns a description of the Object
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

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Item, err error) {
	// defer fs.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, directoryID, false, true, false, func(item *api.Item) bool {
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

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		if errors.Is(err, fs.ErrorObjectNotFound) {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		return time.Now()
	}
	return o.modTime
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(context.TODO())
	if err != nil {
		return -1
	}
	return o.size
}

// Check the interfaces are satisfied
var (
	_ fs.Fs         = (*Fs)(nil)
	_ fs.Purger     = (*Fs)(nil)
	_ fs.Copier     = (*Fs)(nil)
	_ fs.Mover      = (*Fs)(nil)
	_ fs.DirMover   = (*Fs)(nil)
	_ fs.CleanUpper = (*Fs)(nil)
	_ fs.Object     = (*Object)(nil)
)
