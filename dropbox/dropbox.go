// Dropbox interface
package dropbox

/*
Limitations of dropbox

File system is case insensitive!

Can only have 25,000 objects in a directory

/delta might be more efficient than recursing with Metadata

Setting metadata is problematic!  Might have to use a database

Md5sum has to download the file
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
	"github.com/stacktic/dropbox"
)

// Constants
const (
	rcloneAppKey    = "5jcck7diasz0rqy"
	rcloneAppSecret = "1n9m04y2zx7bf26"
	uploadChunkSize = 64 * 1024                    // chunk size for upload
	metadataLimit   = dropbox.MetadataLimitDefault // max items to fetch at once
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
	db        *dropbox.Dropbox // the connection to the dropbox server
	root      string           // the path we are working on
	slashRoot string           // root with "/" prefix and postix
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

	return f, nil
}

// Return an FsObject from a path
func (f *FsDropbox) newFsObjectWithInfo(remote string, info *dropbox.Entry) (fs.Object, error) {
	fs := &FsObjectDropbox{
		dropbox: f,
		remote:  remote,
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
	return time.Second
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
	in, err := o.Open()
	if err != nil {
		return "", err
	}
	defer in.Close()
	hash := md5.New()
	_, err = io.Copy(hash, in)
	if err != nil {
		return "", err
	}
	o.md5sum = fmt.Sprintf("%x", hash.Sum(nil))
	return o.md5sum, nil
}

// Size returns the size of an object in bytes
func (o *FsObjectDropbox) Size() int64 {
	return o.bytes
}

// setMetaData sets the fs data from a dropbox.Entry
func (o *FsObjectDropbox) setMetaData(info *dropbox.Entry) {
	o.bytes = int64(info.Bytes)
	o.modTime = time.Time(info.ClientMtime)
}

// Returns the remote path for the object
func (o *FsObjectDropbox) remotePath() string {
	return o.dropbox.slashRoot + o.remote
}

// readMetaData gets the info if it hasn't already been fetched
func (o *FsObjectDropbox) readMetaData() (err error) {
	if !o.modTime.IsZero() {
		return nil
	}
	entry, err := o.dropbox.db.Metadata(o.remotePath(), false, false, "", "", metadataLimit)
	if err != nil {
		fs.Debug(o, "Couldn't find directory: %s", err)
		return fmt.Errorf("Couldn't find directory: %s", err)
	}
	o.setMetaData(entry)
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

// Sets the modification time of the local fs object
func (o *FsObjectDropbox) SetModTime(modTime time.Time) {
	err := o.readMetaData()
	if err != nil {
		fs.Stats.Error()
		fs.Log(o, "Failed to read metadata: %s", err)
		return
	}
	// fs.Stats.Error()
	fs.Log(o, "FIXME can't update dropbox mtime")
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
	rc := &readCloser{in: in}
	entry, err := o.dropbox.db.UploadByChunk(rc, uploadChunkSize, o.remotePath(), true, "")
	if err != nil {
		return fmt.Errorf("Upload failed: %s", err)
	}
	o.setMetaData(entry)
	return nil
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
