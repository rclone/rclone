// Package movistarcloud implements the Movistar Cloud backend.
package movistarcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/movistarcloud/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep      = 100 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
	rootURL       = "https://micloud.movistar.es"
	uploadURL     = "https://upload.micloud.movistar.es"
	userAgent     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:147.0) Gecko/20100101 Firefox/147.0"
	defaultLimit  = 200
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "movistarcloud",
		Description: "Movistar Cloud",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "jsessionid",
			Help:      "JSESSIONID cookie for authentication.\n\nYou can obtain this by logging in to micloud.movistar.es and extracting the JSESSIONID cookie from your browser.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:     "list_limit",
			Help:     "Maximum number of items to list per request.",
			Default:  defaultLimit,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	JSessionID string               `config:"jsessionid"`
	ListLimit  int                  `config:"list_limit"`
	Enc        encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote Movistar Cloud filesystem
type Fs struct {
	name     string             // name of this remote
	root     string             // the path we are working on
	opt      Options            // parsed options
	features *fs.Features       // optional features
	srv      *rest.Client       // the connection to the main server
	upSrv    *rest.Client       // the connection to the upload server
	dirCache *dircache.DirCache // Map of directory path to directory id
	pacer    *fs.Pacer          // pacer for API calls
}

// Object describes a Movistar Cloud file object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object (string representation of int64)
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
	return fmt.Sprintf("Movistar Cloud root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// parsePath parses a path string
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried. It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// getResponse is a helper to decode the Movistar Cloud GET response wrapper
// The API returns { "responsetime": N, "data": { ... } }
type getResponse struct {
	ResponseTime int64           `json:"responsetime"`
	Data         json.RawMessage `json:"data"`
}

// postResponse is a helper to decode the Movistar Cloud POST response wrapper
// The API returns { "responsetime": N, "success": "...", ... }
type postResponse struct {
	ResponseTime int64  `json:"responsetime"`
	Success      string `json:"success"`
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		return fmt.Errorf("error reading error response: %w", err)
	}
	errResponse := &api.Error{
		Status: resp.StatusCode,
		Code:   resp.Status,
	}
	// Try to decode JSON error
	var jsonErr struct {
		Error *api.Error `json:"error"`
	}
	if json.Unmarshal(body, &jsonErr) == nil && jsonErr.Error != nil {
		errResponse = jsonErr.Error
		if errResponse.Status == 0 {
			errResponse.Status = resp.StatusCode
		}
	} else {
		errResponse.Message = string(body)
	}
	return errResponse
}

// apiGet makes a GET request to the Movistar Cloud API and decodes
// the "data" field of the response into result.
func (f *Fs) apiGet(ctx context.Context, apiPath string, result interface{}) error {
	opts := rest.Opts{
		Method: "GET",
		Path:   apiPath,
	}
	var respWrapper getResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &respWrapper)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(respWrapper.Data, result)
	}
	return nil
}

// apiPost makes a POST request to the Movistar Cloud API with a JSON body
// and decodes the response.
func (f *Fs) apiPost(ctx context.Context, apiPath string, request, result interface{}) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   apiPath,
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, request, result)
		return shouldRetry(ctx, resp, err)
	})
	return err
}

// apiPostGet makes a POST request to the Movistar Cloud API with a JSON body
// and decodes the "data" field of the response into result.
func (f *Fs) apiPostGet(ctx context.Context, apiPath string, request, result interface{}) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   apiPath,
	}
	var respWrapper getResponse
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, request, &respWrapper)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(respWrapper.Data, result)
	}
	return nil
}

// getRootFolderID gets the root folder ID from the API
func (f *Fs) getRootFolderID(ctx context.Context) (string, error) {
	var result api.RootResponse
	err := f.apiGet(ctx, "/sapi/media/folder/root?action=get", &result)
	if err != nil {
		return "", fmt.Errorf("failed to get root folder: %w", err)
	}
	if len(result.Folders) == 0 {
		return "", errors.New("no root folder found")
	}
	return strconv.FormatInt(result.Folders[0].ID, 10), nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	client := fshttp.NewClient(ctx)

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   rest.NewClient(client).SetRoot(rootURL),
		upSrv: rest.NewClient(client).SetRoot(uploadURL),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         false,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            false,
		WriteMimeType:           false,
	}).Fill(ctx, f)

	// Set error handler for the main API client
	f.srv.SetErrorHandler(errorHandler)
	f.upSrv.SetErrorHandler(errorHandler)

	// Set common headers
	f.srv.SetHeader("User-Agent", userAgent)
	f.srv.SetHeader("Accept", "application/json")
	f.srv.SetHeader("Cookie", "JSESSIONID="+opt.JSessionID)

	f.upSrv.SetHeader("User-Agent", userAgent)
	f.upSrv.SetHeader("Accept", "application/json")
	f.upSrv.SetHeader("Cookie", "JSESSIONID="+opt.JSessionID)

	// Get the root folder ID from the API
	rootFolderID, err := f.getRootFolderID(ctx)
	if err != nil {
		return nil, fmt.Errorf("movistarcloud: couldn't get root folder ID: %w", err)
	}

	// Set up directory cache
	f.dirCache = dircache.New(root, rootFolderID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootFolderID, &tempF)
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
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// rootSlash returns root with a slash on if it is not empty, otherwise empty string
func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	parentID, err := strconv.ParseInt(pathID, 10, 64)
	if err != nil {
		return "", false, fmt.Errorf("invalid parent ID %q: %w", pathID, err)
	}

	var result api.ListFoldersResponse
	err = f.apiGet(ctx, fmt.Sprintf("/sapi/media/folder?action=list&parentid=%d&limit=%d", parentID, f.opt.ListLimit), &result)
	if err != nil {
		return "", false, err
	}

	for _, folder := range result.Folders {
		if strings.EqualFold(f.opt.Enc.ToStandardName(folder.Name), leaf) {
			return strconv.FormatInt(folder.ID, 10), true, nil
		}
	}
	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	parentID, err := strconv.ParseInt(pathID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid parent ID %q: %w", pathID, err)
	}

	request := api.CreateFolderRequest{
		Data: api.CreateFolderRequestData{
			Magic:    false,
			Offline:  false,
			Name:     f.opt.Enc.FromStandardName(leaf),
			ParentID: parentID,
		},
	}

	var result api.CreateFolderResponse
	err = f.apiPost(ctx, "/sapi/media/folder?action=save", &request, &result)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(result.ID, 10), nil
}

// listFiles lists the files in a directory
func (f *Fs) listFiles(ctx context.Context, dirID string) ([]api.Media, error) {
	folderID, err := strconv.ParseInt(dirID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid directory ID %q: %w", dirID, err)
	}

	request := api.ListFilesRequest{
		Data: api.ListFilesRequestData{
			Fields: []string{"name", "size", "modificationdate", "creationdate", "etag", "url"},
		},
	}

	var allMedia []api.Media
	// The API uses pagination with "more" field
	limit := f.opt.ListLimit
	apiPath := fmt.Sprintf("/sapi/media?action=get&folderid=%d&limit=%d", folderID, limit)

	var result api.ListFilesResponse
	err = f.apiPostGet(ctx, apiPath, &request, &result)
	if err != nil {
		return nil, err
	}
	allMedia = append(allMedia, result.Media...)

	return allMedia, nil
}

// listFolders lists the subfolders in a directory
func (f *Fs) listFolders(ctx context.Context, dirID string) ([]api.FolderWithParent, error) {
	parentID, err := strconv.ParseInt(dirID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid directory ID %q: %w", dirID, err)
	}

	var result api.ListFoldersResponse
	err = f.apiGet(ctx, fmt.Sprintf("/sapi/media/folder?action=list&parentid=%d&limit=%d", parentID, f.opt.ListLimit), &result)
	if err != nil {
		return nil, err
	}
	return result.Folders, nil
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Media, err error) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	// List files in the directory and find the one with the matching name
	files, err := f.listFiles(ctx, directoryID)
	if err != nil {
		return nil, err
	}

	for i := range files {
		if f.opt.Enc.ToStandardName(files[i].Name) == leaf {
			return &files[i], nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Media) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx)
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote. If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// List the objects and directories in dir into entries. The
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
func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) error {
	lh := list.NewHelper(callback)
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// List subfolders
	folders, err := f.listFolders(ctx, directoryID)
	if err != nil {
		return fmt.Errorf("couldn't list folders: %w", err)
	}
	for _, folder := range folders {
		folderName := f.opt.Enc.ToStandardName(folder.Name)
		remote := path.Join(dir, folderName)
		folderID := strconv.FormatInt(folder.ID, 10)
		f.dirCache.Put(remote, folderID)
		modTime := time.Unix(folder.Date/1000, (folder.Date%1000)*int64(time.Millisecond))
		d := fs.NewDir(remote, modTime).SetID(folderID)
		if err := lh.Add(d); err != nil {
			return err
		}
	}

	// List files
	files, err := f.listFiles(ctx, directoryID)
	if err != nil {
		return fmt.Errorf("couldn't list files: %w", err)
	}
	for i := range files {
		file := &files[i]
		fileName := f.opt.Enc.ToStandardName(file.Name)
		remote := path.Join(dir, fileName)
		o, err := f.newObjectWithInfo(ctx, remote, file)
		if err != nil {
			return err
		}
		if err := lh.Add(o); err != nil {
			return err
		}
	}

	return lh.Flush()
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
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, leaf, directoryID, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.upload(ctx, in, leaf, directoryID, size, modTime, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	folderID, err := strconv.ParseInt(dirID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid directory ID %q: %w", dirID, err)
	}

	// Check if the directory is empty
	folders, err := f.listFolders(ctx, dirID)
	if err != nil {
		return err
	}
	if len(folders) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}
	files, err := f.listFiles(ctx, dirID)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	// Delete the folder
	request := api.DeleteFoldersRequest{
		Data: api.DeleteFoldersRequestData{
			IDs: []int64{folderID},
		},
	}
	var result postResponse
	err = f.apiPost(ctx, "/sapi/media/folder?action=softdelete", &request, &result)
	if err != nil {
		return fmt.Errorf("rmdir failed: %w", err)
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
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

// Hash returns the hash of an object - not supported
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Media) error {
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = info.ModTimeAsTime()
	o.id = info.ID
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
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
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
//
// Movistar Cloud doesn't have a dedicated API for updating modification time,
// so this is a no-op that returns an unsupported error.
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

	// First, get the download URL for this file
	fileID, err := strconv.ParseInt(o.id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid file ID %q: %w", o.id, err)
	}

	request := api.GetFileInfoRequest{
		Data: api.GetFileInfoRequestData{
			IDs:    []int64{fileID},
			Fields: []string{"url"},
		},
	}
	var result api.GetFileInfoResponse
	err = o.fs.apiPostGet(ctx, "/sapi/media?action=get&origin=omh,dropbox", &request, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get download URL: %w", err)
	}
	if len(result.Media) == 0 || result.Media[0].URL == "" {
		return nil, errors.New("no download URL available")
	}

	downloadURL := result.Media[0].URL

	// Now download from the URL
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: downloadURL,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// upload uploads the object
func (o *Object) upload(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time, options ...fs.OpenOption) error {
	folderID, err := strconv.ParseInt(directoryID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid directory ID %q: %w", directoryID, err)
	}

	metadata := api.UploadMetadata{
		Data: api.UploadMetadataData{
			Name:             o.fs.opt.Enc.FromStandardName(leaf),
			Size:             size,
			ModificationDate: api.BasicISO(modTime),
			ContentType:      "application/octet-stream",
			FolderID:         folderID,
		},
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal upload metadata: %w", err)
	}

	// Read all data first since we need it for the multipart upload
	data, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("failed to read upload data: %w", err)
	}

	// Build the multipart body manually
	boundary := "----RcloneFormBoundary" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var body bytes.Buffer

	// Metadata part
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"data\"\r\n\r\n")
	body.Write(metadataJSON)
	body.WriteString("\r\n")

	// File part
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\n", o.fs.opt.Enc.FromStandardName(leaf)))
	body.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	body.Write(data)
	body.WriteString("\r\n")
	body.WriteString("--" + boundary + "--\r\n")

	opts := rest.Opts{
		Method:      "POST",
		Path:        "/sapi/upload?action=save&acceptasynchronous=true",
		Body:        &body,
		ContentType: "multipart/form-data; boundary=" + boundary,
	}

	var result api.UploadResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.upSrv.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

	// Update the object metadata
	o.id = result.ID
	o.size = int64(len(data))
	o.modTime = modTime
	o.hasMetaData = true

	return nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	modTime := src.ModTime(ctx)
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// If file already exists, delete it first
	if o.id != "" {
		err = o.Remove(ctx)
		if err != nil {
			return fmt.Errorf("failed to remove existing file before update: %w", err)
		}
	}

	return o.upload(ctx, in, leaf, directoryID, size, modTime, options...)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	fileID, err := strconv.ParseInt(o.id, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid file ID %q: %w", o.id, err)
	}

	request := api.DeleteFilesRequest{
		Data: api.DeleteFilesRequestData{
			Files: []int64{fileID},
		},
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "/sapi/media/file?action=delete&softdelete=true",
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &request, nil)
		return shouldRetry(ctx, resp, err)
	})
	return err
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ListPer         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
