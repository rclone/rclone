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
	"path"
	"strings"
	"time"

	"github.com/ncw/rclone/dircache"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
	"github.com/ncw/rclone/pacer"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/googleapi"
)

// Constants
const (
	rcloneClientID              = "202264815644.apps.googleusercontent.com"
	rcloneEncryptedClientSecret = "eX8GpZTVx3vxMWVkuuBdDWmAUE6rGhTwVrvG9GhllYccSdj2-mvHVg"
	driveFolderType             = "application/vnd.google-apps.folder"
	timeFormatIn                = time.RFC3339
	timeFormatOut               = "2006-01-02T15:04:05.000000000Z07:00"
	minSleep                    = 10 * time.Millisecond
	defaultExtensions           = "docx,xlsx,pptx,svg"
)

// Globals
var (
	// Flags
	driveFullList      = fs.BoolP("drive-full-list", "", false, "Use a full listing for directory list. More data but usually quicker. (obsolete)")
	driveAuthOwnerOnly = fs.BoolP("drive-auth-owner-only", "", false, "Only consider files owned by the authenticated user. Requires drive-full-list.")
	driveUseTrash      = fs.BoolP("drive-use-trash", "", false, "Send files to the trash instead of deleting permanently.")
	driveExtensions    = fs.StringP("drive-formats", "", defaultExtensions, "Comma separated list of preferred formats for downloading Google docs.")
	// chunkSize is the size of the chunks created during a resumable upload and should be a power of two.
	// 1<<18 is the minimum size supported by the Google uploader, and there is no maximum.
	chunkSize         = fs.SizeSuffix(8 * 1024 * 1024)
	driveUploadCutoff = chunkSize
	// Description of how to auth for this app
	driveConfig = &oauth2.Config{
		Scopes:       []string{"https://www.googleapis.com/auth/drive"},
		Endpoint:     google.Endpoint,
		ClientID:     rcloneClientID,
		ClientSecret: fs.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.TitleBarRedirectURL,
	}
	mimeTypeToExtension = map[string]string{
		"application/epub+zip":                                                      "epub",
		"application/msword":                                                        "doc",
		"application/pdf":                                                           "pdf",
		"application/rtf":                                                           "rtf",
		"application/vnd.ms-excel":                                                  "xls",
		"application/vnd.oasis.opendocument.presentation":                           "odp",
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
		"text/tab-separated-values":                                                 "tsv",
	}
	extensionToMimeType map[string]string
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "drive",
		Description: "Google Drive",
		NewFs:       NewFs,
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
	fs.VarP(&driveUploadCutoff, "drive-upload-cutoff", "", "Cutoff for switching to chunked upload")
	fs.VarP(&chunkSize, "drive-chunk-size", "", "Upload chunk size. Must a power of 2 >= 256k.")

	// Invert mimeTypeToExtension
	extensionToMimeType = make(map[string]string, len(mimeTypeToExtension))
	for mimeType, extension := range mimeTypeToExtension {
		extensionToMimeType[extension] = mimeType
	}
}

// Fs represents a remote drive server
type Fs struct {
	name       string             // name of this remote
	root       string             // the path we are working on
	features   *fs.Features       // optional features
	svc        *drive.Service     // the connection to the drive server
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
	mimeType     string
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

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
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
			return false, errors.Wrap(err, "couldn't list directory")
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
func (f *Fs) parseExtensions(extensions string) error {
	for _, extension := range strings.Split(extensions, ",") {
		extension = strings.ToLower(strings.TrimSpace(extension))
		if _, found := extensionToMimeType[extension]; !found {
			return errors.Errorf("couldn't find mime type for extension %q", extension)
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
	return nil
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, path string) (fs.Fs, error) {
	if !isPowerOfTwo(int64(chunkSize)) {
		return nil, errors.Errorf("drive: chunk size %v isn't a power of two", chunkSize)
	}
	if chunkSize < 256*1024 {
		return nil, errors.Errorf("drive: chunk size can't be less than 256k - was %v", chunkSize)
	}

	oAuthClient, _, err := oauthutil.NewClient(name, driveConfig)
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
		pacer: pacer.New().SetMinSleep(minSleep).SetPacer(pacer.GoogleDrivePacer),
	}
	f.features = (&fs.Features{DuplicateFiles: true, ReadMimeType: true, WriteMimeType: true}).Fill(f)

	// Create a new authorized Drive client.
	f.client = oAuthClient
	f.svc, err = drive.New(f.client)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create Drive client")
	}

	// Read About so we know the root path
	err = f.pacer.Call(func() (bool, error) {
		f.about, err = f.svc.About.Get().Do()
		return shouldRetry(err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't read info about Drive")
	}

	f.dirCache = dircache.New(root, f.about.RootFolderId, f)

	// Parse extensions
	err = f.parseExtensions(*driveExtensions)
	if err != nil {
		return nil, err
	}
	err = f.parseExtensions(defaultExtensions) // make sure there are some sensible ones on there
	if err != nil {
		return nil, err
	}

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
		_, err := newF.newObjectWithInfo(remote, nil)
		if err != nil {
			// File doesn't exist so return old f
			return f, nil
		}
		// return an error with an fs which points to the parent
		return &newF, fs.ErrorIsFile
	}
	// fmt.Printf("Root id %s", f.dirCache.RootID())
	return f, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *drive.File) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		o.setMetaData(info)
	} else {
		err := o.readMetaData() // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
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

// isAuthOwned checks if any of the item owners is the authenticated owner
func isAuthOwned(item *drive.File) bool {
	for _, owner := range item.Owners {
		if owner.IsAuthenticatedUser {
			return true
		}
	}
	return false
}

// findExportFormat works out the optimum extension and download URL
// for this item.
//
// Look through the extensions and find the first format that can be
// converted.  If none found then return "", ""
func (f *Fs) findExportFormat(filepath string, item *drive.File) (extension, link string) {
	// Warn about unknown export formats
	for mimeType := range item.ExportLinks {
		if _, ok := mimeTypeToExtension[mimeType]; !ok {
			fs.Debug(filepath, "Unknown export type %q - ignoring", mimeType)
		}
	}

	// Find the first export format we can
	for _, extension := range f.extensions {
		mimeType := extensionToMimeType[extension]
		if link, ok := item.ExportLinks[mimeType]; ok {
			return extension, link
		}
	}

	// else return empty
	return "", ""
}

// ListDir reads the directory specified by the job into out, returning any more jobs
func (f *Fs) ListDir(out fs.ListOpts, job dircache.ListDirJob) (jobs []dircache.ListDirJob, err error) {
	fs.Debug(f, "Reading %q", job.Path)
	_, err = f.listAll(job.DirID, "", false, false, func(item *drive.File) bool {
		remote := job.Path + item.Title
		switch {
		case *driveAuthOwnerOnly && !isAuthOwned(item):
			// ignore object or directory
		case item.MimeType == driveFolderType:
			if out.IncludeDirectory(remote) {
				dir := &fs.Dir{
					Name:  remote,
					Bytes: -1,
					Count: -1,
				}
				dir.When, _ = time.Parse(timeFormatIn, item.ModifiedDate)
				if out.AddDir(dir) {
					return true
				}
				if job.Depth > 0 {
					jobs = append(jobs, dircache.ListDirJob{DirID: item.Id, Path: remote + "/", Depth: job.Depth - 1})
				}
			}
		case item.Md5Checksum != "":
			// If item has MD5 sum it is a file stored on drive
			o, err := f.newObjectWithInfo(remote, item)
			if err != nil {
				out.SetError(err)
				return true
			}
			if out.Add(o) {
				return true
			}
		case len(item.ExportLinks) != 0:
			// If item has export links then it is a google doc
			extension, link := f.findExportFormat(remote, item)
			if extension == "" {
				fs.Debug(remote, "No export formats found")
			} else {
				o, err := f.newObjectWithInfo(remote+"."+extension, item)
				if err != nil {
					out.SetError(err)
					return true
				}
				obj := o.(*Object)
				obj.isDocument = true
				obj.url = link
				obj.bytes = -1
				if out.Add(o) {
					return true
				}
			}
		default:
			fs.Debug(remote, "Ignoring unknown object")
		}
		return false
	})
	fs.Debug(f, "Finished reading %q", job.Path)
	return jobs, err
}

// List walks the path returning files and directories to out
func (f *Fs) List(out fs.ListOpts, dir string) {
	f.dirCache.List(f, out, dir)
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
		MimeType:     fs.MimeTypeFromName(remote),
		ModifiedDate: modTime.Format(timeFormatOut),
	}
	return o, createInfo, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	exisitingObj, err := f.newObjectWithInfo(src.Remote(), nil)
	switch err {
	case nil:
		return exisitingObj, exisitingObj.Update(in, src)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(in, src)
	default:
		return nil, err
	}
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime()

	o, createInfo, err := f.createFileInfo(remote, modTime, size)
	if err != nil {
		return nil, err
	}

	var info *drive.File
	if size == 0 || size < int64(driveUploadCutoff) {
		// Make the API request to upload metadata and file data.
		// Don't retry, return a retry error instead
		err = f.pacer.CallNoRetry(func() (bool, error) {
			info, err = f.svc.Files.Insert(createInfo).Media(in, googleapi.ContentType("")).Do()
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
func (f *Fs) Mkdir(dir string) error {
	err := f.dirCache.FindRoot(true)
	if err != nil {
		return err
	}
	if dir != "" {
		_, err = f.dirCache.FindDir(dir, true)
	}
	return err
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	root := path.Join(f.root, dir)
	dc := f.dirCache
	rootID, err := dc.FindDir(dir, false)
	if err != nil {
		return err
	}
	var children *drive.ChildList
	err = f.pacer.Call(func() (bool, error) {
		children, err = f.svc.Children.List(rootID).MaxResults(10).Do()
		return shouldRetry(err)
	})
	if err != nil {
		return err
	}
	if len(children.Items) > 0 {
		return errors.Errorf("directory not empty: %#v", children.Items)
	}
	// Delete the directory if it isn't the root
	if root != "" {
		err = f.pacer.Call(func() (bool, error) {
			if *driveUseTrash {
				_, err = f.svc.Files.Trash(rootID).Do()
			} else {
				err = f.svc.Files.Delete(rootID).Do()
			}
			return shouldRetry(err)
		})
		if err != nil {
			return err
		}
	}
	f.dirCache.FlushDir(dir)
	if err != nil {
		return err
	}
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
	if srcObj.isDocument {
		return nil, errors.New("can't copy a Google document")
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
		return errors.New("can't purge root directory")
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
	if srcObj.isDocument {
		return nil, errors.New("can't move a Google document")
	}

	// create the destination directory if necessary
	err := f.dirCache.FindRoot(true)
	if err != nil {
		return nil, err
	}

	// Temporary Object under construction
	dstObj, dstInfo, err := f.createFileInfo(remote, srcObj.ModTime(), srcObj.bytes)
	if err != nil {
		return nil, err
	}

	// Do the move
	var info *drive.File
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Patch(srcObj.id, dstInfo).SetModifiedDate(true).Do()
		return shouldRetry(err)
	})
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
	if f.dirCache.FoundRoot() {
		return fs.ErrorDirExists
	}

	// Refuse to move to or from the root
	if f.root == "" || srcFs.root == "" {
		fs.Debug(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
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
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.svc.Files.Patch(srcFs.dirCache.RootID(), &patch).Do()
		return shouldRetry(err)
	})
	if err != nil {
		return err
	}
	srcFs.dirCache.ResetRoot()
	return nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashMD5)
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
		_, res, err := o.httpResponse("HEAD", nil)
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
	o.mimeType = info.MimeType
}

// readMetaData gets the info if it hasn't already been fetched
func (o *Object) readMetaData() (err error) {
	if o.id != "" {
		return nil
	}

	leaf, directoryID, err := o.fs.dirCache.FindPath(o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
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
		return fs.ErrorObjectNotFound
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
		fs.Log(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	modTime, err := time.Parse(timeFormatIn, o.modifiedDate)
	if err != nil {
		fs.Log(o, "Failed to read mtime from object: %v", err)
		return time.Now()
	}
	return modTime
}

// SetModTime sets the modification time of the drive fs object
func (o *Object) SetModTime(modTime time.Time) error {
	err := o.readMetaData()
	if err != nil {
		return err
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
		return err
	}
	// Update info from read data
	o.setMetaData(info)
	return nil
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// httpResponse gets an http.Response object for the object o.url
// using the method passed in
func (o *Object) httpResponse(method string, options []fs.OpenOption) (req *http.Request, res *http.Response, err error) {
	if o.url == "" {
		return nil, nil, errors.New("forbidden to download - check sharing permission")
	}
	req, err = http.NewRequest(method, o.url, nil)
	if err != nil {
		return req, nil, err
	}
	fs.OpenOptionAddHTTPHeaders(req.Header, options)
	err = o.fs.pacer.Call(func() (bool, error) {
		res, err = o.fs.client.Do(req)
		return shouldRetry(err)
	})
	if err != nil {
		return req, nil, err
	}
	return req, res, nil
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
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	req, res, err := o.httpResponse("GET", options)
	if err != nil {
		return nil, err
	}
	_, isRanging := req.Header["Range"]
	if !(res.StatusCode == http.StatusOK || (isRanging && res.StatusCode == http.StatusPartialContent)) {
		_ = res.Body.Close() // ignore error
		return nil, errors.Errorf("bad response: %d: %s", res.StatusCode, res.Status)
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
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	size := src.Size()
	modTime := src.ModTime()
	if o.isDocument {
		return errors.New("can't update a google document")
	}
	updateInfo := &drive.File{
		Id:           o.id,
		MimeType:     fs.MimeType(src),
		ModifiedDate: modTime.Format(timeFormatOut),
	}

	// Make the API request to upload metadata and file data.
	var err error
	var info *drive.File
	if size == 0 || size < int64(driveUploadCutoff) {
		// Don't retry, return a retry error instead
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			info, err = o.fs.svc.Files.Update(updateInfo.Id, updateInfo).SetModifiedDate(true).Media(in, googleapi.ContentType("")).Do()
			return shouldRetry(err)
		})
		if err != nil {
			return err
		}
	} else {
		// Upload the file in chunks
		info, err = o.fs.Upload(in, size, updateInfo.MimeType, updateInfo, o.remote)
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
		return errors.New("can't delete a google document")
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

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = &Object{}
)
