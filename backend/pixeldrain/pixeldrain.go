// Package pixeldrain provides an interface to the Pixeldrain object storage
// system.
package pixeldrain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

const timeFormat = time.RFC3339Nano

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "pixeldrain",
		Description: "Pixeldrain Filesystem",
		NewFs:       NewFs,
		Config:      nil,
		Options: []fs.Option{{
			Name: "api_key",
			Help: "API key for your pixeldrain account.\n" +
				"Found on https://pixeldrain.com/user/api_keys.",
			Sensitive: true,
		}, {
			Name: "directory_id",
			Help: "Root of the filesystem to use. Set to 'me' to use your personal filesystem.\n" +
				"Set to a shared directory ID to use a shared directory.",
			Default: "me",
		}, {
			Name: "api_url",
			Help: "The API endpoint to connect to. In the vast majority of cases it's fine to leave\n" +
				"this at default. It is only intended to be changed for testing purposes.",
			Default:  "https://pixeldrain.com/api",
			Advanced: true,
			Required: true,
		}},
		MetadataInfo: &fs.MetadataInfo{
			System: map[string]fs.MetadataHelp{
				"mode": {
					Help:    "File mode",
					Type:    "octal, unix style",
					Example: "755",
				},
				"mtime": {
					Help:    "Time of last modification",
					Type:    "RFC 3339",
					Example: timeFormat,
				},
				"btime": {
					Help:    "Time of file birth (creation)",
					Type:    "RFC 3339",
					Example: timeFormat,
				},
			},
			Help: "Pixeldrain supports file modes and creation times.",
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	APIKey      string `config:"api_key"`
	DirectoryID string `config:"directory_id"`
	APIURL      string `config:"api_url"`
}

// Fs represents a remote box
type Fs struct {
	name     string       // name of this remote, as given to NewFS
	root     string       // the path we are working on, as given to NewFS
	opt      Options      // parsed options
	features *fs.Features // optional features
	srv      *rest.Client // the connection to the server
	loggedIn bool         // if the user is authenticated

	// Pathprefix is the directory we're working in. The pathPrefix is stripped
	// from every API response containing a path. The pathPrefix always begins
	// and ends with a slash for concatenation convenience
	pathPrefix string
}

// Object describes a pixeldrain file
type Object struct {
	fs   *Fs            // what this object is part of
	base FilesystemNode // the node this object references
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		srv:  rest.NewClient(fshttp.NewClient(ctx)).SetErrorHandler(apiErrorHandler),
	}
	f.features = (&fs.Features{
		ReadMimeType:            true,
		CanHaveEmptyDirectories: true,
		ReadMetadata:            true,
		WriteMetadata:           true,
	}).Fill(ctx, f)

	// Set the path prefix. This is the path to the root directory on the
	// server. We add it to each request and strip it from each response because
	// rclone does not want to see it
	f.pathPrefix = "/" + path.Join(opt.DirectoryID, f.root) + "/"

	// The root URL equates to https://pixeldrain.com/api/filesystem during
	// normal operation. API handlers need to manually add the pathPrefix to
	// each request
	f.srv.SetRoot(opt.APIURL + "/filesystem")

	// If using an APIKey, set the Authorization header
	if len(opt.APIKey) > 1 {
		f.srv.SetUserPass("", opt.APIKey)

		// Check if credentials are correct
		user, err := f.userInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get user data: %w", err)
		}

		f.loggedIn = true

		fs.Infof(nil,
			"Logged in as '%s', subscription '%s', storage limit %d",
			user.Username, user.Subscription.Name, user.Subscription.StorageSpace,
		)
	}

	if !f.loggedIn && opt.DirectoryID == "me" {
		return nil, errors.New("authentication required: the 'me' directory can only be accessed while logged in")
	}

	// Satisfy TestFsIsFile. This test expects that we throw an error if the
	// filesystem root is a file
	fsp, err := f.stat(ctx, "")
	if err != errNotFound && err != nil {
		// It doesn't matter if the root directory does not exist, as long as it
		// is not a file. This is what the test dictates
		return f, err
	} else if err == nil && fsp.Base().Type == "file" {
		// The filesystem root is a file, rclone wants us to set the root to the
		// parent directory
		f.root = path.Dir(f.root)
		f.pathPrefix = "/" + path.Join(opt.DirectoryID, f.root) + "/"
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// =================================
// Implementation of fs.FS interface
// =================================
var _ fs.Fs = (*Fs)(nil)

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	fsp, err := f.stat(ctx, dir)
	if err == errNotFound {
		return nil, fs.ErrorDirNotFound
	} else if err != nil {
		return nil, err
	} else if fsp.Base().Type == "file" {
		return nil, fs.ErrorIsFile
	}

	entries = make(fs.DirEntries, len(fsp.Children))
	for i := range fsp.Children {
		if fsp.Children[i].Type == "dir" {
			entries[i] = f.nodeToDirectory(fsp.Children[i])
		} else {
			entries[i] = f.nodeToObject(fsp.Children[i])
		}
	}

	return entries, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fsp, err := f.stat(ctx, remote)
	if err == errNotFound {
		return nil, fs.ErrorObjectNotFound
	} else if err != nil {
		return nil, err
	} else if fsp.Base().Type == "dir" {
		return nil, fs.ErrorIsDir
	}
	return f.nodeToObject(fsp.Base()), nil
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	_, err := f.put(ctx, src.Remote(), in, options)
	if err != nil {
		return nil, fmt.Errorf("failed to put object: %w", err)
	}

	meta, err := fs.GetMetadataOptions(ctx, f, src, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata")
	}
	if meta == nil {
		meta = make(fs.Metadata)
	}

	// Overwrite the mtime if it was not already set in the metadata
	if _, ok := meta["mtime"]; !ok {
		meta["mtime"] = src.ModTime(ctx).Format(timeFormat)
	}

	// Can't set modtime in the same request as the original upload. So we send
	// this followup request
	fsp, err := f.update(ctx, src.Remote(), meta)
	if err != nil {
		return nil, err
	}

	return f.nodeToObject(fsp), nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	err = f.mkdir(ctx, dir)
	if err == errNotFound {
		return fs.ErrorDirNotFound
	} else if err == errExists {
		// Spec says we do not return an error if the directory already exists
		return nil
	}
	return err
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) (err error) {
	err = f.delete(ctx, dir, false)
	if err == errNotFound {
		return fs.ErrorDirNotFound
	}
	return err
}

// ===================================
// Implementation of fs.Info interface
// ===================================
var _ fs.Info = (*Fs)(nil)

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string { return fmt.Sprintf("pixeldrain root '%s'", f.root) }

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration { return time.Millisecond }

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.SHA256) }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// ====================================
// Implementation of fs.Purger interface
// ====================================
var _ fs.Purger = (*Fs)(nil)

// Purge all files in the directory specified
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) (err error) {
	err = f.delete(ctx, dir, true)
	if err == errNotFound {
		return fs.ErrorDirNotFound
	}
	return err
}

// ====================================
// Implementation of fs.Mover interface
// ====================================
var _ fs.Mover = (*Fs)(nil)

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		// This is not a pixeldrain object. Can't move
		return nil, fs.ErrorCantMove
	}

	err := f.rename(ctx, srcObj.fs, srcObj.base.Path, remote)
	if err == errIncompatibleSourceFS {
		return nil, fs.ErrorCantMove
	} else if err == errNotFound {
		return nil, fs.ErrorObjectNotFound
	}

	// The node only got its path changed, so we just copy the original Object
	// and change the path variable. This prevents us from having to make
	// another request to get the updated node
	var nodeCopy = srcObj.base
	nodeCopy.Path = remote
	return f.nodeToObject(nodeCopy), err
}

// =======================================
// Implementation of fs.DirMover interface
// =======================================
var _ fs.DirMover = (*Fs)(nil)

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) (err error) {
	err = f.rename(ctx, src, srcRemote, dstRemote)
	if err == errIncompatibleSourceFS {
		return fs.ErrorCantDirMove
	} else if err == errNotFound {
		return fs.ErrorDirNotFound
	} else if err == errExists {
		return fs.ErrorDirExists
	}
	return err
}

// =============================================
// Implementation of fs.ChangeNotifier interface
// =============================================
var _ fs.ChangeNotifier = (*Fs)(nil)

// ChangeNotify calls the passed function with a path
// that has had changes. If the implementation
// uses polling, it should adhere to the given interval.
// At least one value will be written to the channel,
// specifying the initial value and updated values might
// follow. A 0 Duration should pause the polling.
// The ChangeNotify implementation must empty the channel
// regularly. When the channel gets closed, the implementation
// should stop polling and release resources.
func (f *Fs) ChangeNotify(ctx context.Context, notify func(string, fs.EntryType), newInterval <-chan time.Duration) {
	// If the bucket ID is not /me we need to explicitly enable change logging
	// for this directory or file
	if f.pathPrefix != "/me/" {
		_, err := f.update(ctx, "", fs.Metadata{"logging_enabled": "true"})
		if err != nil {
			fs.Errorf(f, "Failed to set up change logging for path '%s': %s", f.pathPrefix, err)
		}
	}

	go f.changeNotify(ctx, notify, newInterval)
}
func (f *Fs) changeNotify(ctx context.Context, notify func(string, fs.EntryType), newInterval <-chan time.Duration) {
	var ticker = time.NewTicker(<-newInterval)
	var lastPoll = time.Now()

	for {
		select {
		case dur, ok := <-newInterval:
			if !ok {
				ticker.Stop()
				return
			}

			fs.Debugf(f, "Polling changes at an interval of %s", dur)
			ticker.Reset(dur)

		case t := <-ticker.C:
			clog, err := f.changeLog(ctx, lastPoll, t)
			if err != nil {
				fs.Errorf(f, "Failed to get change log for path '%s': %s", f.pathPrefix, err)
				continue
			}

			fs.Debugf(f, "Received %d changes for directory %s", len(clog), f.pathPrefix)

			for i := range clog {
				if clog[i].Type == "dir" {
					notify(strings.TrimPrefix(clog[i].Path, "/"), fs.EntryDirectory)
				} else if clog[i].Type == "file" {
					notify(strings.TrimPrefix(clog[i].Path, "/"), fs.EntryObject)
				}
			}

			lastPoll = t
		}
	}
}

// ==========================================
// Implementation of fs.PutStreamer interface
// ==========================================
var _ fs.PutStreamer = (*Fs)(nil)

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Put already supports streaming so we just use that
	return f.Put(ctx, in, src, options...)
}

// ===========================================
// Implementation of fs.PublicLinker interface
// ===========================================
var _ fs.PublicLinker = (*Fs)(nil)

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	fsn, err := f.update(ctx, remote, fs.Metadata{"shared": strconv.FormatBool(!unlink)})
	if err != nil {
		return "", err
	}
	if fsn.ID != "" {
		return strings.Replace(f.opt.APIURL, "/api", "/d/", 1) + fsn.ID, nil
	}
	return "", nil
}

// ======================================
// Implementation of fs.Abouter interface
// ======================================
var _ fs.Abouter = (*Fs)(nil)

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	user, err := f.userInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info: %w", err)
	}

	if user.Subscription.StorageSpace == -1 {
		user.Subscription.StorageSpace = 1e15 // 1 PB
	}

	return &fs.Usage{
		Total: fs.NewUsageValue(user.Subscription.StorageSpace),
		Used:  fs.NewUsageValue(user.StorageSpaceUsed),
	}, nil
}

// =====================================
// Implementation of fs.Object interface
// =====================================
var _ fs.Object = (*Object)(nil)

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) (err error) {
	_, err = o.fs.update(ctx, o.base.Path, fs.Metadata{"mtime": modTime.Format(timeFormat)})
	if err == nil {
		o.base.Modified = modTime
	}
	return err
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	return o.fs.read(ctx, o.base.Path, options)
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	// Copy the parameters and update the object
	o.base.Modified = src.ModTime(ctx)
	o.base.FileSize = src.Size()
	o.base.SHA256Sum, _ = src.Hash(ctx, hash.SHA256)
	_, err = o.fs.Put(ctx, in, o, options...)
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.delete(ctx, o.base.Path, false)
}

// =========================================
// Implementation of fs.ObjectInfo interface
// =========================================
var _ fs.ObjectInfo = (*Object)(nil)

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the SHA-256 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA256 {
		return "", hash.ErrUnsupported
	}
	return o.base.SHA256Sum, nil
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// =======================================
// Implementation of fs.DirEntry interface
// =======================================
var _ fs.DirEntry = (*Object)(nil)

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.base.Path
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.base.Path
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.base.Modified
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.base.FileSize
}

// ========================================
// Implementation of fs.MimeTyper interface
// ========================================
var _ fs.MimeTyper = (*Object)(nil)

// MimeType returns the content type of the Object if known, or "" if not
func (o *Object) MimeType(ctx context.Context) string {
	return o.base.FileType
}

// =========================================
// Implementation of fs.Metadataer interface
// =========================================
var _ fs.Metadataer = (*Object)(nil)

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *Object) Metadata(ctx context.Context) (fs.Metadata, error) {
	return fs.Metadata{
		"mode":  o.base.ModeOctal,
		"mtime": o.base.Modified.Format(timeFormat),
		"btime": o.base.Created.Format(timeFormat),
	}, nil
}
