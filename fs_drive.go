// Drive interface
package main

// FIXME drive code is leaking goroutines somehow - reported bug
// https://code.google.com/p/google-api-go-client/issues/detail?id=23

// FIXME use recursive listing not bound to directory for speed?

// FIXME list containers equivalent should list directories?

// FIXME list directory should list to channel for concurrency not
// append to array

// FIXME perhaps have a drive setup mode where we ask for all the
// params interactively and store them all in one file
// - don't need to store client* apparently

// NB permissions of token file is too open

// FIXME need to deal with some corner cases
// * multiple files with the same name
// * files can be in multiple directories
// * can have directory loops

import (
	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/drive/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

// FsDrive represents a remote drive server
type FsDrive struct {
	svc         *drive.Service // the connection to the drive server
	root        string         // the path we are working on
	client      *http.Client   // authorized client
	about       *drive.About   // information about the drive, including the root
	rootId      string         // Id of the root directory
	foundRoot   sync.Once      // Whether we need to find the root directory or not
	dirCache    lockedMap      // Map of directory path to directory id
	findDirLock sync.Mutex     // Protect findDir from concurrent use
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

// lockedMap is a map with a mutex
type lockedMap struct {
	sync.RWMutex
	cache map[string]string
}

// Make a new locked map
func newLockedMap() lockedMap {
	return lockedMap{cache: make(map[string]string)}
}

// Get an item from the map
func (m *lockedMap) Get(key string) (value string, ok bool) {
	m.RLock()
	value, ok = m.cache[key]
	m.RUnlock()
	return
}

// Put an item to the map
func (m *lockedMap) Put(key, value string) {
	m.Lock()
	m.cache[key] = value
	m.Unlock()
}

// Flush the map of all data
func (m *lockedMap) Flush() {
	m.Lock()
	m.cache = make(map[string]string)
	m.Unlock()
}

// ------------------------------------------------------------

// Constants
const (
	//	defaultDriveTokenFile = ".google-drive-token" // FIXME root in home directory somehow
	driveFolderType = "application/vnd.google-apps.folder"
)

// Globals
var (
	// Flags
	driveClientId     = flag.String("drive-client-id", os.Getenv("GDRIVE_CLIENT_ID"), "Auth URL for server. Defaults to environment var GDRIVE_CLIENT_ID.")
	driveClientSecret = flag.String("drive-client-secret", os.Getenv("GDRIVE_CLIENT_SECRET"), "User name. Defaults to environment var GDRIVE_CLIENT_SECRET.")
	driveTokenFile    = flag.String("drive-token-file", os.Getenv("GDRIVE_TOKEN_FILE"), "API key (password). Defaults to environment var GDRIVE_TOKEN_FILE.")
	driveAuthCode     = flag.String("drive-auth-code", "", "Pass in when requested to make the drive token file.")
)

// String converts this FsDrive to a string
func (f *FsDrive) String() string {
	return fmt.Sprintf("Google drive root '%s'", f.root)
}

// Pattern to match a drive url
var driveMatch = regexp.MustCompile(`^drive://(.*)$`)

// parseParse parses a drive 'url'
func parseDrivePath(path string) (root string, err error) {
	parts := driveMatch.FindAllStringSubmatch(path, -1)
	if len(parts) != 1 || len(parts[0]) != 2 {
		err = fmt.Errorf("Couldn't parse drive url %q", path)
	} else {
		root = parts[0][1]
		root = strings.Trim(root, "/")
	}
	return
}

// Lists the directory required
//
// Search params: https://developers.google.com/drive/search-parameters
func (f *FsDrive) listAll(dirId string, title string, directoriesOnly bool, filesOnly bool) (items []*drive.File, err error) {
	query := fmt.Sprintf("trashed=false and '%s' in parents", dirId)
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
	list := f.svc.Files.List().Q(query)
	for {
		files, err := list.Do()
		if err != nil {
			return nil, fmt.Errorf("Couldn't list directory: %s", err)
		}
		items = append(items, files.Items...)
		if files.NextPageToken == "" {
			break
		}
		list.PageToken(files.NextPageToken)
	}
	return
}

// Ask the user for a new auth
func MakeNewToken(t *oauth.Transport) error {
	if *driveAuthCode == "" {
		// Generate a URL to visit for authorization.
		authUrl := t.Config.AuthCodeURL("state")
		fmt.Fprintf(os.Stderr, "Go to the following link in your browser\n")
		fmt.Fprintf(os.Stderr, "%s\n", authUrl)
		fmt.Fprintf(os.Stderr, "Log in, then re-run this program with the -drive-auth-code parameter\n")
		fmt.Fprintf(os.Stderr, "You only need this parameter once until the drive token file has been created\n")
		return errors.New("Re-run with --drive-auth-code")
	}

	// Read the code, and exchange it for a token.
	//fmt.Printf("Enter verification code: ")
	//var code string
	//fmt.Scanln(&code)
	_, err := t.Exchange(*driveAuthCode)
	return err
}

// NewFsDrive contstructs an FsDrive from the path, container:path
func NewFsDrive(path string) (*FsDrive, error) {
	if *driveClientId == "" {
		return nil, errors.New("Need -drive-client-id or environmental variable GDRIVE_CLIENT_ID")
	}
	if *driveClientSecret == "" {
		return nil, errors.New("Need -drive-client-secret or environmental variable GDRIVE_CLIENT_SECRET")
	}
	if *driveTokenFile == "" {
		return nil, errors.New("Need -drive-token-file or environmental variable GDRIVE_TOKEN_FILE")
	}

	// Settings for authorization.
	var driveConfig = &oauth.Config{
		ClientId:     *driveClientId,
		ClientSecret: *driveClientSecret,
		Scope:        "https://www.googleapis.com/auth/drive",
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		TokenCache:   oauth.CacheFile(*driveTokenFile),
	}

	root, err := parseDrivePath(path)
	if err != nil {
		return nil, err
	}
	f := &FsDrive{root: root, dirCache: newLockedMap()}

	t := &oauth.Transport{
		Config:    driveConfig,
		Transport: http.DefaultTransport,
	}

	// Try to pull the token from the cache; if this fails, we need to get one.
	token, err := driveConfig.TokenCache.Token()
	if err != nil {
		err := MakeNewToken(t)
		if err != nil {
			return nil, fmt.Errorf("Failed to authorise: %s", err)
		}
	} else {
		if *driveAuthCode != "" {
			return nil, fmt.Errorf("Only supply -drive-auth-code once")
		}
	}
	t.Token = token

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
	return f, nil
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsDrive) NewFsObjectWithInfo(remote string, info *drive.File) FsObject {
	fs := &FsObjectDrive{
		drive:  f,
		remote: remote,
	}
	if info != nil {
		fs.setMetaData(info)
	} else {
		err := fs.readMetaData() // reads info and meta, returning an error
		if err != nil {
			// logged already FsDebug("Failed to read info: %s", err)
			return nil
		}
	}
	return fs
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsDrive) NewFsObject(remote string) FsObject {
	return f.NewFsObjectWithInfo(remote, nil)
}

// Path should be directory path either "" or "path/"
func (f *FsDrive) listDir(dirId string, path string, out FsObjectsChan) error {
	// Make the API request
	items, err := f.listAll(dirId, "", false, false)
	if err != nil {
		return err
	}
	for _, item := range items {
		// Recurse on directories
		// FIXME should do this in parallel
		// use a wg to sync then collect error
		if item.MimeType == driveFolderType {
			err := f.listDir(item.Id, path+item.Title+"/", out)
			if err != nil {
				return err
			}
		} else {
			// If item has no MD5 sum it isn't stored on drive, so ignore it
			if item.Md5Checksum == "" {
				continue
			}
			if fs := f.NewFsObjectWithInfo(path+item.Title, item); fs != nil {
				out <- fs
			}
		}
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
	items, err := f.listAll(pathId, leaf, true, false)
	if err != nil {
		return pathId, err
	}
	found := false
	for _, file := range items {
		if file.Title == leaf {
			pathId = file.Id
			found = true
			break
		}
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
	var err error
	f.foundRoot.Do(func() {
		f.rootId, err = f.findDir(f.root, create)
		f.dirCache.Flush()
	})
	return err
}

// Walk the path returning a channel of FsObjects
func (f *FsDrive) List() FsObjectsChan {
	out := make(FsObjectsChan, *checkers)
	go func() {
		defer close(out)
		err := f.findRoot(false)
		if err != nil {
			stats.Error()
			log.Printf("Couldn't find root: %s", err)
		} else {
			err = f.listDir(f.rootId, "", out)
			if err != nil {
				stats.Error()
				log.Printf("List failed: %s", err)
			}
		}
	}()
	return out
}

// Put the FsObject into the container
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created
func (f *FsDrive) Put(in io.Reader, remote string, modTime time.Time, size int64) (FsObject, error) {
	// Temporary FsObject under construction
	fs := &FsObjectDrive{drive: f, remote: remote}

	directory, leaf := splitPath(remote)
	directoryId, err := f.findDir(directory, true)
	if err != nil {
		return nil, fmt.Errorf("Couldn't find or make directory: %s", err)
	}

	// Guess the mime type
	mimeType := mime.TypeByExtension(path.Ext(remote))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Define the metadata for the file we are going to create.
	info := &drive.File{
		Title:       leaf,
		Description: leaf,
		Parents:     []*drive.ParentReference{{Id: directoryId}},
		MimeType:    mimeType,
	}

	// FIXME can't set modified date on initial upload as no
	// .SetModifiedDate().  This agrees with the API docs, but not
	// with the comment on
	// https://developers.google.com/drive/v2/reference/files/insert
	//
	// modifiedDate datetime Last time this file was modified by
	// anyone (formatted RFC 3339 timestamp). This is only mutable
	// on update when the setModifiedDate parameter is set.
	// writable
	//
	// There is no setModifiedDate parameter though

	// Make the API request to upload infodata and file data.
	info, err = f.svc.Files.Insert(info).Media(in).Do()
	if err != nil {
		return nil, fmt.Errorf("Upload failed: %s", err)
	}
	fs.setMetaData(info)

	// Set modified date
	info.ModifiedDate = modTime.Format(time.RFC3339Nano)
	_, err = f.svc.Files.Update(info.Id, info).SetModifiedDate(true).Do()
	if err != nil {
		return fs, fmt.Errorf("Failed to set mtime: %s", err)
	}
	return fs, nil
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
// Returns an error if it isn't empty
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

// Return the remote path
func (fs *FsObjectDrive) Remote() string {
	return fs.remote
}

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (fs *FsObjectDrive) Md5sum() (string, error) {
	return fs.md5sum, nil
}

// Size returns the size of an object in bytes
func (fs *FsObjectDrive) Size() int64 {
	return fs.bytes
}

// setMetaData sets the fs data from a drive.File
func (fs *FsObjectDrive) setMetaData(info *drive.File) {
	fs.id = info.Id
	fs.url = info.DownloadUrl
	fs.md5sum = strings.ToLower(info.Md5Checksum)
	fs.bytes = info.FileSize
	fs.modifiedDate = info.ModifiedDate
}

// readMetaData gets the info if it hasn't already been fetched
func (fs *FsObjectDrive) readMetaData() (err error) {
	if fs.id != "" {
		return nil
	}

	directory, leaf := splitPath(fs.remote)
	directoryId, err := fs.drive.findDir(directory, false)
	if err != nil {
		FsDebug(fs, "Couldn't find directory: %s", err)
		return fmt.Errorf("Couldn't find directory: %s", err)
	}

	items, err := fs.drive.listAll(directoryId, leaf, false, true)
	if err != nil {
		return err
	}
	for _, file := range items {
		if file.Title == leaf {
			fs.setMetaData(file)
			return nil
		}
	}
	FsDebug(fs, "Couldn't find object")
	return fmt.Errorf("Couldn't find object")
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (fs *FsObjectDrive) ModTime() time.Time {
	err := fs.readMetaData()
	if err != nil {
		FsLog(fs, "Failed to read metadata: %s", err)
		return time.Now()
	}
	modTime, err := time.Parse(time.RFC3339, fs.modifiedDate)
	if err != nil {
		FsLog(fs, "Failed to read mtime from object: %s", err)
		return time.Now()
	}
	return modTime
}

// Sets the modification time of the local fs object
func (fs *FsObjectDrive) SetModTime(modTime time.Time) {
	err := fs.readMetaData()
	if err != nil {
		stats.Error()
		FsLog(fs, "Failed to read metadata: %s", err)
		return
	}
	// New metadata
	info := &drive.File{
		ModifiedDate: modTime.Format(time.RFC3339Nano),
	}
	// Set modified date
	_, err = fs.drive.svc.Files.Update(fs.id, info).SetModifiedDate(true).Do()
	if err != nil {
		stats.Error()
		FsLog(fs, "Failed to update remote mtime: %s", err)
	}
}

// Is this object storable
func (fs *FsObjectDrive) Storable() bool {
	return true
}

// Open an object for read
func (fs *FsObjectDrive) Open() (in io.ReadCloser, err error) {
	req, _ := http.NewRequest("GET", fs.url, nil)
	req.Header.Set("User-Agent", "swiftsync/1.0")
	res, err := fs.drive.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("Bad response: %d: %s", res.StatusCode, res.Status)
	}
	return res.Body, nil
}

// Remove an object
func (fs *FsObjectDrive) Remove() error {
	return fs.drive.svc.Files.Delete(fs.id).Do()
}

// Check the interfaces are satisfied
var _ Fs = &FsDrive{}
var _ Purger = &FsDrive{}
var _ FsObject = &FsObjectDrive{}
