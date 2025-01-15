// Package opendrive provides an interface to the OpenDrive storage system.
package opendrive

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

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
)

const (
	defaultEndpoint = "https://dev.opendrive.com/api/v1"
	minSleep        = 10 * time.Millisecond
	maxSleep        = 5 * time.Minute
	decayConstant   = 1 // bigger for slower decay, exponential
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "opendrive",
		Description: "OpenDrive",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "username",
			Help:      "Username.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:       "password",
			Help:       "Password.",
			IsPassword: true,
			Required:   true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// List of replaced characters:
			//   < (less than)     -> '＜' // FULLWIDTH LESS-THAN SIGN
			//   > (greater than)  -> '＞' // FULLWIDTH GREATER-THAN SIGN
			//   : (colon)         -> '：' // FULLWIDTH COLON
			//   " (double quote)  -> '＂' // FULLWIDTH QUOTATION MARK
			//   \ (backslash)     -> '＼' // FULLWIDTH REVERSE SOLIDUS
			//   | (vertical line) -> '｜' // FULLWIDTH VERTICAL LINE
			//   ? (question mark) -> '？' // FULLWIDTH QUESTION MARK
			//   * (asterisk)      -> '＊' // FULLWIDTH ASTERISK
			//
			// Additionally names can't begin or end with an ASCII whitespace.
			// List of replaced characters:
			//     (space)           -> '␠'  // SYMBOL FOR SPACE
			//     (horizontal tab)  -> '␉'  // SYMBOL FOR HORIZONTAL TABULATION
			//     (line feed)       -> '␊'  // SYMBOL FOR LINE FEED
			//     (vertical tab)    -> '␋'  // SYMBOL FOR VERTICAL TABULATION
			//     (carriage return) -> '␍'  // SYMBOL FOR CARRIAGE RETURN
			//
			// Also encode invalid UTF-8 bytes as json doesn't handle them properly.
			//
			// https://www.opendrive.com/wp-content/uploads/guides/OpenDrive_API_guide.pdf
			Default: (encoder.Base |
				encoder.EncodeWin |
				encoder.EncodeLeftCrLfHtVt |
				encoder.EncodeRightCrLfHtVt |
				encoder.EncodeBackSlash |
				encoder.EncodeLeftSpace |
				encoder.EncodeRightSpace |
				encoder.EncodeInvalidUtf8),
		}, {
			Name: "chunk_size",
			Help: `Files will be uploaded in chunks this size.

Note that these chunks are buffered in memory so increasing them will
increase memory use.`,
			Default:  10 * fs.Mebi,
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	UserName  string               `config:"username"`
	Password  string               `config:"password"`
	Enc       encoder.MultiEncoder `config:"encoding"`
	ChunkSize fs.SizeSuffix        `config:"chunk_size"`
}

// Fs represents a remote server
type Fs struct {
	name     string             // name of this remote
	root     string             // the path we are working on
	opt      Options            // parsed options
	features *fs.Features       // optional features
	srv      *rest.Client       // the connection to the server
	pacer    *fs.Pacer          // To pace and retry the API calls
	session  UserSessionInfo    // contains the session data
	dirCache *dircache.DirCache // Map of directory path to directory id
}

// Object describes an object
type Object struct {
	fs      *Fs       // what this object is part of
	remote  string    // The remote path
	id      string    // ID of the file
	parent  string    // ID of the parent directory
	modTime time.Time // The modified time of the object if known
	md5     string    // MD5 hash if known
	size    int64     // Size of the object
}

// parsePath parses an incoming 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
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
	return fmt.Sprintf("OpenDrive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	root = parsePath(root)
	if opt.UserName == "" {
		return nil, errors.New("username not found")
	}
	opt.Password, err = obscure.Reveal(opt.Password)
	if err != nil {
		return nil, errors.New("password could not revealed")
	}
	if opt.Password == "" {
		return nil, errors.New("password not found")
	}

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   rest.NewClient(fshttp.NewClient(ctx)).SetErrorHandler(errorHandler),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.dirCache = dircache.New(root, "0", f)

	// set the rootURL for the REST client
	f.srv.SetRoot(defaultEndpoint)

	// get sessionID
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		account := Account{Username: opt.UserName, Password: opt.Password}

		opts := rest.Opts{
			Method: "POST",
			Path:   "/session/login.json",
		}
		resp, err = f.srv.CallJSON(ctx, &opts, &account, &f.session)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	fs.Debugf(nil, "Starting OpenDrive session with ID: %s", f.session.SessionID)

	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, "0", &tempF)
		tempF.root = newRoot

		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObjectWithInfo(ctx, remote, nil, "")
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// rootSlash returns root with a slash on if it is empty, otherwise empty string
func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	errResponse := new(Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.Info.Code == 0 {
		errResponse.Info.Code = resp.StatusCode
	}
	if errResponse.Info.Message == "" {
		errResponse.Info.Message = "Unknown " + resp.Status
	}
	return errResponse
}

// Mkdir creates the folder if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// fs.Debugf(nil, "Mkdir(\"%s\")", dir)
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) error {
	return f.pacer.Call(func() (bool, error) {
		removeDirData := removeFolder{SessionID: f.session.SessionID, FolderID: id}
		opts := rest.Opts{
			Method:     "POST",
			NoResponse: true,
			Path:       "/folder/remove.json",
		}
		resp, err := f.srv.CallJSON(ctx, &opts, &removeDirData, nil)
		return f.shouldRetry(ctx, resp, err)
	})
}

// purgeCheck remotes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dc := f.dirCache
	rootID, err := dc.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	item, err := f.readMetaDataForFolderID(ctx, rootID)
	if err != nil {
		return err
	}
	if check && len(item.Files) != 0 {
		return errors.New("folder not empty")
	}
	err = f.deleteObject(ctx, rootID)
	if err != nil {
		return err
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// fs.Debugf(nil, "Rmdir(\"%s\")", path.Join(f.root, dir))
	return f.purgeCheck(ctx, dir, true)
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Second
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
	// fs.Debugf(nil, "Copy(%v)", remote)
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	srcPath := srcObj.fs.rootSlash() + srcObj.remote
	dstPath := f.rootSlash() + remote
	if strings.EqualFold(srcPath, dstPath) {
		return nil, fmt.Errorf("can't copy %q -> %q as are same name when lowercase", srcPath, dstPath)
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}
	// fs.Debugf(nil, "...%#v\n...%#v", remote, directoryID)

	// Copy the object
	var resp *http.Response
	response := moveCopyFileResponse{}
	err = f.pacer.Call(func() (bool, error) {
		copyFileData := moveCopyFile{
			SessionID:         f.session.SessionID,
			SrcFileID:         srcObj.id,
			DstFolderID:       directoryID,
			Move:              "false",
			OverwriteIfExists: "true",
			NewFileName:       leaf,
		}
		opts := rest.Opts{
			Method: "POST",
			Path:   "/file/move_copy.json",
		}
		resp, err = f.srv.CallJSON(ctx, &opts, &copyFileData, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	size, _ := strconv.ParseInt(response.Size, 10, 64)
	dstObj.id = response.FileID
	dstObj.size = size

	return dstObj, nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	var uInfo usersInfoResponse
	var resp *http.Response

	err = f.pacer.Call(func() (bool, error) {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/users/info.json/" + f.session.SessionID,
		}
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &uInfo)
		return f.shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return nil, err
	}

	usage = &fs.Usage{
		Used:  fs.NewUsageValue(uInfo.StorageUsed),
		Total: fs.NewUsageValue(uInfo.MaxStorage * 1024 * 1024), // MaxStorage appears to be in MB
		Free:  fs.NewUsageValue(uInfo.MaxStorage*1024*1024 - uInfo.StorageUsed),
	}
	return usage, nil
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
	// fs.Debugf(nil, "Move(%v)", remote)
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// move_copy will silently truncate new filenames
	if len(leaf) > 255 {
		fs.Debugf(src, "Can't move file: name (%q) exceeds 255 char", leaf)
		return nil, fs.ErrorFileNameTooLong
	}

	moveCopyFileData := moveCopyFile{
		SessionID:         f.session.SessionID,
		SrcFileID:         srcObj.id,
		DstFolderID:       directoryID,
		Move:              "true",
		OverwriteIfExists: "true",
		NewFileName:       leaf,
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/file/move_copy.json",
	}
	var request interface{} = moveCopyFileData

	// use /file/rename.json if moving within the same directory
	_, srcDirID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}
	if srcDirID == directoryID {
		fs.Debugf(src, "same parent dir (%v) - using file/rename instead of move_copy for %s", directoryID, remote)
		renameFileData := renameFile{
			SessionID:   f.session.SessionID,
			FileID:      srcObj.id,
			NewFileName: leaf,
		}
		opts.Path = "/file/rename.json"
		request = renameFileData
	}

	// Move the object
	var resp *http.Response
	response := moveCopyFileResponse{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}

	size, _ := strconv.ParseInt(response.Size, 10, 64)
	dstObj.id = response.FileID
	dstObj.size = size

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
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) (err error) {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, srcDirectoryID, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}

	// move_copy will silently truncate new filenames
	if len(dstLeaf) > 255 {
		fs.Debugf(src, "Can't move folder: name (%q) exceeds 255 char", dstLeaf)
		return fs.ErrorFileNameTooLong
	}

	moveFolderData := moveCopyFolder{
		SessionID:     f.session.SessionID,
		FolderID:      srcID,
		DstFolderID:   dstDirectoryID,
		Move:          "true",
		NewFolderName: dstLeaf,
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/folder/move_copy.json",
	}
	var request interface{} = moveFolderData

	// use /folder/rename.json if moving within the same parent directory
	if srcDirectoryID == dstDirectoryID {
		fs.Debugf(dstRemote, "same parent dir (%v) - using folder/rename instead of move_copy", srcDirectoryID)
		renameFolderData := renameFolder{
			SessionID:  f.session.SessionID,
			FolderID:   srcID,
			FolderName: dstLeaf,
		}
		opts.Path = "/folder/rename.json"
		request = renameFolderData
	}

	// Do the move
	var resp *http.Response
	response := moveCopyFolderResponse{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &request, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		fs.Debugf(src, "DirMove error %v", err)
		return err
	}

	srcFs.dirCache.FlushDir(srcRemote)
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

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, file *File, parent string) (fs.Object, error) {
	// fs.Debugf(nil, "newObjectWithInfo(%s, %v)", remote, file)

	var o *Object
	if nil != file {
		o = &Object{
			fs:      f,
			remote:  remote,
			id:      file.FileID,
			parent:  parent,
			modTime: time.Unix(file.DateModified, 0),
			size:    file.Size,
			md5:     file.FileHash,
		}
	} else {
		o = &Object{
			fs:     f,
			remote: remote,
		}

		err := o.readMetaData(ctx)
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// fs.Debugf(nil, "NewObject(\"%s\")", remote)
	return f.newObjectWithInfo(ctx, remote, nil, "")
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, leaf, directoryID, err
	}
	// fs.Debugf(nil, "\n...leaf %#v\n...id %#v", leaf, directoryID)
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, f.opt.Enc.FromStandardName(leaf), directoryID, nil
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForFolderID(ctx context.Context, id string) (info *FolderList, err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method: "GET",
		Path:   "/folder/list.json/" + f.session.SessionID + "/" + id,
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return info, err
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	// fs.Debugf(nil, "Put(%s)", remote)

	o, leaf, directoryID, err := f.createObject(ctx, remote, modTime, size)
	if err != nil {
		return nil, err
	}

	if o.id == "" {
		// Attempt to read ID, ignore error
		// FIXME is this correct?
		_ = o.readMetaData(ctx)
	}

	if o.id == "" {
		// We need to create an ID for this file
		var resp *http.Response
		response := createFileResponse{}
		err := o.fs.pacer.Call(func() (bool, error) {
			createFileData := createFile{
				SessionID: o.fs.session.SessionID,
				FolderID:  directoryID,
				Name:      leaf,
			}
			opts := rest.Opts{
				Method:  "POST",
				Options: options,
				Path:    "/upload/create_file.json",
			}
			resp, err = o.fs.srv.CallJSON(ctx, &opts, &createFileData, &response)
			return o.fs.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %w", err)
		}

		o.id = response.FileID
	}

	return o, o.Update(ctx, in, src, options...)
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	401, // Unauthorized (seen in "Token has expired")
	408, // Request Timeout
	423, // Locked - get this on folders sometimes
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	502, // Bad Gateway when doing big listings
	503, // Service Unavailable
	504, // Gateway Time-out
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// DirCacher methods

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, replaceReservedChars(leaf))
	var resp *http.Response
	response := createFolderResponse{}
	err = f.pacer.Call(func() (bool, error) {
		createDirData := createFolder{
			SessionID:           f.session.SessionID,
			FolderName:          f.opt.Enc.FromStandardName(leaf),
			FolderSubParent:     pathID,
			FolderIsPublic:      0,
			FolderPublicUpl:     0,
			FolderPublicDisplay: 0,
			FolderPublicDnl:     0,
		}
		opts := rest.Opts{
			Method: "POST",
			Path:   "/folder.json",
		}
		resp, err = f.srv.CallJSON(ctx, &opts, &createDirData, &response)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}

	return response.FolderID, nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// fs.Debugf(nil, "FindLeaf(\"%s\", \"%s\")", pathID, leaf)

	if pathID == "0" && leaf == "" {
		// fs.Debugf(nil, "Found OpenDrive root")
		// that's the root directory
		return pathID, true, nil
	}

	// get the folderIDs
	var resp *http.Response
	folderList := FolderList{}
	err = f.pacer.Call(func() (bool, error) {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/folder/list.json/" + f.session.SessionID + "/" + pathID,
		}
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &folderList)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", false, fmt.Errorf("failed to get folder list: %w", err)
	}

	leaf = f.opt.Enc.FromStandardName(leaf)
	for _, folder := range folderList.Folders {
		// fs.Debugf(nil, "Folder: %s (%s)", folder.Name, folder.FolderID)

		if strings.EqualFold(leaf, folder.Name) {
			// found
			return folder.FolderID, true, nil
		}
	}

	return "", false, nil
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
	// fs.Debugf(nil, "List(%v)", dir)
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	opts := rest.Opts{
		Method: "GET",
		Path:   "/folder/list.json/" + f.session.SessionID + "/" + directoryID,
	}
	folderList := FolderList{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &folderList)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if apiError, ok := err.(*Error); ok {
			// Work around a bug maybe in opendrive or maybe in rclone.
			//
			// We should know whether the folder exists or not by the call to
			// FindDir above so exactly why it is not found here is a mystery.
			//
			// This manifests as a failure in fs/sync TestSyncOverlapWithFilter
			if apiError.Info.Message == "Folder is already deleted" {
				return fs.DirEntries{}, nil
			}
		}
		return nil, fmt.Errorf("failed to get folder list: %w", err)
	}

	for _, folder := range folderList.Folders {
		folder.Name = f.opt.Enc.ToStandardName(folder.Name)
		// fs.Debugf(nil, "Folder: %s (%s)", folder.Name, folder.FolderID)
		remote := path.Join(dir, folder.Name)
		// cache the directory ID for later lookups
		f.dirCache.Put(remote, folder.FolderID)
		d := fs.NewDir(remote, time.Unix(folder.DateModified, 0)).SetID(folder.FolderID)
		d.SetItems(int64(folder.ChildFolders))
		d.SetParentID(directoryID)
		entries = append(entries, d)
	}

	for _, file := range folderList.Files {
		file.Name = f.opt.Enc.ToStandardName(file.Name)
		// fs.Debugf(nil, "File: %s (%s)", file.Name, file.FileID)
		remote := path.Join(dir, file.Name)
		o, err := f.newObjectWithInfo(ctx, remote, &file, directoryID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, o)
	}

	return entries, nil
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
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size // Object is likely PENDING
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
	// fs.Debugf(nil, "SetModTime(%v)", modTime.String())
	opts := rest.Opts{
		Method:     "PUT",
		NoResponse: true,
		Path:       "/file/filesettings.json",
	}
	update := modTimeFile{
		SessionID:            o.fs.session.SessionID,
		FileID:               o.id,
		FileModificationTime: strconv.FormatInt(modTime.Unix(), 10),
	}
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &update, nil)
		return o.fs.shouldRetry(ctx, resp, err)
	})

	o.modTime = modTime

	return err
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	// fs.Debugf(nil, "Open(\"%v\")", o.remote)
	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{
		Method:  "GET",
		Path:    "/download/file.json/" + o.id + "?session_id=" + o.fs.session.SessionID,
		Options: options,
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open file): %w", err)
	}

	return resp.Body, nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	// fs.Debugf(nil, "Remove(\"%s\")", o.id)
	return o.fs.pacer.Call(func() (bool, error) {
		opts := rest.Opts{
			Method:     "DELETE",
			NoResponse: true,
			Path:       "/file.json/" + o.fs.session.SessionID + "/" + o.id,
		}
		resp, err := o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	modTime := src.ModTime(ctx)
	// fs.Debugf(nil, "Update(\"%s\", \"%s\")", o.id, o.remote)

	// Open file for upload
	var resp *http.Response
	openResponse := openUploadResponse{}
	err := o.fs.pacer.Call(func() (bool, error) {
		openUploadData := openUpload{SessionID: o.fs.session.SessionID, FileID: o.id, Size: size}
		// fs.Debugf(nil, "PreOpen: %#v", openUploadData)
		opts := rest.Opts{
			Method:  "POST",
			Options: options,
			Path:    "/upload/open_file_upload.json",
		}
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &openUploadData, &openResponse)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	// resp.Body.Close()
	// fs.Debugf(nil, "PostOpen: %#v", openResponse)

	buf := make([]byte, o.fs.opt.ChunkSize)
	chunkOffset := int64(0)
	remainingBytes := size
	chunkCounter := 0

	for remainingBytes > 0 {
		currentChunkSize := int64(o.fs.opt.ChunkSize)
		if currentChunkSize > remainingBytes {
			currentChunkSize = remainingBytes
		}
		remainingBytes -= currentChunkSize
		fs.Debugf(o, "Uploading chunk %d, size=%d, remain=%d", chunkCounter, currentChunkSize, remainingBytes)

		chunk := readers.NewRepeatableLimitReaderBuffer(in, buf, currentChunkSize)
		var reply uploadFileChunkReply
		err = o.fs.pacer.Call(func() (bool, error) {
			// seek to the start in case this is a retry
			if _, err = chunk.Seek(0, io.SeekStart); err != nil {
				return false, err
			}
			opts := rest.Opts{
				Method: "POST",
				Path:   "/upload/upload_file_chunk.json",
				Body:   chunk,
				MultipartParams: url.Values{
					"session_id":    []string{o.fs.session.SessionID},
					"file_id":       []string{o.id},
					"temp_location": []string{openResponse.TempLocation},
					"chunk_offset":  []string{strconv.FormatInt(chunkOffset, 10)},
					"chunk_size":    []string{strconv.FormatInt(currentChunkSize, 10)},
				},
				MultipartContentName: "file_data", // ..name of the parameter which is the attached file
				MultipartFileName:    o.remote,    // ..name of the file for the attached file

			}
			resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &reply)
			return o.fs.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		if reply.TotalWritten != currentChunkSize {
			return fmt.Errorf("failed to create file: incomplete write of %d/%d bytes", reply.TotalWritten, currentChunkSize)
		}

		chunkCounter++
		chunkOffset += currentChunkSize
	}

	// Close file for upload
	closeResponse := closeUploadResponse{}
	err = o.fs.pacer.Call(func() (bool, error) {
		closeUploadData := closeUpload{SessionID: o.fs.session.SessionID, FileID: o.id, Size: size, TempLocation: openResponse.TempLocation}
		// fs.Debugf(nil, "PreClose: %#v", closeUploadData)
		opts := rest.Opts{
			Method: "POST",
			Path:   "/upload/close_file_upload.json",
		}
		resp, err = o.fs.srv.CallJSON(ctx, &opts, &closeUploadData, &closeResponse)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	// fs.Debugf(nil, "PostClose: %#v", closeResponse)

	o.id = closeResponse.FileID
	o.size = closeResponse.Size

	// Set the mod time now
	err = o.SetModTime(ctx, modTime)
	if err != nil {
		return err
	}

	// Set permissions
	err = o.fs.pacer.Call(func() (bool, error) {
		update := permissions{SessionID: o.fs.session.SessionID, FileID: o.id, FileIsPublic: 0}
		// fs.Debugf(nil, "Permissions : %#v", update)
		opts := rest.Opts{
			Method:     "POST",
			NoResponse: true,
			Path:       "/file/access.json",
		}
		resp, err = o.fs.srv.CallJSON(ctx, &opts, &update, nil)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

	return o.readMetaData(ctx)
}

func (o *Object) readMetaData(ctx context.Context) (err error) {
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, o.remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	var resp *http.Response
	fileInfo := File{}

	// If we know the object id perform a direct lookup
	// because the /folder/itembyname.json endpoint is unreliable:
	// newly created objects take an arbitrary amount of time to show up
	if o.id != "" {
		opts := rest.Opts{
			Method: "GET",
			Path: fmt.Sprintf("/file/info.json/%s?session_id=%s",
				o.id, o.fs.session.SessionID),
		}
		err = o.fs.pacer.Call(func() (bool, error) {
			resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &fileInfo)
			return o.fs.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return fmt.Errorf("failed to get fileinfo: %w", err)
		}

		o.id = fileInfo.FileID
		o.modTime = time.Unix(fileInfo.DateModified, 0)
		o.md5 = fileInfo.FileHash
		o.size = fileInfo.Size
		return nil
	}
	folderList := FolderList{}
	opts := rest.Opts{
		Method: "GET",
		Path: fmt.Sprintf("/folder/itembyname.json/%s/%s?name=%s",
			o.fs.session.SessionID, directoryID, url.QueryEscape(o.fs.opt.Enc.FromStandardName(leaf))),
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &folderList)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("failed to get folder list: %w", err)
	}
	if len(folderList.Files) == 0 {
		return fs.ErrorObjectNotFound
	}
	fileInfo = folderList.Files[0]
	o.id = fileInfo.FileID
	o.modTime = time.Unix(fileInfo.DateModified, 0)
	o.md5 = fileInfo.FileHash
	o.size = fileInfo.Size
	return nil
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// ParentID returns the ID of the Object parent directory if known, or "" if not
func (o *Object) ParentID() string {
	return o.parent
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
	_ fs.ParentIDer      = (*Object)(nil)
)
