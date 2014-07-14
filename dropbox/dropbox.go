// Dropbox interface
package dropbox

/*
Limitations of dropbox

File system is case insensitive

The datastore is limited to 100,000 records which therefore is the
limit of the number of files that rclone can use on dropbox.

FIXME only open datastore if we need it?

FIXME Getting this sometimes
Failed to copy: Upload failed: invalid character '<' looking for beginning of value
This is a JSON decode error - from Update / UploadByChunk
- Caused by 500 error from dropbox
- See https://github.com/stacktic/dropbox/issues/1
- Possibly confusing dropbox with excess concurrency?
*/

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/stacktic/dropbox"
)

// Constants
const (
	rcloneAppKey     = "5jcck7diasz0rqy"
	rcloneAppSecret  = "1n9m04y2zx7bf26"
	uploadChunkSize  = 64 * 1024                    // chunk size for upload
	metadataLimit    = dropbox.MetadataLimitDefault // max items to fetch at once
	datastoreName    = "rclone"
	tableName        = "metadata"
	md5sumField      = "md5sum"
	mtimeField       = "mtime"
	maxCommitRetries = 5
	RFC3339In        = time.RFC3339
	RFC3339Out       = "2006-01-02T15:04:05.000000000Z07:00"
)

// Register with Fs
func init() {
	fs.Register(&fs.FsInfo{
		Name:   "dropbox",
		NewFs:  NewFs,
		Config: Config,
		Options: []fs.Option{{
			Name: "app_key",
			Help: "Dropbox App Key - leave blank to use rclone's.",
		}, {
			Name: "app_secret",
			Help: "Dropbox App Secret - leave blank to use rclone's.",
		}},
	})
}

// Configuration helper - called after the user has put in the defaults
func Config(name string) {
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
	db               *dropbox.Dropbox // the connection to the dropbox server
	root             string           // the path we are working on
	slashRoot        string           // root with "/" prefix
	slashRootSlash   string           // root with "/" prefix and postix
	datastoreManager *dropbox.DatastoreManager
	datastore        *dropbox.Datastore
	table            *dropbox.Table
	datastoreMutex   sync.Mutex // lock this when using the datastore
	datastoreErr     error      // pending errors on the datastore
}

// FsObjectDropbox describes a dropbox object
type FsObjectDropbox struct {
	dropbox *FsDropbox // what this object is part of
	remote  string     // The remote path
	md5sum  string     // md5sum of the object
	bytes   int64      // size of the object
	modTime time.Time  // time it was last modified
}

// ------------------------------------------------------------

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
	db := newDropbox(name)
	f := &FsDropbox{
		db: db,
	}
	f.setRoot(root)

	// Read the token from the config file
	token := fs.ConfigFile.MustValue(name, "token")

	// Authorize the client
	db.SetAccessToken(token)

	// Make a db to store rclone metadata in
	f.datastoreManager = db.NewDatastoreManager()

	// Open the datastore in the background
	go f.openDataStore()

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
	f.slashRoot = "/" + f.root
	f.slashRootSlash = f.slashRoot
	if f.root != "" {
		f.slashRootSlash += "/"
	}
}

// Opens the datastore in f
func (f *FsDropbox) openDataStore() {
	f.datastoreMutex.Lock()
	defer f.datastoreMutex.Unlock()
	fs.Debug(f, "Open rclone datastore")
	// Open the rclone datastore
	var err error
	f.datastore, err = f.datastoreManager.OpenDatastore(datastoreName)
	if err != nil {
		fs.Log(f, "Failed to open datastore: %v", err)
		f.datastoreErr = err
		return
	}

	// Get the table we are using
	f.table, err = f.datastore.GetTable(tableName)
	if err != nil {
		fs.Log(f, "Failed to open datastore table: %v", err)
		f.datastoreErr = err
		return
	}
	fs.Debug(f, "Open rclone datastore finished")
}

// Return an FsObject from a path
func (f *FsDropbox) newFsObjectWithInfo(remote string, info *dropbox.Entry) (fs.Object, error) {
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
			return nil, err
		}
	}
	return o, nil
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsDropbox) NewFsObjectWithInfo(remote string, info *dropbox.Entry) fs.Object {
	fs, _ := f.newFsObjectWithInfo(remote, info)
	// Errors have already been logged
	return fs
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func (f *FsDropbox) NewFsObject(remote string) fs.Object {
	return f.NewFsObjectWithInfo(remote, nil)
}

// Strips the root off entry and returns it
func (f *FsDropbox) stripRoot(entry *dropbox.Entry) string {
	path := entry.Path
	if strings.HasPrefix(path, f.slashRootSlash) {
		path = path[len(f.slashRootSlash):]
	}
	return path
}

// Walk the root returning a channel of FsObjects
func (f *FsDropbox) list(out fs.ObjectsChan) {
	cursor := ""
	for {
		deltaPage, err := f.db.Delta(cursor, f.slashRoot)
		if err != nil {
			fs.Stats.Error()
			fs.Log(f, "Couldn't list: %s", err)
			break
		} else {
			if deltaPage.Reset && cursor != "" {
				fs.Log(f, "Unexpected reset during listing - try again")
				fs.Stats.Error()
				break
			}
			fs.Debug(f, "%d delta entries received", len(deltaPage.Entries))
			for i := range deltaPage.Entries {
				deltaEntry := &deltaPage.Entries[i]
				entry := deltaEntry.Entry
				if entry == nil {
					// This notifies of a deleted object
					fs.Debug(f, "Deleting metadata for %q", deltaEntry.Path)
					key := metadataKey(deltaEntry.Path) // Path is lowercased
					err := f.deleteMetadata(key)
					if err != nil {
						fs.Debug(f, "Failed to delete metadata for %q", deltaEntry.Path)
						// Don't accumulate Error here
					}

				} else {
					if entry.IsDir {
						// ignore directories
					} else {
						path := f.stripRoot(entry)
						out <- f.NewFsObjectWithInfo(path, entry)
					}
				}
			}
			if !deltaPage.HasMore {
				break
			}
			cursor = deltaPage.Cursor
		}
	}
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
			fs.Log(f, "Couldn't list directories in root: %s", err)
		} else {
			for i := range entry.Contents {
				entry := &entry.Contents[i]
				if entry.IsDir {
					out <- &fs.Dir{
						Name:  f.stripRoot(entry),
						When:  time.Time(entry.ClientMtime),
						Bytes: int64(entry.Bytes),
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
func (fs *FsDropbox) Precision() time.Duration {
	return time.Nanosecond
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *FsDropbox) Purge() error {
	// Delete metadata first
	var wg sync.WaitGroup
	to_be_deleted := f.List()
	wg.Add(fs.Config.Transfers)
	for i := 0; i < fs.Config.Transfers; i++ {
		go func() {
			defer wg.Done()
			for dst := range to_be_deleted {
				o := dst.(*FsObjectDropbox)
				o.deleteMetadata()
			}
		}()
	}
	wg.Wait()

	// Let dropbox delete the filesystem tree
	_, err := f.db.Delete(f.slashRoot)
	return err
}

// Tries the transaction in fn then calls commit, repeating until retry limit
//
// Holds datastore mutex while in progress
func (f *FsDropbox) transaction(fn func() error) error {
	f.datastoreMutex.Lock()
	defer f.datastoreMutex.Unlock()
	if f.datastoreErr != nil {
		return f.datastoreErr
	}
	var err error
	for i := 1; i <= maxCommitRetries; i++ {
		err = fn()
		if err != nil {
			return err
		}

		err = f.datastore.Commit()
		if err == nil {
			break
		}
		fs.Debug(f, "Retrying transaction %d/%d", i, maxCommitRetries)
	}
	if err != nil {
		return fmt.Errorf("Failed to commit metadata changes: %s", err)
	}
	return nil
}

// Deletes the medadata associated with this key
func (f *FsDropbox) deleteMetadata(key string) error {
	return f.transaction(func() error {
		record, err := f.table.Get(key)
		if err != nil {
			return fmt.Errorf("Couldn't get record: %s", err)
		}
		if record == nil {
			return nil
		}
		record.DeleteRecord()
		return nil
	})
}

// Reads the record attached to key
//
// Holds datastore mutex while in progress
func (f *FsDropbox) readRecord(key string) (*dropbox.Record, error) {
	f.datastoreMutex.Lock()
	defer f.datastoreMutex.Unlock()
	if f.datastoreErr != nil {
		return nil, f.datastoreErr
	}
	return f.table.Get(key)
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
//
// FIXME has to download the file!
func (o *FsObjectDropbox) Md5sum() (string, error) {
	if o.md5sum != "" {
		return o.md5sum, nil
	}
	err := o.readMetaData()
	if err != nil {
		fs.Log(o, "Failed to read metadata: %s", err)
		return "", fmt.Errorf("Failed to read metadata: %s", err)

	}

	// For pre-existing files which have no md5sum can read it and set it?

	// in, err := o.Open()
	// if err != nil {
	// 	return "", err
	// }
	// defer in.Close()
	// hash := md5.New()
	// _, err = io.Copy(hash, in)
	// if err != nil {
	// 	return "", err
	// }
	// o.md5sum = fmt.Sprintf("%x", hash.Sum(nil))
	return o.md5sum, nil
}

// Size returns the size of an object in bytes
func (o *FsObjectDropbox) Size() int64 {
	return o.bytes
}

// setMetadataFromEntry sets the fs data from a dropbox.Entry
//
// This isn't a complete set of metadata and has an inacurate date
func (o *FsObjectDropbox) setMetadataFromEntry(info *dropbox.Entry) {
	o.bytes = int64(info.Bytes)
	o.modTime = time.Time(info.ClientMtime)
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
	return fmt.Sprintf("%x", md5.Sum([]byte(path)))
}

// Returns the key for the metadata database
func (o *FsObjectDropbox) metadataKey() string {
	return metadataKey(o.remotePath())
}

// readMetaData gets the info if it hasn't already been fetched
func (o *FsObjectDropbox) readMetaData() (err error) {
	if o.md5sum != "" {
		return nil
	}

	// fs.Debug(o, "Reading metadata from datastore")
	record, err := o.dropbox.readRecord(o.metadataKey())
	if err != nil {
		fs.Debug(o, "Couldn't read metadata: %s", err)
		record = nil
	}

	if record != nil {
		// Read md5sum
		md5sumInterface, ok, err := record.Get(md5sumField)
		if err != nil {
			return err
		}
		if !ok {
			fs.Debug(o, "Couldn't find md5sum in record")
		} else {
			md5sum, ok := md5sumInterface.(string)
			if !ok {
				fs.Debug(o, "md5sum not a string")
			} else {
				o.md5sum = md5sum
			}
		}

		// read mtime
		mtimeInterface, ok, err := record.Get(mtimeField)
		if err != nil {
			return err
		}
		if !ok {
			fs.Debug(o, "Couldn't find mtime in record")
		} else {
			mtime, ok := mtimeInterface.(string)
			if !ok {
				fs.Debug(o, "mtime not a string")
			} else {
				modTime, err := time.Parse(RFC3339In, mtime)
				if err != nil {
					return err
				}
				o.modTime = modTime
			}
		}
	}

	// Last resort
	o.readEntryAndSetMetadata()
	return nil
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

// Sets the modification time of the local fs object into the record
// FIXME if we don't set md5sum what will that do?
func (o *FsObjectDropbox) setModTimeAndMd5sum(modTime time.Time, md5sum string) error {
	key := o.metadataKey()
	// fs.Debug(o, "Writing metadata to datastore")
	return o.dropbox.transaction(func() error {
		record, err := o.dropbox.table.GetOrInsert(key)
		if err != nil {
			return fmt.Errorf("Couldn't read record: %s", err)
		}

		if md5sum != "" {
			err = record.Set(md5sumField, md5sum)
			if err != nil {
				return fmt.Errorf("Couldn't set md5sum record: %s", err)
			}
		}

		if !modTime.IsZero() {
			mtime := modTime.Format(RFC3339Out)
			err := record.Set(mtimeField, mtime)
			if err != nil {
				return fmt.Errorf("Couldn't set mtime record: %s", err)
			}
		}

		return nil
	})
}

// Deletes the medadata associated with this file
//
// It logs any errors
func (o *FsObjectDropbox) deleteMetadata() {
	fs.Debug(o, "Deleting metadata from datastore")
	err := o.dropbox.deleteMetadata(o.metadataKey())
	if err != nil {
		fs.Log(o, "Error deleting metadata: %v", err)
		fs.Stats.Error()
	}
}

// Sets the modification time of the local fs object
//
// Commits the datastore
func (o *FsObjectDropbox) SetModTime(modTime time.Time) {
	err := o.setModTimeAndMd5sum(modTime, "")
	if err != nil {
		fs.Stats.Error()
		fs.Log(o, err.Error())
	}
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
	// Calculate md5sum as we upload it
	hash := md5.New()
	rc := &readCloser{in: io.TeeReader(in, hash)}
	entry, err := o.dropbox.db.UploadByChunk(rc, uploadChunkSize, o.remotePath(), true, "")
	if err != nil {
		return fmt.Errorf("Upload failed: %s", err)
	}
	o.setMetadataFromEntry(entry)

	md5sum := fmt.Sprintf("%x", hash.Sum(nil))
	return o.setModTimeAndMd5sum(modTime, md5sum)
}

// Remove an object
func (o *FsObjectDropbox) Remove() error {
	o.deleteMetadata()
	_, err := o.dropbox.db.Delete(o.remotePath())
	return err
}

// Check the interfaces are satisfied
var _ fs.Fs = &FsDropbox{}
var _ fs.Purger = &FsDropbox{}
var _ fs.Object = &FsObjectDropbox{}
