// Drive interface
package drive

// Gets this quite often
// Failed to set mtime: googleapi: Error 403: Rate Limit Exceeded

// FIXME list containers equivalent should list directories?

// FIXME list directory should list to channel for concurrency not
// append to array

// FIXME need to deal with some corner cases
// * multiple files with the same name
// * files can be in multiple directories
// * can have directory loops
// * files with / in name

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"code.google.com/p/google-api-go-client/drive/v2"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/googleauth"
	"github.com/ogier/pflag"
)

// Constants
const (
	rcloneClientId     = "202264815644.apps.googleusercontent.com"
	rcloneClientSecret = "X4Z3ca8xfWDb1Voo-F9a7ZxJ"
	driveFolderType    = "application/vnd.google-apps.folder"
	RFC3339In          = time.RFC3339
	RFC3339Out         = "2006-01-02T15:04:05.000000000Z07:00"
)

// Globals
var (
	// Flags
	driveFullList = pflag.BoolP("drive-full-list", "", true, "Use a full listing for directory list. More data but usually quicker.")
	// Description of how to auth for this app
	driveAuth = &googleauth.Auth{
		Scope:               "https://www.googleapis.com/auth/drive",
		DefaultClientId:     rcloneClientId,
		DefaultClientSecret: rcloneClientSecret,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.FsInfo{
		Name:  "drive",
		NewFs: NewFs,
		Config: func(name string) {
			driveAuth.Config(name)
		},
		Options: []fs.Option{{
			Name: "client_id",
			Help: "Google Application Client Id - leave blank to use rclone's.",
		}, {
			Name: "client_secret",
			Help: "Google Application Client Secret - leave blank to use rclone's.",
		}},
	})
}

// FsDrive represents a remote drive server
type FsDrive struct {
	svc          *drive.Service // the connection to the drive server
	root         string         // the path we are working on
	client       *http.Client   // authorized client
	about        *drive.About   // information about the drive, including the root
	rootId       string         // Id of the root directory
	foundRoot    bool           // Whether we have found the root or not
	findRootLock sync.Mutex     // Protect findRoot from concurrent use
	dirCache     dirCache       // Map of directory path to directory id
	findDirLock  sync.Mutex     // Protect findDir from concurrent use
}

// FsObjectDrive describes a drive object
type FsObjectDrive struct {
	drive        *FsDrive // what this object is part of
	remote       string   // The remote path
	id           string   // Drive Id of this object
	url          string   // Download URL of this object
	md5sum       string   // md5sum of the object
	bytes        int64    // size of the object
	modifiedDate string   // RFC3339 time it was last modified
}

// dirCache caches paths to directory Ids and vice versa
type dirCache struct {
	sync.RWMutex
	cache    map[string]string
	invCache map[string]string
}

// Make a new locked map
func newDirCache() dirCache {
	d := dirCache{}
	d.Flush()
	return d
}

// Gets an Id given a path
func (m *dirCache) Get(path string) (id string, ok bool) {
	m.RLock()
	id, ok = m.cache[path]
	m.RUnlock()
	return
}

// GetInv gets a path given an Id
func (m *dirCache) GetInv(path string) (id string, ok bool) {
	m.RLock()
	id, ok = m.invCache[path]
	m.RUnlock()
	return
}

// Put a path, id into the map
func (m *dirCache) Put(path, id string) {
	m.Lock()
	m.cache[path] = id
	m.invCache[id] = path
	m.Unlock()
}

// Flush the map of all data
func (m *dirCache) Flush() {
	m.Lock()
	m.cache = make(map[string]string)
	m.invCache = make(map[string]string)
	m.Unlock()
}

// ------------------------------------------------------------

// String converts this FsDrive to a string
func (f *FsDrive) String() string {
	return fmt.Sprintf("Google drive root '%s'", f.root)
}

// parseParse parses a drive 'url'
func parseDrivePath(path string) (root string, err error) {
	root = strings.Trim(path, "/")
	return
}

// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*drive.File) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
//
// Search params: https://developers.google.com/drive/search-parameters
func (f *FsDrive) listAll(dirId string, title string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	query := fmt.Sprintf("trashed=false")
	if dirId != "" {
		query += fmt.Sprintf(" and '%s' in parents", dirId)
	}
	if title != "" {
		// Escaping the backslash isn't documented but seems to work
		title = strings.Replace(title, `\`, `\\`, -1)
		title = strings.Replace(title, `'`, `\'`, -1)
		query += fmt.Sprintf(" and title='%s'", title)
	}
	if directoriesOnly {
		query += fmt.Sprintf(" and mimeType='%s'", driveFolderType)
	}
	if filesOnly {
		query += fmt.Sprintf(" and mimeType!='%s'", driveFolderType)
	}
	// fmt.Printf("listAll Query = %q\n", query)
	list := f.svc.Files.List().Q(query).MaxResults(1000)
OUTER:
	for {
		files, err := list.Do()
		if err != nil {
			return false, fmt.Errorf("Couldn't list directory: %s", err)
		}
		for _, item := range files.Items {
			if fn(item) {
				found = true
				break OUTER
			}
		}
		if files.NextPageToken == "" {
			break
		}
		list.PageToken(files.NextPageToken)
	}
	return
}

// NewFs contstructs an FsDrive from the path, container:path
func NewFs(name, path string) (fs.Fs, error) {
	t, err := driveAuth.NewTransport(name)
	if err != nil {
		return nil, err
	}

	root, err := parseDrivePath(path)
	if err != nil {
		return nil, err
	}

	f := &FsDrive{
		root:     root,
		dirCache: newDirCache(),
	}

	// Create a new authorized Drive client.
	f.client = t.Client()
	f.svc, err = drive.New(f.client)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create Drive client: %s", err)
	}

	// Read About so we know the root path
	f.about, err = f.svc.About.Get().Do()
	if err != nil {
		return nil, fmt.Errorf("Couldn't read info about Drive: %s", err)
	}

	// Find the Id of the root directory and the Id of its parent
	f.rootId = f.about.RootFolderId
	// Put the root directory in
	f.dirCache.Put("", f.rootId)
	// Find the current root
	err = f.findRoot(false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := splitPath(root)
		newF := *f
		newF.root = newRoot
		// Make new Fs which is the parent
		err = newF.findRoot(false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		obj, err := newF.newFsObjectWithInfo(remote, nil)
		if err != nil {
			// File doesn't exist so return old f
			return f, nil
		}
		// return a Fs Limited to this object
		return fs.NewLimited(&newF, obj), nil
	}
	// fmt.Printf("Root id %s", f.rootId)
	return f, nil
}

// Return an FsObject from a path
func (f *FsDrive) newFsObjectWithInfo(remote string, info *drive.File) (fs.Object, error) {
	fs := &FsObjectDrive{
		drive:  f,
		remote: remote,
	}
	if info != nil {
		fs.setMetaData(info)
	} else {
		err := fs.readMetaData() // reads info and meta, returning an error
		if err != nil {
			// logged already fs.Debug("Failed to read info: %s", err)
			return nil, err
		}
	}
	return fs, nil
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsDrive) NewFsObjectWithInfo(remote string, info *drive.File) fs.Object {
	fs, _ := f.newFsObjectWithInfo(remote, info)
	// Errors have already been logged
	return fs
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsDrive) NewFsObject(remote string) fs.Object {
	return f.NewFsObjectWithInfo(remote, nil)
}

// Path should be directory path either "" or "path/"
//
// List the directory using a recursive list from the root
//
// This fetches the minimum amount of stuff but does more API calls
// which makes it slow
func (f *FsDrive) listDirRecursive(dirId string, path string, out fs.ObjectsChan) error {
	var subError error
	// Make the API request
	_, err := f.listAll(dirId, "", false, false, func(item *drive.File) bool {
		// Recurse on directories
		// FIXME should do this in parallel
		// use a wg to sync then collect error
		if item.MimeType == driveFolderType {
			subError = f.listDirRecursive(item.Id, path+item.Title+"/", out)
			if subError != nil {
				return true
			}
		} else {
			// If item has no MD5 sum it isn't stored on drive, so ignore it
			if item.Md5Checksum != "" {
				if fs := f.NewFsObjectWithInfo(path+item.Title, item); fs != nil {
					out <- fs
				}
			}
		}
		return false
	})
	if err != nil {
		return err
	}
	if subError != nil {
		return subError
	}
	return nil
}

// Path should be directory path either "" or "path/"
//
// List the directory using a full listing and filtering out unwanted
// items
//
// This is fast in terms of number of API calls, but slow in terms of
// fetching more data than it needs
func (f *FsDrive) listDirFull(dirId string, path string, out fs.ObjectsChan) error {
	// Orphans waiting for their parent
	orphans := make(map[string][]*drive.File)

	var outputItem func(*drive.File, string) // forward def for recursive fn

	// Output an item or directory
	outputItem = func(item *drive.File, directory string) {
		// fmt.Printf("found %q %q parent %q dir %q ok %s\n", item.Title, item.Id, parentId, directory, ok)
		path := item.Title
		if directory != "" {
			path = directory + "/" + path
		}
		if item.MimeType == driveFolderType {
			// Put the directory into the dircache
			f.dirCache.Put(path, item.Id)
			// fmt.Printf("directory %s %s %s\n", path, item.Title, item.Id)
			// Collect the orphans if any
			for _, orphan := range orphans[item.Id] {
				// fmt.Printf("rescuing orphan %s %s %s\n", path, orphan.Title, orphan.Id)
				outputItem(orphan, path)
			}
			delete(orphans, item.Id)
		} else {
			// fmt.Printf("file %s %s %s\n", path, item.Title, item.Id)
			// If item has no MD5 sum it isn't stored on drive, so ignore it
			if item.Md5Checksum != "" {
				if fs := f.NewFsObjectWithInfo(path, item); fs != nil {
					out <- fs
				}
			}
		}
	}

	// Make the API request
	_, err := f.listAll("", "", false, false, func(item *drive.File) bool {
		if len(item.Parents) == 0 {
			// fmt.Printf("no parents %s %s: %#v\n", item.Title, item.Id, item)
			return false
		}
		parentId := item.Parents[0].Id
		directory, ok := f.dirCache.GetInv(parentId)
		if !ok {
			// Haven't found the parent yet so add to orphans
			// fmt.Printf("orphan[%s] %s %s\n", parentId, item.Title, item.Id)
			orphans[parentId] = append(orphans[parentId], item)
		} else {
			outputItem(item, directory)
		}
		return false
	})
	if err != nil {
		return err
	}

	if len(orphans) > 0 {
		// fmt.Printf("Orphans!!!! %v", orphans)
	}
	return nil
}

// Splits a path into directory, leaf
//
// Path shouldn't start or end with a /
//
// If there are no slashes then directory will be "" and leaf = path
func splitPath(path string) (directory, leaf string) {
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash >= 0 {
		directory = path[:lastSlash]
		leaf = path[lastSlash+1:]
	} else {
		directory = ""
		leaf = path
	}
	return
}

// Finds the directory passed in returning the directory Id starting from pathId
//
// Path shouldn't start or end with a /
//
// If create is set it will make the directory if not found
//
// Algorithm:
//  Look in the cache for the path, if found return the pathId
//  If not found strip the last path off the path and recurse
//  Now have a parent directory id, so look in the parent for self and return it
func (f *FsDrive) findDir(path string, create bool) (pathId string, err error) {
	pathId = f._findDirInCache(path)
	if pathId != "" {
		return
	}
	f.findDirLock.Lock()
	defer f.findDirLock.Unlock()
	return f._findDir(path, create)
}

// Look for the root and in the cache - safe to call without the findDirLock
func (f *FsDrive) _findDirInCache(path string) string {
	// fmt.Println("Finding",path,"create",create,"cache",cache)
	// If it is the root, then return it
	if path == "" {
		// fmt.Println("Root")
		return f.rootId
	}

	// If it is in the cache then return it
	pathId, ok := f.dirCache.Get(path)
	if ok {
		// fmt.Println("Cache hit on", path)
		return pathId
	}

	return ""
}

// Unlocked findDir - must have findDirLock
func (f *FsDrive) _findDir(path string, create bool) (pathId string, err error) {
	pathId = f._findDirInCache(path)
	if pathId != "" {
		return
	}

	// Split the path into directory, leaf
	directory, leaf := splitPath(path)

	// Recurse and find pathId for directory
	pathId, err = f._findDir(directory, create)
	if err != nil {
		return pathId, err
	}

	// Find the leaf in pathId
	found, err := f.listAll(pathId, leaf, true, false, func(item *drive.File) bool {
		if item.Title == leaf {
			pathId = item.Id
			return true
		}
		return false
	})
	if err != nil {
		return pathId, err
	}

	// If not found create the directory if required or return an error
	if !found {
		if create {
			// fmt.Println("Making", path)
			// Define the metadata for the directory we are going to create.
			info := &drive.File{
				Title:       leaf,
				Description: leaf,
				MimeType:    driveFolderType,
				Parents:     []*drive.ParentReference{{Id: pathId}},
			}
			info, err := f.svc.Files.Insert(info).Do()
			if err != nil {
				return pathId, fmt.Errorf("Failed to make directory")
			}
			pathId = info.Id
		} else {
			return pathId, fmt.Errorf("Couldn't find directory: %q", path)
		}
	}

	// Store the directory in the cache
	f.dirCache.Put(path, pathId)

	// fmt.Println("Dir", path, "is", pathId)
	return pathId, nil
}

// Finds the root directory if not already found
//
// Resets the root directory
//
// If create is set it will make the directory if not found
func (f *FsDrive) findRoot(create bool) error {
	f.findRootLock.Lock()
	defer f.findRootLock.Unlock()
	if f.foundRoot {
		return nil
	}
	rootId, err := f.findDir(f.root, create)
	if err != nil {
		return err
	}
	f.rootId = rootId
	f.dirCache.Flush()
	// Put the root directory in
	f.dirCache.Put("", f.rootId)
	f.foundRoot = true
	return nil
}

// Walk the path returning a channel of FsObjects
func (f *FsDrive) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		err := f.findRoot(false)
		if err != nil {
			fs.Stats.Error()
			log.Printf("Couldn't find root: %s", err)
		} else {
			if *driveFullList {
				err = f.listDirFull(f.rootId, "", out)
			} else {
				err = f.listDirRecursive(f.rootId, "", out)
			}
			if err != nil {
				fs.Stats.Error()
				log.Printf("List failed: %s", err)
			}
		}
	}()
	return out
}

// Walk the path returning a channel of FsObjects
func (f *FsDrive) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		err := f.findRoot(false)
		if err != nil {
			fs.Stats.Error()
			log.Printf("Couldn't find root: %s", err)
		} else {
			_, err := f.listAll(f.rootId, "", true, false, func(item *drive.File) bool {
				dir := &fs.Dir{
					Name:  item.Title,
					Bytes: -1,
					Count: -1,
				}
				dir.When, _ = time.Parse(RFC3339In, item.ModifiedDate)
				out <- dir
				return false
			})
			if err != nil {
				fs.Stats.Error()
				log.Printf("ListDir failed: %s", err)
			}
		}
	}()
	return out
}

// seekWrapper wraps an io.Reader with a basic Seek for
// code.google.com/p/google-api-go-client/googleapi
// to detect the length (see getReaderSize function)
type seekWrapper struct {
	in   io.Reader
	size int64
}

// Read bytes from the object - see io.Reader
func (file *seekWrapper) Read(p []byte) (n int, err error) {
	return file.in.Read(p)
}

// Seek - minimal implementation for Google Drive's length detection
func (file *seekWrapper) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case os.SEEK_CUR:
		return 0, nil
	case os.SEEK_END:
		return file.size, nil
	}
	return 0, nil
}

// Put the object
//
// This assumes that the object doesn't not already exists - if you
// call it when it does exist then it will create a duplicate.  Call
// object.Update() in this case.
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *FsDrive) Put(in io.Reader, remote string, modTime time.Time, size int64) (fs.Object, error) {
	// Temporary FsObject under construction
	o := &FsObjectDrive{drive: f, remote: remote}

	directory, leaf := splitPath(o.remote)
	directoryId, err := f.findDir(directory, true)
	if err != nil {
		return o, fmt.Errorf("Couldn't find or make directory: %s", err)
	}

	// Guess the mime type
	mimeType := mime.TypeByExtension(path.Ext(o.remote))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	modifiedDate := modTime.Format(RFC3339Out)

	// Define the metadata for the file we are going to create.
	info := &drive.File{
		Title:        leaf,
		Description:  leaf,
		Parents:      []*drive.ParentReference{{Id: directoryId}},
		MimeType:     mimeType,
		ModifiedDate: modifiedDate,
	}

	// Make the API request to upload metadata and file data.
	in = &seekWrapper{in: in, size: size}
	info, err = f.svc.Files.Insert(info).Media(in).Do()
	if err != nil {
		return o, fmt.Errorf("Upload failed: %s", err)
	}
	o.setMetaData(info)
	return o, nil
}

// Mkdir creates the container if it doesn't exist
func (f *FsDrive) Mkdir() error {
	return f.findRoot(true)
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *FsDrive) Rmdir() error {
	err := f.findRoot(false)
	if err != nil {
		return err
	}
	children, err := f.svc.Children.List(f.rootId).MaxResults(10).Do()
	if err != nil {
		return err
	}
	if len(children.Items) > 0 {
		return fmt.Errorf("Directory not empty: %#v", children.Items)
	}
	// Delete the directory if it isn't the root
	if f.root != "" {
		err = f.svc.Files.Delete(f.rootId).Do()
		if err != nil {
			return err
		}
	}
	return nil
}

// Return the precision
func (fs *FsDrive) Precision() time.Duration {
	return time.Millisecond
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *FsDrive) Purge() error {
	if f.root == "" {
		return fmt.Errorf("Can't purge root directory")
	}
	err := f.findRoot(false)
	if err != nil {
		return err
	}
	err = f.svc.Files.Delete(f.rootId).Do()
	if err != nil {
		return err
	}
	return nil
}

// ------------------------------------------------------------

// Return the parent Fs
func (o *FsObjectDrive) Fs() fs.Fs {
	return o.drive
}

// Return a string version
func (o *FsObjectDrive) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Return the remote path
func (o *FsObjectDrive) Remote() string {
	return o.remote
}

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (o *FsObjectDrive) Md5sum() (string, error) {
	return o.md5sum, nil
}

// Size returns the size of an object in bytes
func (o *FsObjectDrive) Size() int64 {
	return o.bytes
}

// setMetaData sets the fs data from a drive.File
func (o *FsObjectDrive) setMetaData(info *drive.File) {
	o.id = info.Id
	o.url = info.DownloadUrl
	o.md5sum = strings.ToLower(info.Md5Checksum)
	o.bytes = info.FileSize
	o.modifiedDate = info.ModifiedDate
}

// readMetaData gets the info if it hasn't already been fetched
func (o *FsObjectDrive) readMetaData() (err error) {
	if o.id != "" {
		return nil
	}

	directory, leaf := splitPath(o.remote)
	directoryId, err := o.drive.findDir(directory, false)
	if err != nil {
		fs.Debug(o, "Couldn't find directory: %s", err)
		return fmt.Errorf("Couldn't find directory: %s", err)
	}

	found, err := o.drive.listAll(directoryId, leaf, false, true, func(item *drive.File) bool {
		if item.Title == leaf {
			o.setMetaData(item)
			return true
		}
		return false
	})
	if err != nil {
		return err
	}
	if !found {
		fs.Debug(o, "Couldn't find object")
		return fmt.Errorf("Couldn't find object")
	}
	return nil
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *FsObjectDrive) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %s", err)
		return time.Now()
	}
	modTime, err := time.Parse(RFC3339In, o.modifiedDate)
	if err != nil {
		fs.Log(o, "Failed to read mtime from object: %s", err)
		return time.Now()
	}
	return modTime
}

// Sets the modification time of the local fs object
func (o *FsObjectDrive) SetModTime(modTime time.Time) {
	err := o.readMetaData()
	if err != nil {
		fs.Stats.Error()
		fs.Log(o, "Failed to read metadata: %s", err)
		return
	}
	// New metadata
	info := &drive.File{
		ModifiedDate: modTime.Format(RFC3339Out),
	}
	// Set modified date
	_, err = o.drive.svc.Files.Update(o.id, info).SetModifiedDate(true).Do()
	if err != nil {
		fs.Stats.Error()
		fs.Log(o, "Failed to update remote mtime: %s", err)
	}
}

// Is this object storable
func (o *FsObjectDrive) Storable() bool {
	return true
}

// Open an object for read
func (o *FsObjectDrive) Open() (in io.ReadCloser, err error) {
	req, err := http.NewRequest("GET", o.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fs.UserAgent)
	res, err := o.drive.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("Bad response: %d: %s", res.StatusCode, res.Status)
	}
	return res.Body, nil
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *FsObjectDrive) Update(in io.Reader, modTime time.Time, size int64) error {
	info := &drive.File{
		Id:           o.id,
		ModifiedDate: modTime.Format(RFC3339Out),
	}

	// Make the API request to upload metadata and file data.
	in = &seekWrapper{in: in, size: size}
	info, err := o.drive.svc.Files.Update(info.Id, info).SetModifiedDate(true).Media(in).Do()
	if err != nil {
		return fmt.Errorf("Update failed: %s", err)
	}
	o.setMetaData(info)
	return nil
}

// Remove an object
func (o *FsObjectDrive) Remove() error {
	return o.drive.svc.Files.Delete(o.id).Do()
}

// Check the interfaces are satisfied
var _ fs.Fs = &FsDrive{}
var _ fs.Purger = &FsDrive{}
var _ fs.Object = &FsObjectDrive{}
