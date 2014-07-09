// Dropbox interface
package dropbox

/*
Limitations of dropbox

File system is case insensitive!

Can only have 25,000 objects in a directory

/delta might be more efficient than recursing with Metadata

Setting metadata is problematic!  Might have to use a database

Md5sum has to download the file

FIXME do we need synchronisation for any of the dropbox calls?

FIXME need to delete metadata when we delete files!
*/

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/swift"
	"github.com/stacktic/dropbox"
)

// Constants
const (
	rcloneAppKey    = "5jcck7diasz0rqy"
	rcloneAppSecret = "1n9m04y2zx7bf26"
	uploadChunkSize = 64 * 1024                    // chunk size for upload
	metadataLimit   = dropbox.MetadataLimitDefault // max items to fetch at once
	datastoreName   = "rclone"
	tableName       = "metadata"
	md5sumField     = "md5sum"
	mtimeField      = "mtime"
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
	slashRoot        string           // root with "/" prefix and postix
	datastoreManager *dropbox.DatastoreManager
	datastore        *dropbox.Datastore
	table            *dropbox.Table
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

// parseParse parses a dropbox 'url'
func parseDropboxPath(path string) (root string, err error) {
	root = strings.Trim(path, "/")
	return
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
func NewFs(name, path string) (fs.Fs, error) {
	db := newDropbox(name)

	root, err := parseDropboxPath(path)
	if err != nil {
		return nil, err
	}
	slashRoot := "/" + root
	if root != "" {
		slashRoot += "/"
	}
	f := &FsDropbox{
		root:      root,
		slashRoot: slashRoot,
		db:        db,
	}

	// Read the token from the config file
	token := fs.ConfigFile.MustValue(name, "token")

	// Authorize the client
	db.SetAccessToken(token)

	// Make a db to store rclone metadata in
	f.datastoreManager = db.NewDatastoreManager()

	// Open the rclone datastore
	f.datastore, err = f.datastoreManager.OpenDatastore(datastoreName)
	if err != nil {
		return nil, err
	}

	// Get the table we are using
	f.table, err = f.datastore.GetTable(tableName)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// Return an FsObject from a path
func (f *FsDropbox) newFsObjectWithInfo(remote string, info *dropbox.Entry) (fs.Object, error) {
	o := &FsObjectDropbox{
		dropbox: f,
		remote:  remote,
	}
	if info == nil {
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
	if strings.HasPrefix(path, f.slashRoot) {
		path = path[len(f.slashRoot):]
	}
	return path
}

// Walk the path returning a channel of FsObjects
//
// FIXME could do this in parallel but needs to be limited to Checkers
func (f *FsDropbox) list(path string, out fs.ObjectsChan) {
	entry, err := f.db.Metadata(f.slashRoot+path, true, false, "", "", metadataLimit)
	if err != nil {
		fs.Stats.Error()
		fs.Log(f, "Couldn't list %q: %s", path, err)
	} else {
		for i := range entry.Contents {
			entry := &entry.Contents[i]
			path = f.stripRoot(entry)
			if entry.IsDir {
				f.list(path, out)
			} else {
				out <- f.NewFsObjectWithInfo(path, entry)
			}
		}
	}
}

// Walk the path returning a channel of FsObjects
func (f *FsDropbox) List() fs.ObjectsChan {
	out := make(fs.ObjectsChan, fs.Config.Checkers)
	go func() {
		defer close(out)
		f.list("", out)
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
	_, err := f.db.CreateFolder(f.slashRoot)
	return err
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *FsDropbox) Rmdir() error {
	entry, err := f.db.Metadata(f.slashRoot, true, false, "", "", metadataLimit)
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
// Returns an error if it isn't empty
func (f *FsDropbox) Purge() error {
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
	return o.dropbox.slashRoot + o.remote
}

// Returns the key for the metadata database
func (o *FsObjectDropbox) metadataKey() string {
	// FIXME lower case it?
	key := o.dropbox.slashRoot + o.remote
	return fmt.Sprintf("%x", md5.Sum([]byte(key)))
}

// readMetaData gets the info if it hasn't already been fetched
func (o *FsObjectDropbox) readMetaData() (err error) {
	if o.md5sum != "" {
		return nil
	}

	record, err := o.dropbox.table.Get(o.metadataKey())
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
				modTime, err := swift.FloatStringToTime(mtime)
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
	record, err := o.dropbox.table.GetOrInsert(o.metadataKey())
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
		mtime := swift.TimeToFloatString(modTime)
		err := record.Set(mtimeField, mtime)
		if err != nil {
			return fmt.Errorf("Couldn't set mtime record: %s", err)
		}
	}

	err = o.dropbox.datastore.Commit()
	if err != nil {
		return fmt.Errorf("Failed to commit metadata changes: %s", err)
	}
	return nil
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
	_, err := o.dropbox.db.Delete(o.remotePath())
	return err
}

// Check the interfaces are satisfied
var _ fs.Fs = &FsDropbox{}
var _ fs.Purger = &FsDropbox{}
var _ fs.Object = &FsObjectDropbox{}
