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
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/rclone/dircache"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
	"github.com/ncw/rclone/pacer"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
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
	driveAuthOwnerOnly = fs.BoolP("drive-auth-owner-only", "", false, "Only consider files owned by the authenticated user.")
	driveUseTrash      = fs.BoolP("drive-use-trash", "", true, "Send files to the trash instead of deleting permanently.")
	driveSkipGdocs     = fs.BoolP("drive-skip-gdocs", "", false, "Skip google documents in all listings.")
	driveSharedWithMe  = fs.BoolP("drive-shared-with-me", "", false, "Only show files that are shared with me")
	driveTrashedOnly   = fs.BoolP("drive-trashed-only", "", false, "Only show files that are in the trash")
	driveExtensions    = fs.StringP("drive-formats", "", defaultExtensions, "Comma separated list of preferred formats for downloading Google docs.")
	driveListChunk     = pflag.Int64P("drive-list-chunk", "", 1000, "Size of listing chunk 100-1000. 0 to disable.")
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
	partialFields       = "id,downloadUrl,exportLinks,fileExtension,fullFileExtension,fileSize,labels,md5Checksum,modifiedDate,mimeType,title"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "drive",
		Description: "Google Drive",
		NewFs:       NewFs,
		Config: func(name string) {
			var err error
			if fs.ConfigFileGet(name, "service_account_file") == "" {
				err = oauthutil.Config("drive", name, driveConfig)
				if err != nil {
					log.Fatalf("Failed to configure token: %v", err)
				}
			}
			err = configTeamDrive(name)
			if err != nil {
				log.Fatalf("Failed to configure team drive: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: fs.ConfigClientID,
			Help: "Google Application Client Id - leave blank normally.",
		}, {
			Name: fs.ConfigClientSecret,
			Help: "Google Application Client Secret - leave blank normally.",
		}, {
			Name: "service_account_file",
			Help: "Service Account Credentials JSON file path - needed only if you want use SA instead of interactive login.",
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
	name        string             // name of this remote
	root        string             // the path we are working on
	features    *fs.Features       // optional features
	svc         *drive.Service     // the connection to the drive server
	client      *http.Client       // authorized client
	about       *drive.About       // information about the drive, including the root
	dirCache    *dircache.DirCache // Map of directory path to directory id
	pacer       *pacer.Pacer       // To pace the API calls
	extensions  []string           // preferred extensions to download docs
	teamDriveID string             // team drive ID, may be ""
	isTeamDrive bool               // true if this is a team drive
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

// User function to process a File item from list
//
// Should return true to finish processing
type listFn func(*drive.File) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
//
// Search params: https://developers.google.com/drive/search-parameters
func (f *Fs) list(dirID string, title string, directoriesOnly bool, filesOnly bool, includeAll bool, fn listFn) (found bool, err error) {
	var query []string
	if !includeAll {
		q := "trashed=" + strconv.FormatBool(*driveTrashedOnly)
		if *driveTrashedOnly {
			q = fmt.Sprintf("(mimeType='%s' or %s)", driveFolderType, q)
		}
		query = append(query, q)
	}
	// Search with sharedWithMe will always return things listed in "Shared With Me" (without any parents)
	// We must not filter with parent when we try list "ROOT" with drive-shared-with-me
	// If we need to list file inside those shared folders, we must search it without sharedWithMe
	if *driveSharedWithMe && dirID == f.about.RootFolderId {
		query = append(query, "sharedWithMe=true")
	}
	if dirID != "" && !(*driveSharedWithMe && dirID == f.about.RootFolderId) {
		query = append(query, fmt.Sprintf("'%s' in parents", dirID))
	}
	if title != "" {
		// Escaping the backslash isn't documented but seems to work
		title = strings.Replace(title, `\`, `\\`, -1)
		title = strings.Replace(title, `'`, `\'`, -1)
		// Convert ／ to / for search
		title = strings.Replace(title, "／", "/", -1)
		query = append(query, fmt.Sprintf("title='%s'", title))
	}
	if directoriesOnly {
		query = append(query, fmt.Sprintf("mimeType='%s'", driveFolderType))
	}
	if filesOnly {
		query = append(query, fmt.Sprintf("mimeType!='%s'", driveFolderType))
	}
	list := f.svc.Files.List()
	if len(query) > 0 {
		list = list.Q(strings.Join(query, " and "))
		// fmt.Printf("list Query = %q\n", query)
	}
	if *driveListChunk > 0 {
		list = list.MaxResults(*driveListChunk)
	}
	if f.isTeamDrive {
		list.TeamDriveId(f.teamDriveID)
		list.SupportsTeamDrives(true)
		list.IncludeTeamDriveItems(true)
		list.Corpora("teamDrive")
	}

	var fields = partialFields

	if *driveAuthOwnerOnly {
		fields += ",owners"
	}

	fields = fmt.Sprintf("items(%s),nextPageToken", fields)

OUTER:
	for {
		var files *drive.FileList
		err = f.pacer.Call(func() (bool, error) {
			files, err = list.Fields(googleapi.Field(fields)).Do()
			return shouldRetry(err)
		})
		if err != nil {
			return false, errors.Wrap(err, "couldn't list directory")
		}
		for _, item := range files.Items {
			// Convert / to ／ for listing purposes
			item.Title = strings.Replace(item.Title, "/", "／", -1)
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

// Figure out if the user wants to use a team drive
func configTeamDrive(name string) error {
	teamDrive := fs.ConfigFileGet(name, "team_drive")
	if teamDrive == "" {
		fmt.Printf("Configure this as a team drive?\n")
	} else {
		fmt.Printf("Change current team drive ID %q?\n", teamDrive)
	}
	if !fs.Confirm() {
		return nil
	}
	client, err := authenticate(name)
	if err != nil {
		return errors.Wrap(err, "config team drive failed to authenticate")
	}
	svc, err := drive.New(client)
	if err != nil {
		return errors.Wrap(err, "config team drive failed to make drive client")
	}
	fmt.Printf("Fetching team drive list...\n")
	var driveIDs, driveNames []string
	listTeamDrives := svc.Teamdrives.List().MaxResults(100)
	for {
		var teamDrives *drive.TeamDriveList
		err = newPacer().Call(func() (bool, error) {
			teamDrives, err = listTeamDrives.Do()
			return shouldRetry(err)
		})
		if err != nil {
			return errors.Wrap(err, "list team drives failed")
		}
		for _, drive := range teamDrives.Items {
			driveIDs = append(driveIDs, drive.Id)
			driveNames = append(driveNames, drive.Name)
		}
		if teamDrives.NextPageToken == "" {
			break
		}
		listTeamDrives.PageToken(teamDrives.NextPageToken)
	}
	var driveID string
	if len(driveIDs) == 0 {
		fmt.Printf("No team drives found in your account")
	} else {
		driveID = fs.Choose("Enter a Team Drive ID", driveIDs, driveNames, true)
	}
	fs.ConfigFileSet(name, "team_drive", driveID)
	return nil
}

// newPacer makes a pacer configured for drive
func newPacer() *pacer.Pacer {
	return pacer.New().SetMinSleep(minSleep).SetPacer(pacer.GoogleDrivePacer)
}

func getServiceAccountClient(keyJsonfilePath string) (*http.Client, error) {
	data, err := ioutil.ReadFile(os.ExpandEnv(keyJsonfilePath))
	if err != nil {
		return nil, errors.Wrap(err, "error opening credentials file")
	}
	conf, err := google.JWTConfigFromJSON(data, driveConfig.Scopes...)
	if err != nil {
		return nil, errors.Wrap(err, "error processing credentials")
	}
	ctxWithSpecialClient := oauthutil.Context(fs.Config.Client())
	return oauth2.NewClient(ctxWithSpecialClient, conf.TokenSource(ctxWithSpecialClient)), nil
}

func authenticate(name string) (*http.Client, error) {
	var oAuthClient *http.Client
	var err error

	serviceAccountPath := fs.ConfigFileGet(name, "service_account_file")
	if serviceAccountPath != "" {
		oAuthClient, err = getServiceAccountClient(serviceAccountPath)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to configure drive Service Account")
		}
	} else {
		oAuthClient, _, err = oauthutil.NewClient(name, driveConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to configure drive")
		}
	}

	return oAuthClient, nil
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, path string) (fs.Fs, error) {
	if !isPowerOfTwo(int64(chunkSize)) {
		return nil, errors.Errorf("drive: chunk size %v isn't a power of two", chunkSize)
	}
	if chunkSize < 256*1024 {
		return nil, errors.Errorf("drive: chunk size can't be less than 256k - was %v", chunkSize)
	}

	oAuthClient, _ := authenticate(name)

	root, err := parseDrivePath(path)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:  name,
		root:  root,
		pacer: newPacer(),
	}
	f.teamDriveID = fs.ConfigFileGet(name, "team_drive")
	f.isTeamDrive = f.teamDriveID != ""
	f.features = (&fs.Features{
		DuplicateFiles:          true,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
	}).Fill(f)

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
	// override root folder for a team drive
	if f.isTeamDrive {
		f.about.RootFolderId = f.teamDriveID
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
	found, err = f.list(pathID, leaf, true, false, false, func(item *drive.File) bool {
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
		info, err = f.svc.Files.Insert(createInfo).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
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
			fs.Debugf(filepath, "Unknown export type %q - ignoring", mimeType)
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

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	err = f.dirCache.FindRoot(false)
	if err != nil {
		return nil, err
	}
	directoryID, err := f.dirCache.FindDir(dir, false)
	if err != nil {
		return nil, err
	}

	var iErr error
	_, err = f.list(directoryID, "", false, false, false, func(item *drive.File) bool {
		remote := path.Join(dir, item.Title)
		switch {
		case *driveAuthOwnerOnly && !isAuthOwned(item):
			// ignore object or directory
		case item.MimeType == driveFolderType:
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, item.Id)
			when, _ := time.Parse(timeFormatIn, item.ModifiedDate)
			d := fs.NewDir(remote, when).SetID(item.Id)
			entries = append(entries, d)
		case item.Md5Checksum != "" || item.FileSize > 0:
			// If item has MD5 sum or a length it is a file stored on drive
			o, err := f.newObjectWithInfo(remote, item)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		case len(item.ExportLinks) != 0:
			// If item has export links then it is a google doc
			extension, link := f.findExportFormat(remote, item)
			if extension == "" {
				fs.Debugf(remote, "No export formats found")
			} else {
				o, err := f.newObjectWithInfo(remote+"."+extension, item)
				if err != nil {
					iErr = err
					return true
				}
				if !*driveSkipGdocs {
					obj := o.(*Object)
					obj.isDocument = true
					obj.url = link
					obj.bytes = -1
					entries = append(entries, o)
				} else {
					fs.Debugf(f, "Skip google document: %q", remote)
				}
			}
		default:
			fs.Debugf(remote, "Ignoring unknown object")
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

	leaf, directoryID, err := f.dirCache.FindRootAndPath(remote, true)
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
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	exisitingObj, err := f.newObjectWithInfo(src.Remote(), nil)
	switch err {
	case nil:
		return exisitingObj, exisitingObj.Update(in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(in, src, options...)
	default:
		return nil, err
	}
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(in, src, options...)
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
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
			info, err = f.svc.Files.Insert(createInfo).Media(in, googleapi.ContentType("")).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
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

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(dirs []fs.Directory) error {
	if len(dirs) < 2 {
		return nil
	}
	dstDir := dirs[0]
	for _, srcDir := range dirs[1:] {
		// list the the objects
		infos := []*drive.File{}
		_, err := f.list(srcDir.ID(), "", false, false, true, func(info *drive.File) bool {
			infos = append(infos, info)
			return false
		})
		if err != nil {
			return errors.Wrapf(err, "MergeDirs list failed on %v", srcDir)
		}
		// move them into place
		for _, info := range infos {
			fs.Infof(srcDir, "merging %q", info.Title)
			// Move the file into the destination
			info.Parents = []*drive.ParentReference{{Id: dstDir.ID()}}
			err = f.pacer.Call(func() (bool, error) {
				_, err = f.svc.Files.Patch(info.Id, info).SetModifiedDate(true).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
				return shouldRetry(err)
			})
			if err != nil {
				return errors.Wrapf(err, "MergDirs move failed on %q in %v", info.Title, srcDir)
			}
		}
		// rmdir (into trash) the now empty source directory
		err = f.rmdir(srcDir.ID(), true)
		if err != nil {
			fs.Infof(srcDir, "removing empty directory")
			return errors.Wrapf(err, "MergDirs move failed to rmdir %q", srcDir)
		}
	}
	return nil
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

// Rmdir deletes a directory unconditionally by ID
func (f *Fs) rmdir(directoryID string, useTrash bool) error {
	return f.pacer.Call(func() (bool, error) {
		var err error
		if useTrash {
			_, err = f.svc.Files.Trash(directoryID).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
		} else {
			err = f.svc.Files.Delete(directoryID).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
		}
		return shouldRetry(err)
	})
}

// Rmdir deletes a directory
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	root := path.Join(f.root, dir)
	dc := f.dirCache
	directoryID, err := dc.FindDir(dir, false)
	if err != nil {
		return err
	}
	var trashedFiles = false
	found, err := f.list(directoryID, "", false, false, true, func(item *drive.File) bool {
		if item.Labels == nil || !item.Labels.Trashed {
			fs.Debugf(dir, "Rmdir: contains file: %q", item.Title)
			return true
		}
		fs.Debugf(dir, "Rmdir: contains trashed file: %q", item.Title)
		trashedFiles = true
		return false
	})
	if err != nil {
		return err
	}
	if found {
		return errors.Errorf("directory not empty")
	}
	if root != "" {
		// trash the directory if it had trashed files
		// in or the user wants to trash, otherwise
		// delete it.
		err = f.rmdir(directoryID, trashedFiles || *driveUseTrash)
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
		fs.Debugf(src, "Can't copy - not same remote type")
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
		info, err = o.fs.svc.Files.Copy(srcObj.id, createInfo).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
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
			_, err = f.svc.Files.Trash(f.dirCache.RootID()).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
		} else {
			err = f.svc.Files.Delete(f.dirCache.RootID()).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
		}
		return shouldRetry(err)
	})
	f.dirCache.ResetRoot()
	if err != nil {
		return err
	}
	return nil
}

// CleanUp empties the trash
func (f *Fs) CleanUp() error {
	err := f.pacer.Call(func() (bool, error) {
		err := f.svc.Files.EmptyTrash().Do()
		return shouldRetry(err)
	})

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
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	if srcObj.isDocument {
		return nil, errors.New("can't move a Google document")
	}

	// Temporary Object under construction
	dstObj, dstInfo, err := f.createFileInfo(remote, srcObj.ModTime(), srcObj.bytes)
	if err != nil {
		return nil, err
	}

	// Do the move
	var info *drive.File
	err = f.pacer.Call(func() (bool, error) {
		info, err = f.svc.Files.Patch(srcObj.id, dstInfo).SetModifiedDate(true).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
		return shouldRetry(err)
	})
	if err != nil {
		return nil, err
	}

	dstObj.setMetaData(info)
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)

	// Refuse to move to or from the root
	if srcPath == "" || dstPath == "" {
		fs.Debugf(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
	}

	// find the root src directory
	err := srcFs.dirCache.FindRoot(false)
	if err != nil {
		return err
	}

	// find the root dst directory
	if dstRemote != "" {
		err = f.dirCache.FindRoot(true)
		if err != nil {
			return err
		}
	} else {
		if f.dirCache.FoundRoot() {
			return fs.ErrorDirExists
		}
	}

	// Find ID of dst parent, creating subdirs if necessary
	var leaf, directoryID string
	findPath := dstRemote
	if dstRemote == "" {
		findPath = f.root
	}
	leaf, directoryID, err = f.dirCache.FindPath(findPath, true)
	if err != nil {
		return err
	}

	// Check destination does not exist
	if dstRemote != "" {
		_, err = f.dirCache.FindDir(dstRemote, false)
		if err == fs.ErrorDirNotFound {
			// OK
		} else if err != nil {
			return err
		} else {
			return fs.ErrorDirExists
		}
	}

	// Find ID of src
	srcID, err := srcFs.dirCache.FindDir(srcRemote, false)
	if err != nil {
		return err
	}

	// Do the move
	patch := drive.File{
		Title:   leaf,
		Parents: []*drive.ParentReference{{Id: directoryID}},
	}
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.svc.Files.Patch(srcID, &patch).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(f.isTeamDrive).Do()
		return shouldRetry(err)
	})
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// DirChangeNotify polls for changes from the remote and hands the path to the
// given function. Only changes that can be resolved to a path through the
// DirCache will handled.
//
// Automatically restarts itself in case of unexpected behaviour of the remote.
//
// Close the returned channel to stop being notified.
func (f *Fs) DirChangeNotify(notifyFunc func(string), pollInterval time.Duration) chan bool {
	quit := make(chan bool)
	go func() {
		select {
		case <-quit:
			return
		default:
			for {
				f.dirchangeNotifyRunner(notifyFunc, pollInterval)
				fs.Debugf(f, "Notify listener service ran into issues, restarting shortly.")
				time.Sleep(pollInterval)
			}
		}
	}()
	return quit
}

func (f *Fs) dirchangeNotifyRunner(notifyFunc func(string), pollInterval time.Duration) {
	var err error
	var changeList *drive.ChangeList
	var pageToken string
	var largestChangeID int64

	var startPageToken *drive.StartPageToken
	err = f.pacer.Call(func() (bool, error) {
		startPageToken, err = f.svc.Changes.GetStartPageToken().SupportsTeamDrives(f.isTeamDrive).Do()
		return shouldRetry(err)
	})
	if err != nil {
		fs.Debugf(f, "Failed to get StartPageToken: %v", err)
		return
	}
	pageToken = startPageToken.StartPageToken

	for {
		fs.Debugf(f, "Checking for changes on remote")
		err = f.pacer.Call(func() (bool, error) {
			changesCall := f.svc.Changes.List().PageToken(pageToken).Fields(googleapi.Field("nextPageToken,largestChangeId,newStartPageToken,items(fileId,file/parents(id))"))
			if largestChangeID != 0 {
				changesCall = changesCall.StartChangeId(largestChangeID)
			}
			if *driveListChunk > 0 {
				changesCall = changesCall.MaxResults(*driveListChunk)
			}
			changeList, err = changesCall.SupportsTeamDrives(f.isTeamDrive).Do()
			return shouldRetry(err)
		})
		if err != nil {
			fs.Debugf(f, "Failed to get Changes: %v", err)
			return
		}

		pathsToClear := make([]string, 0)
		for _, change := range changeList.Items {
			if path, ok := f.dirCache.GetInv(change.FileId); ok {
				pathsToClear = append(pathsToClear, path)
			}

			if change.File != nil {
				for _, parent := range change.File.Parents {
					if path, ok := f.dirCache.GetInv(parent.Id); ok {
						pathsToClear = append(pathsToClear, path)
					}
				}
			}
		}
		lastNotifiedPath := ""
		sort.Strings(pathsToClear)
		for _, path := range pathsToClear {
			if lastNotifiedPath != "" && (path == lastNotifiedPath || strings.HasPrefix(path+"/", lastNotifiedPath)) {
				continue
			}
			lastNotifiedPath = path
			notifyFunc(path)
		}

		if changeList.LargestChangeId != 0 {
			largestChangeID = changeList.LargestChangeId
		}
		if changeList.NewStartPageToken != "" {
			pageToken = changeList.NewStartPageToken
			fs.Debugf(f, "All changes were processed. Waiting for more.")
			time.Sleep(pollInterval)
		} else if changeList.NextPageToken != "" {
			pageToken = changeList.NextPageToken
			fs.Debugf(f, "There are more changes pending, checking now.")
		} else {
			fs.Debugf(f, "Did not get any page token, something went wrong! %+v", changeList)
			return
		}
	}
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
			fs.Errorf(o, "Error reading size: %v", err)
			return 0
		}
		_ = res.Body.Close()
		o.bytes = res.ContentLength
		// fs.Debugf(o, "Read size of document: %v", o.bytes)
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

	leaf, directoryID, err := o.fs.dirCache.FindRootAndPath(o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}

	found, err := o.fs.list(directoryID, leaf, false, true, false, func(item *drive.File) bool {
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
		fs.Debugf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	modTime, err := time.Parse(timeFormatIn, o.modifiedDate)
	if err != nil {
		fs.Debugf(o, "Failed to read mtime from object: %v", err)
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
		info, err = o.fs.svc.Files.Update(o.id, updateInfo).SetModifiedDate(true).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(o.fs.isTeamDrive).Do()
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
		// fs.Debugf(file.o, "Updating size of doc after download to %v", file.bytes)
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
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
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
			info, err = o.fs.svc.Files.Update(updateInfo.Id, updateInfo).SetModifiedDate(true).Media(in, googleapi.ContentType("")).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(o.fs.isTeamDrive).Do()
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
			_, err = o.fs.svc.Files.Trash(o.id).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(o.fs.isTeamDrive).Do()
		} else {
			err = o.fs.svc.Files.Delete(o.id).Fields(googleapi.Field(partialFields)).SupportsTeamDrives(o.fs.isTeamDrive).Do()
		}
		return shouldRetry(err)
	})
	return err
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType() string {
	err := o.readMetaData()
	if err != nil {
		fs.Debugf(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.mimeType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs                = (*Fs)(nil)
	_ fs.Purger            = (*Fs)(nil)
	_ fs.CleanUpper        = (*Fs)(nil)
	_ fs.PutStreamer       = (*Fs)(nil)
	_ fs.Copier            = (*Fs)(nil)
	_ fs.Mover             = (*Fs)(nil)
	_ fs.DirMover          = (*Fs)(nil)
	_ fs.DirCacheFlusher   = (*Fs)(nil)
	_ fs.DirChangeNotifier = (*Fs)(nil)
	_ fs.PutUncheckeder    = (*Fs)(nil)
	_ fs.MergeDirser       = (*Fs)(nil)
	_ fs.Object            = (*Object)(nil)
	_ fs.MimeTyper         = &Object{}
)
