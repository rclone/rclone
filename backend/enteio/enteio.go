// Package enteio provides an interface to Ente.io encrypted cloud storage
package enteio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential

	defaultEndpoint = "https://api.ente.io"
)

var (
	errCanNotUploadFileWithUnknownSize = errors.New("ente.io can't upload files with unknown size")
	errCanNotPurgeRootDirectory        = errors.New("can't purge root directory")
	errNotImplemented                  = errors.New("operation not implemented")
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "enteio",
		Description: "Ente.io encrypted cloud storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "email",
			Help:     "The email address of your Ente account.",
			Required: true,
		}, {
			Name:       "password",
			Help:       "The password for your Ente account.",
			Required:   true,
			IsPassword: true,
		}, {
			Name: "endpoint",
			Help: `The API endpoint to use.

Default is the production Ente API server. Self-hosted users should
set this to their own server URL.`,
			Default:  defaultEndpoint,
			Advanced: true,
		}, {
			Name:      "auth_token",
			Help:      "Authentication token (internal use only).",
			Required:  false,
			Advanced:  true,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:      "key_encryption_key",
			Help:      "Key encryption key (internal use only).",
			Required:  false,
			Advanced:  true,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeLeftSpace |
				encoder.EncodeRightSpace),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Email            string               `config:"email"`
	Password         string               `config:"password"`
	Endpoint         string               `config:"endpoint"`
	AuthToken        string               `config:"auth_token"`
	KeyEncryptionKey string               `config:"key_encryption_key"`
	Enc              encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote Ente.io server
type Fs struct {
	name     string       // name of this remote
	root     string       // the path we are working on
	opt      Options      // parsed config options
	features *fs.Features // optional features
	pacer    *fs.Pacer    // pacer for API calls
	client   *Client      // Ente API client
	dirCache *dircache.DirCache
	rootID   string // ID of the root collection
}

// Object describes an Ente.io object
type Object struct {
	fs           *Fs       // what this object is part of
	remote       string    // remote path
	size         int64     // size of the object
	modTime      time.Time // modification time
	id           string    // ID of the file
	collectionID string    // ID of the collection this file belongs to
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.opt.Enc.ToStandardPath(f.root)
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("ente.io root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// sanitizePath cleans up a path for internal use
func (f *Fs) sanitizePath(_path string) string {
	_path = path.Clean(_path)
	if _path == "." || _path == "/" {
		return ""
	}
	return f.opt.Enc.FromStandardPath(_path)
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if opt.Password != "" {
		var err error
		opt.Password, err = obscure.Reveal(opt.Password)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt password: %w", err)
		}
	}

	root = strings.Trim(root, "/")

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		// Ente uses E2E encryption, so we can't do server-side operations
		NoMultiThreading: true,
	}).Fill(ctx, f)

	// Create API client
	f.client = NewClient(opt.Endpoint, opt.Email, opt.Password)

	// Try to authenticate
	if opt.AuthToken != "" {
		f.client.SetAuthToken(opt.AuthToken)
	} else {
		err = f.client.Authenticate(ctx)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		// Save the token
		m.Set("auth_token", f.client.GetAuthToken())
	}

	// Get root collection ID
	collections, err := f.client.GetCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get collections: %w", err)
	}

	// Find the root collection or use the first one
	if len(collections) > 0 {
		f.rootID = collections[0].ID
	} else {
		f.rootID = "root"
	}

	// Set up directory cache
	root = f.sanitizePath(root)
	f.dirCache = dircache.New(root, f.rootID, f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		if err != fs.ErrorDirNotFound {
			return nil, fmt.Errorf("couldn't initialize root: %w", err)
		}

		// Check if root might be a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.rootID, &tempF)
		tempF.root = newRoot
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			return f, nil
		}
		_, err := tempF.newObject(ctx, remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// newObject creates a new object from remote path
func (f *Fs) newObject(ctx context.Context, remote string) (*Object, error) {
	leaf, dirID, err := f.dirCache.FindPath(ctx, f.sanitizePath(remote), false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	// Get files in the collection
	files, err := f.client.GetFilesInCollection(ctx, dirID)
	if err != nil {
		return nil, err
	}

	// Find the file
	for _, file := range files {
		if file.Name == leaf {
			return &Object{
				fs:           f,
				remote:       remote,
				size:         file.Size,
				modTime:      file.ModTime,
				id:           file.ID,
				collectionID: dirID,
			}, nil
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObject(ctx, remote)
}

// List the objects and directories in dir
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	dirID, err := f.dirCache.FindDir(ctx, f.sanitizePath(dir), false)
	if err != nil {
		return nil, err
	}

	entries := make(fs.DirEntries, 0)

	// Get collections (subdirectories)
	collections, err := f.client.GetCollections(ctx)
	if err != nil {
		return nil, err
	}

	for _, coll := range collections {
		if coll.ParentID == dirID {
			remote := path.Join(dir, f.opt.Enc.ToStandardName(coll.Name))
			f.dirCache.Put(remote, coll.ID)
			d := fs.NewDir(remote, coll.ModTime).SetID(coll.ID)
			entries = append(entries, d)
		}
	}

	// Get files
	files, err := f.client.GetFilesInCollection(ctx, dirID)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(file.Name))
		o := &Object{
			fs:           f,
			remote:       remote,
			size:         file.Size,
			modTime:      file.ModTime,
			id:           file.ID,
			collectionID: dirID,
		}
		entries = append(entries, o)
	}

	return entries, nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (string, bool, error) {
	collections, err := f.client.GetCollections(ctx)
	if err != nil {
		return "", false, err
	}

	for _, coll := range collections {
		if coll.ParentID == pathID && coll.Name == leaf {
			return coll.ID, true, nil
		}
	}

	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (string, error) {
	collection, err := f.client.CreateCollection(ctx, leaf, pathID)
	if err != nil {
		return "", err
	}
	return collection.ID, nil
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	size := src.Size()
	if size < 0 {
		return nil, errCanNotUploadFileWithUnknownSize
	}

	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		remote := src.Remote()
		modTime := src.ModTime(ctx)
		obj, err := f.createObject(ctx, remote, modTime, size)
		if err != nil {
			return nil, err
		}
		return obj, obj.Update(ctx, in, src, options...)
	default:
		return nil, err
	}
}

// createObject creates a new object
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (*Object, error) {
	_, collectionID, err := f.dirCache.FindPath(ctx, f.sanitizePath(remote), true)
	if err != nil {
		return nil, err
	}

	return &Object{
		fs:           f,
		remote:       remote,
		size:         size,
		modTime:      modTime,
		collectionID: collectionID,
	}, nil
}

// Mkdir creates the directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, f.sanitizePath(dir), true)
	return err
}

// Rmdir removes the directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	dirID, err := f.dirCache.FindDir(ctx, f.sanitizePath(dir), false)
	if err != nil {
		return err
	}

	// Check if directory is empty
	files, err := f.client.GetFilesInCollection(ctx, dirID)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	err = f.client.DeleteCollection(ctx, dirID)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(f.sanitizePath(dir))
	return nil
}

// Purge all files in the directory
func (f *Fs) Purge(ctx context.Context, dir string) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errCanNotPurgeRootDirectory
	}

	dirID, err := f.dirCache.FindDir(ctx, f.sanitizePath(dir), false)
	if err != nil {
		return err
	}

	err = f.client.DeleteCollection(ctx, dirID)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// DirCacheFlush resets the directory cache
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Object methods

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

// Hash returns the hash of an object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open opens the file for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	return o.fs.client.DownloadFile(ctx, o.id, o.collectionID)
}

// Update the object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	leaf := path.Base(o.remote)
	size := src.Size()
	modTime := src.ModTime(ctx)

	fileInfo, err := o.fs.client.UploadFile(ctx, o.collectionID, leaf, in, size, modTime)
	if err != nil {
		return err
	}

	o.id = fileInfo.ID
	o.size = fileInfo.Size
	o.modTime = fileInfo.ModTime
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.client.DeleteFile(ctx, o.id, o.collectionID)
}

// ID returns the ID of the Object
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
