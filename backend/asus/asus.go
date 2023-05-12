package asus

// Package skeleton provides an dummy interface to cloud storage.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"time"

	"github.com/rclone/rclone/backend/asus/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "asus",
		Description: "ASUS WebStorage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "userid",
			Help:     "Username (ASUS Cloud ID)",
			Required: true,
		}, {
			Name:       "password",
			Help:       "Password",
			Required:   true,
			IsPassword: true,
		}, {
			Name:     "root_folder_id",
			Help:     "Root folder ID",
			Default:  "",
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Userid       string `config:"userid"`
	Password     string `config:"password"`
	RootFolderId string `config:"root_folder_id"`
}

// Fs represents a remote mega
type Fs struct {
	name     string             // name of this remote
	root     string             // the path we are working on
	opt      Options            // parsed config options
	ci       *fs.ConfigInfo     // global config
	features *fs.Features       // optional features
	srv      api.AsusAPI        // the connection to the server
	pacer    *fs.Pacer          // pacer for API calls
	dirCache *dircache.DirCache // Map of directory path to directory id
}

// Object describes an object
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	createdTime time.Time
	id          string // ID of the object
}

//------------------------------------------------------------------------------

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
	return fmt.Sprintf("root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
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

	ci := fs.GetConfig(ctx)

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		ci:    ci,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	var srv api.AsusAPI
	srv, err = api.NewAsusAPI(ctx, opt.Userid, opt.Password, f.pacer)
	if err != nil {
		fmt.Println(err)
		return f, err
	}

	f.srv = srv

	f.features = (&fs.Features{
		CaseInsensitive:         true, // has case insensitive files
		CanHaveEmptyDirectories: true, // can have empty directories
	}).Fill(ctx, f)

	rootFolderId := opt.RootFolderId
	// -5 -  top-level in which MySyncFolder is created
	// 0  - cloud-only files
	if rootFolderId == "" {
		// Get rootFolderID
		rootID, err := f.srv.GetMySyncFolder(ctx)
		if err != nil {
			fmt.Println("Cannot find root folder")
			return f, err
		}
		rootFolderId = rootID.Id
	}

	f.dirCache = dircache.New(root, rootFolderId, f)

	return f, nil
}

//------------------------------------------------------------------------------

// CleanUp deletes all files currently in trash
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	return err
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {

	dir, file := path.Split(remote)
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	resp, err := f.srv.PropFind(ctx, file, directoryID, "system.file")
	if err != nil {
		return nil, err
	}
	if resp.Type == "system.notfound" {
		return nil, fs.ErrorObjectNotFound
	}

	o := &Object{
		fs:     f,
		remote: remote,
		id:     resp.Id,
		size:   int64(resp.Size),
	}

	return o, nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFolderFn func(api.Folder) bool
type listAllFileFn func(api.File) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, fnD listAllFolderFn, fnF listAllFileFn) (found bool, err error) {
	offset := 0
OUTER:
	for {
		resp, err := f.srv.BrowseFolder(ctx, dirID, offset, 200)

		if err != nil {
			return found, fmt.Errorf("couldn't list files: %w", err)
		}
		if len(resp.File) == 0 && len(resp.Folder) == 0 {
			break
		}

		if !filesOnly {
			for _, item := range resp.Folder {
				if fnD(item) {
					found = true
					break OUTER
				}
			}
		}

		if !directoriesOnly {
			for _, item := range resp.File {
				if fnF(item) {
					found = true
					break OUTER
				}
			}
		}

		offset += 1
	}
	return
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

func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, false, false,
		func(info api.Folder) bool {
			remote := path.Join(dir, info.RawFolderName)
			f.dirCache.Put(remote, info.Id)

			created, err := time.Parse("2006-01-02 15:04:05.999", info.CreatedTime)
			if err != nil {
				fmt.Println("Error parse time")
			}
			d := fs.NewDir(remote, created)
			entries = append(entries, d)
			return false
		},
		func(info api.File) bool {
			remote := path.Join(dir, info.RawFileName)
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
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

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info api.File) (fs.Object, error) {
	createdTime, _ := time.Parse("2006-01-02 15:04:05.999", info.CreatedTime)
	modTimestamp, _ := strconv.ParseInt(info.LastModifiedTime, 10, 64)
	modTime := time.Unix(modTimestamp, 0)
	o := &Object{
		fs:          f,
		remote:      remote,
		size:        int64(info.Size),
		createdTime: createdTime,
		modTime:     modTime,
		id:          info.Id,
	}
	return o, nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	if leaf == "" {
		pathIDOut = pathID
		found = true
		return pathIDOut, found, nil
	}

	resp, err := f.srv.PropFind(ctx, leaf, pathID, "system.unknown")
	if err != nil {
		return pathIDOut, false, err
	}

	switch resp.Type {
	case "system.notfound":
		found = false
	case "system.folder":
		found = true
		pathIDOut = resp.Id
	case "system.file":
		found = true
		pathIDOut = resp.Id
	}

	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	resp, err := f.srv.CreateFolder(ctx, leaf, pathID, int(time.Now().Unix()))
	if err != nil {
		fmt.Printf("...Error %v\n", err)
		return "", err
	}
	return resp.Id, nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs, fs.Update(ctx, in, src, options...)
}

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists.
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs, nil
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	directoryID, ok := f.dirCache.Get(dir)
	if !ok {
		return fmt.Errorf("Cannot find ID for dir %s\n", dir)
	}
	_, err := f.srv.RemoveFolder(ctx, directoryID)
	if err != nil {
		return err
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

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
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	srcLeaf, srcParentID, err := f.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	dstLeaf, dstParentID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		return nil, err
	}

	needRename := srcLeaf != dstLeaf
	needMove := srcParentID != dstParentID

	// rename the leaf to a temporary name if we are moving to
	// another directory to make sure we don't overwrite something
	// in the source directory by accident
	if needRename && needMove {
		tmpLeaf := "rcloneTemp" + random.String(8)
		if _, err := f.srv.RenameFile(ctx, srcObj.id, tmpLeaf); err != nil {
			return nil, fmt.Errorf("move: pre move rename failed: %w", err)
		}
	}

	// do the move if required
	if needMove {
		_, err := f.srv.MoveFile(ctx, srcObj.id, dstParentID)
		if err != nil {
			return nil, err
		}
	}

	// rename the leaf to its final name
	if needRename {
		_, err := f.srv.RenameFile(ctx, srcObj.id, dstLeaf)
		if err != nil {
			return nil, err
		}
	}

	return srcObj, nil

}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}
	// same parent only need to rename
	if srcDirectoryID == dstDirectoryID {
		_, err = f.srv.RenameFolder(ctx, srcID, dstLeaf)
		return err
	}

	// do the move
	_, err = f.srv.MoveFolder(ctx, srcID, dstDirectoryID)
	if err != nil {
		return fmt.Errorf("couldn't dir move: %w", err)
	}

	// Can't copy and change name in one step so we have to check if we have
	// the correct name after copy
	if srcLeaf != dstLeaf {
		_, err = f.srv.RenameFolder(ctx, srcID, dstLeaf)
		if err != nil {
			return fmt.Errorf("dirmove: couldn't rename moved dir: %w", err)
		}
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// DirCacheFlush an optional interface to flush internal directory cache
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {

	info, err := f.srv.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	total := int64(info.Package.Capacity * 1024 * 1024)
	used := int64(info.UsedCapacity * 1024 * 1024)
	free := int64(info.FreeCapacity * 1024 * 1024)

	usage = &fs.Usage{
		Total: &total,
		Used:  &used,
		Free:  &free,
	}

	return usage, nil
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

// Hash returns the hashes of an object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	var start, end int64 = 0, o.size
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			start = x.Offset
		case *fs.RangeOption:
			if x.Start >= 0 {
				start = x.Start
				if x.End > 0 && x.End <= o.size {
					end = x.End
				}
			} else {
				start = o.size - x.End
			}
		default:
			if option.Mandatory() {
				fs.Logf(nil, "Unsupported mandatory option: %v", option)
			}
		}
	}

	resp, err := o.fs.srv.DirectDownload(ctx, o.id, start, end)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// get id of object if it exists
	id, _, err := o.fs.FindLeaf(ctx, directoryID, leaf)
	if err != nil {
		return err
	}

	timestamp := int(src.ModTime(ctx).Unix())
	resp1, err := o.fs.srv.InitBinaryUpload(ctx, directoryID, leaf, id, nil, &timestamp)
	if err != nil {
		fmt.Printf("Init upload error: %s\n", err)
		return err
	}

	_, err = o.fs.srv.ResumeBinaryUpload(ctx, resp1.TransactionId, 0, int(size), in)
	if err != nil {
		fmt.Printf("Resume upload error: %s\n", err)
		return err
	}

	resp3, err := o.fs.srv.FinishBinaryUpload(ctx, resp1.TransactionId, &resp1.LatestCheckSum)
	if err != nil {
		fmt.Printf("Finish upload error: %s\n", err)
		return err
	}

	fileId := resp3.FileId

	o.size = size
	o.createdTime = src.ModTime(ctx)
	o.modTime = src.ModTime(ctx)
	o.id = fileId

	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	_, err := o.fs.srv.RemoveFile(ctx, o.id)
	if err != nil {
		return err
	}
	return nil
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
