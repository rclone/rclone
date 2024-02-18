// Package quatrix provides an interface to the Quatrix by Maytech
// object storage system.
package quatrix

// FIXME Quatrix only supports file names of 255 characters or less. Names
// that will not be supported are those that contain non-printable
// ascii, / or \, names with trailing spaces, and the special names
// “.” and “..”.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/quatrix/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
	rootURL       = "https://%s/api/1.0/"
	uploadURL     = "https://%s/upload/chunked/"

	unlimitedUserQuota = -1
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "quatrix",
		Description: "Quatrix by Maytech",
		NewFs:       NewFs,
		Options: fs.Options{
			{
				Name:      "api_key",
				Help:      "API key for accessing Quatrix account",
				Required:  true,
				Sensitive: true,
			},
			{
				Name:     "host",
				Help:     "Host name of Quatrix account",
				Required: true,
			},
			{
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: encoder.Standard |
					encoder.EncodeBackSlash |
					encoder.EncodeInvalidUtf8,
			},
			{
				Name:     "effective_upload_time",
				Help:     "Wanted upload time for one chunk",
				Advanced: true,
				Default:  "4s",
			},
			{
				Name:     "minimal_chunk_size",
				Help:     "The minimal size for one chunk",
				Advanced: true,
				Default:  fs.SizeSuffix(10_000_000),
			},
			{
				Name:     "maximal_summary_chunk_size",
				Help:     "The maximal summary for all chunks. It should not be less than 'transfers'*'minimal_chunk_size'",
				Advanced: true,
				Default:  fs.SizeSuffix(100_000_000),
			},
			{
				Name:     "hard_delete",
				Help:     "Delete files permanently rather than putting them into the trash",
				Advanced: true,
				Default:  false,
			},
			{
				Name:     "skip_project_folders",
				Help:     "Skip project folders in operations",
				Advanced: true,
				Default:  false,
			},
		},
	})
}

// Options defines the configuration for Quatrix backend
type Options struct {
	APIKey                  string               `config:"api_key"`
	Host                    string               `config:"host"`
	Enc                     encoder.MultiEncoder `config:"encoding"`
	EffectiveUploadTime     fs.Duration          `config:"effective_upload_time"`
	MinimalChunkSize        fs.SizeSuffix        `config:"minimal_chunk_size"`
	MaximalSummaryChunkSize fs.SizeSuffix        `config:"maximal_summary_chunk_size"`
	HardDelete              bool                 `config:"hard_delete"`
	SkipProjectFolders      bool                 `config:"skip_project_folders"`
}

// Fs represents remote Quatrix fs
type Fs struct {
	name                string
	root                string
	description         string
	features            *fs.Features
	opt                 Options
	ci                  *fs.ConfigInfo
	srv                 *rest.Client // the connection to the quatrix server
	pacer               *fs.Pacer    // pacer for API calls
	dirCache            *dircache.DirCache
	uploadMemoryManager *UploadMemoryManager
}

// Object describes a quatrix object
type Object struct {
	fs          *Fs
	remote      string
	size        int64
	modTime     time.Time
	id          string
	hasMetaData bool
	obType      string
}

// trimPath trims redundant slashes from quatrix 'url'
func trimPath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}

	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)

	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// http client
	client := fshttp.NewClient(ctx)

	// since transport is a global variable that is initialized only once (due to sync.Once)
	// we need to reset it to have correct transport per each client (with proper values extracted from rclone config)
	client.Transport = fshttp.NewTransportCustom(ctx, nil)

	root = trimPath(root)

	ci := fs.GetConfig(ctx)

	f := &Fs{
		name:        name,
		description: "Quatrix FS for account " + opt.Host,
		root:        root,
		opt:         *opt,
		ci:          ci,
		srv:         rest.NewClient(client).SetRoot(fmt.Sprintf(rootURL, opt.Host)),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		CanHaveEmptyDirectories: true,
		PartialUploads:          true,
	}).Fill(ctx, f)

	if f.opt.APIKey != "" {
		f.srv.SetHeader("Authorization", "Bearer "+f.opt.APIKey)
	}

	f.uploadMemoryManager = NewUploadMemoryManager(f.ci, &f.opt)

	// get quatrix root(home) id
	rootID, found, err := f.fileID(ctx, "", "")
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, errors.New("root not found")
	}

	f.dirCache = dircache.New(root, rootID.FileID, f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		fileID, found, err := f.fileID(ctx, "", root)
		if err != nil {
			return nil, fmt.Errorf("find root %s: %w", root, err)
		}

		if !found {
			return f, nil
		}

		if fileID.IsFile() {
			root, _ = dircache.SplitPath(root)
			f.dirCache = dircache.New(root, rootID.FileID, f)

			return f, fs.ErrorIsFile
		}
	}

	return f, nil
}

// fileID gets id, parent and type of path in given parentID
func (f *Fs) fileID(ctx context.Context, parentID, path string) (result *api.FileInfo, found bool, err error) {
	opts := rest.Opts{
		Method:       "POST",
		Path:         "file/id",
		IgnoreStatus: true,
	}

	payload := api.FileInfoParams{
		Path:     f.opt.Enc.FromStandardPath(path),
		ParentID: parentID,
	}

	result = &api.FileInfo{}

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, payload, result)
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to get file id: %w", err)
	}

	if result.FileID == "" {
		return nil, false, nil
	}

	return result, true, nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (folderID string, found bool, err error) {
	result, found, err := f.fileID(ctx, pathID, leaf)
	if err != nil {
		return "", false, fmt.Errorf("find leaf: %w", err)
	}

	if !found {
		return "", false, nil
	}

	if result.IsFile() {
		return "", false, nil
	}

	return result.FileID, true, nil
}

// createDir creates directory in pathID with name leaf
//
// resolve - if true will resolve name conflict on server side, if false - will return error if object with this name exists
func (f *Fs) createDir(ctx context.Context, pathID, leaf string, resolve bool) (newDir *api.File, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "file/makedir",
	}

	payload := api.CreateDirParams{
		Name:    f.opt.Enc.FromStandardName(leaf),
		Target:  pathID,
		Resolve: resolve,
	}

	newDir = &api.File{}

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, payload, newDir)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (dirID string, err error) {
	dir, err := f.createDir(ctx, pathID, leaf, false)
	if err != nil {
		return "", err
	}

	return dir.ID, nil
}

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
	return f.description
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Microsecond
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return 0
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
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

	folder, err := f.metadata(ctx, directoryID, true)
	if err != nil {
		return nil, err
	}

	for _, file := range folder.Content {
		if f.skipFile(&file) {
			continue
		}

		remote := path.Join(dir, f.opt.Enc.ToStandardName(file.Name))
		if file.IsDir() {
			f.dirCache.Put(remote, file.ID)

			d := fs.NewDir(remote, time.Time(file.Modified)).SetID(file.ID).SetItems(file.Size)
			// FIXME more info from dir?
			entries = append(entries, d)
		} else {
			o := &Object{
				fs:     f,
				remote: remote,
			}

			err = o.setMetaData(&file)
			if err != nil {
				fs.Debugf(file, "failed to set object metadata: %s", err)
			}

			entries = append(entries, o)
		}
	}

	return entries, nil
}

func (f *Fs) skipFile(file *api.File) bool {
	return f.opt.SkipProjectFolders && file.IsProjectFolder()
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, leaf, directoryID, nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	mtime := src.ModTime(ctx)

	o := &Object{
		fs:      f,
		remote:  remote,
		size:    size,
		modTime: mtime,
	}

	return o, o.Update(ctx, in, src, options...)
}

func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
}

func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.File) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.File) (err error) {
	if info.IsDir() {
		fs.Debugf(o, "%q is %q", o.remote, info.Type)
		return fs.ErrorIsDir
	}

	if !info.IsFile() {
		fs.Debugf(o, "%q is %q", o.remote, info.Type)
		return fmt.Errorf("%q is %q: %w", o.remote, info.Type, fs.ErrorNotAFile)
	}

	o.size = info.Size
	o.modTime = time.Time(info.ModifiedMS)
	o.id = info.ID
	o.hasMetaData = true
	o.obType = info.Type

	return nil
}

func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}

	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}

	file, found, err := o.fs.fileID(ctx, directoryID, leaf)
	if err != nil {
		return fmt.Errorf("read metadata: fileID: %w", err)
	}

	if !found {
		fs.Debugf(nil, "object not found: remote %s: directory %s: leaf %s", o.remote, directoryID, leaf)
		return fs.ErrorObjectNotFound
	}

	result, err := o.fs.metadata(ctx, file.FileID, false)
	if err != nil {
		return fmt.Errorf("get file metadata: %w", err)
	}

	return o.setMetaData(result)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

func (f *Fs) metadata(ctx context.Context, id string, withContent bool) (result *api.File, err error) {
	parameters := url.Values{}
	if !withContent {
		parameters.Add("content", "0")
	}

	opts := rest.Opts{
		Method:     "GET",
		Path:       path.Join("file/metadata", id),
		Parameters: parameters,
	}

	result = &api.File{}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fs.ErrorObjectNotFound
		}

		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	return result, nil
}

func (f *Fs) setMTime(ctx context.Context, id string, t time.Time) (result *api.File, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "file/metadata",
	}

	params := &api.SetMTimeParams{
		ID:    id,
		MTime: api.JSONTime(t),
	}

	result = &api.File{}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, params, result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fs.ErrorObjectNotFound
		}

		return nil, fmt.Errorf("failed to set file metadata: %w", err)
	}

	return result, nil
}

func (f *Fs) deleteObject(ctx context.Context, id string) error {
	payload := &api.DeleteParams{
		IDs:               []string{id},
		DeletePermanently: f.opt.HardDelete,
	}

	result := &api.IDList{}

	opts := rest.Opts{
		Method: "POST",
		Path:   "file/delete",
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, payload, result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

	for _, removedID := range result.IDs {
		if removedID == id {
			return nil
		}
	}

	return fmt.Errorf("file %s was not deleted successfully", id)
}

func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}

	rootID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	if check {
		file, err := f.metadata(ctx, rootID, false)
		if err != nil {
			return err
		}

		if file.IsFile() {
			return fs.ErrorIsFile
		}

		if file.Size != 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	err = f.deleteObject(ctx, rootID)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)

	return nil
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	if srcObj.fs == f {
		srcPath := srcObj.rootPath()
		dstPath := f.rootPath(remote)
		if srcPath == dstPath {
			return nil, fmt.Errorf("can't copy %q -> %q as they are same", srcPath, dstPath)
		}
	}

	err := srcObj.readMetaData(ctx)
	if err != nil {
		fs.Debugf(srcObj, "read metadata for %s: %s", srcObj.rootPath(), err)
		return nil, err
	}

	_, _, err = srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	dstObj, dstLeaf, directoryID, err := f.createObject(ctx, remote)
	if err != nil {
		fs.Debugf(srcObj, "create empty object for %s: %s", dstObj.rootPath(), err)
		return nil, err
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "file/copyone",
	}

	params := &api.FileCopyMoveOneParams{
		ID:          srcObj.id,
		Target:      directoryID,
		Resolve:     true,
		MTime:       api.JSONTime(srcObj.ModTime(ctx)),
		Name:        dstLeaf,
		ResolveMode: api.OverwriteMode,
	}

	result := &api.File{}

	var resp *http.Response

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, params, result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fs.ErrorObjectNotFound
		}

		return nil, fmt.Errorf("failed to copy: %w", err)
	}

	err = dstObj.setMetaData(result)
	if err != nil {
		return nil, err
	}

	return dstObj, nil
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

	_, _, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObj, dstLeaf, directoryID, err := f.createObject(ctx, remote)
	if err != nil {
		return nil, err
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "file/moveone",
	}

	params := &api.FileCopyMoveOneParams{
		ID:          srcObj.id,
		Target:      directoryID,
		Resolve:     true,
		MTime:       api.JSONTime(srcObj.ModTime(ctx)),
		Name:        dstLeaf,
		ResolveMode: api.OverwriteMode,
	}

	var resp *http.Response
	result := &api.File{}

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, params, result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fs.ErrorObjectNotFound
		}

		return nil, fmt.Errorf("failed to move: %w", err)
	}

	err = dstObj.setMetaData(result)
	if err != nil {
		return nil, err
	}

	return dstObj, nil
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

	srcID, _, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	srcInfo, err := f.metadata(ctx, srcID, false)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method: "POST",
		Path:   "file/moveone",
	}

	params := &api.FileCopyMoveOneParams{
		ID:      srcID,
		Target:  dstDirectoryID,
		Resolve: false,
		MTime:   srcInfo.ModifiedMS,
		Name:    dstLeaf,
	}

	var resp *http.Response
	result := &api.File{}

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, params, result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return fs.ErrorObjectNotFound
		}

		return fmt.Errorf("failed to move dir: %w", err)
	}

	srcFs.dirCache.FlushDir(srcRemote)

	return nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "profile/info",
	}
	var (
		user api.ProfileInfo
		resp *http.Response
	)

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &user)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read profile info: %w", err)
	}

	free := user.AccLimit - user.UserUsed

	if user.UserLimit > unlimitedUserQuota {
		free = user.UserLimit - user.UserUsed
	}

	usage = &fs.Usage{
		Used:  fs.NewUsageValue(user.UserUsed), // bytes in use
		Total: fs.NewUsageValue(user.AccLimit), // bytes total
		Free:  fs.NewUsageValue(free),          // bytes free
	}

	return usage, nil
}

// Fs return the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns object remote path
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

// rootPath returns a path for use in server given a remote
func (f *Fs) rootPath(remote string) string {
	return f.rootSlash() + remote
}

// rootPath returns a path for use in local functions
func (o *Object) rootPath() string {
	return o.fs.rootPath(o.remote)
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData(context.TODO())
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}

	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}

	return o.modTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Hash returns the SHA-1 of an object. Not supported yet.
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	err := o.fs.deleteObject(ctx, o.id)
	if err != nil {
		return err
	}

	if o.obType != "F" {
		o.fs.dirCache.FlushDir(o.remote)
	}

	return nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}

	linkID, err := o.fs.downloadLink(ctx, o.id)
	if err != nil {
		return nil, err
	}

	fs.FixRangeOption(options, o.size)

	opts := rest.Opts{
		Method:  "GET",
		Path:    "/file/download/" + linkID,
		Options: options,
	}

	var resp *http.Response

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

func (f *Fs) downloadLink(ctx context.Context, id string) (linkID string, err error) {
	linkParams := &api.IDList{
		IDs: []string{id},
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "file/download-link",
	}

	var resp *http.Response
	link := &api.DownloadLinkResponse{}

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, linkParams, &link)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	return link.ID, nil
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	file, err := o.fs.setMTime(ctx, o.id, t)
	if err != nil {
		return fmt.Errorf("set mtime: %w", err)
	}

	return o.setMetaData(file)
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	modTime := src.ModTime(ctx)
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	uploadSession, err := o.uploadSession(ctx, directoryID, leaf)
	if err != nil {
		return fmt.Errorf("object update: %w", err)
	}

	o.id = uploadSession.FileID

	defer func() {
		if err == nil {
			return
		}

		deleteErr := o.fs.deleteObject(ctx, o.id)
		if deleteErr != nil {
			fs.Logf(o.remote, "remove: %s", deleteErr)
		}
	}()

	return o.dynamicUpload(ctx, size, modTime, in, uploadSession, options...)
}

// dynamicUpload uploads object in chunks, which are being dynamically recalculated on each iteration
// depending on upload speed in order to make upload faster
func (o *Object) dynamicUpload(ctx context.Context, size int64, modTime time.Time, in io.Reader,
	uploadSession *api.UploadLinkResponse, options ...fs.OpenOption) error {
	var (
		speed      float64
		localChunk int64
	)

	defer o.fs.uploadMemoryManager.Return(o.id)

	for offset := int64(0); offset < size; offset += localChunk {
		localChunk = o.fs.uploadMemoryManager.Consume(o.id, size-offset, speed)

		rw := multipart.NewRW()

		_, err := io.CopyN(rw, in, localChunk)
		if err != nil {
			return fmt.Errorf("read chunk with offset %d size %d: %w", offset, localChunk, err)
		}

		start := time.Now()

		err = o.upload(ctx, uploadSession.UploadKey, rw, size, offset, localChunk, options...)
		if err != nil {
			return fmt.Errorf("upload chunk with offset %d size %d: %w", offset, localChunk, err)
		}

		speed = float64(localChunk) / (float64(time.Since(start)) / 1e9)
	}

	o.fs.uploadMemoryManager.Return(o.id)

	finalizeResult, err := o.finalize(ctx, uploadSession.UploadKey, modTime)
	if err != nil {
		return fmt.Errorf("upload %s finalize: %w", uploadSession.UploadKey, err)
	}

	if size >= 0 && finalizeResult.FileSize != size {
		return fmt.Errorf("expected size %d, got %d", size, finalizeResult.FileSize)
	}

	o.size = size
	o.modTime = modTime

	return nil
}

func (f *Fs) uploadLink(ctx context.Context, parentID, name string) (upload *api.UploadLinkResponse, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "upload/link",
	}

	payload := api.UploadLinkParams{
		Name:     name,
		ParentID: parentID,
		Resolve:  false,
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &payload, &upload)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get upload link: %w", err)
	}

	return upload, nil
}

func (f *Fs) modifyLink(ctx context.Context, fileID string) (upload *api.UploadLinkResponse, err error) {
	opts := rest.Opts{
		Method: "POST",
		Path:   "file/modify",
	}

	payload := api.FileModifyParams{
		ID:       fileID,
		Truncate: 0,
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, &payload, &upload)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get modify link: %w", err)
	}

	return upload, nil
}

func (o *Object) uploadSession(ctx context.Context, parentID, name string) (upload *api.UploadLinkResponse, err error) {
	encName := o.fs.opt.Enc.FromStandardName(name)
	fileID, found, err := o.fs.fileID(ctx, parentID, encName)
	if err != nil {
		return nil, fmt.Errorf("get file_id: %w", err)
	}

	if found {
		return o.fs.modifyLink(ctx, fileID.FileID)
	}

	return o.fs.uploadLink(ctx, parentID, encName)
}

func (o *Object) upload(ctx context.Context, uploadKey string, chunk io.Reader, fullSize int64, offset int64, chunkSize int64, options ...fs.OpenOption) (err error) {
	opts := rest.Opts{
		Method:        "POST",
		RootURL:       fmt.Sprintf(uploadURL, o.fs.opt.Host) + uploadKey,
		Body:          chunk,
		ContentLength: &chunkSize,
		ContentRange:  fmt.Sprintf("bytes %d-%d/%d", offset, offset+chunkSize-1, fullSize),
		Options:       options,
	}

	var fileID string

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &fileID)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to get upload chunk: %w", err)
	}

	return nil
}

func (o *Object) finalize(ctx context.Context, uploadKey string, mtime time.Time) (result *api.UploadFinalizeResponse, err error) {
	queryParams := url.Values{}
	queryParams.Add("mtime", strconv.FormatFloat(float64(mtime.UTC().UnixNano())/1e9, 'f', 6, 64))

	opts := rest.Opts{
		Method:     "GET",
		Path:       path.Join("upload/finalize", uploadKey),
		Parameters: queryParams,
	}

	result = &api.UploadFinalizeResponse{}

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to finalize: %w", err)
	}

	return result, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
