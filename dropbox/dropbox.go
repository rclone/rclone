// Dropbox interface
package dropbox

/*
Limitations of dropbox

File system is case insensitive
*/

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/oauthutil"
	"github.com/spf13/pflag"
	"github.com/stacktic/dropbox"
)

// Constants
const (
	rcloneAppKey    = "5jcck7diasz0rqy"
	rcloneAppSecret = "1n9m04y2zx7bf26"
	metadataLimit   = dropbox.MetadataLimitDefault // max items to fetch at once
)

var (
	// A regexp matching path names for files Dropbox ignores
	// See https://www.dropbox.com/en/help/145 - Ignored files
	ignoredFiles = regexp.MustCompile(`(?i)(^|/)(desktop\.ini|thumbs\.db|\.ds_store|icon\r|\.dropbox|\.dropbox.attr)$`)
	// Upload chunk size - setting too small makes uploads slow.
	// Chunks aren't buffered into memory though so can set large.
	uploadChunkSize    = fs.SizeSuffix(128 * 1024 * 1024)
	maxUploadChunkSize = fs.SizeSuffix(150 * 1024 * 1024)
)

// Register with Fs
func init() {
	fs.Register(&fs.FsInfo{
		Name:   "dropbox",
		NewFs:  NewFs,
		Config: configHelper,
		Options: []fs.Option{{
			Name: "app_key",
			Help: "Dropbox App Key - leave blank to use rclone's.",
		}, {
			Name: "app_secret",
			Help: "Dropbox App Secret - leave blank to use rclone's.",
		}},
	})
	pflag.VarP(&uploadChunkSize, "dropbox-chunk-size", "", fmt.Sprintf("Upload chunk size. Max %v.", maxUploadChunkSize))
}

// Configuration helper - called after the user has put in the defaults
func configHelper(name string) {
	// See if already have a token
	token := fs.ConfigFile.MustValue(name, "token")
	if token != "" {
		fmt.Printf("Already have a dropbox token - refresh?\n")
		if !fs.Confirm() {
			return
		}
	}

	// Get a dropbox
	db := newDropbox(name)

	// This method will ask the user to visit an URL and paste the generated code.
	if err := db.Auth(); err != nil {
		log.Fatalf("Failed to authorize: %v", err)
	}

	// Get the token
	token = db.AccessToken()

	// Stuff it in the config file if it has changed
	old := fs.ConfigFile.MustValue(name, "token")
	if token != old {
		fs.ConfigFile.SetValue(name, "token", token)
		fs.SaveConfig()
	}
}

// FsDropbox represents a remote dropbox server
type FsDropbox struct {
	name           string           // name of this remote
	db             *dropbox.Dropbox // the connection to the dropbox server
	root           string           // the path we are working on
	slashRoot      string           // root with "/" prefix, lowercase
	slashRootSlash string           // root with "/" prefix and postfix, lowercase
}

// FsObjectDropbox describes a dropbox object
type FsObjectDropbox struct {
	dropbox     *FsDropbox // what this object is part of
	remote      string     // The remote path
	bytes       int64      // size of the object
	modTime     time.Time  // time it was last modified
	hasMetadata bool       // metadata is valid
}

// ------------------------------------------------------------

// The name of the remote (as passed into NewFs)
func (f *FsDropbox) Name() string {
	return f.name
}

// The root of the remote (as passed into NewFs)
func (f *FsDropbox) Root() string {
	return f.root
}

// String converts this FsDropbox to a string
func (f *FsDropbox) String() string {
	return fmt.Sprintf("Dropbox root '%s'", f.root)
}

// Makes a new dropbox from the config
func newDropbox(name string) *dropbox.Dropbox {
	db := dropbox.NewDropbox()

	appKey := fs.ConfigFile.MustValue(name, "app_key")
	if appKey == "" {
		appKey = rcloneAppKey
	}
	appSecret := fs.ConfigFile.MustValue(name, "app_secret")
	if appSecret == "" {
		appSecret = rcloneAppSecret
	}

	db.SetAppInfo(appKey, appSecret)

	return db
}

// NewFs contstructs an FsDropbox from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	if uploadChunkSize > maxUploadChunkSize {
		return nil, fmt.Errorf("Chunk size too big, must be < %v", maxUploadChunkSize)
	}
	db := newDropbox(name)
	f := &FsDropbox{
		name: name,
		db:   db,
	}
	f.setRoot(root)

	// Read the token from the config file
	token := fs.ConfigFile.MustValue(name, "token")

	// Set our custom context which enables our custom transport for timeouts etc
	db.SetContext(oauthutil.Context())

	// Authorize the client
	db.SetAccessToken(token)

	// See if the root is actually an object
	entry, err := f.db.Metadata(f.slashRoot, false, false, "", "", metadataLimit)
	if err == nil && !entry.IsDir {
		remote := path.Base(f.root)
		newRoot := path.Dir(f.root)
		if newRoot == "." {
			newRoot = ""
		}
		f.setRoot(newRoot)
		obj := f.NewFsObject(remote)
		// return a Fs Limited to this object
		return fs.NewLimited(f, obj), nil
	}

	return f, nil
}

// Sets root in f
func (f *FsDropbox) setRoot(root string) {
	f.root = strings.Trim(root, "/")
	lowerCaseRoot := strings.ToLower(f.root)

	f.slashRoot = "/" + lowerCaseRoot
	f.slashRootSlash = f.slashRoot
	if lowerCaseRoot != "" {
		f.slashRootSlash += "/"
	}
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsDropbox) newFsObjectWithInfo(remote string, info *dropbox.Entry) fs.Object {
	o := &FsObjectDropbox{
		dropbox: f,
		remote:  remote,
	}
	if info != nil {
		o.setMetadataFromEntry(info)
	} else {
		err := o.readEntryAndSetMetadata()
		if err != nil {
			// logged already fs.Debug("Failed to read info: %s", err)
			return nil
		}
	}
	return o
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsDropbox) NewFsObject(remote string) fs.Object {
	return f.newFsObjectWithInfo(remote, nil)
}

// Strips the root off path and returns it
func (f *FsDropbox) stripRoot(path string) *string {
	lowercase := strings.ToLower(path)

	if !strings.HasPrefix(lowercase, f.slashRootSlash) {
		fs.Stats.Error()
		fs.ErrorLog(f, "Path '%s' is not under root '%s'", path, f.slashRootSlash)
		return nil
	}

	stripped := path[len(f.slashRootSlash):]
	return &stripped
}

// Walk the root returning a channel of FsObjects
func (f *FsDropbox) list(out fs.ObjectsChan) {
	// Track path component case, it could be different for entries coming from DropBox API
	// See https://www.dropboxforum.com/hc/communities/public/questions/201665409-Wrong-character-case-of-folder-name-when-calling-listFolder-using-Sync-API?locale=en-us
	// and https://github.com/ncw/rclone/issues/53
	nameTree := NewNameTree()
	cursor := ""
	for {
		deltaPage, err := f.db.Delta(cursor, f.slashRoot)
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't list: %s", err)
			break
		} else {
			if deltaPage.Reset && cursor != "" {
				fs.ErrorLog(f, "Unexpected reset during listing - try again")
				fs.Stats.Error()
				break
			}
			fs.Debug(f, "%d delta entries received", len(deltaPage.Entries))
			for i := range deltaPage.Entries {
				deltaEntry := &deltaPage.Entries[i]
				entry := deltaEntry.Entry
				if entry == nil {
					// This notifies of a deleted object
				} else {
					if len(entry.Path) <= 1 || entry.Path[0] != '/' {
						fs.Stats.Error()
						fs.ErrorLog(f, "dropbox API inconsistency: a path should always start with a slash and be at least 2 characters: %s", entry.Path)
						continue
					}

					lastSlashIndex := strings.LastIndex(entry.Path, "/")

					var parentPath string
					if lastSlashIndex == 0 {
						parentPath = ""
					} else {
						parentPath = entry.Path[1:lastSlashIndex]
					}
					lastComponent := entry.Path[lastSlashIndex+1:]

					if entry.IsDir {
						nameTree.PutCaseCorrectDirectoryName(parentPath, lastComponent)
					} else {
						parentPathCorrectCase := nameTree.GetPathWithCorrectCase(parentPath)
						if parentPathCorrectCase != nil {
							path := f.stripRoot(*parentPathCorrectCase + "/" + lastComponent)
							if path == nil {
								// an error occurred and logged by stripRoot
								continue
							}

							out <- f.newFsObjectWithInfo(*path, entry)
						} else {
							nameTree.PutFile(parentPath, lastComponent, entry)
						}
					}
				}
			}
			if !deltaPage.HasMore {
				break
			}
			cursor = deltaPage.Cursor.Cursor
		}
	}

	walkFunc := func(caseCorrectFilePath string, entry *dropbox.Entry) {
		path := f.stripRoot("/" + caseCorrectFilePath)
		if path == nil {
			// an error occurred and logged by stripRoot
			return
		}

		out <- f.newFsObjectWithInfo(*path, entry)
	}
	nameTree.WalkFiles(f.root, walkFunc)
}

// Walk the path returning a channel of FsObjects
func (f *FsDropbox) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		f.list(out)
	}()
	return out
}

// Walk the path returning a channel of FsObjects
func (f *FsDropbox) ListDir() fs.DirChan {
	out := make(fs.DirChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		entry, err := f.db.Metadata(f.root, true, false, "", "", metadataLimit)
		if err != nil {
			fs.Stats.Error()
			fs.ErrorLog(f, "Couldn't list directories in root: %s", err)
		} else {
			for i := range entry.Contents {
				entry := &entry.Contents[i]
				if entry.IsDir {
					name := f.stripRoot(entry.Path)
					if name == nil {
						// an error occurred and logged by stripRoot
						continue
					}

					out <- &fs.Dir{
						Name:  *name,
						When:  time.Time(entry.ClientMtime),
						Bytes: entry.Bytes,
						Count: -1,
					}
				}
			}
		}
	}()
	return out
}

// A read closer which doesn't close the input
type readCloser struct {
	in io.Reader
}

// Read bytes from the object - see io.Reader
func (rc *readCloser) Read(p []byte) (n int, err error) {
	return rc.in.Read(p)
}

// Dummy close function
func (rc *readCloser) Close() error {
	return nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *FsDropbox) Put(in io.Reader, remote string, modTime time.Time, size int64) (fs.Object, error) {
	// Temporary FsObject under construction
	o := &FsObjectDropbox{dropbox: f, remote: remote}
	return o, o.Update(in, modTime, size)
}

// Mkdir creates the container if it doesn't exist
func (f *FsDropbox) Mkdir() error {
	entry, err := f.db.Metadata(f.slashRoot, false, false, "", "", metadataLimit)
	if err == nil {
		if entry.IsDir {
			return nil
		}
		return fmt.Errorf("%q already exists as file", f.root)
	}
	_, err = f.db.CreateFolder(f.slashRoot)
	return err
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *FsDropbox) Rmdir() error {
	entry, err := f.db.Metadata(f.slashRoot, true, false, "", "", 16)
	if err != nil {
		return err
	}
	if len(entry.Contents) != 0 {
		return errors.New("Directory not empty")
	}
	return f.Purge()
}

// Return the precision
func (f *FsDropbox) Precision() time.Duration {
	return fs.ModTimeNotSupported
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
func (f *FsDropbox) Copy(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*FsObjectDropbox)
	if !ok {
		fs.Debug(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	// Temporary FsObject under construction
	dstObj := &FsObjectDropbox{dropbox: f, remote: remote}

	srcPath := srcObj.remotePath()
	dstPath := dstObj.remotePath()
	entry, err := f.db.Copy(srcPath, dstPath, false)
	if err != nil {
		return nil, fmt.Errorf("Copy failed: %s", err)
	}
	dstObj.setMetadataFromEntry(entry)
	return dstObj, nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *FsDropbox) Purge() error {
	// Let dropbox delete the filesystem tree
	_, err := f.db.Delete(f.slashRoot)
	return err
}

// ------------------------------------------------------------

// Return the parent Fs
func (o *FsObjectDropbox) Fs() fs.Fs {
	return o.dropbox
}

// Return a string version
func (o *FsObjectDropbox) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Return the remote path
func (o *FsObjectDropbox) Remote() string {
	return o.remote
}

// Md5sum returns the Md5sum of an object returning a lowercase hex string
func (o *FsObjectDropbox) Md5sum() (string, error) {
	return "", nil
}

// Size returns the size of an object in bytes
func (o *FsObjectDropbox) Size() int64 {
	return o.bytes
}

// setMetadataFromEntry sets the fs data from a dropbox.Entry
//
// This isn't a complete set of metadata and has an inacurate date
func (o *FsObjectDropbox) setMetadataFromEntry(info *dropbox.Entry) {
	o.bytes = info.Bytes
	o.modTime = time.Time(info.ClientMtime)
	o.hasMetadata = true
}

// Reads the entry from dropbox
func (o *FsObjectDropbox) readEntry() (*dropbox.Entry, error) {
	entry, err := o.dropbox.db.Metadata(o.remotePath(), false, false, "", "", metadataLimit)
	if err != nil {
		fs.Debug(o, "Error reading file: %s", err)
		return nil, fmt.Errorf("Error reading file: %s", err)
	}
	return entry, nil
}

// Read entry if not set and set metadata from it
func (o *FsObjectDropbox) readEntryAndSetMetadata() error {
	// Last resort set time from client
	if !o.modTime.IsZero() {
		return nil
	}
	entry, err := o.readEntry()
	if err != nil {
		return err
	}
	o.setMetadataFromEntry(entry)
	return nil
}

// Returns the remote path for the object
func (o *FsObjectDropbox) remotePath() string {
	return o.dropbox.slashRootSlash + o.remote
}

// Returns the key for the metadata database for a given path
func metadataKey(path string) string {
	// NB File system is case insensitive
	path = strings.ToLower(path)
	hash := md5.New()
	_, _ = hash.Write([]byte(path))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// Returns the key for the metadata database
func (o *FsObjectDropbox) metadataKey() string {
	return metadataKey(o.remotePath())
}

// readMetaData gets the info if it hasn't already been fetched
func (o *FsObjectDropbox) readMetaData() (err error) {
	if o.hasMetadata {
		return nil
	}
	// Last resort
	return o.readEntryAndSetMetadata()
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *FsObjectDropbox) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %s", err)
		return time.Now()
	}
	return o.modTime
}

// Sets the modification time of the local fs object
//
// Commits the datastore
func (o *FsObjectDropbox) SetModTime(modTime time.Time) {
	// FIXME not implemented
	return
}

// Is this object storable
func (o *FsObjectDropbox) Storable() bool {
	return true
}

// Open an object for read
func (o *FsObjectDropbox) Open() (in io.ReadCloser, err error) {
	in, _, err = o.dropbox.db.Download(o.remotePath(), "", 0)
	return
}

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *FsObjectDropbox) Update(in io.Reader, modTime time.Time, size int64) error {
	remote := o.remotePath()
	if ignoredFiles.MatchString(remote) {
		fs.ErrorLog(o, "File name disallowed - not uploading")
		return nil
	}
	entry, err := o.dropbox.db.UploadByChunk(ioutil.NopCloser(in), int(uploadChunkSize), remote, true, "")
	if err != nil {
		return fmt.Errorf("Upload failed: %s", err)
	}
	o.setMetadataFromEntry(entry)
	return nil
}

// Remove an object
func (o *FsObjectDropbox) Remove() error {
	_, err := o.dropbox.db.Delete(o.remotePath())
	return err
}

// Check the interfaces are satisfied
var _ fs.Fs = &FsDropbox{}
var _ fs.Copier = &FsDropbox{}
var _ fs.Purger = &FsDropbox{}
var _ fs.Object = &FsObjectDropbox{}
