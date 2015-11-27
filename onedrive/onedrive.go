// Package onedrive provides an interface to the Microsoft One Drive
// object storage system.
package onedrive

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/dircache"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
	"github.com/ncw/rclone/onedrive/api"
	"github.com/ncw/rclone/pacer"
	"github.com/ncw/rclone/rest"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID     = "0000000044165769"
	rcloneClientSecret = "0+be4+jYw+7018HY6P3t/Izo+pTc+Yvt8+fy8NHU094="
	minSleep           = 10 * time.Millisecond
	maxSleep           = 2 * time.Second
	decayConstant      = 2                               // bigger for slower decay, exponential
	rootURL            = "https://api.onedrive.com/v1.0" // root URL for requests
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: []string{
			"wl.signin",          // Allow single sign-on capabilities
			"wl.offline_access",  // Allow receiving a refresh token
			"onedrive.readwrite", // r/w perms to all of a user's OneDrive files
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.live.com/oauth20_authorize.srf",
			TokenURL: "https://login.live.com/oauth20_token.srf",
		},
		ClientID:     rcloneClientID,
		ClientSecret: fs.Reveal(rcloneClientSecret),
		RedirectURL:  oauthutil.RedirectPublicURL,
	}
	chunkSize    = fs.SizeSuffix(10 * 1024 * 1024)
	uploadCutoff = fs.SizeSuffix(10 * 1024 * 1024)
)

// Register with Fs
func init() {
	fs.Register(&fs.Info{
		Name:  "onedrive",
		NewFs: NewFs,
		Config: func(name string) {
			err := oauthutil.Config(name, oauthConfig)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: oauthutil.ConfigClientID,
			Help: "Microsoft App Client Id - leave blank normally.",
		}, {
			Name: oauthutil.ConfigClientSecret,
			Help: "Microsoft App Client Secret - leave blank normally.",
		}},
	})
	pflag.VarP(&chunkSize, "onedrive-chunk-size", "", "Above this size files will be chunked - must be multiple of 320k.")
	pflag.VarP(&uploadCutoff, "onedrive-upload-cutoff", "", "Cutoff for switching to chunked upload - must be <= 100MB")
}

// Fs represents a remote one drive
type Fs struct {
	name     string             // name of this remote
	srv      *rest.Client       // the connection to the one drive server
	root     string             // the path we are working on
	dirCache *dircache.DirCache // Map of directory path to directory id
	pacer    *pacer.Pacer       // pacer for API calls
}

// Object describes a one drive object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
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
	return fmt.Sprintf("One drive root '%s'", f.root)
}

// Pattern to match a one drive path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parsePath parses an one drive 'url'
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
func shouldRetry(resp *http.Response, err error) (bool, error) {
	return fs.ShouldRetry(err) || fs.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(path string) (info *api.Item, resp *http.Response, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/drive/root:/" + replaceReservedChars(path),
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, nil, &info)
		return shouldRetry(resp, err)
	})
	return info, resp, err
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		return err
	}
	if errResponse.ErrorInfo.Code == "" {
		errResponse.ErrorInfo.Code = resp.Status
	}
	return errResponse
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	root = parsePath(root)
	oAuthClient, err := oauthutil.NewClient(name, oauthConfig)
	if err != nil {
		log.Fatalf("Failed to configure One Drive: %v", err)
	}

	f := &Fs{
		name:  name,
		root:  root,
		srv:   rest.NewClient(oAuthClient, rootURL),
		pacer: pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
	}
	f.srv.SetErrorHandler(errorHandler)

	// Get rootID
	rootInfo, _, err := f.readMetaDataForPath("")
	if err != nil || rootInfo.ID == "" {
		return nil, fmt.Errorf("Failed to get root: %v", err)
	}

	f.dirCache = dircache.New(root, rootInfo.ID, f)

	// Find the current root
	err = f.dirCache.FindRoot(false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		newF := *f
		newF.dirCache = dircache.New(newRoot, rootInfo.ID, &newF)
		newF.root = newRoot
		// Make new Fs which is the parent
		err = newF.dirCache.FindRoot(false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		obj := newF.newObjectWithInfo(remote, nil)
		if obj == nil {
			// File doesn't exist so return old f
			return f, nil
		}
		// return a Fs Limited to this object
		return fs.NewLimited(&newF, obj), nil
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
// May return nil if an error occurred
func (f *Fs) newObjectWithInfo(remote string, info *api.Item) fs.Object {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		// Set info
		o.setMetaData(info)
	} else {
		err := o.readMetaData() // reads info and meta, returning an error
		if err != nil {
			// logged already FsDebug("Failed to read info: %s", err)
			return nil
		}
	}
	return o
}

// NewFsObject returns an Object from a path
//
// May return nil if an error occurred
func (f *Fs) NewFsObject(remote string) fs.Object {
	return f.newObjectWithInfo(remote, nil)
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error) {
	// fs.Debug(f, "FindLeaf(%q, %q)", pathID, leaf)
	parent, ok := f.dirCache.GetInv(pathID)
	if !ok {
		return "", false, fmt.Errorf("Couldn't find parent ID")
	}
	path := leaf
	if parent != "" {
		path = parent + "/" + path
	}
	if f.dirCache.FoundRoot() {
		path = f.rootSlash() + path
	}
	info, resp, err := f.readMetaDataForPath(path)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	if info.Folder == nil {
		return "", false, fmt.Errorf("Found file when looking for folder")
	}
	return info.ID, true, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(pathID, leaf string) (newID string, err error) {
	// fs.Debug(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	var info *api.Item
	opts := rest.Opts{
		Method: "POST",
		Path:   "/drive/items/" + pathID + "/children",
	}
	mkdir := api.CreateItemRequest{
		Name:             replaceReservedChars(leaf),
		ConflictBehavior: "fail",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, &mkdir, &info)
		return shouldRetry(resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}
	//fmt.Printf("...Id %q\n", *info.Id)
	return info.ID, nil
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
func (f *Fs) listAll(dirID string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	// Top parameter asks for bigger pages of data
	// https://dev.onedrive.com/odata/optional-query-parameters.htm
	opts := rest.Opts{
		Method: "GET",
		Path:   "/drive/items/" + dirID + "/children?top=1000",
	}
OUTER:
	for {
		var result api.ListChildrenResponse
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(&opts, nil, &result)
			return shouldRetry(resp, err)
		})
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't list files: %v", err)
			break
		}
		if len(result.Value) == 0 {
			break
		}
		for i := range result.Value {
			item := &result.Value[i]
			isFolder := item.Folder != nil
			if isFolder {
				if filesOnly {
					continue
				}
			} else {
				if directoriesOnly {
					continue
				}
			}
			if item.Deleted != nil {
				continue
			}
			item.Name = restoreReservedChars(item.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		if result.NextLink == "" {
			break
		}
		opts.Path = result.NextLink
		opts.Absolute = true
	}
	return
}

// Path should be directory path either "" or "path/"
//
// List the directory using a recursive list from the root
//
// This fetches the minimum amount of stuff but does more API calls
// which makes it slow
func (f *Fs) listDirRecursive(dirID string, path string, out fs.ObjectsChan) error {
	var subError error
	// Make the API request
	var wg sync.WaitGroup
	_, err := f.listAll(dirID, false, false, func(info *api.Item) bool {
		// Recurse on directories
		if info.Folder != nil {
			wg.Add(1)
			folder := path + info.Name + "/"
			fs.Debug(f, "Reading %s", folder)
			go func() {
				defer wg.Done()
				err := f.listDirRecursive(info.ID, folder, out)
				if err != nil {
					subError = err
					fs.ErrorLog(f, "Error reading %s:%s", folder, err)
				}
			}()
		} else {
			if fs := f.newObjectWithInfo(path+info.Name, info); fs != nil {
				out <- fs
			}
		}
		return false
	})
	wg.Wait()
	fs.Debug(f, "Finished reading %s", path)
	if err != nil {
		return err
	}
	if subError != nil {
		return subError
	}
	return nil
}

// List walks the path returning a channel of Objects
func (f *Fs) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		err := f.dirCache.FindRoot(false)
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't find root: %s", err)
		} else {
			err = f.listDirRecursive(f.dirCache.RootID(), "", out)
			if err != nil {
				fs.Stats.Error()
				fs.ErrorLog(f, "List failed: %s", err)
			}
		}
	}()
	return out
}

// ListDir lists the directories
func (f *Fs) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		err := f.dirCache.FindRoot(false)
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't find root: %s", err)
		} else {
			_, err := f.listAll(f.dirCache.RootID(), true, false, func(item *api.Item) bool {
				dir := &fs.Dir{
					Name:  item.Name,
					Bytes: -1,
					Count: -1,
					When:  time.Time(item.LastModifiedDateTime),
				}
				if item.Folder != nil {
					dir.Count = item.Folder.ChildCount
				}
				out <- dir
				return false
			})
			if err != nil {
				fs.Stats.Error()
				fs.ErrorLog(f, "ListDir failed: %s", err)
			}
		}
	}()
	return out
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error
//
// Used to create new objects
func (f *Fs) createObject(remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindPath(remote, true)
	if err != nil {
		return nil, leaf, directoryID, err
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, leaf, directoryID, nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, remote string, modTime time.Time, size int64) (fs.Object, error) {
	o, _, _, err := f.createObject(remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(in, modTime, size)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir() error {
	return f.dirCache.FindRoot(true)
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(id string) error {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/drive/items/" + id,
		NoResponse: true,
	}
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(check bool) error {
	if f.root == "" {
		return fmt.Errorf("Can't purge root directory")
	}
	dc := f.dirCache
	err := dc.FindRoot(false)
	if err != nil {
		return err
	}
	rootID := dc.RootID()
	item, _, err := f.readMetaDataForPath(f.root)
	if err != nil {
		return err
	}
	if item.Folder == nil {
		return fmt.Errorf("Not a folder")
	}
	if check && item.Folder.ChildCount != 0 {
		return fmt.Errorf("Folder not empty")
	}
	err = f.deleteObject(rootID)
	if err != nil {
		return err
	}
	f.dirCache.ResetRoot()
	if err != nil {
		return err
	}
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir() error {
	return f.purgeCheck(true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// waitForJob waits for the job with status in url to complete
func (f *Fs) waitForJob(location string, o *Object) error {
	deadline := time.Now().Add(fs.Config.Timeout)
	for time.Now().Before(deadline) {
		opts := rest.Opts{
			Method:   "GET",
			Path:     location,
			Absolute: true,
		}
		var resp *http.Response
		var err error
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.Call(&opts)
			return shouldRetry(resp, err)
		})
		if err != nil {
			return err
		}
		if resp.StatusCode == 202 {
			var status api.AsyncOperationStatus
			err = rest.DecodeJSON(resp, &status)
			if err != nil {
				return err
			}
			if status.Status == "failed" || status.Status == "deleteFailed" {
				return fmt.Errorf("Async operation %q returned %q", status.Operation, status.Status)
			}
		} else {
			var info api.Item
			err = rest.DecodeJSON(resp, &info)
			if err != nil {
				return err
			}
			o.setMetaData(&info)
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("Async operation didn't complete after %v", fs.Config.Timeout)
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
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debug(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := srcObj.readMetaData()
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/drive/items/" + srcObj.id + "/action.copy",
		ExtraHeaders: map[string]string{"Prefer": "respond-async"},
		NoResponse:   true,
	}
	replacedLeaf := replaceReservedChars(leaf)
	copy := api.CopyItemRequest{
		Name: &replacedLeaf,
		ParentReference: api.ItemReference{
			ID: directoryID,
		},
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, &copy, nil)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}

	// read location header
	location := resp.Header.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("Didn't receive location header in copy response")
	}

	// Wait for job to finish
	err = f.waitForJob(location, dstObj)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	return f.purgeCheck(false)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Fs {
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

// srvPath returns a path for use in server
func (o *Object) srvPath() string {
	return replaceReservedChars(o.fs.rootSlash() + o.remote)
}

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Md5sum() (string, error) {
	return "", nil // not supported by one drive
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %s", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) {
	o.hasMetaData = true
	o.size = info.Size
	if info.FileSystemInfo != nil {
		o.modTime = time.Time(info.FileSystemInfo.LastModifiedDateTime)
	} else {
		o.modTime = time.Time(info.LastModifiedDateTime)
	}
	o.id = info.ID
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	if o.hasMetaData {
		return nil
	}
	// leaf, directoryID, err := o.fs.dirCache.FindPath(o.remote, false)
	// if err != nil {
	// 	return err
	// }
	info, _, err := o.fs.readMetaDataForPath(o.srvPath())
	if err != nil {
		fs.Debug(o, "Failed to read info: %s", err)
		return err
	}
	o.setMetaData(info)
	return nil
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %s", err)
		return time.Now()
	}
	return o.modTime
}

// setModTime sets the modification time of the local fs object
func (o *Object) setModTime(modTime time.Time) (*api.Item, error) {
	opts := rest.Opts{
		Method: "PATCH",
		Path:   "/drive/root:/" + o.srvPath(),
	}
	update := api.SetFileSystemInfo{
		FileSystemInfo: api.FileSystemInfoFacet{
			CreatedDateTime:      api.Timestamp(modTime),
			LastModifiedDateTime: api.Timestamp(modTime),
		},
	}
	var info *api.Item
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(&opts, &update, &info)
		return shouldRetry(resp, err)
	})
	return info, err
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) {
	info, err := o.setModTime(modTime)
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to update remote mtime: %v", err)
	}
	o.setMetaData(info)
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open() (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, fmt.Errorf("Can't download no id")
	}
	var resp *http.Response
	opts := rest.Opts{
		Method: "GET",
		Path:   "/drive/items/" + o.id + "/content",
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// createUploadSession creates an upload session for the object
func (o *Object) createUploadSession() (response *api.CreateUploadResponse, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "/drive/root:/" + o.srvPath() + ":/upload.createSession",
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(&opts, nil, &response)
		return shouldRetry(resp, err)
	})
	return
}

// uploadFragment uploads a part
func (o *Object) uploadFragment(url string, start int64, totalSize int64, buf []byte) (err error) {
	bufSize := int64(len(buf))
	opts := rest.Opts{
		Method:        "PUT",
		Path:          url,
		Absolute:      true,
		ContentLength: &bufSize,
		ContentRange:  fmt.Sprintf("bytes %d-%d/%d", start, start+bufSize-1, totalSize),
		Body:          bytes.NewReader(buf),
	}
	var response api.UploadFragmentResponse
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(&opts, nil, &response)
		return shouldRetry(resp, err)
	})
	return err
}

// cancelUploadSession cancels an upload session
func (o *Object) cancelUploadSession(url string) (err error) {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       url,
		Absolute:   true,
		NoResponse: true,
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	return
}

// uploadMultipart uploads a file using multipart upload
func (o *Object) uploadMultipart(in io.Reader, size int64) (err error) {
	if chunkSize%(320*1024) != 0 {
		return fmt.Errorf("Chunk size %d is not a multiple of 320k", chunkSize)
	}

	// Create upload session
	fs.Debug(o, "Starting multipart upload")
	session, err := o.createUploadSession()
	if err != nil {
		return err
	}
	uploadURL := session.UploadURL

	// Cancel the session if something went wrong
	defer func() {
		if err != nil {
			fs.Debug(o, "Cancelling multipart upload")
			cancelErr := o.cancelUploadSession(uploadURL)
			if cancelErr != nil {
				fs.Log(o, "Failed to cancel multipart upload: %v", err)
			}
		}
	}()

	// Upload the chunks
	remaining := size
	position := int64(0)
	buf := make([]byte, int64(chunkSize))
	for remaining > 0 {
		n := int64(chunkSize)
		if remaining < n {
			n = remaining
			buf = buf[:n]
		}
		_, err = io.ReadFull(in, buf)
		if err != nil {
			return err
		}
		fs.Debug(o, "Uploading segment %d/%d size %d", position, size, n)
		err = o.uploadFragment(uploadURL, position, size, buf)
		if err != nil {
			return err
		}
		remaining -= n
		position += n
	}

	return nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, modTime time.Time, size int64) (err error) {
	var info *api.Item
	if size <= int64(uploadCutoff) {
		// This is for less than 100 MB of content
		var resp *http.Response
		opts := rest.Opts{
			Method: "PUT",
			Path:   "/drive/root:/" + o.srvPath() + ":/content",
			Body:   in,
		}
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			resp, err = o.fs.srv.CallJSON(&opts, nil, &info)
			return shouldRetry(resp, err)
		})
		if err != nil {
			return err
		}
		o.setMetaData(info)
	} else {
		err = o.uploadMultipart(in, size)
		if err != nil {
			return err
		}
	}
	// Set the mod time now and read metadata
	info, err = o.setModTime(modTime)
	if err != nil {
		return err
	}
	o.setMetaData(info)
	return nil
}

// Remove an object
func (o *Object) Remove() error {
	return o.fs.deleteObject(o.id)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Purger = (*Fs)(nil)
	_ fs.Copier = (*Fs)(nil)
	// _ fs.Mover    = (*Fs)(nil)
	// _ fs.DirMover = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
)
