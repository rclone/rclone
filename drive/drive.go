// Package drive interfaces with the Google Drive object storage system
package drive

// FIXME need to deal with some corner cases
// * multiple files with the same name
// * files can be in multiple directories
// * can have directory loops
// * files with / in name

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/googleapi"

	"github.com/ncw/rclone/dircache"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
	"github.com/ncw/rclone/pacer"
	"github.com/spf13/pflag"
)

// Constants
const (
	rcloneClientID     = "202264815644.apps.googleusercontent.com"
	rcloneClientSecret = "8p/yms3OlNXE9OTDl/HLypf9gdiJ5cT3"
	driveFolderType    = "application/vnd.google-apps.folder"
	timeFormatIn       = time.RFC3339
	timeFormatOut      = "2006-01-02T15:04:05.000000000Z07:00"
	minSleep           = 10 * time.Millisecond
	maxSleep           = 2 * time.Second
	decayConstant      = 2 // bigger for slower decay, exponential
	defaultExtensions  = "docx,xlsx,pptx,svg"
)

// Globals
var (
	// Flags
	driveFullList      = pflag.BoolP("drive-full-list", "", true, "Use a full listing for directory list. More data but usually quicker.")
	driveAuthOwnerOnly = pflag.BoolP("drive-auth-owner-only", "", false, "Only consider files owned by the authenticated user. Requires drive-full-list.")
	driveUseTrash      = pflag.BoolP("drive-use-trash", "", false, "Send files to the trash instead of deleting permanently.")
	driveExtensions    = pflag.StringP("drive-formats", "", defaultExtensions, "Comma separated list of preferred formats for downloading Google docs.")
	// chunkSize is the size of the chunks created during a resumable upload and should be a power of two.
	// 1<<18 is the minimum size supported by the Google uploader, and there is no maximum.
	chunkSize         = fs.SizeSuffix(256 * 1024)
	driveUploadCutoff = chunkSize
	// Description of how to auth for this app
	driveConfig = &oauth2.Config{
		Scopes:       []string{"https://www.googleapis.com/auth/drive"},
		Endpoint:     google.Endpoint,
		ClientID:     rcloneClientID,
		ClientSecret: fs.Reveal(rcloneClientSecret),
		RedirectURL:  oauthutil.TitleBarRedirectURL,
	}
	mimeTypeToExtension = map[string]string{
		"application/msword":                                                        "doc",
		"application/pdf":                                                           "pdf",
		"application/rtf":                                                           "rtf",
		"application/vnd.ms-excel":                                                  "xls",
		"application/vnd.oasis.opendocument.spreadsheet":                            "ods",
		"application/vnd.oasis.opendocument.text":                                   "odt",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   "docx",
		"application/x-vnd.oasis.opendocument.spreadsheet":                          "ods",
		"application/zip":                                                           "zip",
		"image/jpeg":                                                                "jpg",
		"image/png":                                                                 "png",
		"image/svg+xml":                                                             "svg",
		"text/csv":                                                                  "csv",
		"text/html":                                                                 "html",
		"text/plain":                                                                "txt",
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.Info{
		Name:  "drive",
		NewFs: NewFs,
		Config: func(name string) {
			err := oauthutil.Config("drive", name, driveConfig)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: fs.ConfigClientID,
			Help: "Google Application Client Id - leave blank normally.",
		}, {
			Name: fs.ConfigClientSecret,
			Help: "Google Application Client Secret - leave blank normally.",
		}},
	})
	pflag.VarP(&driveUploadCutoff, "drive-upload-cutoff", "", "Cutoff for switching to chunked upload")
	pflag.VarP(&chunkSize, "drive-chunk-size", "", "Upload chunk size. Must a power of 2 >= 256k.")
}

// Fs represents a remote drive server
type Fs struct {
	name       string             // name of this remote
	svc        *drive.Service     // the connection to the drive server
	root       string             // the path we are working on
	client     *http.Client       // authorized client
	about      *drive.About       // information about the drive, including the root
	dirCache   *dircache.DirCache // Map of directory path to directory id
	pacer      *pacer.Pacer       // To pace the API calls
	extensions []string           // preferred extensions to download docs
}

// Object describes a drive object
type Object struct {
	fs           *Fs    // what this object is part of
	remote       string // The remote path
	id           string // Drive Id of this object
	url          string // Download URL of this object
	md5sum       string // md5sum of the object
	bytes        int64  // size of the object
	modifiedDate string // RFC3339 time it was last modified
	isDocument   bool   // if set this is a Google doc
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
	return fmt.Sprintf("Google drive root '%s'", f.root)
}

// shouldRetry determines whehter a given err rates being retried
func shouldRetry(err error) (again bool, errOut error) {
	again = false
	if err != nil {
		if fs.ShouldRetry(err) {
			again = true
		} else {
			switch gerr := err.(type) {
			case *googleapi.Error:
				if gerr.Code >= 500 && gerr.Code < 600 {
					// All 5xx errors should be retried
					again = true
				} else if len(gerr.Errors) > 0 {
					reason := gerr.Errors[0].Reason
					if reason == "rateLimitExceeded" || reason == "userRateLimitExceeded" {
						again = true
					}
				}
			}
		}
	}
	return again, err
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
func (f *Fs) listAll(dirID string, title string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	query := fmt.Sprintf("trashed=false")
	if dirID != "" {
		query += fmt.Sprintf(" and '%s' in parents", dirID)
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
		var files *drive.FileList
		err = f.pacer.Call(func() (bool, error) {
			files, err = list.Do()
			return shouldRetry(err)
		})
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

// Returns true of x is a power of 2 or zero
func isPowerOfTwo(x int64) bool {
	switch {
	case x == 0:
		return true
	case x < 0:
		return false
	default:
		return (x & (x - 1)) == 0
	}
}

// parseExtensions parses drive export extensions from a string
func (f *Fs) parseExtensions(extensions string) {
	// Invert mimeTypeToExtension
	var extensionToMimeType = make(map[string]string, len(mimeTypeToExtension))
	for mimeType, extension := range mimeTypeToExtension {
		extensionToMimeType[extension] = mimeType
	}
	for _, extension := range strings.Split(extensions, ",") {
		extension = strings.ToLower(strings.TrimSpace(extension))
		if _, found := extensionToMimeType[extension]; !found {
			log.Fatalf("Couldn't find mime type for extension %q", extension)
		}
		found := false
		for _, existingExtension := range f.extensions {
			if extension == existingExtension {
				found = true
				break
			}
		}
		if !found {
			f.extensions = append(f.extensions, extension)
		}
	}
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, path string) (fs.Fs, error) {
	if !isPowerOfTwo(int64(chunkSize)) {
		return nil, fmt.Errorf("drive: chunk size %v isn't a power of two", chunkSize)
	}
	if chunkSize < 256*1024 {
		return nil, fmt.Errorf("drive: chunk size can't be less than 256k - was %v", chunkSize)
	}

	oAuthClient, err := oauthutil.NewClient(name, driveConfig)
	if err != nil {
		log.Fatalf("Failed to configure drive: %v", err)
	}

	root, err := parseDrivePath(path)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:  name,
		root:  root,
		pacer: pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
	}

	// Create a new authorized Drive client.
	f.client = oAuthClient
	f.svc, err = drive.New(f.client)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create Drive client: %s", err)
	}

	// Read About so we know the root path
	err = f.pacer.Call(func() (bool, error) {
		f.about, err = f.svc.About.Get().Do()
		return shouldRetry(err)
	})
	if err != nil {
		return nil, fmt.Errorf("Couldn't read info about Drive: %s", err)
	}

	f.dirCache = dircache.New(root, f.about.RootFolderId, f)

	// Parse extensions
	f.parseExtensions(*driveExtensions)
	f.parseExtensions(defaultExtensions) // make sure there are some sensible ones on there

	// Find the current root
	err = f.dirCache.FindRoot(false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		newF := *f
		newF.dirCache = dircache.New(newRoot, f.about.RootFolderId, &newF)
		newF.root = newRoot
		// Make new Fs which is the parent
		err = newF.dirCache.FindRoot(false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		obj, err := newF.newFsObjectWithInfoErr(remote, nil)
		if err != nil {
			// File doesn't exist so return old f
			return f, nil
		}
		// return a Fs Limited to this object
		return fs.NewLimited(&newF, obj), nil
	}
	// fmt.Printf("Root id %s", f.dirCache.RootID())
	return f, nil
}

// Return an FsObject from a path
func (f *Fs) newFsObjectWithInfoErr(remote string, info *drive.File) (fs.Object, error) {
	fs := &Object{
		fs:     f,
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
func (f *Fs) newFsObjectWithInfo(remote string, info *drive.File) fs.Object {
	fs, _ := f.newFsObjectWithInfoErr(remote, info)
	// Errors have already been logged
	return fs
}

// NewFsObject returns an FsObject from a path
//
// May return nil if an error occurred
func (f *Fs) NewFsObject(remote string) fs.Object {
	return f.newFsObjectWithInfo(remote, nil)
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.listAll(pathID, leaf, true, false, func(item *drive.File) bool {
		if item.Title == leaf {
			pathIDOut = item.Id
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(pathID, leaf string) (newID string, err error) {
	// fmt.Println("Making", path)
	// Define the metadata for the directory we are going to create.
	createInfo := &drive.File{
		Title:       leaf,
		Description: leaf,
		MimeType:    driveFolderType,
		Parents:     []*drive.ParentReference{{Id: pathID}},
	}
	var info *drive.File
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Insert(createInfo).Do()
		return shouldRetry(err)
	})
	if err != nil {
		return "", err
	}
	return info.Id, nil
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
	_, err := f.listAll(dirID, "", false, false, func(item *drive.File) bool {
		// Recurse on directories
		if item.MimeType == driveFolderType {
			wg.Add(1)
			folder := path + item.Title + "/"
			fs.Debug(f, "Reading %s", folder)

			go func() {
				defer wg.Done()
				err := f.listDirRecursive(item.Id, folder, out)
				if err != nil {
					subError = err
					fs.ErrorLog(f, "Error reading %s:%s", folder, err)
				}

			}()
		} else {
			filepath := path + item.Title
			if item.Md5Checksum != "" {
				// If item has MD5 sum it is a file stored on drive
				if o := f.newFsObjectWithInfo(filepath, item); o != nil {
					out <- o
				}
			} else if len(item.ExportLinks) != 0 {
				// If item has export links then it is a google doc
				var firstExtension, firstLink string
				var extension, link string
			outer:
				for exportMimeType, exportLink := range item.ExportLinks {
					exportExtension, ok := mimeTypeToExtension[exportMimeType]
					if !ok {
						fs.Debug(filepath, "Unknown export type %q - ignoring", exportMimeType)
						continue
					}
					if firstExtension == "" {
						firstExtension = exportExtension
						firstLink = exportLink
					}
					for _, preferredExtension := range f.extensions {
						if exportExtension == preferredExtension {
							extension = exportExtension
							link = exportLink
							break outer
						}
					}
				}
				if extension == "" {
					extension = firstExtension
					link = firstLink
				}
				if extension == "" {
					fs.Debug(filepath, "No export formats found")
				} else {
					if o := f.newFsObjectWithInfo(filepath+"."+extension, item); o != nil {
						obj := o.(*Object)
						obj.isDocument = true
						obj.url = link
						obj.bytes = -1
						out <- o
					}
				}
			} else {
				fs.Debug(filepath, "Ignoring unknown object")
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

// isAuthOwned checks if any of the item owners is the authenticated owner
func isAuthOwned(item *drive.File) bool {
	for _, owner := range item.Owners {
		if owner.IsAuthenticatedUser {
			return true
		}
	}
	return false
}

// Path should be directory path either "" or "path/"
//
// List the directory using a full listing and filtering out unwanted
// items
//
// This is fast in terms of number of API calls, but slow in terms of
// fetching more data than it needs
func (f *Fs) listDirFull(dirID string, path string, out fs.ObjectsChan) error {
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
		if *driveAuthOwnerOnly && !isAuthOwned(item) {
			return
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
				if fs := f.newFsObjectWithInfo(path, item); fs != nil {
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
		parentID := item.Parents[0].Id
		directory, ok := f.dirCache.GetInv(parentID)
		if !ok {
			// Haven't found the parent yet so add to orphans
			// fmt.Printf("orphan[%s] %s %s\n", parentID, item.Title, item.Id)
			orphans[parentID] = append(orphans[parentID], item)
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

// List walks the path returning a channel of FsObjects
func (f *Fs) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		err := f.dirCache.FindRoot(false)
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't find root: %s", err)
		} else {
			if f.root == "" && *driveFullList {
				err = f.listDirFull(f.dirCache.RootID(), "", out)
			} else {
				err = f.listDirRecursive(f.dirCache.RootID(), "", out)
			}
			if err != nil {
				fs.Stats.Error()
				fs.ErrorLog(f, "List failed: %s", err)
			}
		}
	}()
	return out
}

// ListDir walks the path returning a channel of directories
func (f *Fs) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		err := f.dirCache.FindRoot(false)
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't find root: %s", err)
		} else {
			_, err := f.listAll(f.dirCache.RootID(), "", true, false, func(item *drive.File) bool {
				dir := &fs.Dir{
					Name:  item.Title,
					Bytes: -1,
					Count: -1,
				}
				dir.When, _ = time.Parse(timeFormatIn, item.ModifiedDate)
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

// Creates a drive.File info from the parameters passed in and a half
// finished Object which must have setMetaData called on it
//
// Used to create new objects
func (f *Fs) createFileInfo(remote string, modTime time.Time, size int64) (*Object, *drive.File, error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
		bytes:  size,
	}

	leaf, directoryID, err := f.dirCache.FindPath(remote, true)
	if err != nil {
		return nil, nil, err
	}

	// Define the metadata for the file we are going to create.
	createInfo := &drive.File{
		Title:        leaf,
		Description:  leaf,
		Parents:      []*drive.ParentReference{{Id: directoryID}},
		MimeType:     fs.MimeType(o),
		ModifiedDate: modTime.Format(timeFormatOut),
	}
	return o, createInfo, nil
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
func (f *Fs) Put(in io.Reader, remote string, modTime time.Time, size int64) (fs.Object, error) {
	o, createInfo, err := f.createFileInfo(remote, modTime, size)
	if err != nil {
		return nil, err
	}

	var info *drive.File
	if size == 0 || size < int64(driveUploadCutoff) {
		// Make the API request to upload metadata and file data.
		// Don't retry, return a retry error instead
		err = f.pacer.CallNoRetry(func() (bool, error) {
			info, err = f.svc.Files.Insert(createInfo).Media(in).Do()
			return shouldRetry(err)
		})
		if err != nil {
			return o, err
		}
	} else {
		// Upload the file in chunks
		info, err = f.Upload(in, size, createInfo.MimeType, createInfo, remote)
		if err != nil {
			return o, err
		}
	}
	o.setMetaData(info)
	return o, nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir() error {
	return f.dirCache.FindRoot(true)
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir() error {
	err := f.dirCache.FindRoot(false)
	if err != nil {
		return err
	}
	var children *drive.ChildList
	err = f.pacer.Call(func() (bool, error) {
		children, err = f.svc.Children.List(f.dirCache.RootID()).MaxResults(10).Do()
		return shouldRetry(err)
	})
	if err != nil {
		return err
	}
	if len(children.Items) > 0 {
		return fmt.Errorf("Directory not empty: %#v", children.Items)
	}
	// Delete the directory if it isn't the root
	if f.root != "" {
		err = f.pacer.Call(func() (bool, error) {
			if *driveUseTrash {
				_, err = f.svc.Files.Trash(f.dirCache.RootID()).Do()
			} else {
				err = f.svc.Files.Delete(f.dirCache.RootID()).Do()
			}
			return shouldRetry(err)
		})
		if err != nil {
			return err
		}
	}
	f.dirCache.ResetRoot()
	return nil
}

// Precision of the object storage system
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
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

	o, createInfo, err := f.createFileInfo(remote, srcObj.ModTime(), srcObj.bytes)
	if err != nil {
		return nil, err
	}

	var info *drive.File
	err = o.fs.pacer.Call(func() (bool, error) {
		info, err = o.fs.svc.Files.Copy(srcObj.id, createInfo).Do()
		return shouldRetry(err)
	})
	if err != nil {
		return nil, err
	}

	o.setMetaData(info)
	return o, nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	if f.root == "" {
		return fmt.Errorf("Can't purge root directory")
	}
	err := f.dirCache.FindRoot(false)
	if err != nil {
		return err
	}
	err = f.pacer.Call(func() (bool, error) {
		if *driveUseTrash {
			_, err = f.svc.Files.Trash(f.dirCache.RootID()).Do()
		} else {
			err = f.svc.Files.Delete(f.dirCache.RootID()).Do()
		}
		return shouldRetry(err)
	})
	f.dirCache.ResetRoot()
	if err != nil {
		return err
	}
	return nil
}

// Move src to this remote using server side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debug(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Temporary FsObject under construction
	dstObj, dstInfo, err := f.createFileInfo(remote, srcObj.ModTime(), srcObj.bytes)
	if err != nil {
		return nil, err
	}

	// Do the move
	info, err := f.svc.Files.Patch(srcObj.id, dstInfo).SetModifiedDate(true).Do()
	if err != nil {
		return nil, err
	}

	dstObj.setMetaData(info)
	return dstObj, nil
}

// DirMove moves src directory to this remote using server side move
// operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debug(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	// Check if destination exists
	f.dirCache.ResetRoot()
	err := f.dirCache.FindRoot(false)
	if err == nil {
		return fs.ErrorDirExists
	}

	// Find ID of parent
	leaf, directoryID, err := f.dirCache.FindPath(f.root, true)
	if err != nil {
		return err
	}

	// Do the move
	patch := drive.File{
		Title:   leaf,
		Parents: []*drive.ParentReference{{Id: directoryID}},
	}
	_, err = f.svc.Files.Patch(srcFs.dirCache.RootID(), &patch).Do()
	if err != nil {
		return err
	}
	srcFs.dirCache.ResetRoot()
	return nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
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

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(t fs.HashType) (string, error) {
	if t != fs.HashMD5 {
		return "", fs.ErrHashUnsupported
	}
	return o.md5sum, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	if o.isDocument && o.bytes < 0 {
		// If it is a google doc then we must HEAD it to see
		// how big it is
		res, err := o.httpResponse("HEAD")
		if err != nil {
			fs.ErrorLog(o, "Error reading size: %v", err)
			return 0
		}
		_ = res.Body.Close()
		o.bytes = res.ContentLength
		// fs.Debug(o, "Read size of document: %v", o.bytes)
	}
	return o.bytes
}

// setMetaData sets the fs data from a drive.File
func (o *Object) setMetaData(info *drive.File) {
	o.id = info.Id
	o.url = info.DownloadUrl
	o.md5sum = strings.ToLower(info.Md5Checksum)
	o.bytes = info.FileSize
	o.modifiedDate = info.ModifiedDate
}

// readMetaData gets the info if it hasn't already been fetched
func (o *Object) readMetaData() (err error) {
	if o.id != "" {
		return nil
	}

	leaf, directoryID, err := o.fs.dirCache.FindPath(o.remote, false)
	if err != nil {
		return err
	}

	found, err := o.fs.listAll(directoryID, leaf, false, true, func(item *drive.File) bool {
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
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %s", err)
		return time.Now()
	}
	modTime, err := time.Parse(timeFormatIn, o.modifiedDate)
	if err != nil {
		fs.Log(o, "Failed to read mtime from object: %s", err)
		return time.Now()
	}
	return modTime
}

// SetModTime sets the modification time of the drive fs object
func (o *Object) SetModTime(modTime time.Time) {
	err := o.readMetaData()
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to read metadata: %s", err)
		return
	}
	// New metadata
	updateInfo := &drive.File{
		ModifiedDate: modTime.Format(timeFormatOut),
	}
	// Set modified date
	var info *drive.File
	err = o.fs.pacer.Call(func() (bool, error) {
		info, err = o.fs.svc.Files.Update(o.id, updateInfo).SetModifiedDate(true).Do()
		return shouldRetry(err)
	})
	if err != nil {
		fs.Stats.Error()
		fs.ErrorLog(o, "Failed to update remote mtime: %s", err)
		return
	}
	// Update info from read data
	o.setMetaData(info)
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// httpResponse gets an http.Response object for the object o.url
// using the method passed in
func (o *Object) httpResponse(method string) (res *http.Response, err error) {
	if o.url == "" {
		return nil, fmt.Errorf("Forbidden to download - check sharing permission")
	}
	req, err := http.NewRequest(method, o.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fs.UserAgent)
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.client.Do(req)
		return shouldRetry(err)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// openFile represents an Object open for reading
type openFile struct {
	o     *Object       // Object we are reading for
	in    io.ReadCloser // reading from here
	bytes int64         // number of bytes read on this connection
	eof   bool          // whether we have read end of file
}

// Read bytes from the object - see io.Reader
func (file *openFile) Read(p []byte) (n int, err error) {
	n, err = file.in.Read(p)
	file.bytes += int64(n)
	if err == io.EOF {
		file.eof = true
	}
	return
}

// Close the object and update bytes read
func (file *openFile) Close() (err error) {
	// If end of file, update bytes read
	if file.eof {
		// fs.Debug(file.o, "Updating size of doc after download to %v", file.bytes)
		file.o.bytes = file.bytes
	}
	return file.in.Close()
}

// Check it satisfies the interfaces
var _ io.ReadCloser = &openFile{}

// Open an object for read
func (o *Object) Open() (in io.ReadCloser, err error) {
	res, err := o.httpResponse("GET")
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		_ = res.Body.Close() // ignore error
		return nil, fmt.Errorf("Bad response: %d: %s", res.StatusCode, res.Status)
	}
	// If it is a document, update the size with what we are
	// reading as it can change from the HEAD in the listing to
	// this GET.  This stops rclone marking the transfer as
	// corrupted.
	if o.isDocument {
		return &openFile{o: o, in: res.Body}, nil
	}
	return res.Body, nil
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, modTime time.Time, size int64) error {
	if o.isDocument {
		return fmt.Errorf("Can't update a google document")
	}
	updateInfo := &drive.File{
		Id:           o.id,
		ModifiedDate: modTime.Format(timeFormatOut),
	}

	// Make the API request to upload metadata and file data.
	var err error
	var info *drive.File
	if size == 0 || size < int64(driveUploadCutoff) {
		// Don't retry, return a retry error instead
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			info, err = o.fs.svc.Files.Update(updateInfo.Id, updateInfo).SetModifiedDate(true).Media(in).Do()
			return shouldRetry(err)
		})
		if err != nil {
			return err
		}
	} else {
		// Upload the file in chunks
		info, err = o.fs.Upload(in, size, fs.MimeType(o), updateInfo, o.remote)
		if err != nil {
			return err
		}
	}
	o.setMetaData(info)
	return nil
}

// Remove an object
func (o *Object) Remove() error {
	if o.isDocument {
		return fmt.Errorf("Can't delete a google document")
	}
	var err error
	err = o.fs.pacer.Call(func() (bool, error) {
		if *driveUseTrash {
			_, err = o.fs.svc.Files.Trash(o.id).Do()
		} else {
			err = o.fs.svc.Files.Delete(o.id).Do()
		}
		return shouldRetry(err)
	})
	return err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs       = (*Fs)(nil)
	_ fs.Purger   = (*Fs)(nil)
	_ fs.Copier   = (*Fs)(nil)
	_ fs.Mover    = (*Fs)(nil)
	_ fs.DirMover = (*Fs)(nil)
	_ fs.Object   = (*Object)(nil)
)
